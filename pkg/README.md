# VCLTest Packages

Quick reference for the VCLTest package structure.

## Core Orchestration

### pkg/service
Orchestrates startup and lifecycle of varnishadm server and varnish daemon with proper initialization order. Manages event-driven coordination between VCL loading and cache startup, and provides interfaces for issuing commands and controlling fake time for temporal testing.

### pkg/runner
Orchestrates VCL test execution by coordinating varnishadm commands, mock backends, VCL loading, and assertion validation. Manages shared VCL across multiple tests, performs AST-based backend replacement, and collects execution traces for test failure analysis.

## Varnish Integration

### pkg/varnish
Manages the varnishd process lifecycle including workspace preparation, command-line argument construction, process startup and monitoring, and time manipulation through libfaketime integration for temporal testing.

### pkg/varnishadm
Implements the varnishadm server protocol and command interface for managing Varnish via TCP. Handles CLI wire protocol authentication, VCL management, parameter control, and TLS operations.

### pkg/recorder
Captures varnishlog output in real-time during test execution, parsing and filtering raw logs to extract VCL execution traces (executed lines, backend calls, function flow). Provides structured access to trace data for failure analysis.

## VCL Processing

### pkg/vclmod
Parses and modifies VCL files using AST-based transformation to replace backend host and port addresses while validating that all test YAML backends exist in the VCL and warning about unused VCL backends. Handles VCL include directives while preserving structure and comments.

### pkg/vclloader
Provides VCL file loading and activation with support for includes, retrieves VCL-to-config mappings for trace analysis, and publishes events to coordinate the startup sequence. Includes a simple address parser for backend configuration.

## Testing Infrastructure

### pkg/testspec
Parses YAML test specifications with support for single-request and multi-step scenario-based temporal tests. Validates test structure, applies default values, and resolves VCL file paths from CLI flags or same-named files.

### pkg/backend
Provides HTTP mock backend servers that return configured responses for testing, tracks request call counts, and supports dynamic configuration updates without restart.

### pkg/client
Provides an HTTP client for making test requests to Varnish with customizable method, headers, and body. Prevents automatic redirect following to test redirect responses themselves.

### pkg/assertion
Validates test expectations against actual HTTP responses by checking status codes, backend calls, headers, body content, cache state, age constraints, and staleness. Provides structured results with detailed error messages.

### pkg/freeport
Finds available TCP ports for dynamic port assignment, enabling parallel test execution by avoiding port conflicts between concurrent test harness instances.

## Output and Formatting

### pkg/formatter
Formats VCL source code with execution trace visualization for terminal output, using ANSI color codes to highlight executed lines with green checkmarks and non-executed lines in gray. Supports both colored terminal output and plain text fallback.

---

For detailed documentation of each package, see [CLAUDE.md](../CLAUDE.md).
