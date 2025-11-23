# VCLTest Feature Development Plan

This document outlines planned features and enhancements for VCLTest, organized by priority and with detailed implementation plans.

## Feature Priority

- **Must-Have**: Critical for real-world VCL testing, needed soon
- **Should-Have**: Important features that significantly improve testing capabilities
- **Nice-to-Have**: Advanced features that expand coverage but aren't essential

---

## MUST-HAVE Features

### 1. POST/PUT with Request Bodies

**Status**: ✅ Already Implemented (needs examples & documentation)

**Current State**:
- `RequestSpec.Body` field exists (pkg/testspec/types.go:26)
- Client properly sends request bodies (pkg/client/client.go:26-28)
- No examples or documentation demonstrating this capability

**Implementation Tasks**:

1. **Add Examples** (Priority: High)
   - Create `examples/post-requests.yaml` and `examples/post-requests.vcl`
   - Demonstrate:
     - POST with JSON body
     - PUT with form data
     - Request body validation in VCL
     - Content-Type header handling
   - Example test structure:
     ```yaml
     name: POST JSON data
     request:
       method: POST
       url: /api/data
       headers:
         Content-Type: "application/json"
       body: '{"key": "value", "count": 42}'
     expect:
       status: 200
       body_contains: "received"
     ```

2. **Update Documentation** (Priority: High)
   - Add POST/PUT section to README.md
   - Document common patterns (JSON APIs, form submissions)
   - Note Content-Length is auto-calculated by Go's http.Client

3. **Consider Backend Echo Enhancement** (Priority: Medium)
   - Current backend mock returns static responses
   - For request body validation, may need backend to echo request details
   - Add optional "echo mode" to backend mock:
     ```go
     type Config struct {
         Status  int
         Headers map[string]string
         Body    string
         EchoRequest bool  // NEW: echo request body/headers in response
     }
     ```
   - Enables tests like:
     ```yaml
     backend:
       echo_request: true  # Response body contains request info
     expect:
       body_contains: '{"key": "value"}'  # Verify VCL forwarded body correctly
     ```

**Estimated Effort**: 2-4 hours (mostly examples & docs)

**Dependencies**: None

---

### 2. Cookie Handling

**Status**: ⚠️ Partially Supported (via headers, needs better support)

**Current State**:
- Request cookies: Set via `headers: {Cookie: "session=abc123"}`
- Response cookies: Can assert on `Set-Cookie` header
- No dedicated cookie helpers or examples
- Common VCL cookie patterns not documented

**Implementation Tasks**:

1. **Add Cookie Examples** (Priority: High)
   - Create `examples/cookies.yaml` and `examples/cookies.vcl`
   - Demonstrate:
     - Setting request cookies
     - Validating response Set-Cookie headers
     - Cookie-based routing in VCL
     - Session cookie vs persistent cookie
     - Cookie stripping/manipulation
   - Example VCL patterns:
     ```vcl
     # Strip tracking cookies before cache lookup
     if (req.http.Cookie ~ "session=") {
         set req.http.Cookie = regsuball(req.http.Cookie, "(^|;\s*)(_ga|_utm[a-z_]+)=[^;]*", "");
     }
     ```

2. **Add Cookie Assertion Helpers** (Priority: Medium)
   - Extend `ExpectSpec` with cookie-specific fields:
     ```go
     type ExpectSpec struct {
         // ... existing fields ...
         Cookies map[string]string `yaml:"cookies,omitempty"`  // Expected Set-Cookie values
         CookiePresent string       `yaml:"cookie_present,omitempty"`  // Cookie name must exist
         CookieAbsent  string       `yaml:"cookie_absent,omitempty"`   // Cookie name must not exist
     }
     ```
   - Implement parser in pkg/assertion/assertion.go to parse Set-Cookie headers
   - Example usage:
     ```yaml
     expect:
       cookies:
         session: "abc123; Path=/; HttpOnly"
       cookie_present: "session"
       cookie_absent: "tracking_id"
     ```

