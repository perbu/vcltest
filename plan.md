# VCLTest Implementation Plan

This document outlines the phased implementation approach for VCLTest, a VCL testing framework that helps developers verify their Varnish Configuration Language code.

## Overview

VCLTest provides a declarative YAML-based testing framework for VCL with automatic instrumentation, execution tracing, and clear pass/fail results. The implementation is divided into three main phases, each with specific deliverables.

---

## Phase 1: Minimal Viable Product (MVP)

**Goal**: Create a functional single-test runner with core features and basic assertions.

### Deliverables

#### 1.1 Project Structure & Dependencies
**Tasks**:
- Set up Go module with `go.mod`
- Create directory structure:
  ```
  vcltest/
  ├── cmd/vcltest/main.go
  ├── pkg/
  │   ├── testspec/
  │   ├── instrument/
  │   ├── backend/
  │   ├── varnish/
  │   ├── runner/
  │   └── assertion/
  ├── examples/
  └── README.md
  ```
- Add dependencies:
  - `github.com/perbu/vclparser` for VCL parsing
  - `gopkg.in/yaml.v3` for YAML parsing
  - Standard library only for all other components

**Acceptance Criteria**:
- Project compiles successfully
- `go mod tidy` runs without errors
- Directory structure matches specification

#### 1.2 YAML Test Specification Parser
**File**: `pkg/testspec/spec.go`

**Tasks**:
- Define Go structs for test specification:
  - `TestSpec` (root structure)
  - `RequestSpec` (HTTP request configuration)
  - `BackendSpec` (mock backend configuration)
  - `ExpectSpec` (test assertions)
- Implement YAML unmarshaling
- Support multiple tests per file (YAML multi-document with `---` separator)
- Implement field defaults:
  - `request.method`: GET
  - `backend.status`: 200
  - `backend.headers`: empty map
  - `backend.body`: empty string

**Acceptance Criteria**:
- Parse valid YAML test files successfully
- Parse multiple tests from single YAML file
- Apply default values correctly
- Return clear error messages for invalid YAML
- Unit tests cover all struct fields and defaults

#### 1.3 VCL Instrumenter
**File**: `pkg/instrument/instrument.go`

**Tasks**:
- Use vclparser library to parse VCL into AST
- Implement AST visitor pattern for instrumentation
- Inject trace logs in format: `std.log("TRACE:<line>:<subroutine>")`
- Instrumentation points:
  - Before each statement in subroutines
  - Inside if/elsif/else branches
  - Before return statements
- Add `import std;` if not present
- Replace backend definitions with mock backend address (host:port)
- Preserve all comments and formatting where possible
- Keep line number references to original VCL

**Acceptance Criteria**:
- Successfully parse valid VCL 4.1 files
- Inject trace logs at all specified points
- Generated VCL is valid and parseable
- Backend replaced with mock address
- Line numbers in traces match original VCL
- Unit tests verify instrumentation correctness
- Handle edge cases: comments, strings, inline C blocks

#### 1.4 Mock Backend Server
**File**: `pkg/backend/mock.go`

**Tasks**:
- Implement simple HTTP server using `net/http`
- Accept `BackendConfig` with:
  - Status code
  - Headers map
  - Body string
- Listen on random available port (127.0.0.1:0)
- Return configured response to all requests
- Track number of requests received
- Provide graceful shutdown

**Acceptance Criteria**:
- Server starts on random port
- Returns configured status, headers, and body
- Counts requests accurately
- Shuts down cleanly
- Unit tests verify response configuration

#### 1.5 Varnish Process Manager
**File**: `pkg/varnish/manager.go`

**Tasks**:
- Start varnishd instance with unique name/workdir
- Load VCL configuration into varnishd
- Manage varnishd lifecycle (start/stop/cleanup)
- Reuse varnishd instance across multiple tests
- Handle cleanup on errors and shutdown

**Acceptance Criteria**:
- Successfully start varnishd
- Load instrumented VCL
- Clean shutdown and cleanup
- Handle errors gracefully
- Works with system-installed Varnish

#### 1.6 Varnishlog Parser
**File**: `pkg/varnish/log.go`

**Tasks**:
- Start varnishlog process for request capture
- Parse varnishlog output to extract:
  - `TRACE:line:sub` log entries
  - Backend connection events
- Count backend calls
- Track executed VCL line numbers

**Acceptance Criteria**:
- Capture varnishlog output
- Parse TRACE entries correctly
- Count backend calls accurately
- Handle varnishlog errors

#### 1.7 Test Runner
**File**: `pkg/runner/runner.go`

**Tasks**:
- Orchestrate test lifecycle:
  1. Parse test specification
  2. Instrument VCL
  3. Start mock backend
  4. Load VCL into varnishd
  5. Start varnishlog
  6. Execute HTTP request
  7. Collect response and logs
  8. Evaluate assertions
  9. Cleanup
