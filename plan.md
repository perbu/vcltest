# VCLTest Future Plans

VCLTest is a testing framework for Varnish VCL that uses Varnish's built-in `feature=+trace` for observability.

## Current Status

Phase 1 is **COMPLETE**:
- ✅ YAML-based test specification
- ✅ Varnish lifecycle management (varnishd + varnishadm)
- ✅ VCL execution tracing via `feature=+trace`
- ✅ Colored terminal output with execution markers
- ✅ Mock backend support
- ✅ Basic assertions (status, backend_calls, headers, body_contains)

## Phase 2: Temporal Testing (Future)

### The Problem

Cache testing requires time manipulation:

```yaml
# Request at T=0, cached with max-age=60
# Request at T=30s - should hit cache
# Request at T=70s - should be stale
```

Real-time waiting is impractical for CI/CD.

### Solution: libfaketime

Use libfaketime to control varnishd's perception of time via LD_PRELOAD.

```bash
# Control time with a file
echo '@2024-01-01 00:00:00' > /tmp/faketime.rc
faketime -f /tmp/faketime.rc varnishd ...

# Advance time
echo '@2024-01-01 00:00:30' > /tmp/faketime.rc
```

### Architecture Changes

1. **pkg/varnish - Time Control**
   ```go
   type TimeConfig struct {
       Enabled     bool
       ControlFile string
       StartTime   time.Time
   }

   func (m *Manager) AdvanceTime(d time.Duration) error
   ```

2. **Test Spec - Scenario Format**
   ```yaml
   name: Cache TTL test
   vcl: cache.vcl

   scenario:
     - at: "0s"
       request: {url: /article}
       backend: {status: 200, headers: {Cache-Control: "max-age=60"}}
       expect: {status: 200, cached: false}

     - at: "30s"
       request: {url: /article}
       expect: {status: 200, cached: true, age_lt: 35}

     - at: "70s"
       request: {url: /article}
       expect: {status: 200, stale: true}
   ```

3. **New Assertions**
   - `cached: true/false` - Check if response was cached
   - `age_lt: N` - Age header less than N
   - `age_gt: N` - Age header greater than N
   - `stale: true/false` - Stale content served

### Requirements

- libfaketime installed (`apt install faketime` or `brew install libfaketime`)
- Linux or macOS

## Phase 3: Multi-Backend Support (Future)

### The Problem

Production VCLs use multiple backends for failover, load balancing, etc.

### Solution: resolv_wrapper + Loopback Aliases

```yaml
backends:
  origin1.example.com:8080:
    ip: "127.0.0.2"
    port: 8080
    responses:
      "/api": {status: 200, body: "origin1"}

  origin2.example.com:8080:
    ip: "127.0.0.3"
    port: 8080
    responses:
      "/api": {status: 200, body: "origin2"}

scenario:
  - at: "0s"
    request: {url: /api}
    expect: {backend_target: "origin1.example.com:8080"}

  - at: "5s"
    backend_action: {target: "origin1.example.com:8080", state: down}

  - at: "6s"
    request: {url: /api}
    expect: {backend_target: "origin2.example.com:8080"}  # Failover
```

### Requirements

- resolv_wrapper for DNS interception
- Loopback IP aliases (may need sudo/capabilities)

## Phase 4: Future Enhancements (Maybe)

Only add when actually needed:

- **VCL flow assertions** - Assert specific execution paths
- **Variable inspection** - Check VCL variable values
- **Performance testing** - Load testing scenarios
- **HTML reports** - Rich test output
- **Parallel execution** - Run tests concurrently
- **Watch mode** - Auto-rerun on file changes

## Implementation Principles

1. **Simplicity** - Use Varnish built-ins (feature=+trace), don't reinvent
2. **Minimal deps** - stdlib + yaml.v3, optional tools for advanced features
3. **Clear errors** - Show VCL execution trace on failure
4. **Deterministic** - Isolated tests, controlled time, reproducible results

## Success Criteria

**Phase 1** (✅ Done):
- Single-request tests work
- VCL trace shows executed lines with color
- Clear pass/fail output

**Phase 2** (Temporal):
- Scenario-based tests work
- Time advances correctly
- Cache TTL tests pass
- Works on Linux and macOS

**Phase 3** (Multi-Backend):
- Multiple backends per test
- Backend state control (up/down/slow)
- DNS resolution via resolv_wrapper
- Failover scenarios work

## Notes

The original plan called for VCL instrumentation with AST parsing. That proved unnecessary - Varnish's `feature=+trace` provides execution logging out of the box. Always prefer built-in features when possible.
