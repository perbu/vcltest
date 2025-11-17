The goal of this project is to help users verify that the VCL they've made is working.

# Overview

VCL (Varnish Configuration Language) is powerful but can be difficult to debug and test. This tool provides
a test framework that allows developers to:

1. Write declarative test specifications in YAML
2. Use Varnish's built-in VCL tracing for observability
3. Execute tests against a real Varnish instance
4. Verify behavior with clear pass/fail results

# How This Differs from VTest2

**VTest2 operates at the HTTP protocol level** - it's excellent for testing Varnish behavior from an HTTP
client/server perspective, but it's **too low-level for testing VCL logic**.

## VTest2 Limitations for VCL Testing

VTest2 focuses on:
- Raw HTTP protocol interactions (bytes on the wire)
- Varnish internals and edge cases at the C level
- Low-level cache behavior and state transitions
- Protocol conformance and HTTP semantics

**What VTest2 cannot easily do:**
1. **Verify VCL execution paths** - You can't easily confirm which branches of your VCL logic were executed
2. **Debug VCL flow** - When a test fails, you don't get a trace showing which lines ran
3. **Test VCL logic in isolation** - VTest2 tests the entire Varnish stack, making it hard to pinpoint VCL-specific issues

## This Tool's VCL-First Approach

This tool is **purpose-built for testing VCL logic**:

1. **VCL-Level Observability**: Uses Varnish's built-in `vcl_trace` parameter to trace execution
2. **Simple Assertions**: Assert on status codes, backend calls, and response content
3. **Clear VCL Debugging**: When tests fail, see exactly which lines of VCL were executed
4. **Declarative YAML Format**: Write simple YAML test specifications

**Example contrast:**

VTest2 test:
```
client c1 {
    txreq -url "/api/users"
    rxresp
    expect resp.status == 200
} -run
```
*What happened inside VCL? We don't know.*

This tool's test:
```yaml
name: API request test
vcl: test.vcl

request:
  url: /api/users

expect:
  status: 200
  backend_calls: 1
```

When this test runs, you'll see:
```
PASS: API request test (45ms)

VCL executed lines: 10, 12, 15, 18, 22
```

*You know exactly which VCL lines were executed.*

## When to Use Each Tool

**Use VTest2 when:**
- Testing Varnish core functionality and edge cases
- Verifying HTTP protocol compliance
- Testing complex client/server interaction scenarios
- Debugging low-level Varnish behavior

**Use this tool when:**
- Developing and testing your VCL configuration
- Debugging why your VCL logic isn't working as expected
- Verifying routing and access control rules
- Ensuring VCL changes don't break expected behavior
- Testing VCL in a CI/CD pipeline

**Both tools are complementary** - VTest2 for Varnish internals, this tool for your VCL logic.

# Common Use Cases

Multiple tests can be defined in a single YAML file using `---` as the document separator:

```yaml
---
name: API requests bypass cache
vcl: production.vcl

request:
  url: /api/users/123

backend:
  status: 200
  body: '{"user":"test"}'

expect:
  status: 200
  backend_calls: 1
  body_contains: '"user"'

---
name: Block admin requests
vcl: production.vcl

request:
  url: /admin

expect:
  status: 403
  backend_calls: 0

---
name: Static assets are cached
vcl: production.vcl

request:
  url: /images/logo.jpg
  headers:
    Cookie: session=abc123

backend:
  status: 200
  headers:
    Content-Type: image/jpeg
  body: fake-image-data

expect:
  status: 200
  headers:
    Content-Type: image/jpeg
  body_contains: fake-image-data
  backend_calls: 1
```

This keeps related tests together in a single file, reducing clutter and making test organization clearer.

# Test Specification Format (YAML)

## Multiple Tests Per File

YAML files can contain multiple test documents separated by `---`:

```yaml
---
name: First test
vcl: test.vcl
request:
  url: /
expect:
  status: 200

---
name: Second test
vcl: test.vcl
request:
  url: /admin
expect:
  status: 403
```

Each `---` separator starts a new test. This is standard YAML multi-document syntax.

## Minimal Test

A single test file:

```yaml
name: Basic test
vcl: test.vcl

request:
  url: /

expect:
  status: 200
```

## Full Test Specification

