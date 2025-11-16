The goal of this project is to help users verify that the VCL they've made is working.

# Overview

VCL (Varnish Configuration Language) is powerful but can be difficult to debug and test. This tool provides
a test framework that allows developers to:

1. Write declarative test specifications
2. Automatically instrument VCL code for observability
3. Execute tests against a real Varnish instance
4. Verify both behavior and execution flow

# High-Level Architecture

The application will do the following:

1. Start a varnishd instance (reused across tests for efficiency)

Then for each test specification we do the following:

1. Start varnishlog-json to capture execution traces
2. Create one HTTP backend server according to test specification
3. Parse and instrument the VCL file to make it "traceable"
4. Connect to the varnishd instance and load the instrumented VCL
5. Execute the HTTP requests from the test specification
6. Capture varnishlog output
7. Verify expectations against the response and trace
8. Cleanup (stop backend, reset state)

# Core Components

## Test Runner
- Orchestrates the test lifecycle
- Manages process lifecycle (varnishd, varnishlog, backends)
- Collects results and generates reports
- Handles cleanup on failures

## VCL Instrumenter
- Parses VCL source code
- Injects trace statements at strategic points
- Preserves original semantics
- Maintains line number mappings

## Mock Backend Server
- Simple HTTP server that responds according to specification
- Configurable response (status, headers, body, delays)
- Can simulate various backend behaviors

## Assertion Engine
- Evaluates test expectations
- Provides clear failure messages
- Supports multiple assertion types

This raises quite a few questions, which we'll address below.

# VCL Parsing Challenges

VCL has unique syntax that requires careful handling:

## Subroutine Detection
```vcl
sub vcl_recv {
    # Body
}

sub custom_logic {
    # User-defined subroutine
}
```

Pattern: `sub\s+(\w+)\s*{`

## Variable Assignment
```vcl
set req.http.X-Custom = "value";
set req.backend_hint = my_backend;
unset req.http.Cookie;
```

Pattern: `(set|unset)\s+(req|bereq|beresp|resp|obj)\.([\w.]+)`

## Conditional Statements
```vcl
if (req.url ~ "^/api/") {
    # Then branch
} elsif (req.url ~ "^/admin/") {
    # Else-if branch
} else {
    # Else branch
}
```

Need to track nesting levels and match braces correctly.

## Return Statements
```vcl
return (pass);
return (hash);
return (deliver);
```

Pattern: `return\s*\(\s*(\w+)\s*\)`

## Challenges
1. **Comments**: Must preserve and not instrument (`#` and `/* */`)
2. **Strings**: Don't instrument inside string literals
3. **Multiline**: Statements can span multiple lines
4. **Inline C**: `C{ ... }C` blocks must be left untouched
5. **Operators**: VCL has unique operators (`~`, `!~`, `==`, `!=`)
6. **Built-in Functions**: Recognize VMOD calls (e.g., `std.log()`, `std.ip()`)

## Example Complete Transformation

**Original VCL:**
```vcl
vcl 4.1;

backend default {
    .host = "api.example.com";
    .port = "80";
}

sub vcl_recv {
    # Remove cookies for static assets
    if (req.url ~ "\.(jpg|png|css|js)$") {
        unset req.http.Cookie;
        return (hash);
    }

    # API requests always go to backend
    if (req.url ~ "^/api/") {
        return (pass);
    }
}

sub vcl_backend_response {
    # Cache successful responses for 5 minutes
    if (beresp.status == 200) {
        set beresp.ttl = 5m;
    }
}
```

**Instrumented VCL:**
```vcl
vcl 4.1;

import std;

backend default {
    .host = "127.0.0.1";
    .port = "45678";
}

sub vcl_recv {
    std.log("TRACE:vcl_recv:entry:line=9");

    # Remove cookies for static assets
    std.log("TRACE:vcl_recv:condition:line=11:req.url=" + req.url);
    if (req.url ~ "\.(jpg|png|css|js)$") {
        std.log("TRACE:vcl_recv:condition_true:line=11");
        unset req.http.Cookie;
        std.log("TRACE:vcl_recv:unset:line=12:req.http.Cookie");
        std.log("TRACE:vcl_recv:return:line=13:action=hash");
        return (hash);
    }

    # API requests always go to backend
    std.log("TRACE:vcl_recv:condition:line=17:req.url=" + req.url);
    if (req.url ~ "^/api/") {
        std.log("TRACE:vcl_recv:condition_true:line=17");
        std.log("TRACE:vcl_recv:return:line=18:action=pass");
        return (pass);
    }

    std.log("TRACE:vcl_recv:exit:line=20");
}

sub vcl_backend_response {
    std.log("TRACE:vcl_backend_response:entry:line=23");

    # Cache successful responses for 5 minutes
    std.log("TRACE:vcl_backend_response:condition:line=25:beresp.status=" + beresp.status);
    if (beresp.status == 200) {
        std.log("TRACE:vcl_backend_response:condition_true:line=25");
        set beresp.ttl = 5m;
        std.log("TRACE:vcl_backend_response:set:line=26:beresp.ttl=5m");
    }

    std.log("TRACE:vcl_backend_response:exit:line=28");
}
```

