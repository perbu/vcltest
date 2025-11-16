# VCLTest - VCL Testing Framework

VCLTest is a declarative testing framework for Varnish Configuration Language (VCL) that makes it easy to verify your VCL logic with automatic instrumentation and execution tracing.

## Features

- **Declarative YAML-based tests** - Write tests in simple, readable YAML format
- **Automatic VCL instrumentation** - Traces execution flow without manual logging
- **Mock backend support** - Controlled backend responses for deterministic testing
- **Clear pass/fail output** - Shows which VCL lines executed and which assertions failed
- **Multiple assertions** - Status codes, backend calls, headers, and body content
- **Multi-test files** - Run multiple tests from a single YAML file
- **Color output** - Easy-to-read terminal output with color highlighting

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/perbu/vcltest.git
cd vcltest

# Build the binary
go build -o vcltest ./cmd/vcltest

# Run example tests
./vcltest examples/basic.yaml
```

### Prerequisites

- Go 1.23 or later
- Varnish 7.x or later installed and available in PATH
- `varnishd` and `varnishlog` commands available

### Your First Test

Create a simple VCL file (`hello.vcl`):

```vcl
vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    if (req.url == "/hello") {
        return (synth(200, "OK"));
    }
    return (pass);
}

sub vcl_synth {
    if (resp.status == 200) {
        set resp.http.Content-Type = "text/plain";
        set resp.body = "Hello, VCL!";
    }
    return (deliver);
}
```

Create a test file (`hello.yaml`):

```yaml
name: Hello endpoint test
vcl: hello.vcl
request:
  url: /hello
expect:
  status: 200
  backend_calls: 0
  headers:
    Content-Type: "text/plain"
  body_contains: "Hello"
```

Run the test:

```bash
./vcltest hello.yaml
```

## Test Format

### Basic Structure

```yaml
name: Test name
vcl: path/to/file.vcl
request:
  method: GET           # Optional, defaults to GET
  url: /path            # Required
  headers:              # Optional
    Header-Name: value
  body: "request body"  # Optional

backend:
  status: 200           # Optional, defaults to 200
  headers:              # Optional
    Header-Name: value
  body: "response"      # Optional

expect:
  status: 200                    # Required - expected HTTP status
  backend_calls: 1               # Optional - expected backend requests
  headers:                       # Optional - expected response headers
    Header-Name: expected-value
  body_contains: "text"          # Optional - substring in response body
```

### Multiple Tests

Use `---` to separate multiple tests in a single file:

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

## Usage

```bash
# Run a single test file
vcltest test.yaml

# Verbose output (shows VCL execution trace)
vcltest -v test.yaml
vcltest --verbose test.yaml

# Disable color output
vcltest --no-color test.yaml

# Show version
vcltest --version
```

## Assertions

VCLTest supports four types of assertions:

### Status Code (Required)

```yaml
expect:
  status: 200
```

Checks that the HTTP response status code matches exactly.

### Backend Calls (Optional)

```yaml
expect:
  backend_calls: 1
```

Verifies the number of backend requests made during the test. Useful for testing cache behavior.

### Headers (Optional)

```yaml
expect:
  headers:
    Content-Type: "application/json"
    X-Custom-Header: "value"
```

Checks that specific response headers have the expected values.

### Body Contains (Optional)

```yaml
expect:
  body_contains: "expected text"
```

Verifies that the response body contains the specified substring.

## Output

### Passing Tests

```
PASS: Test name (45ms)
```

### Failing Tests

```
FAIL: Test name (52ms)

Failed assertions:
  - expected status 200, got 404
    Expected: 200
    Actual:   404

VCL execution:
  * 1  vcl 4.1;
  | 2
  * 3  backend default {
  * 4      .host = "127.0.0.1";
  * 5      .port = "8080";
  * 6  }
  * 7
  * 8  sub vcl_recv {
  * 9      if (req.url == "/test") {
  | 10         return (synth(200, "OK"));
  | 11     }
  * 12     return (pass);
  * 13 }
```

Lines marked with `*` (green in color mode) were executed. Lines marked with `|` were not executed.

## Comparison with VTest

VCLTest is inspired by Varnish's VTest but takes a different approach:

| Feature | VCLTest | VTest |
|---------|---------|-------|
| **Format** | YAML | Custom DSL |
| **Backend mocking** | Automatic | Manual configuration |
| **VCL instrumentation** | Automatic | Manual logging |
| **Execution trace** | Built-in | Manual analysis |
| **Learning curve** | Low | Moderate |
| **Flexibility** | Focused on common cases | Highly flexible |

VCLTest is ideal for:
- Quick VCL validation during development
- CI/CD integration
- Teams new to VCL testing
- Standard HTTP request/response testing

Use VTest when you need:
- Complex multi-client scenarios
- Low-level Varnish protocol testing
- Advanced timing controls
- ESI testing

## Examples

See the `examples/` directory for working examples:

- `examples/basic.vcl` - Simple VCL with multiple endpoints
- `examples/basic.yaml` - Three test cases demonstrating different features

## Architecture

VCLTest consists of several components:

1. **Test Specification Parser** (`pkg/testspec`) - Parses YAML test files
2. **VCL Instrumenter** (`pkg/instrument`) - Adds trace logging to VCL
3. **Mock Backend** (`pkg/backend`) - HTTP server for controlled responses
4. **Varnish Manager** (`pkg/varnish`) - Manages varnishd lifecycle
5. **Log Parser** (`pkg/varnish/log.go`) - Extracts execution traces
6. **Test Runner** (`pkg/runner`) - Orchestrates test execution
7. **Assertion Engine** (`pkg/assertion`) - Evaluates test expectations
8. **Output Formatter** (`pkg/runner/output.go`) - Formats results

## Development

### Running Tests

```bash
# Run all unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/testspec
```

### Project Structure

```
vcltest/
├── cmd/vcltest/          # CLI application
├── pkg/
│   ├── testspec/         # YAML test parsing
│   ├── instrument/       # VCL instrumentation
│   ├── backend/          # Mock HTTP backend
│   ├── varnish/          # Varnish process management
│   ├── runner/           # Test execution
│   └── assertion/        # Assertion evaluation
├── examples/             # Example tests
└── README.md
```

## Roadmap

See `plan.md` for the full implementation roadmap.

### Phase 1 (Current) - MVP

- ✅ Single test file execution
- ✅ Basic assertions (status, backend_calls, headers, body_contains)
- ✅ VCL execution tracing
- ✅ Color output

### Phase 2 - Test Discovery

- [ ] Directory scanning
- [ ] Batch execution
- [ ] Summary reporting
- [ ] Test filtering

### Phase 3 - Advanced Features

- [ ] Multiple backends per test
- [ ] Multi-request tests
- [ ] Cache behavior testing
- [ ] HTML reports
- [ ] Watch mode
- [ ] Parallel execution

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

[License TBD]

## Credits

Created by [Per Buer](https://github.com/perbu)

Inspired by Varnish VTest and the need for simpler VCL testing workflows.