```yaml
name: Full featured test
vcl: test.vcl

request:
  method: GET          # Optional, defaults to GET
  url: /api/users/123
  headers:             # Optional
    Host: example.com
    Cookie: session=abc
  body: ""             # Optional, for POST/PUT

backend:               # Optional, defaults to 200 OK
  status: 200
  headers:
    Content-Type: application/json
  body: '{"user":"test"}'

expect:
  status: 200                    # Required
  backend_calls: 1               # Optional, verify backend was/wasn't called
  headers:                       # Optional, verify specific headers
    Content-Type: application/json
  body_contains: '"user":"test"' # Optional, verify response body
```

## Field Defaults

- `request.method`: GET
- `backend.status`: 200
- `backend.headers`: empty
- `backend.body`: empty
- All `expect.*` fields are optional

# High-Level Architecture

For each test:

1. Start varnishd instance with `vcl_trace=on` parameter (reused across tests)
2. Start mock backend server
3. Modify VCL to use mock backend address and load into varnishd
4. Start varnishlog to capture VCL trace logs
5. Execute HTTP request
6. Verify expectations
7. Cleanup (stop backend)

# Core Components

## Test Runner
- Orchestrates test lifecycle
- Manages varnishd (with `vcl_trace=on`), varnishlog, mock backends
- Collects results and reports pass/fail
- Handles cleanup on failures

## VCL Backend Replacer
- Simple text replacement to change backend address to mock backend
- Minimal VCL modification (only backend host/port)
- Preserves all VCL logic unchanged

## Mock Backend Server
- Simple HTTP server
- Returns configured status/headers/body
- Tracks number of requests received

## Trace Parser
- Parses varnishlog output with `vcl_trace` enabled
- Extracts which VCL lines were executed
- Counts backend connection events

## Assertion Engine
- Evaluates test expectations
- Provides clear failure messages
- Shows which VCL lines executed

# VCL Tracing with vcl_trace

Varnish has a built-in parameter `vcl_trace` that provides extensive logging about VCL execution. When enabled,
Varnish automatically logs which VCL lines are executed during request processing.

## Why vcl_trace?

Using Varnish's built-in tracing offers major advantages:

1. **No VCL Modification**: VCL runs exactly as written, no parsing or instrumentation needed
2. **Zero Overhead**: Tracing is built into Varnish, optimized and reliable
3. **Complete Coverage**: Varnish traces all VCL execution automatically
4. **Production-Ready**: This is Varnish's own debugging feature, battle-tested

## How vcl_trace Works

When you start varnishd with the `vcl_trace` parameter enabled, Varnish logs VCL execution to varnishlog:

```bash
varnishd -n /tmp/varnish_test -a :8080 -p vcl_trace=on ...
```

During request processing, varnishlog will show entries like:

```
*   << Request  >> 12345
-   VCL_call     RECV
-   VCL_trace    vcl_recv:8
-   VCL_trace    vcl_recv:10
-   VCL_trace    vcl_recv:11
-   VCL_return   synth
```

Each `VCL_trace` line shows:
- The subroutine name (e.g., `vcl_recv`)
- The line number that was executed (e.g., `:8`)

## Backend Replacement

The only VCL modification needed is replacing backend addresses with the mock backend:

**Original VCL:**
```vcl
vcl 4.1;

backend default {
    .host = "api.example.com";
    .port = "80";
}

sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403));
    }
    if (req.url ~ "^/api/") {
        return (pass);
    }
    return (hash);
}
```

**Modified VCL (for testing):**
```vcl
vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "45678";
}

sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403));
    }
    if (req.url ~ "^/api/") {
        return (pass);
    }
    return (hash);
}
```

**Key Changes:**
1. Only backend host/port are changed (simple text replacement)
2. All VCL logic remains completely unchanged
3. No imports or trace logs needed

## Log Capture

Use varnishlog to capture trace events:

```bash
varnishlog -n <instance> -g request -i VCL_trace,VCL_call,VCL_return,BackendOpen
```

Parse the output to extract:
- `VCL_trace` entries (shows which VCL lines executed)
- `BackendOpen` events (to count backend calls)
- `VCL_call` and `VCL_return` (to understand subroutine flow)

# Mock Backend

Simple HTTP server that responds according to test spec:

```go
type BackendConfig struct {
    Status  int               // HTTP status code
    Headers map[string]string // Response headers
    Body    string            // Response body
}

func StartBackend(config BackendConfig) (*http.Server, string, error) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        for k, v := range config.Headers {
            w.Header().Set(k, v)
        }
        w.WriteHeader(config.Status)
        w.Write([]byte(config.Body))
    })

    // Listen on random port
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return nil, "", err
    }

    server := &http.Server{Handler: handler}
    go server.Serve(listener)

    addr := listener.Addr().String()
    return server, addr, nil
}
```

