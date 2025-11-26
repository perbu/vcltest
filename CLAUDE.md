# VCLTest Package Documentation

This document provides terse descriptions of key packages in VCLTest. For overall project goals and usage,
see [README.md](README.md). For a quick 2-4 line overview of each package, see [pkg/README.md](pkg/README.md).

## pkg/varnish

Manages the varnishd process lifecycle and time control for temporal testing.

**Key types:**

- `Manager` - Controls varnishd startup, workspace preparation, process monitoring, and time manipulation
- `Config` - Configuration for varnish command-line arguments (ports, storage, parameters)
- `TimeConfig` - Configuration for libfaketime integration (enabled, lib path)

**Main operations:**

- `New()` - Creates manager with work directory and logger
- `PrepareWorkspace()` - Sets up directories, secret file, and license file
- `Start()` - Starts varnishd process with given arguments and blocks until exit
- `BuildArgs()` - Constructs varnishd command-line from Config struct
- `AdvanceTimeBy(offset)` - Advances fake time to testStartTime + offset (absolute, not relative)
- `GetCurrentFakeTime()` - Returns current fake time from control file mtime

**Responsibilities:**

- Directory setup with proper permissions
- Secret generation for varnishadm authentication
- License file handling for Varnish Enterprise
- Command-line argument construction (including `-p feature=+trace`)
- Process output routing to structured logs
- libfaketime integration for time manipulation (Phase 2)
- Control file creation and mtime manipulation for time advancement

## pkg/varnishadm

Implements the varnishadm server protocol and command interface.

**Key types:**

- `Server` - Listens for varnishd connections and handles CLI protocol
- `VarnishadmInterface` - Command interface for Varnish management operations
- `VCLListResult` - Parsed VCL list with entries and active VCL
- `VCLShowResult` - Parsed VCL source with config ID to filename mapping
- `TLSCertListResult` - Parsed TLS certificate list

**Protocol details:**

- Implements Varnish CLI wire protocol (status code + length + payload)
- Challenge-response authentication using SHA256
- Request/response pattern over TCP connection

**Main operations:**

- `New()` - Creates server with port, secret, and logger
- `Run()` - Starts server and accepts connections (blocks)
- `Exec()` - Executes arbitrary varnishadm commands
- VCL commands: `VCLLoad()`, `VCLUse()`, `VCLDiscard()`, `VCLList()`, `VCLListStructured()`
- `VCLShow()` / `VCLShowStructured()` - Show VCL source with config ID to filename mapping
- Parameter commands: `ParamShow()`, `ParamSet()`
- TLS commands: `TLSCertLoad()`, `TLSCertList()`, `TLSCertCommit()`, etc.
- `Start()` - Starts the varnish child/cache process

**Responsibilities:**

- Listen for varnishd connections on specified port
- Authenticate varnishd using shared secret
- Parse CLI protocol messages
- Execute commands and return structured responses
- Parse complex responses (VCL list, VCL show with config mapping, TLS cert list)
- Provide config ID to filename mapping for trace log analysis

## pkg/service

Orchestrates startup and lifecycle of varnishadm server and varnish daemon. Implements TimeController interface for temporal testing.

**Key types:**

- `Manager` - Coordinates both services with proper initialization order, implements TimeController
- `Config` - Combined configuration for both varnishadm and varnish

**Startup sequence:**

1. Start varnishadm server (runs in background)
2. Prepare varnish workspace
3. Build varnish arguments (includes faketime setup if enabled)
4. Start varnish daemon (connects to varnishadm, child auto-starts)
5. Monitor both services until failure or context cancellation

**Main operations:**

- `NewManager()` - Creates orchestrator with validation
- `Start()` - Starts both services and blocks until error or shutdown
- `GetVarnishadm()` - Returns interface for issuing varnishadm commands
- `GetVarnishManager()` - Returns varnish manager instance
- `AdvanceTimeBy(offset)` - Delegates to varnish Manager for time control (TimeController interface)

**Responsibilities:**

