# VCLTest Future Development

Planned features and enhancements for VCLTest, organized by priority.

## Priority Levels

- **Must-Have**: Critical for real-world VCL testing
- **Should-Have**: Important features that significantly improve testing capabilities
- **Nice-to-Have**: Advanced features that expand coverage but aren't essential

---

## MUST-HAVE Features

### 1. Backend Failure Modes

**Status**: Implemented

Backend failure modes enable testing grace mode, error pages, and timeout handling.

**Usage**:
```yaml
backend:
  failure_mode: failed  # Connection reset immediately
  # or
  failure_mode: frozen  # Never responds (triggers timeout)
```

| Mode | Behavior | Varnish sees |
|------|----------|--------------|
| `failed` | Connection reset immediately | `FetchError` → `vcl_backend_error` |
| `frozen` | Never responds, hangs until timeout | `Timeout` → `vcl_backend_error` |

See `examples/backend-failure.yaml` for usage.

**Enables**:
- Grace mode testing
- Custom error page testing (`vcl_backend_error`)
- Timeout/retry testing

---

### 2. Grace Mode / Stale Content Delivery

**Status**: Implemented

Grace mode testing combines scenario-based time manipulation with backend failure modes to verify stale content delivery.

**Usage**:
```yaml
name: Grace mode - serve stale on backend failure
scenario:
  - at: "0s"
    request:
      url: /article
    backend:
      status: 200
      body: "Article content"
    expectations:
      cache:
        hit: false

  - at: "35s"  # TTL (30s) expired, backend fails
    request:
      url: /article
    backend:
      failure_mode: failed
    expectations:
      response:
        status: 200
      cache:
        stale: true
```

**VCL Requirements**:

The VCL must set `X-Varnish-Stale` header for reliable stale detection:
```vcl
sub vcl_backend_response {
    set beresp.ttl = 30s;
    set beresp.grace = 300s;  # Serve stale for 5min if backend down
}

sub vcl_deliver {
    if (obj.ttl < 0s) {
        set resp.http.X-Varnish-Stale = "1";
    }
}
```

See `examples/grace-mode.yaml` and `examples/grace-mode.vcl` for a complete working example.