A simple text replacement updates backend definitions in VCL with the mock backend address.

# Assertions

## Supported Assertions (Phase 1)

Keep it simple:

1. **status** - Response status code (required)
2. **backend_calls** - Number of times backend was called (optional)
3. **headers** - Exact match on specific headers (optional)
4. **body_contains** - Substring match in response body (optional)

```yaml
expect:
  status: 200
  backend_calls: 1
  headers:
    Content-Type: application/json
  body_contains: '"success":true'
```

## Future Assertions (Later Phases)

- `body_matches` - Regex match on body
- `headers_exist` - Header presence check
- `cache_hit` - Verify cache behavior
- etc.

# Test Output

## Passing Test

```
PASS: API requests bypass cache (45ms)
```

## Failing Test

When a test fails, the output shows the VCL with executed lines highlighted:

```
FAIL: Block admin requests (23ms)

Expected status: 403
Actual status: 200

Backend was called 1 time (expected 0)

VCL (examples/api.vcl):
     8 * sub vcl_recv {
     9 |     # Block admin access
    10 |     if (req.url ~ "^/admin") {
    11 |         return (synth(403, "Forbidden"));
    12 |     }
    13 |
    14 *     # API requests bypass cache
    15 *     if (req.url ~ "^/api/") {
    16 *         return (pass);
    17 |     }
    18 |
    19 |     # Everything else gets cached
    20 |     return (hash);
    21 | }
```

**Legend:**
- `*` = Line was executed (shown in green when colors are enabled)
- `|` = Line was not executed

**Color Output:**
When outputting to a terminal, executed lines are shown in green. Colors are automatically disabled when:
- `NO_COLOR` environment variable is set
- Output is piped to another command
- `TERM=dumb`
- `--no-color` flag is used

## Verbose Output

With `-v` flag, passing tests also show VCL execution:

```
PASS: API requests bypass cache (45ms)

VCL executed lines: 8, 14, 15, 16
Backend calls: 1
Response status: 200
```

# Implementation Strategy

## Phase 1: Minimal Viable Product

**Goal**: Single test runner with basic assertions

1. **YAML Parser**:
   - Parse test specification from YAML file
   - Support multiple tests per file (separated by `---`)
2. **VCL Backend Replacer**:
   - Simple text replacement to change backend address to mock backend
   - Minimal modification (only host/port)
3. **Mock Backend**: HTTP server that returns configured response
4. **Test Runner**:
   - Start varnishd with `vcl_trace=on` (or reuse)
   - Load modified VCL
   - Start varnishlog to capture VCL_trace events
   - Make HTTP request
   - Capture response and trace logs
5. **Trace Parser**: Parse varnishlog output to extract executed line numbers
6. **Assertions**: Verify status, backend_calls, headers, body_contains
7. **Output**:
   - Simple PASS/FAIL with timing
   - On failure: show VCL with executed lines marked (`*` for executed, `|` for not)
   - Color-code executed lines (green) when outputting to terminal
   - Support `--no-color` flag and auto-detect piped output

**Explicitly NOT in Phase 1:**
- ~~Multiple backends~~
- ~~Multiple requests per test~~
- ~~VCL flow assertions (path, variables, etc.)~~
- ~~Timing/performance checks~~
- ~~HTML reports~~
- ~~Watch mode~~
- ~~Parallel test execution~~

## Phase 2: Multiple Tests

1. **Test Discovery**: Auto-discover `*.yaml` files in directory
2. **Sequential Execution**: Run tests one by one
3. **Summary Report**: Show total passed/failed

## Phase 3: Advanced Features

Only add if users request:
- VCL flow/path assertions
- Variable tracking
- Multiple backends
- Multiple requests per test
- Parallel execution
- HTML reports

## Directory Structure

```
vcltest/
├── cmd/
│   └── vcltest/
│       └── main.go          # CLI entry point
├── pkg/
│   ├── testspec/
│   │   └── spec.go          # YAML test spec parsing
│   ├── vcl/
│   │   └── backend.go       # Simple backend address replacement
│   ├── backend/
│   │   └── mock.go          # Mock HTTP backend
│   ├── varnish/
│   │   ├── manager.go       # Varnish process management
│   │   └── trace.go         # varnishlog trace parsing
│   ├── runner/
│   │   └── runner.go        # Test execution
│   └── assertion/
│       └── assertion.go     # Assertion evaluation
├── examples/
│   ├── basic.vcl
│   └── basic.yaml
├── go.mod
└── README.md
```