3. **Document Cookie Patterns** (Priority: Medium)
   - Add "Cookie Handling" section to README.md
   - Document common VCL cookie patterns:
     - Session cookie pass-through
     - Cookie normalization for cache keys
     - Cookie-based cache segmentation
     - Cookie security (HttpOnly, Secure flags)

4. **Advanced: Cookie Parsing in VCL** (Priority: Low)
   - Example showing how to parse multiple cookies from Cookie header
   - Testing VCL that extracts specific cookie values
   - May require backend echo mode to verify cookie forwarding

**Estimated Effort**: 6-8 hours

**Dependencies**: None (but enhanced by backend echo feature)

---

### 3. Grace Mode / Stale Content Delivery

**Status**: ⚠️ Partially Supported (stale assertion exists, needs grace testing)

**Current State**:
- `stale` assertion exists (pkg/testspec/types.go:46, pkg/assertion/assertion.go:136-142)
- Checks for X-Varnish-Stale header or Warning: 110
- No way to trigger backend failures in tests
- No grace period examples
- Scenario-based tests support time manipulation (good for TTL+grace)

**Grace Mode Background**:
```vcl
sub vcl_backend_response {
    set beresp.ttl = 60s;      # Fresh for 60s
    set beresp.grace = 300s;   # Serve stale for 5min if backend down
}
```

**Implementation Tasks**:

1. **Extend Backend Mock with Failure Modes** (Priority: High)
   - Current backend always succeeds
   - Add failure configuration to BackendSpec:
     ```go
     type BackendSpec struct {
         Status  int               `yaml:"status,omitempty"`
         Headers map[string]string `yaml:"headers,omitempty"`
         Body    string            `yaml:"body,omitempty"`
         Fail    bool              `yaml:"fail,omitempty"`     // Backend unreachable
         Timeout bool              `yaml:"timeout,omitempty"`  // Backend timeout
         Delay   string            `yaml:"delay,omitempty"`    // Response delay (e.g., "5s")
     }
     ```
   - Implement in pkg/backend/mock.go:
     ```go
     func (m *MockBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
         if m.config.Fail {
             // Close connection immediately (simulates unreachable backend)
             hj, _ := w.(http.Hijacker)
             conn, _, _ := hj.Hijack()
             conn.Close()
             return
         }
         if m.config.Timeout {
             // Never respond (client will timeout)
             time.Sleep(time.Hour)
             return
         }
         if m.config.Delay != "" {
             delay, _ := time.ParseDuration(m.config.Delay)
             time.Sleep(delay)
         }
         // ... normal response
     }
     ```

2. **Add Grace Mode Examples** (Priority: High)
   - Create `examples/grace-mode.yaml` and `examples/grace-mode.vcl`
   - Demonstrate:
     - Object cached with TTL + grace period
     - Backend failure triggers stale delivery
     - Testing both hit-for-miss and hit-for-pass with grace
   - Example test:
     ```yaml
     name: Grace mode - serve stale on backend failure
     scenario:
       - at: "0s"  # Initial request, cache object
         request:
           url: /article
         backend:
           status: 200
           headers:
             Cache-Control: "max-age=30"
           body: "Article v1"
         expect:
           status: 200
           cached: false

       - at: "60s"  # TTL expired, but backend fails
         request:
           url: /article
         backend:
           fail: true  # Backend unreachable
         expect:
           status: 200
           stale: true  # Served stale from grace
           body_contains: "Article v1"
     ```

