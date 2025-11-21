# VCLTest Implementation Plan

VCLTest is a testing framework for Varnish VCL that uses Varnish's built-in `vcl_trace` parameter for observability.

## Core Architecture

### Current Implementation (Phase 1 - Complete)

1. **Varnish Management** (`pkg/varnish`)
   - Start varnishd with `vcl_trace=on` parameter
   - No VCL instrumentation needed - Varnish logs execution automatically
   - Reusable across tests

2. **Varnishadm Server** (`pkg/varnishadm`)
   - CLI protocol server for varnishd management
   - VCL loading, parameter control, TLS cert management
   - Structured response parsing

3. **Service Orchestration** (`pkg/service`)
   - Coordinates varnishadm and varnishd startup
   - Ensures proper initialization order
   - Lifecycle management

4. **Log Recording** (`pkg/recorder`)
   - Captures varnishlog output to binary file
   - Parses VCL_trace entries showing executed lines
   - Counts BackendOpen events
   - Filters user VCL (config=0) from built-in VCL

5. **Test Specification** (`pkg/config`)
   - YAML-based test format
   - Multiple tests per file (YAML multi-document)
   - Simple assertions: status, backend_calls, headers, body_contains

### What Works Now

```yaml
name: API requests bypass cache
vcl: api.vcl

request:
  url: /api/users/123

backend:
  status: 200
  body: '{"user":"test"}'

expect:
  status: 200
  backend_calls: 1
```

Varnish's `vcl_trace` parameter automatically logs execution:
```
-   VCL_trace    vcl_recv:8
-   VCL_trace    vcl_recv:10
-   VCL_trace    vcl_recv:11
```

No VCL modification needed. Clean, simple, works.

## Phase 2: Temporal Testing

### The Problem

Cache testing requires time progression:

```yaml
# Request at T=0, cached with max-age=60
# Request at T=30s - should hit cache
# Request at T=70s - should be stale/revalidate
```

Waiting real time is impractical. Need time manipulation. Note that time must only advance forward, never backwards.

### Solution: libfaketime

**libfaketime** intercepts time syscalls via LD_PRELOAD. Works on Linux and macOS.

```bash
# Start varnishd with controlled time
echo '@2024-01-01 00:00:00' > /tmp/faketime.rc
faketime -f /tmp/faketime.rc varnishd ...

# Advance time for testing
echo '@2024-01-01 00:00:30' > /tmp/faketime.rc
```

### Architecture Changes

1. **pkg/varnish - Time Control**
   ```go
   type TimeConfig struct {
       Enabled     bool
       ControlFile string  // faketime control file path
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
       request:
         url: /article
       backend:
         status: 200
         headers:
           Cache-Control: "max-age=60, stale-while-revalidate=30"
       expect:
         status: 200
         cached: false

     - at: "30s"
       request:
         url: /article
       expect:
         status: 200
         cached: true
         age_lt: 35

     - at: "70s"
       backend_state: down

     - at: "71s"
       request:
         url: /article
       expect:
         status: 200
         stale: true
         body: original-content
   ```

3. **Test Runner - Scenario Execution**
   - Initialize time at T=0 for the first test.
   - For each scenario step:
     - Calculate target time from `at` field
     - Advance varnishd time via control file
     - Execute backend state change if specified
     - Execute request if specified
     - Verify expectations

4. **New Assertions**
   - `cached: true/false` - Check X-Varnish-Cache header
   - `age_lt: N` - Verify Age header is less than N
   - `age_gt: N` - Verify Age header is greater than N
   - `stale: true/false` - Check if stale content served
   - `backend_state: up/down/slow` - Control backend availability

### Implementation Tasks

1. **Detect libfaketime**
   - Check if `faketime` binary exists, fail if not
   
2. **Time Control File Management**
   - Create control file in workspace
   - Write timestamps in libfaketime format
   - Clean up on test completion

3. **Scenario Parser**
   - Parse `scenario` array from YAML
   - Validate `at` timestamps are monotonically increasing
   - Support duration syntax: "0s", "30s", "2m", "1h"

4. **Backend State Control**
   - Mock backend can simulate: down, slow, error states
   - `backend_state: down` - refuse connections
   - `backend_state: slow` - add delay
   - `backend_state: error` - return 5xx

5. **Cache-Aware Assertions**
   - Parse Varnish response headers (X-Varnish-Cache, Age, etc.)
   - Detect cache hits vs misses from log analysis
   - Verify TTL behavior

### Backwards Compatibility

Single-request tests (current format) continue to work unchanged:
```yaml
name: Simple test
vcl: test.vcl
request:
  url: /
expect:
  status: 200
```

Scenario tests use new format:
```yaml
name: Temporal test
vcl: test.vcl
scenario:
  - at: "0s"
    request: {...}
```

### Testing Strategy

1. **Unit Tests**
   - Time control file writing
   - Scenario parsing
   - Duration parsing

2. **Integration Tests**
   - Require libfaketime installed
   - Verify time actually advances in varnishd
   - Test cache TTL behavior
   - Test stale-while-revalidate

3. **Platform Testing**
   - Linux (primary)
   - macOS (secondary)
   - Document libfaketime installation

## Phase 3: Future Enhancements

Only add when needed:

- **Multiple backends** - Different mock backends per test
- **VCL flow assertions** - Assert specific execution paths
- **Variable tracking** - Inspect VCL variable values
- **Performance testing** - Load testing scenarios
- **HTML reports** - Rich test output
- **Parallel execution** - Run tests concurrently
- **Watch mode** - Auto-rerun on file changes

## Implementation Principles

1. **Simplicity** - Use Varnish's built-in features (vcl_trace), don't reinvent
2. **Minimal deps** - stdlib + yaml.v3, libfaketime for time control
3. **Clear errors** - Show VCL execution trace on failure
4. **Deterministic** - Isolated tests, controlled time, reproducible

## Success Criteria

**Phase 1** (Done):
- Single-request tests work
- VCL trace shows executed lines
- Clear pass/fail output

**Phase 2** (Temporal):
- Scenario-based tests work
- Time advances correctly
- Cache TTL tests pass
- Works on Linux and macOS with libfaketime

## Dependencies

- Varnish 6.0+ (for vcl_trace parameter)
- libfaketime (for temporal testing)
- Go 1.21+
- gopkg.in/yaml.v3

## Notes

The original plan called for VCL instrumentation with AST parsing. That's unnecessary - Varnish's `vcl_trace` parameter provides execution logging out of the box. Use built-in features when possible.

Temporal testing is the key missing piece. libfaketime is proven, cross-platform (Linux/macOS), and doesn't require kernel features. It's the pragmatic choice.