- Execute single test from YAML file
- Execute multiple tests from YAML file (sequential)
- Measure test execution time
- Handle errors at each stage
- Clean up resources on failure

**Acceptance Criteria**:
- Successfully run complete test lifecycle
- Execute all tests in multi-document YAML file
- Cleanup on both success and failure
- Report execution timing
- Integration tests verify end-to-end flow

#### 1.8 Assertion Engine
**File**: `pkg/assertion/assertion.go`

**Tasks**:
- Implement assertion evaluators:
  - `status`: Exact match on HTTP status code (required)
  - `backend_calls`: Exact match on number of backend requests (optional)
  - `headers`: Exact match on specific header values (optional)
  - `body_contains`: Substring match in response body (optional)
- Generate clear error messages for failures
- Track which assertions passed/failed

**Acceptance Criteria**:
- All assertion types work correctly
- Clear error messages show expected vs actual
- Unit tests cover all assertion types
- Edge cases handled (missing headers, empty body, etc.)

#### 1.9 Output Formatting
**Tasks**:
- Implement passing test output:
  ```
  PASS: Test name (45ms)
  ```
- Implement failing test output showing:
  - Test name and execution time
  - Which assertion(s) failed
  - Expected vs actual values
  - VCL source with execution markers:
    - `*` for executed lines (green when color enabled)
    - `|` for non-executed lines
- Implement verbose mode (`-v` flag):
  - Show VCL execution for passing tests
  - Show backend call count
  - Show response details
- Color output:
  - Green for executed lines
  - Auto-detect terminal support
  - Support `--no-color` flag
  - Disable when output is piped
  - Respect `NO_COLOR` environment variable

**Acceptance Criteria**:
- Clean, readable output format
- Colors work in terminal
- Colors disabled appropriately
- Verbose mode shows additional details
- VCL execution trace is clear

#### 1.10 CLI Interface
**File**: `cmd/vcltest/main.go`

**Tasks**:
- Parse command-line arguments:
  - Test file/directory path
  - `-v` / `--verbose` flag
  - `--no-color` flag
- Run single test file
- Exit with code 0 on all pass, 1 on any failure
- Handle errors gracefully

**Acceptance Criteria**:
- Binary builds successfully
- Accepts test file path
- Flags work correctly
- Exit codes are correct
- Error messages are helpful

#### 1.11 Examples & Documentation
**Files**: `examples/`, `README.md`

**Tasks**:
- Create example VCL file (`examples/basic.vcl`)
- Create example test file (`examples/basic.yaml`)
- Write README.md with:
  - Quick start guide
  - Installation instructions
  - Usage examples
  - Test format documentation
  - Comparison with VTest2
- Ensure examples actually work

**Acceptance Criteria**:
- Examples run successfully
- README is clear and comprehensive
- Documentation matches implementation
- Examples demonstrate key features

### Phase 1 Exit Criteria

- [ ] Single test file execution works end-to-end
- [ ] Multiple tests in one file execute sequentially
- [ ] All four assertion types work correctly
- [ ] VCL instrumentation is accurate
- [ ] Output clearly shows pass/fail with VCL execution trace
- [ ] Color output works correctly
- [ ] Examples run successfully
- [ ] Unit tests achieve >80% code coverage
- [ ] Integration tests verify end-to-end functionality
- [ ] README documentation is complete

### Phase 1 Explicitly Excluded

The following features are **NOT** in Phase 1:
- Multiple backends per test
- Multiple requests per test
- VCL flow/path assertions
- Variable value assertions
- Timing/performance checks
- HTML reports
- Watch mode
- Parallel test execution
- Test discovery (auto-finding test files)

---

## Phase 2: Test Discovery & Batch Execution

**Goal**: Enable running multiple test files and provide comprehensive test reporting.

### Deliverables

#### 2.1 Test Discovery
**Tasks**:
- Auto-discover `*.yaml` files in directory
- Recursively search subdirectories
- Support glob patterns (e.g., `tests/**/*.yaml`)
- Filter out non-test YAML files

**Acceptance Criteria**:
- `vcltest tests/` finds all test files in directory
- Recursive discovery works
- Only valid test files are included

#### 2.2 Batch Test Runner
**Tasks**:
- Execute multiple test files sequentially
- Track results across all tests
- Handle failures gracefully (continue running remaining tests)
- Reuse varnishd instance across test files

**Acceptance Criteria**:
- All discovered tests execute
- Failures don't stop execution
- Varnishd reused efficiently

#### 2.3 Summary Report
**Tasks**:
- Show per-test results
- Show aggregate statistics:
  - Total tests run
  - Passed count
  - Failed count
  - Total execution time
- List failed tests at end
- Exit with appropriate code

**Acceptance Criteria**:
- Clear summary after all tests
- Failed tests are easy to identify
- Statistics are accurate
- Exit code reflects overall result