3. **Enhance Stale Detection** (Priority: Medium)
   - Current implementation checks X-Varnish-Stale or Warning: 110
   - Standard Varnish doesn't set these by default in grace
   - Update assertion.go to detect grace delivery:
     ```go
     // Grace delivery indicators:
     // - Age > TTL (if we know TTL from Cache-Control)
     // - X-Varnish shows cache hit (reused VXID)
     // - Backend was unavailable (context from test)
     ```
   - May need to track expected TTL in test context
   - Or require VCL to set X-Varnish-Stale header when serving from grace:
     ```vcl
     sub vcl_deliver {
         if (obj.ttl < 0s) {
             set resp.http.X-Varnish-Stale = "1";
         }
     }
     ```

4. **Document Grace Testing** (Priority: High)
   - Add "Grace Mode Testing" section to README.md
   - Explain TTL vs grace period
   - Show how to test grace delivery
   - Document backend failure modes
   - Note that VCL should set X-Varnish-Stale for reliable detection

**Estimated Effort**: 8-12 hours

**Dependencies**:
- Time manipulation (already implemented via scenario tests)
- Backend failure modes (new)

**Technical Challenges**:
- Detecting grace delivery without VCL modification is unreliable
- Recommend documenting that tests should include VCL that sets X-Varnish-Stale

---

## SHOULD-HAVE Features

### 4. Vary Header Testing

**Status**: ✅ Already Supported (needs examples & documentation)

**Current State**:
- No special support needed
- Vary headers affect cache key generation
- Can already test with multiple requests and cache assertions
- No examples demonstrating Vary behavior

**Vary Header Background**:
```vcl
sub vcl_backend_response {
    set beresp.http.Vary = "Accept-Encoding, Accept-Language";
    # Creates separate cache entries per encoding/language combination
}
```

**Implementation Tasks**:

1. **Add Vary Header Examples** (Priority: High)
   - Create `examples/vary-headers.yaml` and `examples/vary-headers.vcl`
   - Demonstrate:
     - Same URL, different Accept-Encoding → separate cache entries
     - Same URL, different Accept-Language → separate cache entries
     - Vary on custom headers (X-Device-Type, etc.)
   - Example test:
     ```yaml
     name: Vary on Accept-Encoding
     scenario:
       - at: "0s"  # First request with gzip
         request:
           url: /content
           headers:
             Accept-Encoding: "gzip"
         backend:
           status: 200
           headers:
             Vary: "Accept-Encoding"
           body: "compressed content"
         expect:
           status: 200
           cached: false

       - at: "1s"  # Same URL, different encoding → cache miss
         request:
           url: /content
           headers:
             Accept-Encoding: "br"
         expect:
           status: 200
           cached: false  # Different Vary value = different cache object

       - at: "2s"  # Same URL, same encoding → cache hit
         request:
           url: /content
           headers:
             Accept-Encoding: "gzip"
         expect:
           status: 200
           cached: true  # Matches first request
     ```

2. **Document Vary Testing Patterns** (Priority: Medium)
   - Add "Testing Cache Variance" section to README.md
   - Explain how Vary affects cache keys
   - Show common Vary patterns (Accept-Encoding, User-Agent, etc.)
   - Document limitations (tests must manually vary request headers)

3. **Consider Vary-Specific Assertions** (Priority: Low)
   - Could add helpers to make Vary testing clearer:
     ```yaml
     expect:
       vary_header: "Accept-Encoding"  # Assert Vary header is set
       cache_key_includes: ["Accept-Encoding"]  # Document what affects cache key
     ```
   - Low priority since current approach works fine

**Estimated Effort**: 3-4 hours (mostly examples)

**Dependencies**: Scenario-based tests (already implemented)

---

### 5. Custom Error Pages (vcl_synth, vcl_backend_error)

**Status**: ❌ Not Supported (needs implementation)

**Current State**:
- No way to test synthetic responses
- No way to trigger specific error conditions
- No examples of error handling VCL

**VCL Error Handling Background**:
```vcl
sub vcl_recv {
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));  # Custom error page
    }
}

sub vcl_synth {
    set resp.http.Content-Type = "text/html";
    set resp.body = "<h1>Access Denied</h1>";
    return (deliver);
}

sub vcl_backend_error {
    set beresp.http.Content-Type = "text/html";
    synthetic("<h1>Backend Error</h1>");
    return (deliver);
}
```