**Key Transformations:**
1. Added `import std;` at top
2. Changed backend host/port to mock backend address
3. Added entry/exit logs for each subroutine
4. Added logs before conditionals (with variable values)
5. Added logs when condition is true
6. Added logs for set/unset operations
7. Added logs before return statements
8. Preserved all comments
9. Maintained line number references to original file

# Trace VCL

It is crucial that when we are executing the tests, we need to understand what is happening in the
VCL runtime. The way we should do this is by having a log statement for every line of logic. This way
we can see trace the log execution of the VCL code and follow the flow.

## Instrumentation Strategy

The instrumenter should inject `std.log()` calls at these strategic points:

1. **Subroutine Entry/Exit**: Log when entering/exiting vcl_recv, vcl_backend_fetch, etc.
2. **Conditional Branches**: Log before if/elseif/else to show which path was taken
3. **State Changes**: Log before return statements (pass, hit, fetch, etc.)
4. **Variable Assignments**: Log after setting important variables (especially headers)
5. **Function Calls**: Log before calling VMODs or built-in functions

## Log Format

Each trace log should include:
- Line number in original VCL
- Current subroutine name
- Action being performed
- Relevant variable values (especially req.*, bereq.*, resp.*, beresp.*)

Example instrumented VCL:
```vcl
sub vcl_recv {
    std.log("TRACE:vcl_recv:entry:line=10");

    if (req.url ~ "^/api/") {
        std.log("TRACE:vcl_recv:condition:line=12:req.url=" + req.url + ":matched=/api/");
        set req.backend_hint = api_backend;
        std.log("TRACE:vcl_recv:set:line=13:req.backend_hint=api_backend");
    }

    std.log("TRACE:vcl_recv:return:line=15:action=hash");
    return (hash);
}
```

## Correlation

When we trigger a client request, varnishlog-json will capture the log entries. We then:
1. Parse the JSON log output
2. Extract TRACE: prefixed entries
3. Build an execution trace showing the flow through subroutines
4. Allow assertions on which code paths were executed

# Client Request

Initially I think we should make the client requests simple. We can just specify a METHOD/URL, headers and what the
expected response should be.

## Request Specification Format

A test request should be specified as a simple struct/JSON:

```go
type Request struct {
    Method  string            // GET, POST, PUT, DELETE, etc.
    URL     string            // Path and query string
    Headers map[string]string // Request headers
    Body    string            // Request body (optional)
}
```

Example:
```json
{
    "method": "GET",
    "url": "/api/users/123",
    "headers": {
        "Host": "example.com",
        "User-Agent": "vcltest/1.0",
        "X-Custom-Header": "value"
    }
}
```

# Client Response

We need to check that the response is what we expected. Drawing inspiration from popular testing
frameworks like Jest, Chai, and Go's testify, we should support clear, expressive assertions.

## Suggested Expect Statements

### Response Status
```go
expect.Status(200)
expect.StatusOK()
expect.StatusNotFound()
expect.StatusIn(200, 201, 204) // Any of these
```

### Response Headers
```go
expect.Header("Content-Type", "application/json")
expect.HeaderContains("Cache-Control", "max-age")
expect.HeaderExists("X-Cache-Hit")
expect.HeaderNotExists("X-Debug")
expect.HeaderMatches("X-Request-ID", "^[a-f0-9-]+$") // Regex
```

### Response Body
```go
expect.BodyEquals("exact content")
expect.BodyContains("partial match")
expect.BodyMatches("regex pattern")
expect.BodyJSON(map[string]interface{}{...}) // JSON equality
expect.BodyEmpty()
```

### Cache Behavior
```go
expect.CacheHit()        // Verify X-Cache or Age header
expect.CacheMiss()
expect.CachePass()       // Not cached
expect.Age(lessThan(60)) // Age header value
```

