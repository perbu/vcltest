# Varnish VCL Trace Specification

This document specifies how to use Varnish's built-in VCL tracing feature for testing VCL logic.

## Enabling VCL Trace

Start varnishd with the `feature=+trace` parameter:

```bash
varnishd -n <workdir> -a :6081 -f <vcl_file> -p feature=+trace
```

This enables the `req.trace` and `bereq.trace` features by default, which causes Varnish to log VCL execution details.

## Capturing Trace Logs

Use varnishlog to capture trace events:

```bash
# Write to binary file
varnishlog -n <workdir> -g request -w output.bin

# Read binary file back as text
varnishlog -r output.bin

# Filter for specific log types
varnishlog -n <workdir> -g request -i VCL_trace,VCL_call,VCL_return,BackendOpen
```

## VCL_trace Log Format

Each VCL line that executes generates a `VCL_trace` log entry:

```
VCL_trace <vcl_name> <trace_id> <source_location>
```

### Fields

1. **vcl_name**: Name of the VCL configuration (e.g., "boot" for the default)
2. **trace_id**: Sequential trace point ID (internal, can be ignored)
3. **source_location**: Format is `<config>.<line>.<column>`
   - **config**: VCL configuration number
     - `0` = Main VCL file
     - `1, 2, ...` = Included VCL files (in order of inclusion)
     - High numbers = Built-in VCL
   - **line**: Source line number in the VCL file
   - **column**: Column position where the traced statement starts

### Example

```
VCL_trace      boot 1 0.10.5
```

This means:
- VCL configuration: "boot"
- Trace point: 1
- Source location: config=0 (user VCL), line=10, column=5

## Parsing VCL Execution

### Extracting Line Numbers

To get which lines of user VCL were executed:

1. Filter for `VCL_trace` log entries
2. Parse the third field (source_location)
3. Split on `.` to get `[config, line, column]`
4. **Filter for config=0** (user's VCL only)
5. Collect the `line` numbers

Example in Go:

```go
import "strings"

func parseVCLTrace(logLine string) (int, bool) {
    // Example: "-   VCL_trace      boot 1 0.10.5"
    fields := strings.Fields(logLine)
    if len(fields) < 5 || fields[1] != "VCL_trace" {
        return 0, false
    }

    sourceLocation := fields[4] // "0.10.5"
    parts := strings.Split(sourceLocation, ".")
    if len(parts) != 3 {
        return 0, false
    }

    config := parts[0]
    if config != "0" {
        return 0, false // Skip built-in VCL
    }

    line, err := strconv.Atoi(parts[1])
    if err != nil {
        return 0, false
    }

    return line, true
}
```

### Handling Include Files

When VCL files use `include` statements, get the config→filename mapping from Varnish:

```bash
varnishadm -n <workdir> vcl.show -v <vclname>
```

This outputs headers showing which config number corresponds to which file:

```
// VCL.SHOW 0 356 /path/to/main.vcl
// VCL.SHOW 1 173 /path/to/included.vcl
// VCL.SHOW 2 7158 <builtin>
```

Format: `// VCL.SHOW <config> <size_bytes> <filename>`

Parse these headers to build the mapping:

```go
func GetVCLConfigMap(workdir, vclName string) (map[int]string, error) {
    cmd := exec.Command("varnishadm", "-n", workdir, "vcl.show", "-v", vclName)
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    configMap := make(map[int]string)
    vclShowRegex := regexp.MustCompile(`^//\s+VCL\.SHOW\s+(\d+)\s+\d+\s+(.+)$`)

    for _, line := range strings.Split(string(output), "\n") {
        if matches := vclShowRegex.FindStringSubmatch(line); len(matches) == 3 {
            config, _ := strconv.Atoi(matches[1])
            filename := strings.TrimSpace(matches[2])
            if filename != "<builtin>" {
                configMap[config] = filename
            }
        }
    }
    return configMap, nil
}
```

Then when parsing traces, look up the filename:

```go
type TraceLine struct {
    Filename string
    Line     int
}

func parseVCLTraceWithFiles(logLine string, configMap map[int]string) (TraceLine, bool) {
    fields := strings.Fields(logLine)
    if len(fields) < 5 || fields[1] != "VCL_trace" {
        return TraceLine{}, false
    }

    parts := strings.Split(fields[4], ".") // "1.4.5"
    config, _ := strconv.Atoi(parts[0])
    line, _ := strconv.Atoi(parts[1])

    filename, ok := configMap[config]
    if !ok {
        return TraceLine{}, false // Skip built-in VCL
    }

    return TraceLine{Filename: filename, Line: line}, true
}
```

Display with filename:line format:

```
Executed lines:
  main.vcl:12         call custom_check;
  included.vcl:4      if (req.url ~ "^/admin")
  included.vcl:5      return (synth(403));
  main.vcl:16         return (pass);