#### 2.4 CLI Enhancements
**Tasks**:
- Support directory arguments
- Support glob patterns
- Add `-h` / `--help` flag
- Add `--version` flag
- Better error messages for missing files

**Acceptance Criteria**:
- Help text is clear
- Version information displays correctly
- Directory execution works
- Error messages are helpful

### Phase 2 Exit Criteria

- [ ] Test discovery works for directories
- [ ] Batch execution runs all tests
- [ ] Summary report is clear and accurate
- [ ] Help and version flags work
- [ ] Documentation updated for new features

---

## Phase 3: Advanced Features (Future)

**Goal**: Add advanced features based on user feedback and real-world usage.

### Potential Deliverables

These features will be prioritized based on user requests:

#### 3.1 Advanced Assertions
- `body_matches`: Regex pattern matching on body
- `headers_exist`: Check for header presence (not just value)
- `cache_hit` / `cache_miss`: Verify cache behavior
- `ttl_min` / `ttl_max`: Verify TTL values
- `vcl_path`: Assert specific VCL execution path

#### 3.2 Multiple Backends
- Support multiple backend definitions in test spec
- Replace each backend in VCL with corresponding mock
- Track calls per backend

#### 3.3 Multi-Request Tests
- Support sequence of requests in single test
- Share state between requests (cookies, sessions)
- Verify cache behavior across requests

#### 3.4 Performance & Optimization
- Parallel test execution
- Configurable worker pool size
- Test execution timeout configuration
- Performance benchmarking mode

#### 3.5 Enhanced Reporting
- HTML report generation
- JUnit XML output for CI integration
- JSON output mode
- Test coverage reporting (which VCL lines ever executed)

#### 3.6 Developer Experience
- Watch mode (`--watch` flag)
- Interactive debugging mode
- VCL syntax validation before testing
- Better error messages with suggestions

#### 3.7 CI/CD Integration
- GitHub Actions template
- GitLab CI template
- Docker image for testing
- Pre-built binaries for major platforms

### Phase 3 Approach

Phase 3 features will be implemented on demand:
1. Gather user feedback from Phase 1 & 2 usage
2. Prioritize features based on actual needs
3. Implement highest-value features first
4. Maintain backward compatibility
5. Keep simplicity as core principle

---

## Implementation Principles

Throughout all phases, maintain these principles:

### Simplicity Over Flexibility
- YAML for human readability
- Minimal assertion types (start small, add as needed)
- Simple trace format
- Clear, concise error messages
- No complex abstractions

### Minimal Dependencies
- vclparser for VCL parsing
- gopkg.in/yaml.v3 for YAML
- Standard library for everything else
- No external services required

### Deterministic Testing
- Each test is isolated
- Mock backends are fully controlled
- No reliance on external services
- Tests are reproducible

### Clear Error Messages
- Show which assertion failed
- Show expected vs actual values
- Show which VCL lines executed
- Make debugging obvious

### Code Quality
- Readability over cleverness
- Comprehensive error handling with context
- Unit tests for all components
- Integration tests for workflows
- Examples that actually work

---

## Success Metrics

### Phase 1 Success
- Tool can run basic VCL tests end-to-end
- Output is clear and actionable
- Examples work out of the box
- Documentation is complete
- At least 3 external users successfully use the tool

### Phase 2 Success
- Can run full test suites
- CI/CD integration is straightforward
- Test discovery works reliably
- At least 10 external users adopt the tool

### Phase 3 Success
- Advanced features meet real user needs
- Tool is widely adopted in VCL development workflows
- Positive community feedback
- Active contribution from community

---

## Timeline Estimates

**Note**: These are rough estimates and will be adjusted based on actual progress.

- **Phase 1**: 2-3 weeks (MVP with core features)
- **Phase 2**: 1 week (test discovery and reporting)
- **Phase 3**: Ongoing (feature-by-feature based on demand)

---

## Risk Mitigation

### Technical Risks
- **vclparser compatibility**: Mitigated by using well-tested library
- **varnishd version differences**: Document supported versions, test against multiple versions
- **Platform differences**: Start with Linux, add macOS/Windows support later
- **Performance with large VCL files**: Profile and optimize in Phase 3 if needed

### Scope Risks
- **Feature creep**: Stick to phased approach, resist adding features prematurely
- **Over-engineering**: Follow "simplicity first" principle
- **Compatibility concerns**: Version the tool, document breaking changes

### User Adoption Risks
- **Poor documentation**: Invest heavily in examples and README
- **Unclear error messages**: Prioritize message clarity from day 1
- **Complex setup**: Make installation and first test as simple as possible

---

## Next Steps

1. ✅ Review and approve this implementation plan
2. Set up project structure (Deliverable 1.1)
3. Begin Phase 1 implementation in order of deliverables
4. Create tracking issues for each deliverable
5. Set up CI/CD for automated testing
6. Regular progress reviews after each deliverable