### VCL Flow (using trace data)
```go
expect.VCLPath("vcl_recv", "vcl_hash", "vcl_hit", "vcl_deliver")
expect.VCLReturned("vcl_recv", "hash")
expect.VCLExecuted("vcl_recv:line=15") // Specific line was executed
expect.VCLNotExecuted("vcl_recv:line=20") // Line was skipped
expect.VCLVariableSet("bereq.http.X-Custom", "value")
```

### Timing
```go
expect.ResponseTime(lessThan(100 * time.Millisecond))
expect.BackendCalls(1) // Number of backend requests
```

## Test Specification Format

Complete test specification:

```json
{
    "name": "API request with caching",
    "vcl_file": "test.vcl",
    "backend": {
        "status": 200,
        "headers": {
            "Content-Type": "application/json",
            "Cache-Control": "max-age=3600"
        },
        "body": "{\"user\":\"test\"}"
    },
    "request": {
        "method": "GET",
        "url": "/api/users/123",
        "headers": {
            "Host": "example.com"
        }
    },
    "expect": {
        "status": 200,
        "headers": {
            "Content-Type": "application/json",
            "X-Cache": "MISS"
        },
        "body_contains": "\"user\":\"test\"",
        "vcl_path": ["vcl_recv", "vcl_hash", "vcl_miss", "vcl_backend_fetch", "vcl_backend_response", "vcl_deliver"],
        "backend_calls": 1
    }
}
```

# HTTP Backend

We should just spin up a simple HTTP server that responds according to the test specification.

## Backend Server Implementation

Using Go's stdlib `net/http`:

```go
type BackendConfig struct {
    Status  int               // HTTP status code
    Headers map[string]string // Response headers
    Body    string            // Response body
    Delay   time.Duration     // Simulate slow backend (optional)
}

func StartBackend(config BackendConfig) (*http.Server, string, error) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Optional delay to simulate slow backend
        if config.Delay > 0 {
            time.Sleep(config.Delay)
        }

        // Set response headers
        for k, v := range config.Headers {
            w.Header().Set(k, v)
        }

        // Write status and body
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

    // Return server and the address (127.0.0.1:PORT)
    addr := listener.Addr().String()
    return server, addr, nil
}
```

## Backend Configuration in VCL

The instrumenter needs to:
1. Parse the VCL to find backend definitions
2. Replace the backend address with the mock backend address
3. Or inject a default backend if none exists

Example VCL transformation:
```vcl
# Original
backend default {
    .host = "api.example.com";
    .port = "80";
}

# Instrumented (backend address replaced)
backend default {
    .host = "127.0.0.1";
    .port = "54321";  # Dynamically assigned port
}
```

# Log Capture

When starting a test, we start by executing varnishlog-json and capture the output.

## Implementation Details

1. **Start varnishlog**:
   ```bash
   varnishlog -n <varnish_instance> -g request -j
   ```
   - `-n`: Specify varnish instance name
   - `-g request`: Group by request (correlates all log entries for one request)
   - `-j`: JSON output format

2. **Capture Output**:
   - Start varnishlog as subprocess before making request
   - Read stdout continuously
   - Parse JSON lines as they arrive
   - Stop after request completes (can use timeout)

3. **Parse Trace Entries**:
   ```go
   type TraceEntry struct {
       Line      int               // Original VCL line number
       Subroutine string           // vcl_recv, vcl_fetch, etc.
       Action    string            // entry, set, return, condition
       Variables map[string]string // Variable values at this point
   }
   ```

4. **Correlation**:
   - Each request has a unique VXID (Varnish Transaction ID)
   - Group all log entries by VXID
   - Extract TRACE: entries and build execution flow
   - Make trace available for assertions

# Implementation Strategy

## Phase 1: Core Functionality
1. **VCL Parser**: Simple regex-based parser to identify subroutines, if statements, set operations, and return statements
2. **Instrumenter**: Inject std.log() calls at strategic points
3. **Mock Backend**: HTTP server with configurable responses
4. **Test Runner**: Execute single test, capture logs, verify basic assertions
5. **Basic Assertions**: Status code, header presence/value, body contains

## Phase 2: Enhanced Tracing
1. **VCL Flow Tracking**: Build execution path from trace logs
2. **Variable Tracking**: Capture header and variable values at each step
3. **Advanced Assertions**: VCL path verification, cache behavior checks
4. **Better Error Messages**: Show diff between expected and actual, highlight failed assertions