- Ensure varnishadm is listening before starting varnish
- Handle errors from either service
- Graceful shutdown via context cancellation
- Provide unified interface for service management
- Provide TimeController interface for scenario-based tests (Phase 2)

## pkg/recorder

Records and parses varnishlog output for VCL execution analysis.

**Key types:**

- `Recorder` - Manages varnishlog recording lifecycle
- `Message` - Structured log message with type and parsed fields
- `VCLTrace` - Parsed VCL_trace entry (config, line, column)
- `BackendCall` - Parsed BackendOpen entry
- `VCLTraceSummary` - Execution summary (lines, backend calls, VCL flow)

**Main operations:**

- `New()` - Creates recorder with work directory and logger
- `Start()` - Begins recording (varnishlog -g request, stdout to file)
- `Stop()` - Gracefully stops recording (sends SIGINT)
- `GetMessages()` - Reads log file and returns parsed messages
- `GetVCLMessages()` - Filters for VCL-related messages only
- `GetTraceSummary()` - Returns execution summary with line numbers and backend count

**Parsing functions:**

- `ParseVCLTrace()` - Extracts config/line/column from trace messages
- `ParseBackendCall()` - Extracts backend connection details
- `GetExecutedLines()` - Returns unique line numbers from user VCL (filters built-in)
- `CountBackendCalls()` - Counts BackendOpen entries

**Responsibilities:**

- Capture varnishlog output during test execution
- Parse varnishlog text format
- Filter user VCL traces (config=0) from built-in VCL
- Provide structured access to VCL execution data
- Support single recording at a time

## pkg/formatter

Formats VCL source code with execution trace visualization for terminal output.

**Key types:**

- None (pure functions)

**Main operations:**

- `FormatVCLWithTrace()` - Formats VCL with checkmarks on executed lines
- `FormatTestFailure()` - Complete failure output with VCL trace
- `ShouldUseColor()` - Detects if color output is appropriate

**Color scheme:**

- Green ✓ - Executed lines
- Gray/dimmed - Non-executed lines
- Red ✗ - Failed assertions
- Yellow - Section headers
- Bold - Emphasis

**Responsibilities:**

- Generate colored terminal output using ANSI escape codes
- Format VCL source with line numbers and execution markers
- Create readable error messages with context
- Provide plain text fallback when color not supported

## pkg/testspec

Parses YAML test specification files. Supports both single-request and scenario-based temporal tests.

**Key types:**

- `TestSpec` - Complete test specification (name, request/backends/expectations OR scenario) - VCL is resolved separately
- `ScenarioStep` - Single step in temporal test scenario (at, request, backends, expectations)
- `RequestSpec` - HTTP request definition (method, URL, headers, body)
- `BackendSpec` - Mock backend response (status, headers, body, failure_mode)
- `ExpectationsSpec` - Nested test expectations structure containing:
  - `ResponseExpectations` - Response validation (status, headers, body_contains)
  - `BackendExpectations` - Backend interaction (calls, used)
  - `CacheExpectations` - Cache behavior (hit, age_lt, age_gt)

**Backend specification:**

- Use `backends:` (plural) with named backends map - there is no singular `backend:` field
- Each backend needs a name (e.g., `default`, `api`, `web`)
- For single-backend tests, use `backends: { default: { ... } }`
- Scenario steps can override backends with step-level `backends:` map

**Main operations:**

- `Load()` - Loads and parses YAML file(s), returns slice of TestSpec
- `ApplyDefaults()` - Sets default values for optional fields (handles both test types)
- `IsScenario()` - Returns true if test is scenario-based
- `ResolveVCL()` - Determines VCL file path (priority: CLI flag, then same-named .vcl file)

**Responsibilities:**

- Parse YAML test files (supports multi-document YAML with `---`)
- Validate test specification structure (single-request OR scenario)
- Apply sensible defaults (GET method, 404 backend status, 200 expected response status)
- Resolve VCL file path from CLI or test file name
- Support cache-specific assertions (cached, age_lt, age_gt) - Phase 2
- Support scenario-based temporal tests with time offsets - Phase 2
- Return structured test definitions for runner

