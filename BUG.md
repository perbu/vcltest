# Bug: VCL Synth Responses Not Working Correctly [RESOLVED]

## Status: FIXED ✅

This bug has been resolved. It had two root causes:

1. **VCL Issue**: Incorrect understanding of Varnish state machine
2. **Client Issue**: HTTP client automatically following redirects

## Root Causes

### 1. VCL State Machine Misconception

**Problem**: The example VCL was setting response headers in `vcl_deliver`, but synthetic responses never reach `vcl_deliver`.

**Varnish Behavior**: When `vcl_recv` returns `synth()`, the request flow is:
```
RECV → SYNTH → (direct to client, bypassing DELIVER)
```

**NOT**:
```
RECV → SYNTH → DELIVER  ❌ This never happens!
```

**Key Facts**:
- `vcl_deliver` is ONLY called for backend responses and cached objects
- Synthetic responses bypass `vcl_deliver` entirely
- All response manipulation for synthetic responses must happen in `vcl_synth`
- Response body must be set using `synthetic("body text")` in `vcl_synth`
- The second parameter to `synth(status, reason)` is the HTTP reason phrase, NOT the body

**Fixed in**: `examples/basic.vcl`

### 2. HTTP Client Automatically Following Redirects

**Problem**: Go's default `http.Client` automatically follows HTTP redirects (301, 302, etc.).

**Impact**: When testing redirect responses:
- Varnish correctly returns 301 with Location header
- Test client automatically follows the redirect
- Test sees the final destination response (often 200) instead of the redirect (301)
- Tests fail thinking Varnish returned the wrong status

**Fixed in**: `pkg/client/client.go` by setting `CheckRedirect` to return `http.ErrUseLastResponse`

## Investigation Process

The investigation revealed several helpful techniques:

1. **Confirmed VCL validity**: Used `varnishtest` to verify the VCL itself was valid
2. **Isolated the problem**: Tested individual components to narrow down the issue
3. **Read Varnish logs**: Examined varnishlog output to understand actual request flow
4. **Traced code execution**: VCL trace showed which lines executed, confirming logic paths

## Lessons Learned

1. **Varnish State Machine**: Understanding when each VCL subroutine is called is critical
   - Synthetic responses: `vcl_recv` → `vcl_synth` (no `vcl_deliver`)
   - Backend responses: `vcl_recv` → `vcl_backend_response` → `vcl_deliver`
   - Cached responses: `vcl_recv` → `vcl_deliver`

2. **HTTP Client Defaults**: When testing HTTP behavior, be aware of client-side automatic behaviors:
   - Redirect following (301, 302, 303, 307, 308)
   - Automatic decompression (gzip, deflate)
   - Connection pooling and keep-alive
   - TLS verification

3. **Testing Tools**: `varnishtest` is invaluable for validating VCL behavior in isolation

## Verification

All tests now pass:
```bash
$ ./vcltest examples/basic.yaml
Test 1: Health check endpoint
  ✓ PASSED
Test 2: Backend pass-through
  ✓ PASSED
Test 3: Redirect handling
  ✓ PASSED

Tests passed: 3/3
```

## Related Documentation

- [Varnish VCL State Machine](https://varnish-cache.org/docs/trunk/users-guide/vcl-built-in-subs.html)
- [VCL synth() function](https://varnish-cache.org/docs/trunk/users-guide/vcl-built-in-subs.html#vcl-synth)
- [Go http.Client redirect handling](https://pkg.go.dev/net/http#Client)