**Detection Methods**:
- `X-Varnish-Stale` header (recommended, set by VCL)
- `Warning: 110` HTTP header (standard, but Varnish doesn't set automatically)

---

### 3. Backend Echo Mode

**Status**: Not implemented

Echo mode allows tests to verify what VCL sent to the backend (headers added/removed, URL rewrites).

**Implementation**:

Add to `BackendSpec`:
```go
EchoRequest bool `yaml:"echo_request,omitempty"`
```

Implement in `pkg/backend/mock.go`:
```go
func (m *MockBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
    if m.config.EchoRequest {
        echo := map[string]interface{}{
            "method":  r.Method,
            "url":     r.URL.String(),
            "headers": r.Header,
            "body":    readBody(r),
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(echo)
        return
    }
    // ... normal response
}
```

**Usage**:
```yaml
name: VCL adds X-Forwarded-Proto header
request:
  url: /api/data
backend:
  echo_request: true
expectations:
  response:
    status: 200
    body_contains: '"X-Forwarded-Proto"'
```

**Enables**:
- Request header manipulation testing
- URL rewrite verification
- Request body forwarding validation

---

## SHOULD-HAVE Features

### 4. Cookie Handling Examples & Helpers

**Status**: Works via headers, needs examples and optional helpers

**Current State**:
- Request cookies: `headers: {Cookie: "session=abc123"}`
- Response cookies: Assert on `Set-Cookie` header
- No dedicated examples

**Tasks**:

1. Create `examples/cookies.yaml` and `examples/cookies.vcl` demonstrating:
   - Setting request cookies
   - Cookie-based routing
   - Cookie stripping/manipulation
   - Validating `Set-Cookie` responses

2. (Optional) Add cookie assertion helpers to `ExpectationsSpec`:
   ```go
   CookiePresent string `yaml:"cookie_present,omitempty"`
   CookieAbsent  string `yaml:"cookie_absent,omitempty"`
   ```

---

### 5. Vary Header Examples

**Status**: Already works, needs examples

Vary headers affect cache key generation. The existing scenario-based tests support this pattern.

**Task**: Create `examples/vary-headers.yaml` and `examples/vary-headers.vcl`:
```yaml
name: Vary on Accept-Encoding
scenario:
  - at: "0s"
    request:
      url: /content
      headers:
        Accept-Encoding: "gzip"
    backend:
      status: 200
      headers:
        Vary: "Accept-Encoding"
    expectations:
      response:
        status: 200
      cache:
        hit: false

  - at: "1s"  # Different encoding = cache miss
    request:
      url: /content
      headers:
        Accept-Encoding: "br"
    expectations:
      cache:
        hit: false

  - at: "2s"  # Same encoding = cache hit
    request:
      url: /content
      headers:
        Accept-Encoding: "gzip"
    expectations:
      cache:
        hit: true
```

---

### 6. Custom Error Pages (vcl_synth, vcl_backend_error)

**Status**: Partially supported

**Current State**:
- `vcl_synth` responses already work (VCL can return synthetic responses)
- `vcl_backend_error` requires backend failure modes

**Tasks**:

1. Create `examples/error-pages.yaml` and `examples/error-pages.vcl` demonstrating:
   - Custom 403/404 pages via `vcl_synth`
   - Backend error handling via `vcl_backend_error`
   - Maintenance mode patterns

Example:
```yaml
name: Custom 403 page
request:
  url: /admin/users
expectations:
  response:
    status: 403
    body_contains: "Access Denied"
  backend:
    calls: 0  # Synthetic response, no backend call
```

**Dependencies**: Backend failure modes (for `vcl_backend_error` testing)

---

### 7. Header Manipulation Examples

**Status**: Response headers work, request headers need echo mode

**Tasks**:

Create `examples/header-manipulation.yaml` and `examples/header-manipulation.vcl` demonstrating:
- Adding request headers (X-Forwarded-*, X-Real-IP)
- Removing sensitive headers
- Response header modification (Cache-Control, CORS)

**Dependencies**: Backend echo mode (for request header verification)

---

## NICE-TO-HAVE Features

### 8. IP-Based Logic Testing

**Status**: Not supported

**Challenge**: Go's http.Client connects from 127.0.0.1. Cannot control source IP.

**Workaround**: Use `X-Forwarded-For` header and modify VCL to trust it for testing:
```go
type RequestSpec struct {
    // ...
    ClientIP string `yaml:"client_ip,omitempty"`  // Maps to X-Forwarded-For
}
```

**Limitation**: Not true `client.ip` testing. VCL must be modified to use the header.

**Recommendation**: Defer unless strong user demand. Document that IP-based VCL should be tested in staging/integration environments.

---

### 9. Compression Testing

**Status**: Not supported

**Implementation** (if needed):
- Add `Compress string` to BackendSpec ("gzip", "br", "deflate")
- Handle Go's auto-decompression behavior

**Recommendation**: Defer. Compression is better tested in integration environments.

---

### 10. ESI (Edge Side Includes) Testing

**Status**: Not planned

ESI requires multiple backend fragments and complex setup. Use VTest for ESI scenarios.

---

## Implementation Order

1. ~~**Backend failure modes**~~ ✅ Implemented
2. **Backend echo mode** (foundational - enables request verification)
3. ~~**Grace mode examples**~~ ✅ Implemented
4. **Error page examples**
5. **Cookie examples**
6. **Vary header examples**
7. **Header manipulation examples**
8. IP-based logic (if needed)
9. Compression (if needed)

---

## Example Files Needed

| File | Status |
|------|--------|
| `examples/grace-mode.{yaml,vcl}` | ✅ Implemented |
| `examples/error-pages.{yaml,vcl}` | Pending (backend failure modes ready) |
| `examples/cookies.{yaml,vcl}` | Pending |
| `examples/vary-headers.{yaml,vcl}` | Pending |
| `examples/header-manipulation.{yaml,vcl}` | Pending (needs backend echo mode) |

---

## Open Questions

1. **Backend echo format**: JSON (recommended for easy assertion matching)
2. **Grace detection**: Require VCL to set `X-Varnish-Stale` header (recommended)
3. **Cookie assertions**: String match initially, parsing later if needed

---

*Last updated: 2025-11-25*