```

## Counting Backend Calls

Backend connections are logged as `BackendOpen` entries:

```
BackendOpen    22 default 127.0.0.1 8080 127.0.0.1 56783 connect
```

To count backend calls:
1. Filter for `BackendOpen` log entries within a request group
2. Count the number of occurrences

Example:
```bash
varnishlog -n <workdir> -g request -q 'ReqURL eq "/api/users"' -i BackendOpen
```

## Example Traces

### Request to `/admin` (synth response, no backend)

VCL:
```vcl
sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }
}
```

Trace:
```
-   ReqURL         /admin
-   VCL_call       RECV
-   VCL_trace      boot 1 0.10.5    # Line 10: if (req.url ~ "^/admin")
-   VCL_trace      boot 2 0.11.9    # Line 11: return (synth(403, "Forbidden"));
-   VCL_return     synth
```

**Backend calls: 0** (no BackendOpen)

### Request to `/api/users` (pass through)

VCL:
```vcl
sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }
    if (req.url ~ "^/api/") {
        return (pass);
    }
}
```

Trace:
```
-   ReqURL         /api/users
-   VCL_call       RECV
-   VCL_trace      boot 1 0.10.5    # Line 10: if (req.url ~ "^/admin")
-   VCL_trace      boot 3 0.15.5    # Line 15: if (req.url ~ "^/api/")
-   VCL_trace      boot 4 0.16.9    # Line 16: return (pass);
-   VCL_return     pass
...
-   BackendOpen    22 default 127.0.0.1 8080 127.0.0.1 56783 connect
```

**Backend calls: 1** (one BackendOpen)

### Request to `/static/logo.png` (cache with cookie removal)

VCL:
```vcl
sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }
    if (req.url ~ "^/api/") {
        return (pass);
    }
    if (req.url ~ "\.(jpg|png|css|js)$") {
        unset req.http.Cookie;
        return (hash);
    }
}
```

Trace:
```
-   ReqURL         /static/logo.png
-   VCL_call       RECV
-   VCL_trace      boot 1 0.10.5    # Line 10: if (req.url ~ "^/admin")
-   VCL_trace      boot 3 0.15.5    # Line 15: if (req.url ~ "^/api/")
-   VCL_trace      boot 5 0.20.5    # Line 20: if (req.url ~ "\.(jpg|png|css|js)$")
-   VCL_trace      boot 6 0.21.9    # Line 21: unset req.http.Cookie;
-   VCL_return     hash
...
-   BackendOpen    22 default 127.0.0.1 8080 127.0.0.1 56826 connect
```

**Backend calls: 1** (cache miss, fetched from backend)

## Key Insights

1. **Every condition is traced**, even if it doesn't match (e.g., line 10 and 15 in the static asset example)
2. **Line numbers are accurate** to the source VCL file
3. **Built-in VCL traces** have config > 0 and should be filtered out
4. **Backend calls** are counted via `BackendOpen` entries
5. **Request grouping** is done via varnishlog's `-g request` option

## Implementation Notes

### Starting Varnish for Tests

```go
cmd := exec.Command("varnishd",
    "-n", workdir,
    "-a", "127.0.0.1:6081",
    "-f", vclFile,
    "-p", "feature=+trace",
    "-F", // Foreground mode for testing
)
```

### Capturing Logs

```go
// Start varnishlog in background
logCmd := exec.Command("varnishlog",
    "-n", workdir,
    "-g", "request",
    "-w", outputFile,
)
logCmd.Start()

// Make HTTP request to varnish
// ...

// Stop varnishlog
logCmd.Process.Signal(os.Interrupt)
logCmd.Wait()

// Read logs back
readCmd := exec.Command("varnishlog",
    "-r", outputFile,
)
output, _ := readCmd.Output()
```

### Parsing Example

```go
type TraceResult struct {
    ExecutedLines []int
    BackendCalls  int
}

func parseTrace(logOutput string) TraceResult {
    result := TraceResult{
        ExecutedLines: []int{},
    }

    lines := strings.Split(logOutput, "\n")
    for _, line := range lines {
        if strings.Contains(line, "VCL_trace") {
            if lineNum, ok := parseVCLTrace(line); ok {
                result.ExecutedLines = append(result.ExecutedLines, lineNum)
            }
        }
        if strings.Contains(line, "BackendOpen") {
            result.BackendCalls++
        }
    }

    // Remove duplicates and sort
    result.ExecutedLines = uniqueSorted(result.ExecutedLines)

    return result
}
```

## Testing Checklist

When implementing VCL trace parsing:

- [ ] Parse VCL_trace source_location field correctly (config.line.column format)
- [ ] Get config→filename mapping using `varnishadm vcl.show -v`
- [ ] Parse VCL.SHOW headers to build config map
- [ ] Filter out built-in VCL (configs not in map)
- [ ] Extract line numbers and filenames for each trace
- [ ] Count BackendOpen entries for backend calls
- [ ] Group logs by request (use -g request)
- [ ] Handle multiple requests in a single log file
- [ ] Deduplicate trace lines (same line may execute multiple times)
- [ ] Display as filename:line format
