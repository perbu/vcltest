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
