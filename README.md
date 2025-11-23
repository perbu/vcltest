# VCLTest

Unlike VTest2, which is made to test varnishd, this tool is made explicitly to test your VCL logic in a performant
manner, without requiring a complete recompile of your VCL for every test executed.

Write tests in YAML, see exactly which VCL lines are executed when tests fail.

![VCLTest failure output showing VCL execution trace](examples/screenshot-failure.png)

## Installation

```bash
git clone https://github.com/perbu/vcltest.git
cd vcltest
go build -o vcltest ./cmd/vcltest
```

### Requirements

- Go 1.21+
- Varnish 7.x+ (`varnishd` and `varnishlog` in PATH)
- libfaketime (optional, for cache TTL tests): `brew install libfaketime` or `apt install faketime`

## Usage

```bash
vcltest [options] <test-file.yaml>

Options:
  -v, -verbose       Enable verbose debug logging
  -vcl <path>        VCL file to use (overrides auto-detection)
  -debug-dump        Preserve all artifacts in /tmp for debugging (no cleanup)
  -version           Show version information
```

## Quick Start

**hello.vcl:**

```vcl
vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    if (req.url == "/hello") {
        return (synth(200, "OK"));
    }
}

sub vcl_synth {
    set resp.body = "Hello, VCL!";
    return (deliver);
}
```

**hello.yaml:**

```yaml
name: Hello endpoint returns 200

backends:
  default:
    status: 200
    body: "backend response"

request:
  url: /hello

expect:
  status: 200
  body_contains: "Hello"
```

**Run:**

```bash
./vcltest hello.yaml
```

## Test Format

### Basic Test

```yaml
name: Test description

backends:
  default:                 # Must match backend name in VCL
    status: 200            # Optional, default: 200
    headers:               # Optional
      X-Backend: value
    body: "response"       # Optional

request:
  url: /path
  method: GET              # Optional, default: GET
  headers:                 # Optional
    X-Custom: value
  body: "request body"     # Optional

expect:
  status: 200              # Required
  headers:                 # Optional
    X-Header: expected
  body_contains: "text"    # Optional
  cached: true             # Optional
  age_lt: 60               # Optional
  age_gt: 10               # Optional
```

### Cache TTL Tests

Test time-dependent behavior with dynamic backend configuration:

```yaml
name: Cache TTL test

scenario:
  # Step 1: Initial request - cache miss
  - at: "0s"
    request:
      url: /article
    backend:                    # Backend config for this step
      status: 200
      headers:
        Cache-Control: "max-age=60"
      body: "Article content"
    expect:
      cached: false

  # Step 2: Request at 30s - cache hit
  - at: "30s"
    request:
      url: /article
    expect:
      cached: true

  # Step 3: Request at 70s - cache expired, new backend fetch
  - at: "70s"
    request:
      url: /article
    backend:                    # Backend can return different content/headers
      status: 200
      headers:
        Cache-Control: "max-age=120"
      body: "Updated content"
    expect:
      cached: false
```

**Key features:**
- Time offsets are absolute from test start
- Backend configuration can be specified per-step for dynamic responses
- Backends are reconfigured on-the-fly without restarting
- Requires libfaketime for time manipulation

### Multiple Tests

Separate tests with `---`:

```yaml
name: Test 1

backends:
  default:
    status: 200

request:
  url: /path1

expect:
  status: 200
---
name: Test 2

backends:
  default:
    status: 404

request:
  url: /path2

expect:
  status: 404
```

## When Tests Fail

VCLTest shows which VCL lines executed (green ✓), making debugging straightforward. See screenshot above.

## Backend Override

VCLTest uses AST-based backend replacement. Use real hostnames in your VCL:

```vcl
backend api {
    .host = "api.production.com";
    .port = "443";
}
```

Then specify matching backend names in your test YAML:

```yaml
backends:
  api:                     # Must match VCL backend name
    status: 200
    body: '{"ok": true}'
```

VCLTest automatically replaces the production hostname/port with test mock servers. Your VCL backend names must match the YAML backend names.

## Debugging Failed Tests

When tests fail, use the `-debug-dump` flag to preserve all artifacts for inspection:

```bash
vcltest -debug-dump examples/cache-ttl.yaml
```

This creates a timestamped directory in `/tmp` containing:
- Original and modified VCL files
- Complete varnishlog output
- Test specification YAML
- Faketime control file (for time-based tests)
- README with debugging instructions

The debug dump makes it easy to understand what happened during test execution without re-running tests.

## Examples

See [examples/README.md](examples/README.md) for routing, access control, cache TTL, and multi-backend tests.

## How It Works

VCLTest starts varnishd with `feature=+trace`, captures varnishlog output, and parses VCL_trace messages to show
execution flow. See [CLAUDE.md](CLAUDE.md) for architecture details.

## License

[License TBD]
