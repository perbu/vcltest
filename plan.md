# VCLTest Future Plans

VCLTest is a testing framework for Varnish VCL that uses Varnish's built-in `feature=+trace` for observability.

## Current Status

**Phase 1** ✅ COMPLETE - Basic VCL testing framework
**Phase 2** ✅ COMPLETE - Temporal testing with cache TTL support
**Phase 3** ✅ COMPLETE - Multi-backend support for testing VCL routing

See [README.md](README.md) for current features and capabilities.

## Future Enhancements (Maybe)

These features are tracked but not prioritized. Only implement when actually needed:

### VCL Flow Assertions

Assert specific execution paths:

```yaml
expect:
  status: 200
  vcl_flow:
    - recv: pass
    - backend_response: deliver
```

### Variable Inspection

Check VCL variable values:

```yaml
expect:
  status: 200
  vcl_variables:
    req.http.X-Custom: "expected-value"
    bereq.url: "/modified-url"
```

### Performance Testing

Load testing scenarios:

```yaml
performance:
  duration: "30s"
  requests_per_second: 100
  expect:
    p50_latency_ms: 10
    p95_latency_ms: 50
    error_rate: 0.01
```

### HTML Reports

Rich test output with:
- Interactive VCL execution visualization
- Coverage metrics
- Historical trend tracking
- Export to CI/CD systems

### Parallel Execution

Run tests concurrently:
- Isolated Varnish instances per test
- Faster test suite execution
- Resource pooling for efficiency

### Watch Mode

Auto-rerun on file changes:
```bash
vcltest --watch examples/*.yaml
```

## Implementation Principles

1. **Simplicity** - Use Varnish built-ins (feature=+trace), don't reinvent
2. **Minimal deps** - stdlib + yaml.v3, optional tools for advanced features
3. **Clear errors** - Show VCL execution trace on failure
4. **Deterministic** - Isolated tests, controlled time, reproducible results
5. **No over-engineering** - Only add features when actually needed

## Notes

**VCL Instrumentation:** The original plan called for VCL instrumentation with AST parsing. That proved unnecessary - Varnish's `feature=+trace` provides execution logging out of the box. Always prefer built-in features when possible.

**Time Control Implementation:** The libfaketime integration uses file modification timestamps (mtime) rather than writing time values to file contents. This proved simpler and more reliable than the original approach. Environment variable injection (`FAKETIME='%'` + `FAKETIME_FOLLOW_FILE`) allows dynamic time updates without restarting varnishd.

**Absolute Time Offsets:** Tests specify time as absolute offsets from test start (t0), not relative increments. This simplifies test logic - the test framework doesn't need to track elapsed time or calculate deltas. `AdvanceTimeBy(30*time.Second)` always means "30 seconds after test started", regardless of previous calls.

**Multi-Backend Design:** The multi-backend implementation uses explicit named placeholders (`__BACKEND_HOST_BACKENDNAME__`) rather than attempting VCL parsing. Multiple mock backends run on different localhost ports. The varnishlog `-g request` flag groups backend connections with requests, allowing `BackendOpen` messages to be correlated with requests. Backend health/probe simulation was explicitly excluded - the goal is testing VCL routing semantics, not infrastructure failure scenarios.
