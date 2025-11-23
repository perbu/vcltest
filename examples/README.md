# VCLTest Examples

This directory contains example VCL tests demonstrating various testing patterns and VCLTest features.

## Quick Reference

| Example | Demonstrates | Key Features |
|---------|--------------|--------------|
| [basic](#basic) | Simple request routing | Health checks, redirects, synthetic responses |
| [access-control](#access-control) | Header-based authorization | Request header inspection, 403 responses |
| [cache-ttl](#cache-ttl) | Cache expiration testing | Time manipulation, cache hit/miss detection, age assertions |
| [routing](#routing) | Multi-backend routing | Backend selection, multiple mock backends |
| [error-demo](#error-demo) | Test failure visualization | VCL execution tracing on failure |
| [overhead](#overhead) | Performance benchmarking | Shared VCL mode for fast multi-test execution |

## Examples

### basic

**Files:** `basic.yaml`, `basic.vcl`

**Purpose:** Demonstrates fundamental VCL testing patterns including synthetic responses, redirects, and backend pass-through.

**What it tests:**
- Health check endpoint returning synthetic 200 response
- Backend pass-through with custom headers
- 301 redirect with Location header

**Key concepts:**
- Using `vcl_recv` for routing decisions
- Synthetic responses via `return (synth(...))`
- Adding custom headers in `vcl_deliver` and `vcl_backend_response`
- Multiple test cases in one YAML file (separated by `---`)

**Run it:**
```bash
vcltest examples/basic.yaml
```

---

### access-control

**Files:** `access-control.yaml`, `access-control.vcl`

**Purpose:** Shows how to test header-based access control and authorization logic.

**What it tests:**
- Request without required header is denied (403)
- Request with correct header is allowed (200)

**Key concepts:**
- Request header inspection in `vcl_recv`
- Conditional logic based on headers
- Testing both positive and negative cases
- Mock backend responses defined in YAML

**Run it:**
```bash
vcltest examples/access-control.yaml
```

---

### cache-ttl

**Files:** `cache-ttl.yaml`, `cache-ttl.vcl`

**Purpose:** Demonstrates temporal testing with time manipulation for cache TTL validation.

**What it tests:**
- Initial request is a cache miss
- Request at t=30s hits the cache (within TTL)
- Request at t=70s is a cache miss (TTL expired)

**Key concepts:**
- **Scenario-based testing** with time advancement
- Time offsets (`at: "0s"`, `at: "30s"`) are absolute from test start
- Cache assertions: `cached: true/false`, `age_lt`, `age_gt`
- Requires libfaketime for time manipulation
- Backend placeholder format: `__BACKEND_HOST_DEFAULT__`, `__BACKEND_PORT_DEFAULT__`

**Run it:**
```bash
vcltest examples/cache-ttl.yaml
```

**Note:** Requires libfaketime installed (`brew install libfaketime` or `apt install faketime`)

---

### routing

**Files:** `routing.yaml`, `routing.vcl`

**Purpose:** Demonstrates multi-backend routing based on URL patterns.

**What it tests:**
- `/api/*` requests route to `api_backend`
- Other requests route to `web_backend`
- Different responses from each backend

**Key concepts:**
- Multiple backend definitions in VCL
- Backend selection via `req.backend_hint`
- Named backend placeholders: `__BACKEND_HOST_API_BACKEND__`, `__BACKEND_PORT_WEB_BACKEND__`
- Multiple named backends in YAML (`backends:` section)
- Scenario-based tests for sequential requests

**Run it:**
```bash
vcltest examples/routing.yaml
```

---

### error-demo

**Files:** `error-demo.yaml`, `error-demo.vcl`

**Purpose:** Intentionally failing test to demonstrate VCLTest's error visualization.

**What it shows:**
- VCL execution trace with green ✓ marks on executed lines
- Clear error messages showing expected vs actual values
- Backend call count in output
- VCL flow diagram (RECV → PASS → DELIVER)

**Run it:**
```bash
vcltest examples/error-demo.yaml
```

Expected output: Test failure with annotated VCL showing which lines executed.

---

### overhead

**Files:** `overhead.yaml`, `overhead.vcl`

**Purpose:** Performance benchmarking with multiple simple tests to measure shared VCL mode efficiency.

**What it tests:**
- 10 identical test cases
- Demonstrates shared VCL performance (VCL loaded once, reused across tests)
- Minimal VCL for baseline performance measurement

**Key concepts:**
- Shared VCL mode (default behavior)
- Performance optimization for multi-test files
- 10-100x faster than loading VCL per test

**Run it:**
```bash
vcltest examples/overhead.yaml
```

---

## Standalone VCL Files

These VCL files don't have matching YAML tests but are useful references:

- **`test.vcl`** - General-purpose VCL with admin blocking, API pass-through, static asset caching
- **`empty.vcl`** - Minimal VCL that always returns 200 OK
- **`time_test.vcl`** - Demonstrates time manipulation with `std.log()` and time headers
- **`test_with_include.vcl` / `included.vcl`** - VCL file inclusion example

## Backend Placeholders

VCLTest automatically replaces backend placeholders in your VCL:

### Single Backend
```vcl
backend default {
    .host = "__BACKEND_HOST__";
    .port = "__BACKEND_PORT__";
}
```

### Named Backends
```vcl
backend api_backend {
    .host = "__BACKEND_HOST_API_BACKEND__";
    .port = "__BACKEND_PORT_API_BACKEND__";
}

backend web_backend {
    .host = "__BACKEND_HOST_WEB_BACKEND__";
    .port = "__BACKEND_PORT_WEB_BACKEND__";
}
```

The pattern is: `__BACKEND_HOST_{UPPERCASE_BACKEND_NAME}__` and `__BACKEND_PORT_{UPPERCASE_BACKEND_NAME}__`

## Test File Structure

### Single-Request Test
```yaml
name: Test description
request:
  url: /path
backend:
  status: 200
  body: "response"
expect:
  status: 200
```

### Scenario-Based Test (with time)
```yaml
name: Cache test
scenario:
  - at: "0s"
    request:
      url: /article
    backend:
      status: 200
    expect:
      status: 200
      cached: false

  - at: "30s"
    request:
      url: /article
    expect:
      status: 200
      cached: true
```

### Multi-Backend Test
```yaml
name: Routing test
backends:
  api_backend:
    status: 200
    body: "api response"
  web_backend:
    status: 200
    body: "web response"

scenario:
  - at: "0s"
    request:
      url: /api/users
    expect:
      status: 200
      backend_used: "api_backend"
```

## VCL Resolution

VCL files are resolved automatically:

1. **CLI flag:** `vcltest -vcl custom.vcl tests.yaml` (highest priority)
2. **Same-named file:** `tests.yaml` → `tests.vcl` (auto-detected)
3. **Error:** If neither found

## Running Examples

Run a single example:
```bash
vcltest examples/basic.yaml
```

Run all examples:
```bash
vcltest examples/*.yaml
```

Run with custom VCL:
```bash
vcltest -vcl examples/custom.vcl examples/basic.yaml
```

## Tips

1. **Start simple** - Begin with `basic.yaml` to understand the structure
2. **Use multi-test files** - Separate tests with `---` for better organization
3. **Shared VCL mode** - Default behavior, much faster for multiple tests
4. **Time testing** - Install libfaketime for cache TTL and temporal tests
5. **Error visualization** - Intentionally fail a test to see VCL execution trace
6. **Backend placeholders** - Always use placeholders, never hardcode addresses

## Next Steps

After exploring these examples:

1. Read the main [README.md](../README.md) for full documentation
2. Check [CLAUDE.md](../CLAUDE.md) for architecture details
3. Write your own tests for your VCL configuration
4. Contribute new examples via pull request