**Implementation Tasks**:

1. **Backend Error Triggering** (Priority: High)
   - Already covered by grace mode backend failure implementation
   - Backend failures trigger vcl_backend_error
   - Example:
     ```yaml
     backend:
       fail: true  # Triggers vcl_backend_error
     expect:
       status: 503  # Or custom status from VCL
       body_contains: "Backend Error"
     ```

2. **Synthetic Response Testing** (Priority: High)
   - Add examples for vcl_synth usage
   - Create `examples/error-pages.yaml` and `examples/error-pages.vcl`
   - Demonstrate:
     - Custom 403 Forbidden page
     - Custom 404 Not Found page
     - Maintenance mode (return synth(503))
     - Rate limiting (return synth(429))
   - Example:
     ```yaml
     name: Custom 403 page
     request:
       url: /admin/users
     backend:
       status: 200  # Backend would respond OK, but VCL blocks it
     expect:
       status: 403
       headers:
         Content-Type: "text/html"
       body_contains: "Access Denied"
       backend_calls: 0  # Synthetic response, no backend call
     ```

3. **Document Error Handling** (Priority: Medium)
   - Add "Testing Error Pages" section to README.md
   - Explain vcl_synth vs vcl_backend_error
   - Show common error page patterns
   - Document how to test maintenance mode

**Estimated Effort**: 4-6 hours

**Dependencies**: Backend failure modes (from grace mode implementation)

---

### 6. Request/Response Modification Testing

**Status**: ⚠️ Partially Supported (needs examples & backend echo)

**Current State**:
- Can assert on response headers (already works)
- Cannot verify request modifications (no visibility into bereq)
- No examples showing header manipulation
- Backend echo mode would enable bereq validation

**VCL Modification Patterns**:
```vcl
sub vcl_recv {
    # Add custom header
    set req.http.X-Forwarded-Proto = "https";

    # Remove headers
    unset req.http.Cookie;

    # Normalize URL
    set req.url = regsub(req.url, "\?.*$", "");
}

sub vcl_backend_fetch {
    # Modify backend request
    set bereq.http.X-Client-IP = client.ip;
}

sub vcl_backend_response {
    # Add cache headers
    set beresp.http.Cache-Control = "public, max-age=300";

    # Remove backend headers
    unset beresp.http.Set-Cookie;
}
```

**Implementation Tasks**:

1. **Implement Backend Echo Mode** (Priority: High)
   - Extend backend mock to echo request details in response
   - Allow tests to verify what backend received:
     ```go
     type Config struct {
         // ... existing fields ...
         EchoRequest bool `yaml:"echo_request,omitempty"`  // Return request details in response
     }

     func (m *MockBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
         if m.config.EchoRequest {
             // Build JSON response with request details
             echo := map[string]interface{}{
                 "method": r.Method,
                 "url":    r.URL.String(),
                 "headers": r.Header,
                 "body":   readBody(r),
             }
             json.NewEncoder(w).Encode(echo)
             return
         }
         // ... normal response
     }
     ```
   - Usage:
     ```yaml
     backend:
       echo_request: true
     expect:
       body_contains: '"X-Client-IP": "127.0.0.1"'  # Verify VCL added header
     ```

2. **Add Header Manipulation Examples** (Priority: High)
   - Create `examples/header-manipulation.yaml` and `examples/header-manipulation.vcl`
   - Demonstrate:
     - Adding request headers (X-Forwarded-*, X-Real-IP)
     - Removing sensitive headers (Authorization, Cookie)
     - Response header modification (Cache-Control, CORS)
     - Header normalization
   - Example:
     ```yaml
     name: VCL adds X-Forwarded-Proto header
     request:
       url: /api/data
     backend:
       echo_request: true  # Backend echoes what it received
     expect:
       status: 200
       body_contains: '"X-Forwarded-Proto": "https"'
     ```

