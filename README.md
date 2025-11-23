# VCLTest - VCL Testing Framework

VCLTest is a declarative testing framework for Varnish Configuration Language (VCL) that provides automatic execution tracing and clear error reporting.

## Usage

```bash
vcltest <test-file.yaml>
```

Example:
```bash
vcltest examples/access-control.yaml
```

## Features

- **YAML-based tests** - Simple, declarative test format
- **VCL execution tracing** - See exactly which lines of VCL executed (via Varnish's `feature=+trace`)
- **Colored error output** - Failed tests show VCL source with green ✓ marks on executed lines
- **Mock backend** - Controlled backend responses for deterministic testing
- **Multiple assertions** - Status codes, backend calls, headers, body content, cache hits
- **Multi-test files** - Run multiple test cases from a single YAML file
- **Temporal testing** - Time manipulation for cache TTL and time-dependent VCL testing
- **Scenario-based tests** - Multi-step tests with time advancement between steps

## Quick Start

### Prerequisites

- Go 1.21 or later
- Varnish 7.x or later (with `varnishd` and `varnishlog` in PATH)
- libfaketime (optional, for temporal/cache testing): `brew install libfaketime` or `apt install faketime`

### Installation

```bash
git clone https://github.com/perbu/vcltest.git
cd vcltest
go build -o vcltest ./cmd/vcltest
```

### Your First Test

Create a VCL file (`hello.vcl`):

```vcl
vcl 4.1;

backend default {
    .host = "__BACKEND_HOST__";
    .port = "__BACKEND_PORT__";
}

sub vcl_recv {
    if (req.url == "/hello") {
        return (synth(200, "OK"));
    }
    return (pass);
}

sub vcl_synth {
    set resp.http.Content-Type = "text/plain";
    set resp.body = "Hello, VCL!";
    return (deliver);
}
```

Create a test file (`hello.yaml`):

```yaml
name: Hello endpoint returns 200
vcl: hello.vcl

request:
  url: /hello

backend:
  status: 200
  body: "backend response"

expect:
  status: 200
  backend_calls: 0
  body_contains: "Hello"
```

Run the test:

```bash
./vcltest hello.yaml
```

## Test Format

### Basic Single-Request Test

```yaml
name: Test description
vcl: path/to/file.vcl

request:
  method: GET              # Optional, defaults to GET
  url: /path               # Required
  headers:                 # Optional
    Header-Name: value
  body: "request body"     # Optional

backend:
  status: 200              # Optional, defaults to 200
  headers:                 # Optional
    Header-Name: value
  body: "response"         # Optional

expect:
  status: 200                    # Required
  backend_calls: 1               # Optional - number of backend requests
  headers:                       # Optional - expected response headers
    Header-Name: expected-value
  body_contains: "text"          # Optional - substring match
  cached: true                   # Optional - check if cached (Phase 2)
  age_lt: 35                     # Optional - Age header < N seconds (Phase 2)
  age_gt: 5                      # Optional - Age header > N seconds (Phase 2)
  stale: false                   # Optional - check if stale (Phase 2)
```

### Scenario-Based Temporal Test

For cache TTL testing and time-dependent VCL logic:

```yaml
name: Cache TTL test
vcl: cache.vcl

scenario:
  # Step 1: Initial request at t=0s
  - at: "0s"
    request:
      url: /article
    backend:
      status: 200
      headers:
        Cache-Control: "max-age=60"
      body: "Article content"
    expect:
      status: 200
      cached: false
      backend_calls: 1

  # Step 2: Request at t=30s - should hit cache
  - at: "30s"
    request:
      url: /article
    expect:
      status: 200
      cached: true
      age_lt: 35
      backend_calls: 1

  # Step 3: Request at t=70s - cache expired
  - at: "70s"
    request:
      url: /article
    expect:
      status: 200
      cached: false
      backend_calls: 2
```

**Key points:**
- Time offsets (`at: "30s"`) are absolute from test start, not incremental
- Requires libfaketime installed
- Faketime automatically enabled when scenario tests detected

### Multiple Tests

Use `---` to separate multiple tests:

```yaml
name: Test 1
vcl: test.vcl
request:
  url: /path1
expect:
  status: 200

---

name: Test 2
vcl: test.vcl
request:
  url: /path2
expect:
  status: 404
```

## Output

### Passing Tests

```
Test 1: Hello endpoint returns 200
  ✓ PASSED
```

### Failing Tests with VCL Trace

When a test fails, VCLTest shows which VCL lines executed:

```
Test 1: Wrong status expectation
FAILED: Wrong status expectation
  ✗ Status code: expected 404, got 200

VCL Execution Trace:
    1 | vcl 4.1;
    2 |
    3 | backend default {
    4 |     .host = "127.0.0.1";
    5 |     .port = "8080";
    6 | }
    7 |
    8 | sub vcl_recv {
    9 |     # Block admin paths
✓  10 |     if (req.url ~ "^/admin") {
   11 |         return (synth(403, "Forbidden"));
   12 |     }
   13 |
   14 |     # Allow API paths
✓  15 |     if (req.url ~ "^/api/") {
✓  16 |         return (pass);
   17 |     }
   ...

Backend Calls: 1
VCL Flow: RECV → PASS → DELIVER
```

Lines with green ✓ marks were executed. Gray lines were not executed.

## Backend Placeholders

VCLTest automatically replaces these placeholders in your VCL:

- `__BACKEND_HOST__` - Replaced with mock backend host
- `__BACKEND_PORT__` - Replaced with mock backend port

This allows tests to work without hardcoding mock backend addresses.

## Examples

See [examples/README.md](examples/README.md) for a comprehensive guide to example tests, including basic routing, access control, cache TTL testing, multi-backend routing, and more.

## Architecture

VCLTest uses several packages:

- **pkg/testspec** - YAML test file parser
- **pkg/runner** - Test orchestration and execution
- **pkg/varnish** - varnishd process management
- **pkg/varnishadm** - Varnish CLI protocol implementation
- **pkg/service** - Service lifecycle coordination
- **pkg/recorder** - varnishlog capture and parsing
- **pkg/formatter** - VCL source formatting with execution highlights
- **pkg/backend** - Mock HTTP backend server
- **pkg/client** - HTTP client for making test requests
- **pkg/assertion** - Test expectation verification

## How It Works

1. Start varnishd with `feature=+trace` enabled
2. Load your VCL (with backend placeholders replaced)
3. Start varnishlog recorder
4. Execute HTTP request through Varnish
5. Stop recorder and parse VCL_trace messages
6. Check assertions and format output
7. On failure, show VCL with execution markers

## Comparison with VTest

| Feature | VCLTest | VTest |
|---------|---------|-------|
| **Format** | YAML | Custom DSL |
| **VCL tracing** | Automatic (feature=+trace) | Manual logging |
| **Error output** | Colored, annotated VCL | Text logs |
| **Backend mocking** | Automatic | Manual setup |
| **Learning curve** | Low | Moderate |
| **Use case** | VCL unit testing | Complex integration testing |

VCLTest is designed for quick, readable VCL unit tests with clear error output. VTest is better for complex scenarios, ESI testing, and low-level protocol testing.

## License

[License TBD]

## Contributing

Contributions welcome! Please open an issue or PR.
