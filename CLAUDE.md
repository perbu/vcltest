# VCLTest Package Documentation

This document provides terse descriptions of key packages in VCLTest. For overall project goals and usage,
see [README.md](README.md).

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

- `TestSpec` - Complete test specification (name, VCL, request/backend/expect OR scenario)
- `ScenarioStep` - Single step in temporal test scenario (at, request, backend, expect)
- `RequestSpec` - HTTP request definition (method, URL, headers, body)
- `BackendSpec` - Mock backend response (status, headers, body)
- `ExpectSpec` - Test assertions (status, backend_calls, headers, body_contains, cached, age_lt, age_gt, stale)

**Main operations:**

- `Load()` - Loads and parses YAML file(s), returns slice of TestSpec
- `ApplyDefaults()` - Sets default values for optional fields (handles both test types)
- `IsScenario()` - Returns true if test is scenario-based

**Responsibilities:**

- Parse YAML test files (supports multi-document YAML with `---`)
- Validate test specification structure (single-request OR scenario)
- Apply sensible defaults (GET method, 200 status, etc.)
- Support cache-specific assertions (cached, age_lt, age_gt, stale) - Phase 2
- Support scenario-based temporal tests with time offsets - Phase 2
- Return structured test definitions for runner

## pkg/runner

Orchestrates test execution and coordinates all components. Supports both single-request and scenario-based tests.

**Key types:**

- `Runner` - Test executor (has varnishadm, varnishURL, workDir, logger, timeController)
- `TestResult` - Result of a single test (passed, errors, VCL trace, source)
- `VCLTraceInfo` - Execution trace data (executed lines, backend calls, VCL flow)
- `TimeController` - Interface for time manipulation (implemented by service.Manager)

**Main operations:**

- `New()` - Creates runner with varnishadm interface, varnish URL, workDir, logger
- `SetTimeController()` - Sets time controller for scenario-based tests
- `RunTest()` - Executes a single test case end-to-end (dispatches to appropriate method)
- `runSingleRequestTest()` - Executes traditional single-request test
- `runScenarioTest()` - Executes scenario-based temporal test (Phase 2)

**Test execution flow (single-request):**

1. Start mock backend server
2. Replace VCL backend placeholders (`__BACKEND_HOST__`, `__BACKEND_PORT__`)
3. Load VCL into Varnish via varnishadm
4. Activate VCL
5. Make HTTP request through Varnish
6. Check assertions
7. Clean up VCL
8. Return TestResult with trace data (on failure)

**Test execution flow (scenario):**

1. Start mock backend server (reused across steps)
2. Load and activate VCL once
3. For each scenario step:
   - Advance time to step's offset (absolute from t0)
   - Make HTTP request
   - Check assertions
4. Collect errors from all steps
5. Clean up VCL
6. Return TestResult with trace data (on failure)

**Responsibilities:**

- Coordinate all test components
- Manage test lifecycle
- Execute scenario-based temporal tests with time advancement (Phase 2)
- Collect VCL execution traces on test failure
- Provide detailed error information with trace context

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

- `Check()` - Validates ExpectSpec against Response and backend call count

**Supported assertions:**

- Status code (required) - Exact match
- Backend calls (optional) - Count match
- Headers (optional) - Key-value exact match
- Body contains (optional) - Substring match
- Cached (optional) - Cache hit/miss detection via X-Varnish header (Phase 2)
- Age less than (optional) - Age header < N seconds (Phase 2)
- Age greater than (optional) - Age header > N seconds (Phase 2)
- Stale (optional) - Stale content detection via X-Varnish-Stale or Warning: 110 (Phase 2)

**Helper functions:**

- `checkIfCached()` - Detects cache hits using X-Varnish header format and Age header
- `checkIfStale()` - Detects stale content via custom headers or HTTP warnings

**Responsibilities:**

- Compare expected vs actual results
- Generate clear error messages for failures
- Return structured result with all failures
- Cache-aware assertions for temporal testing (Phase 2)
- Simple, straightforward validation logic

## pkg/vcl

VCL file loading and backend placeholder replacement.

**Key types:**

- None (pure functions)

**Main operations:**

- `LoadAndReplace()` - Loads VCL file and replaces backend placeholders
- `ReplaceBackend()` - Replaces `__BACKEND_HOST__` and `__BACKEND_PORT__`
- `ParseAddress()` - Parses "host:port" string into components

**Responsibilities:**

- Read VCL files from disk
- Replace test-time placeholders with actual mock backend address
- Simple text-based replacement (no VCL parsing)

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