## Phase 3: Polish
1. **Test Discovery**: Auto-discover test files in directory
2. **Parallel Execution**: Run independent tests concurrently
3. **HTML Reports**: Generate visual test reports
4. **Watch Mode**: Re-run tests on file changes

## Directory Structure

```
vcltest/
├── cmd/
│   └── vcltest/
│       └── main.go          # CLI entry point
├── pkg/
│   ├── parser/
│   │   └── vcl.go           # VCL parsing
│   ├── instrument/
│   │   └── instrument.go    # VCL instrumentation
│   ├── backend/
│   │   └── mock.go          # Mock HTTP backend
│   ├── varnish/
│   │   ├── manager.go       # Varnish process management
│   │   └── log.go           # varnishlog capture/parsing
│   ├── runner/
│   │   └── runner.go        # Test execution
│   ├── assertion/
│   │   └── expect.go        # Assertion engine
│   └── testspec/
│       └── spec.go          # Test specification types
├── examples/
│   ├── basic.vcl
│   └── basic_test.json
└── README.md
```

## Key Design Decisions

### Simplicity Over Flexibility
- Use simple data structures (structs, maps)
- JSON for test specifications (human-readable, easy to edit)
- Avoid complex abstractions or frameworks
- Stdlib only - no external dependencies

### Deterministic Testing
- Each test is isolated (fresh VCL load)
- Mock backends are fully controlled
- No reliance on external services
- Tests should be reproducible

### Clear Error Messages
When a test fails, show:
- Which assertion failed
- Expected vs actual values
- Relevant VCL code snippet
- Execution trace leading to the failure

Example failure output:
```
FAIL: API request with caching

Assertion failed: expect.Header("X-Cache", "MISS")
  Expected: "MISS"
  Actual:   "HIT"

VCL Trace:
  vcl_recv:10  -> entry
  vcl_recv:12  -> condition matched: req.url="/api/users/123"
  vcl_recv:15  -> return: hash
  vcl_hash:20  -> entry
  vcl_hit:30   -> entry (unexpected: cache hit occurred)
  vcl_deliver:40 -> set resp.http.X-Cache="HIT"

Backend was not called (expected 1 call, got 0)
```

# Later Expansion

## Multiple HTTP Backends
- Support multiple backend definitions in VCL
- Different mock backends for different backend definitions
- Backend selection verification

## Multiple Client Requests
- Sequential requests in single test (e.g., test caching)
- Verify cache HIT on second request
- State accumulation across requests

## Programmable Client Requests
```go
// Instead of static request spec, allow functions
func requestGenerator(attempt int) Request {
    return Request{
        Method: "GET",
        URL:    fmt.Sprintf("/page/%d", attempt),
    }
}
```

## Programmable Backend Responses
```go
// Backend can respond differently based on request
type BackendHandler func(r *http.Request) Response

// Example: Return 500 on first call, 200 on retry
callCount := 0
backend := func(r *http.Request) Response {
    callCount++
    if callCount == 1 {
        return Response{Status: 500}
    }
    return Response{Status: 200}
}
```

## Advanced Features
- **State Inspection**: Pause test execution, inspect Varnish cache state
- **Performance Testing**: Measure cache performance, backend load
- **Edge Cases**: Test VCL behavior under various failure conditions
- **Coverage**: Show which VCL lines were/weren't executed across all tests
- **Visual Flow**: Generate flowchart of VCL execution path
- **Fuzzing**: Generate random requests to find edge cases
- **Regression Testing**: Capture actual responses as golden files

# Instructions for Code Style

**Simplicity over flexibility.**

- Tests should only use the stdlib
- No external dependencies unless absolutely necessary
- Clear variable names, comprehensive comments
- Error handling: always check errors, provide context
- Testing: unit tests for parsers and instrumenters
- Examples: provide working examples in examples/ directory

## Code Principles

1. **Readability**: Code should be self-documenting
2. **Correctness**: Prefer correct over clever
3. **Maintainability**: Future developers should understand the code easily
4. **Performance**: Don't optimize prematurely, but avoid obvious inefficiencies

## Error Handling Pattern

```go
if err != nil {
    return fmt.Errorf("failed to instrument VCL: %w", err)
}
```

Always wrap errors with context about what operation failed.

# Complete End-to-End Example

## Example Test File: `examples/caching_test.json`