**VCL Resolution:**

VCL is no longer specified in test YAML files. Instead:
1. Use `-vcl <path>` CLI flag (highest priority)
2. Auto-detect same-named .vcl file (e.g., `tests.yaml` → `tests.vcl`)
3. Error if neither found

## pkg/runner

Orchestrates test execution and coordinates all components. Supports both single-request and scenario-based tests with shared VCL.

**Key types:**

- `Runner` - Test executor (has varnishadm, varnishURL, workDir, logger, timeController, loaded VCL state)
- `TestResult` - Result of a single test (passed, errors, VCL trace, source)
- `VCLTraceInfo` - Execution trace data (executed lines, backend calls, VCL flow)
- `TimeController` - Interface for time manipulation (implemented by service.Manager)

**Main operations:**

- `New()` - Creates runner with varnishadm interface, varnish URL, workDir, logger
- `SetTimeController()` - Sets time controller for scenario-based tests
- `replaceBackendsInVCL()` - Replaces backends using AST parser (vclmod)
- `LoadVCL()` - Loads VCL once with backend addresses replaced, stores for reuse
- `UnloadVCL()` - Cleans up shared VCL
- `RunTestWithSharedVCL()` - Executes test using pre-loaded shared VCL (preferred)
- `RunTest()` - Legacy method that loads VCL per test (for compatibility)

**Shared VCL approach (new):**

1. VCL is loaded once for all tests in a file
2. Backend mock servers started once with unified configuration
3. Each test executes against the same VCL and backends
4. Significantly faster for multiple tests (10-100x for large VCL files)
5. Trade-off: VCL state leaks between tests, backend responses cannot vary per test

**Test execution flow (shared VCL):**

1. Resolve VCL path via ResolveVCL()
2. Start all backend mock servers needed across all tests (once)
3. Load VCL once via LoadVCL() with backend addresses
4. For each test:
   - Make HTTP request through Varnish
   - Check assertions (backend_calls not supported in shared mode)
   - Return TestResult with trace data (on failure)
5. Unload shared VCL and stop backends

**Test execution flow (scenario with shared VCL):**

1. Load VCL once (already loaded)
2. For each scenario step:
   - Advance time to step's offset (absolute from t0)
   - Make HTTP request
   - Check assertions
3. Collect errors from all steps
4. Return TestResult with trace data (on failure)

**Responsibilities:**

- Coordinate all test components
- Replace backend addresses in VCL using AST-based modification
- Manage shared VCL lifecycle for performance
- Execute scenario-based temporal tests with time advancement (Phase 2)
- Collect VCL execution traces on test failure
- Provide detailed error information with trace context
- Validate backend names and log warnings for unused backends

**Limitations:**

- Backend call counts not tracked in shared VCL mode (accumulate across tests)
- VCL state may leak between tests (cache, variables, etc.)
- Backend responses are uniform across all tests in a file
- Tests should be designed to be order-independent when possible

## pkg/backend

Simple HTTP mock backend server for test responses.

**Key types:**

- `Server` - HTTP server with configurable response
- `Config` - Response configuration (status, headers, body)

**Main operations:**

- `New()` - Creates backend with config
- `Start()` - Starts HTTP server on random port, returns address
- `Stop()` - Stops server
- `GetCallCount()` - Returns number of requests received

**Responsibilities:**

- Serve deterministic HTTP responses for tests
- Count backend requests (for backend_calls assertion)
- Auto-select available port
- Simple, predictable behavior

## pkg/client

HTTP client for making test requests to Varnish.

**Key types:**

- `Response` - HTTP response (status, headers, body)

**Main operations:**

- `MakeRequest()` - Makes HTTP request with given spec, returns Response

**Responsibilities:**

- Execute HTTP requests against Varnish
- Support custom methods, headers, body
- Return structured response for assertion checking
- Handle connection errors gracefully

## pkg/assertion

Validates test expectations against actual results. Supports cache-specific assertions (Phase 2).

**Key types:**

- `Result` - Assertion result (passed, errors list)

**Main operations:**