# CLI Interface

## Phase 1

```bash
vcltest test.yaml              # Run single test (or all tests in file)
vcltest tests/                 # Run all .yaml files in directory
vcltest -v test.yaml           # Verbose output (show VCL execution for passing tests)
vcltest --no-color test.yaml   # Disable color output
```

## Future

```bash
vcltest -h                     # Help
vcltest --version              # Version info
```

# Key Design Decisions

## Simplicity Over Flexibility

- YAML for human readability
- Minimal assertion types
- Simple trace format (just line numbers)
- Clear, concise error messages
- No complex abstractions

## Minimal Dependencies

- **gopkg.in/yaml.v3** for YAML parsing (supports multiple documents)
- **Standard library only** for everything else:
  - `net/http` for mock backends and HTTP requests
  - `os/exec` for varnishd and varnishlog
  - `testing` for unit tests
  - `strings` and `regexp` for simple backend address replacement
  - ANSI color codes (simple strings, no library needed)

## Deterministic Testing

- Each test is isolated (fresh VCL load)
- Mock backends are fully controlled
- No reliance on external services
- Tests are reproducible

## Clear Error Messages

When a test fails:
- Show which assertion failed
- Show expected vs actual values
- Show which VCL lines executed
- User looks at their VCL to debug

# Complete End-to-End Example

## Example VCL: `examples/api.vcl`

```vcl
vcl 4.1;

backend default {
    .host = "api.example.com";
    .port = "80";
}

sub vcl_recv {
    # Block admin access
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }

    # API requests bypass cache
    if (req.url ~ "^/api/") {
        return (pass);
    }

    # Everything else gets cached
    return (hash);
}
```

## Example Test: `examples/api.yaml`

```yaml
name: API requests bypass cache
vcl: examples/api.vcl

request:
  url: /api/users/123

backend:
  status: 200
  headers:
    Content-Type: application/json
  body: '{"user":"test"}'

expect:
  status: 200
  backend_calls: 1
  headers:
    Content-Type: application/json
  body_contains: '"user"'
```

## Running the Test

```bash
$ vcltest examples/api.yaml
PASS: API requests bypass cache (45ms)
```

## Running with Verbose Output

```bash
$ vcltest -v examples/api.yaml
PASS: API requests bypass cache (45ms)

VCL executed lines: 8, 11, 14, 15
Backend calls: 1
Response status: 200
Response headers:
  Content-Type: application/json
```

## Example Failure

If the VCL had a bug and returned 403:

```bash
$ vcltest examples/api.yaml
FAIL: API requests bypass cache (23ms)

Expected status: 200
Actual status: 403

VCL (examples/api.vcl):
     8 * sub vcl_recv {
     9 *     # Block admin access
    10 *     if (req.url ~ "^/admin") {
    11 *         return (synth(403, "Forbidden"));
    12 |     }
    13 |
    14 |     # API requests bypass cache
    15 |     if (req.url ~ "^/api/") {
    16 |         return (pass);
    17 |     }
    18 |
    19 |     # Everything else gets cached
    20 |     return (hash);
    21 | }

The request matched the admin check and returned 403.
```

The output shows that lines 8-11 were executed (marked with `*`), revealing that the `/api/users/123` URL unexpectedly matched the `"^/admin"` pattern.

# Code Style and Principles

**Simplicity over flexibility.**

## Code Principles

1. **Readability**: Code should be self-documenting
2. **Correctness**: Prefer correct over clever
3. **Maintainability**: Future developers should understand the code easily
4. **Minimal dependencies**: stdlib + yaml.v3 only

## Error Handling

Always wrap errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to parse varnishlog trace: %w", err)
}
```

## Testing

- Unit tests for trace parser, backend replacer, assertions
- Integration tests for full test runner
- Examples in `examples/` directory that actually work

# Success Criteria

A test **passes** when ALL assertions succeed.

A test **fails** if ANY assertion fails, showing:
- Which assertion failed
- Expected vs actual value
- Which VCL lines executed

Output should be clear enough that users can immediately understand what went wrong and where to look in their VCL.