```json
{
    "name": "Static assets are cached without cookies",
    "vcl_file": "examples/caching.vcl",
    "backend": {
        "status": 200,
        "headers": {
            "Content-Type": "image/jpeg",
            "Cache-Control": "max-age=3600"
        },
        "body": "fake-image-data"
    },
    "request": {
        "method": "GET",
        "url": "/images/logo.jpg",
        "headers": {
            "Host": "example.com",
            "Cookie": "session=abc123"
        }
    },
    "expect": {
        "status": 200,
        "headers": {
            "Content-Type": "image/jpeg"
        },
        "body_contains": "fake-image-data",
        "vcl_path": ["vcl_recv", "vcl_hash", "vcl_miss", "vcl_backend_fetch", "vcl_backend_response", "vcl_deliver"],
        "vcl_variable_unset": "req.http.Cookie",
        "backend_calls": 1
    }
}
```

## Execution Flow

1. **Start**: `vcltest run examples/caching_test.json`

2. **Setup Phase**:
   - Start varnishd instance (if not running)
   - Parse `examples/caching.vcl`
   - Instrument VCL (inject trace logs)
   - Start mock backend on random port (e.g., 127.0.0.1:45678)
   - Update backend definition in VCL
   - Load instrumented VCL into varnishd

3. **Execution Phase**:
   - Start varnishlog with `-g request -j`
   - Make HTTP request to varnishd:
     ```
     GET /images/logo.jpg HTTP/1.1
     Host: example.com
     Cookie: session=abc123
     ```
   - Capture varnishlog output

4. **Verification Phase**:
   - Parse response (status, headers, body)
   - Parse varnishlog JSON
   - Extract TRACE entries
   - Build execution flow
   - Run assertions:
     - ✓ Status is 200
     - ✓ Content-Type is image/jpeg
     - ✓ Body contains "fake-image-data"
     - ✓ VCL path matches expected
     - ✓ req.http.Cookie was unset
     - ✓ Backend was called once

5. **Cleanup**:
   - Stop mock backend
   - Stop varnishlog

6. **Report**:
   ```
   PASS: Static assets are cached without cookies (127ms)

   VCL Flow:
     vcl_recv:9        -> entry
     vcl_recv:11       -> condition: req.url="/images/logo.jpg" matched "\.(jpg|png|css|js)$"
     vcl_recv:12       -> unset req.http.Cookie
     vcl_recv:13       -> return: hash
     vcl_hash:22       -> entry
     vcl_miss:30       -> entry
     vcl_backend_fetch:40 -> entry
     vcl_backend_response:50 -> entry
     vcl_backend_response:52 -> condition: beresp.status=200 matched
     vcl_backend_response:53 -> set beresp.ttl=5m
     vcl_deliver:60    -> entry

   Backend Requests: 1
   ```

## CLI Usage Examples

```bash
# Run single test
vcltest run test.json

# Run all tests in directory
vcltest run tests/

# Run with verbose output
vcltest run -v test.json

# Run and show VCL trace
vcltest run --trace test.json

# Keep varnish running between tests
vcltest run --keep-alive tests/

# Generate HTML report
vcltest run --report=html tests/

# Watch mode (re-run on file changes)
vcltest watch tests/
```

## Success Criteria

A test passes when ALL assertions succeed:
- Response status matches
- All expected headers match
- Body assertions pass
- VCL execution path matches
- Backend call count matches
- Any custom assertions pass

A test fails if ANY assertion fails, with detailed output showing:
- Which assertion failed
- Expected vs actual values
- VCL trace showing what actually happened
- Relevant VCL source code snippet

## Performance Considerations

- **Varnish Reuse**: Keep varnishd running across tests (startup is slow)
- **Parallel Tests**: Run independent tests concurrently
- **Fast Failures**: Stop test execution on first assertion failure
- **Minimal Logging**: Only log what's necessary for trace reconstruction
- **Cleanup**: Ensure all resources are freed even on failure

## Real-World Usage

```bash
# Typical workflow
$ vcltest run tests/api_tests.json
PASS: API requests bypass cache (45ms)
PASS: API responses have CORS headers (38ms)
PASS: API backend selection works (52ms)

$ vcltest run tests/static_tests.json
PASS: Static assets are cached (67ms)
FAIL: CSS files have correct content-type (43ms)

  Assertion failed: expect.Header("Content-Type", "text/css")
    Expected: "text/css"
    Actual:   "application/octet-stream"

  VCL did not set Content-Type. Backend returned "application/octet-stream"
  Consider setting beresp.http.Content-Type in vcl_backend_response

PASS: Images are cached for 1 hour (51ms)

Tests: 4 passed, 1 failed, 5 total
Time: 296ms
```

This comprehensive design should provide a solid foundation for implementing a VCL testing framework that is both powerful and simple to use.