3. **Add URL Rewriting Examples** (Priority: Medium)
   - Test URL normalization, query string stripping, rewrites
   - Use backend echo to verify URL changes:
     ```yaml
     name: Strip query strings
     request:
       url: /page?tracking=123&session=abc
     backend:
       echo_request: true
     expect:
       body_contains: '"/page"'  # Query string removed
     ```

4. **Document Request/Response Modification** (Priority: Medium)
   - Add "Testing Header Manipulation" section to README.md
   - Explain bereq vs beresp modifications
   - Show how to use echo mode
   - Document common patterns

**Estimated Effort**: 6-8 hours

**Dependencies**: Backend echo mode (new feature)

---

## NICE-TO-HAVE Features

### 7. IP-Based Logic Testing

**Status**: ❌ Not Supported (requires test format extension)

**Current State**:
- No way to control client IP in tests
- VCL using `client.ip` cannot be tested properly
- Common use case: ACLs, rate limiting, geo-blocking

**VCL IP Patterns**:
```vcl
acl internal {
    "192.168.0.0"/16;
    "10.0.0.0"/8;
}

sub vcl_recv {
    if (client.ip ~ internal) {
        # Allow internal traffic
    } else {
        return (synth(403));
    }
}
```

**Implementation Challenges**:
- Go's http.Client connects from 127.0.0.1 (loopback)
- Cannot easily control source IP
- Options:
  1. **X-Forwarded-For approach**: VCL reads IP from header
  2. **Multiple network interfaces**: Complex, requires setup
  3. **VCL mock client.ip**: Not realistic

**Implementation Tasks** (if pursued):

1. **Test Format Extension** (Priority: Medium)
   - Add `client_ip` field to RequestSpec:
     ```go
     type RequestSpec struct {
         Method   string            `yaml:"method,omitempty"`
         URL      string            `yaml:"url"`
         Headers  map[string]string `yaml:"headers,omitempty"`
         Body     string            `yaml:"body,omitempty"`
         ClientIP string            `yaml:"client_ip,omitempty"`  // NEW: override client IP
     }
     ```
   - Map to X-Forwarded-For header in client
   - VCL must be written to trust X-Forwarded-For

2. **Add IP-Based Examples** (Priority: Low)
   - Create `examples/ip-acl.yaml` and `examples/ip-acl.vcl`
   - Demonstrate:
     - ACL testing (internal vs external IPs)
     - Rate limiting by IP
     - Geo-blocking simulation
   - Example:
     ```yaml
     name: Internal IP allowed
     request:
       url: /admin
       client_ip: "192.168.1.100"  # Internal IP
     expect:
       status: 200

     ---

     name: External IP blocked
     request:
       url: /admin
       client_ip: "203.0.113.45"  # External IP
     expect:
       status: 403
     ```
   - VCL must use X-Forwarded-For:
     ```vcl
     sub vcl_recv {
         # Trust X-Forwarded-For from test framework
         if (req.http.X-Forwarded-For) {
             # Parse IP from header for ACL check
         }
     }
     ```

3. **Document Limitations** (Priority: High if implemented)
   - Clearly state that IP testing uses X-Forwarded-For
   - VCL must be modified to read from header
   - Not suitable for testing real client.ip ACLs
   - Consider this a simulation/approximation

**Estimated Effort**: 6-10 hours

**Dependencies**: None

**Recommendation**:
- Lower priority due to complexity and limitations
- X-Forwarded-For approach is not true client.ip testing
- Consider deferring until there's strong user demand
- Alternative: Document that IP-based VCL should be tested in staging/integration

---

### 8. Compression Testing (gzip, brotli)

**Status**: ❌ Not Supported (requires significant work)

