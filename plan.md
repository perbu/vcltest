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
- ✅ **libfaketime integration** - Time control for cache testing

## Phase 2: Temporal Testing (In Progress)

### The Problem

Cache testing requires time manipulation:

```yaml
# Request at T=0, cached with max-age=60
# Request at T=30s - should hit cache
# Request at T=70s - should be stale
```

Real-time waiting is impractical for CI/CD.

### Solution: libfaketime ✅ IMPLEMENTED

libfaketime integration is **COMPLETE**. It controls varnishd's perception of time using file modification timestamps.

**Implementation:**
- Time starts at real "now" when varnish starts
- Control file mtime manipulated via `os.Chtimes()`
- Environment variables inject libfaketime into varnishd process
- Platform-aware (Darwin: `DYLD_INSERT_LIBRARIES`, Linux: `LD_PRELOAD`)

### Current Time Control API

1. **pkg/varnish - Time Control** ✅ IMPLEMENTED
   ```go
   type TimeConfig struct {
       Enabled bool   // Enable faketime (default: false)
       LibPath string // Optional: override auto-detected library path
   }

   // Manager methods:
   func (m *Manager) AdvanceTimeBy(offset time.Duration) error
   func (m *Manager) GetCurrentFakeTime() time.Time
   ```

   **Usage:**
   ```go
   cfg := &varnish.Config{
       Varnish: varnish.VarnishConfig{
           Time: varnish.TimeConfig{Enabled: true},
       },
   }

   // Start varnish (t0 captured automatically)
   manager.Start(ctx, cmd, args, &cfg.Varnish.Time)

   // Advance to t+5s from test start
   manager.AdvanceTimeBy(5 * time.Second)

   // Advance to t+30s from test start (not +25s!)
   manager.AdvanceTimeBy(30 * time.Second)
   ```

   **Key Design:**
   - All time offsets are **absolute from test start (t0)**
   - `AdvanceTimeBy(30*time.Second)` means "30 seconds after test started"
   - No need to calculate elapsed time - framework handles it
   - Forward-only time movement (prevents varnishd panics)

### Remaining Work for Phase 2

1. **Test Spec - Scenario Format** (TODO)

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

2. **Runner Integration** (TODO)
   - Parse `scenario` section from YAML
   - Call `manager.AdvanceTimeBy()` between steps
   - Example: `at: "30s"` → `manager.AdvanceTimeBy(30 * time.Second)`

3. **New Assertions** (TODO)
   - `cached: true/false` - Check if response was cached (via X-Varnish, Age headers)
   - `age_lt: N` - Age header less than N seconds
   - `age_gt: N` - Age header greater than N seconds
   - `stale: true/false` - Check if stale content served (requires VCL support)

### Requirements ✅

- libfaketime installed (`apt install faketime` or `brew install libfaketime`)
- Linux or macOS (both supported)
- Auto-detects library path on both platforms

### Testing

Verified working with:
- Unit tests: `pkg/varnish/process_test.go`
- Integration test: `test_faketime_simple.sh`
- VCL sees fake time via `now` variable
- Time advances correctly via control file mtime manipulation

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

**Phase 2** (Temporal - In Progress):
- ✅ Time control infrastructure (libfaketime integration)
- ✅ `AdvanceTimeBy()` API with absolute offsets from t0
- ✅ Works on Linux and macOS
- ✅ Time advances correctly
- ⏳ Scenario-based YAML format
- ⏳ Runner integration with time advancement
- ⏳ Cache-specific assertions (cached, age_lt, age_gt, stale)
- ⏳ Cache TTL tests pass

**Phase 3** (Multi-Backend):
- Multiple backends per test
- Backend state control (up/down/slow)
- DNS resolution via resolv_wrapper
- Failover scenarios work

## Notes

**VCL Instrumentation:** The original plan called for VCL instrumentation with AST parsing. That proved unnecessary - Varnish's `feature=+trace` provides execution logging out of the box. Always prefer built-in features when possible.

**Time Control Implementation:** The libfaketime integration uses file modification timestamps (mtime) rather than writing time values to file contents. This proved simpler and more reliable than the original approach. Environment variable injection (`FAKETIME='%'` + `FAKETIME_FOLLOW_FILE`) allows dynamic time updates without restarting varnishd.

**Absolute Time Offsets:** Tests specify time as absolute offsets from test start (t0), not relative increments. This simplifies test logic - the test framework doesn't need to track elapsed time or calculate deltas. `AdvanceTimeBy(30*time.Second)` always means "30 seconds after test started", regardless of previous calls.
