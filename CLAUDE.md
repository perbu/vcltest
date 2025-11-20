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
- Command-line argument construction
- Process output routing to structured logs

## pkg/varnishadm

Implements the varnishadm server protocol and command interface.

**Key types:**

- `Server` - Listens for varnishd connections and handles CLI protocol
- `VarnishadmInterface` - Command interface for Varnish management operations

**Protocol details:**

- Implements Varnish CLI wire protocol (status code + length + payload)
- Challenge-response authentication using SHA256
- Request/response pattern over TCP connection

**Main operations:**

- `New()` - Creates server with port, secret, and logger
- `Run()` - Starts server and accepts connections (blocks)
- `Exec()` - Executes arbitrary varnishadm commands
- High-level commands: `VCLLoad()`, `VCLUse()`, `ParamSet()`, `TLSCertLoad()`, etc.

**Responsibilities:**

- Listen for varnishd connections on specified port
- Authenticate varnishd using shared secret
- Parse CLI protocol messages
- Execute commands and return structured responses
- Parse complex responses (VCL list, TLS cert list)

## pkg/service

Orchestrates startup and lifecycle of varnishadm server and varnish daemon.

**Key types:**

- `Manager` - Coordinates both services with proper initialization order
- `Config` - Combined configuration for both varnishadm and varnish

**Startup sequence:**

1. Start varnishadm server (runs in background)
2. Prepare varnish workspace
3. Build varnish arguments
4. Start varnish daemon (connects to varnishadm)
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
- `Start()` - Begins recording to binary file (varnishlog -g request -w)
- `Stop()` - Gracefully stops recording
- `GetMessages()` - Reads binary log and returns parsed messages
- `GetVCLMessages()` - Filters for VCL-related messages only
- `GetTraceSummary()` - Returns execution summary with line numbers and backend count

**Parsing functions:**

- `ParseVCLTrace()` - Extracts config/line/column from trace messages
- `ParseBackendCall()` - Extracts backend connection details
- `GetExecutedLines()` - Returns unique line numbers from user VCL (filters built-in)
- `CountBackendCalls()` - Counts BackendOpen entries

**Responsibilities:**

- Capture varnishlog output during test execution
- Parse Varnish CLI protocol log format per VARNISH_TRACE_SPEC.md
- Filter user VCL traces (config=0) from built-in VCL
- Provide structured access to VCL execution data
- Support single recording at a time
