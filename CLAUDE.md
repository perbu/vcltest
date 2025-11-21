# VCLTest Package Documentation

This document provides terse descriptions of key packages in VCLTest. For overall project goals and usage,
see [README.md](README.md).

## pkg/varnish

Manages the varnishd process lifecycle.

**Key types:**

- `Manager` - Controls varnishd startup, workspace preparation, and process monitoring
- `Config` - Configuration for varnish command-line arguments (ports, storage, parameters)

**Main operations:**

- `New()` - Creates manager with work directory and logger
- `PrepareWorkspace()` - Sets up directories, secret file, and license file
- `Start()` - Starts varnishd process with given arguments and blocks until exit
- `BuildArgs()` - Constructs varnishd command-line from Config struct

**Responsibilities:**

- Directory setup with proper permissions
- Secret generation for varnishadm authentication
- License file handling for Varnish Enterprise
- Command-line argument construction (including `-p feature=+trace`)
- Process output routing to structured logs

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

Orchestrates startup and lifecycle of varnishadm server and varnish daemon.

**Key types:**

- `Manager` - Coordinates both services with proper initialization order
- `Config` - Combined configuration for both varnishadm and varnish

**Startup sequence:**

1. Start varnishadm server (runs in background)
2. Prepare varnish workspace
3. Build varnish arguments
4. Start varnish daemon (connects to varnishadm, child auto-starts)
5. Monitor both services until failure or context cancellation

**Main operations:**

- `NewManager()` - Creates orchestrator with validation
- `Start()` - Starts both services and blocks until error or shutdown
- `GetVarnishadm()` - Returns interface for issuing varnishadm commands
- `GetVarnishManager()` - Returns varnish manager instance

**Responsibilities:**

- Ensure varnishadm is listening before starting varnish
- Handle errors from either service
- Graceful shutdown via context cancellation
- Provide unified interface for service management

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

Parses YAML test specification files.

**Key types:**

- `TestSpec` - Complete test specification (name, VCL, request, backend, expect)
- `RequestSpec` - HTTP request definition (method, URL, headers, body)
- `BackendSpec` - Mock backend response (status, headers, body)
- `ExpectSpec` - Test assertions (status, backend_calls, headers, body_contains)

**Main operations:**

- `Load()` - Loads and parses YAML file(s), returns slice of TestSpec
- `ApplyDefaults()` - Sets default values for optional fields

**Responsibilities:**

- Parse YAML test files (supports multi-document YAML with `---`)
- Validate test specification structure
- Apply sensible defaults (GET method, 200 status, etc.)
- Return structured test definitions for runner

## pkg/runner

Orchestrates test execution and coordinates all components.

**Key types:**

- `Runner` - Test executor (has varnishadm, varnishURL, workDir, logger)
- `TestResult` - Result of a single test (passed, errors, VCL trace, source)
- `VCLTraceInfo` - Execution trace data (executed lines, backend calls, VCL flow)

**Main operations:**

- `New()` - Creates runner with varnishadm interface, varnish URL, workDir, logger
- `RunTest()` - Executes a single test case end-to-end

**Test execution flow:**

1. Start mock backend server
2. Replace VCL backend placeholders (`__BACKEND_HOST__`, `__BACKEND_PORT__`)
3. Load VCL into Varnish via varnishadm
4. Activate VCL
5. Start varnishlog recorder
6. Make HTTP request through Varnish
7. Stop recorder and parse trace
8. Check assertions
9. Clean up VCL
10. Return TestResult with trace data (on failure)

**Responsibilities:**

- Coordinate all test components
- Manage test lifecycle
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

Validates test expectations against actual results.

**Key types:**

- `Result` - Assertion result (passed, errors list)

**Main operations:**

- `Check()` - Validates ExpectSpec against Response and backend call count

**Supported assertions:**

- Status code (required) - Exact match
- Backend calls (optional) - Count match
- Headers (optional) - Key-value exact match
- Body contains (optional) - Substring match

**Responsibilities:**

- Compare expected vs actual results
- Generate clear error messages for failures
- Return structured result with all failures
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