- `Check()` - Validates ExpectationsSpec against Response and backend call count

**Supported assertions:**

Response expectations (required):
- Status code - Exact match
- Headers - Key-value exact match
- Body contains - Substring match

Backend expectations (optional):
- Calls - Count match
- Used - Verify which backend was called

Cache expectations (optional):
- Hit - Cache hit/miss detection via X-Varnish header
- Age less than - Age header < N seconds
- Age greater than - Age header > N seconds

**Helper functions:**

- `checkIfCached()` - Detects cache hits using X-Varnish header format and Age header

**Responsibilities:**

- Compare expected vs actual results
- Generate clear error messages for failures
- Return structured result with all failures
- Cache-aware assertions for temporal testing (Phase 2)
- Simple, straightforward validation logic

## pkg/vclmod

AST-based VCL backend modification using vclparser. Enables testing production VCL files without modification.

**Key types:**

- `BackendAddress` - Backend host and port configuration
- `ValidationResult` - Contains warnings and errors from backend validation

**Main operations:**

- `ValidateBackends()` - Validates YAML backends exist in VCL, warns about unused VCL backends
- `ModifyBackends()` - Parses VCL AST, replaces backend .host and .port, returns modified VCL
- `findClosestMatch()` - Suggests similar backend names for typo detection

**Backend Mapping:**

- **YAML backends MUST have explicit names** matching VCL backend declarations
- **YAML backend not in VCL** → FATAL ERROR (prevents typos)
- **VCL backend not in YAML** → WARNING (may be unused in this test)
- **Always overrides both .host and .port** in VCL (ignores original port)

**Validation:**

- Parses VCL using vclparser to extract backend declarations
- Checks all YAML backends exist in VCL (with helpful suggestions for typos)
- Warns about VCL backends not used in test (informational)
- Returns detailed error messages with available backends listed

**Modification:**

- Parses VCL to AST using vclparser
- Walks AST to find BackendDecl nodes
- Modifies .host and .port properties (creates if missing)
- Renders modified AST back to VCL preserving comments and structure
- Backends not in YAML remain unchanged

**Error Messages:**

Example error:
```
FATAL: Backend 'api_server' defined in test YAML not found in VCL
  VCL file: production.vcl
  Available backends in VCL: [api, web, cache]
  Did you mean 'api'?
```

Example warning:
```
WARNING: Backend 'legacy_backend' defined in VCL not used in test
  This backend will not be overridden. Test may fail if VCL tries to use it.
```

**Responsibilities:**

- Parse VCL files using vclparser for AST-based modification
- Validate backend names match between YAML and VCL
- Modify backend addresses while preserving VCL structure and comments
- Provide clear error messages with suggestions for mismatched backends
- Enable testing production VCL files with real hostnames

## pkg/vcl

Utility functions for VCL testing.

**Key types:**

- `BackendAddress` - Backend host and port configuration

**Main operations:**

- `ParseAddress()` - Parses "host:port" string into components

**Responsibilities:**

- Provide utility types and functions for VCL testing
- Parse network addresses for backend configuration

## pkg/cache

Event-driven cache process starter (listens for VCL load events).

**Key types:**

- `Starter` - Listens for EventVCLLoaded and starts cache process

**Main operations:**

- `New()` - Creates starter with varnishadm, event broker, logger
- `Start()` - Begins listening for events
- `GetVCLMapping()` - Returns stored VCL config mapping

**Responsibilities:**

- React to VCL load events
- Issue `start` command to varnishadm
- Publish EventCacheStarted when ready
- Store VCL config mapping for trace analysis

**Note:** In current implementation, the child process starts automatically when varnishd is launched with a VCL file, so explicit start may not be needed.

## pkg/events

Event definitions for event-driven architecture (used by pkg/cache and pkg/vcl).

**Key types:**

- Various event structs for lifecycle coordination
- EventVCLLoaded, EventCacheStarted, EventReady, etc.

**Responsibilities:**

- Define event types for pub/sub communication
- Enable decoupled service coordination
- Support event-driven startup sequence