**Current State**:
- No compression handling in backend mock
- No decompression in client
- No examples

**VCL Compression Patterns**:
```vcl
sub vcl_backend_response {
    if (beresp.http.Content-Type ~ "text|javascript|json") {
        set beresp.do_gzip = true;
    }
}
```

**Implementation Challenges**:
- Backend must serve compressed content
- Client must handle compressed responses
- Tests must verify compression happened
- Multiple compression algorithms (gzip, brotli, etc.)

**Implementation Tasks** (if pursued):

1. **Backend Compression Support** (Priority: Medium)
   - Add compression config to BackendSpec:
     ```go
     type BackendSpec struct {
         // ... existing fields ...
         Compress string `yaml:"compress,omitempty"`  // "gzip", "br", "deflate"
     }
     ```
   - Implement in backend mock:
     ```go
     func (m *MockBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
         body := []byte(m.config.Body)

         if m.config.Compress == "gzip" {
             w.Header().Set("Content-Encoding", "gzip")
             gz := gzip.NewWriter(w)
             defer gz.Close()
             gz.Write(body)
         } else {
             w.Write(body)
         }
     }
     ```

2. **Client Decompression** (Priority: Medium)
   - Go's http.Client auto-decompresses if Accept-Encoding set
   - May need to disable auto-decompression to test VCL behavior
   - Or test that Content-Encoding header is present

3. **Add Compression Examples** (Priority: Low)
   - Create `examples/compression.yaml` and `examples/compression.vcl`
   - Demonstrate:
     - VCL enables compression for text content
     - Accept-Encoding negotiation
     - Compressed vs uncompressed responses
   - Example:
     ```yaml
     name: VCL compresses text content
     request:
       url: /large-page
       headers:
         Accept-Encoding: "gzip, br"
     backend:
       status: 200
       body: "Large HTML content..."
     expect:
       status: 200
       headers:
         Content-Encoding: "gzip"
     ```

**Estimated Effort**: 10-15 hours

**Dependencies**: None

**Recommendation**:
- Defer unless there's specific user need
- Compression is complex and may be better tested in integration
- Focus on more common use cases first

---

### 9. ESI (Edge Side Includes) Testing

**Status**: ❌ Not Supported (very complex)

**Current State**:
- ESI requires multiple backend fragments
- VCL must enable ESI processing
- Very advanced feature

**VCL ESI Pattern**:
```vcl
sub vcl_backend_response {
    if (bereq.url == "/page") {
        set beresp.do_esi = true;
    }
}

# Backend returns:
# <html>
#   <esi:include src="/fragment1"/>
#   <esi:include src="/fragment2"/>
# </html>
```

**Implementation Challenges**:
- Requires multiple mock backends for fragments
- Need to verify ESI processing happened
- Complex setup and validation
- Less common use case

**Recommendation**:
- **Defer indefinitely** - out of scope for phase 1-3
- ESI is advanced and better suited for integration testing
- VTest is better tool for ESI scenarios
- Only implement if users specifically request it

**Estimated Effort**: 20+ hours

---

## Implementation Roadmap

### Phase 1: Quick Wins (1-2 weeks)
Focus on features that are mostly implemented or require just examples/docs:

1. **POST/PUT bodies** - Add examples and docs (4h)
2. **Vary headers** - Add examples and docs (4h)
3. **Custom error pages** - Add examples (6h)

**Total**: ~14 hours, high user value

### Phase 2: Core Enhancements (2-3 weeks)
Features requiring moderate implementation work:

1. **Cookie handling** - Examples + assertion helpers (8h)
2. **Request/response modification** - Backend echo + examples (8h)
3. **Grace mode** - Backend failures + examples (12h)

**Total**: ~28 hours, critical for real-world testing

### Phase 3: Advanced Features (3-4 weeks, optional)
Nice-to-have features for comprehensive testing:

1. **IP-based logic** - Test format extension + examples (10h)
2. **Compression** - Backend/client updates + examples (15h)

**Total**: ~25 hours, lower priority

### Not Planned
- **ESI testing** - Too complex, recommend VTest for this use case

---

## Technical Dependencies

### Cross-Feature Dependencies

1. **Backend Echo Mode** enables:
   - Request/response modification testing
   - Request body validation
   - Header manipulation verification

2. **Backend Failure Modes** enable:
   - Grace mode testing
   - Custom error page testing
   - Timeout/retry testing

3. **Scenario Tests** (already implemented) enable:
   - Grace mode (time + failures)
   - Vary header testing (multiple requests)
   - Cache TTL testing (already working)

### Recommended Implementation Order

1. Backend echo mode (foundational)
2. Backend failure modes (foundational)
3. Examples for already-supported features (POST, Vary)
4. Cookie helpers and examples
5. Error page examples
6. Request/response modification examples
7. Grace mode examples
8. IP-based logic (if needed)
9. Compression (if needed)

---

## Testing Strategy

For each new feature:

1. **Unit tests** for new pkg code (backend echo, failure modes, assertions)
2. **Integration examples** in examples/ directory
3. **Documentation** in README.md
4. **Validation** - run all examples as regression tests

---

## Documentation Updates Needed

### README.md Sections to Add

1. **Advanced Request Features**
   - POST/PUT with bodies
   - Cookie handling
   - Custom headers

2. **Testing Cache Behavior**
   - Vary headers and cache keys
   - Grace mode and stale content
   - TTL testing (already covered)

3. **Testing VCL Logic**
   - Header manipulation
   - Request/response modification
   - Error pages and synthetic responses

4. **Backend Mock Features**
   - Echo mode for request validation
   - Failure modes for error testing
   - Delays for timeout testing

### New Example Files Needed

- `examples/post-requests.{yaml,vcl}` - POST/PUT bodies
- `examples/cookies.{yaml,vcl}` - Cookie handling
- `examples/grace-mode.{yaml,vcl}` - Stale content delivery
- `examples/vary-headers.{yaml,vcl}` - Cache variance
- `examples/error-pages.{yaml,vcl}` - Custom error pages
- `examples/header-manipulation.{yaml,vcl}` - Request/response mods
- `examples/ip-acl.{yaml,vcl}` - IP-based logic (optional)

---

## Success Metrics

### Phase 1 Success Criteria
- All must-have features have working examples
- Documentation covers 80% of common VCL patterns
- Users can test typical VCL without external docs

### Phase 2 Success Criteria
- Real-world VCL can be tested without workarounds
- Cookie, grace, and header manipulation fully supported
- Examples cover 90% of VCL use cases

### Phase 3 Success Criteria
- Advanced features (IP, compression) available if needed
- VCLTest competitive with VTest for unit testing
- Community adoption growing

---

## Open Questions

1. **Backend echo format**: JSON or custom text format?
   - Recommendation: JSON for easy parsing in assertions

2. **IP testing approach**: X-Forwarded-For or something else?
   - Recommendation: X-Forwarded-For, clearly document limitations

3. **Compression**: Auto-decompress or preserve encoding?
   - Recommendation: Make it configurable based on test needs

4. **Cookie assertions**: String match or parse attributes?
   - Recommendation: Start with string match, add parsing later if needed

5. **Grace detection**: Require VCL to set header or infer from Age?
   - Recommendation: Document that VCL should set X-Varnish-Stale header

---

## Notes

- This plan prioritizes **real-world VCL testing** over comprehensive feature coverage
- Features are ordered by **user impact** and **implementation effort**
- Some features (ESI, complex compression) may be better suited for VTest
- VCLTest focuses on **fast, clear unit tests** with great error messages
- Community feedback should drive priority adjustments

---

*Last updated: 2025-11-23*
*Status: Planning phase - not yet implemented*
