package instrument

import (
	"fmt"
	"os"
	"strings"

	"github.com/perbu/vclparser"
)

// Config holds configuration for VCL instrumentation
type Config struct {
	VCLPath        string
	BackendAddress string // host:port for mock backend
}

// Result contains the instrumented VCL and metadata
type Result struct {
	VCL       string            // Instrumented VCL code
	LineMap   map[int]int       // Maps instrumented line to original line
	VCLSource []string          // Original VCL source lines
	Executed  map[int]bool      // Tracks which lines were executed (for output)
}

// Instrument reads a VCL file, instruments it with trace logs, and replaces backends
func Instrument(config Config) (*Result, error) {
	// Read VCL file
	vclContent, err := os.ReadFile(config.VCLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read VCL file: %w", err)
	}

	// Parse VCL
	vcl, err := vclparser.Parse(string(vclContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse VCL: %w", err)
	}

	// Store original source lines
	vclSource := strings.Split(string(vclContent), "\n")

	// Build instrumented VCL
	var instrumented strings.Builder

	// Add std import if not present
	hasStdImport := false
	for _, stmt := range vcl.Statements {
		if imp, ok := stmt.(*vclparser.Import); ok && imp.Name == "std" {
			hasStdImport = true
			break
		}
	}

	if !hasStdImport {
		instrumented.WriteString("import std;\n\n")
	}

	// Track line mappings (for now, simplified - we'll improve this later)
	lineMap := make(map[int]int)

	// Process statements
	for _, stmt := range vcl.Statements {
		switch s := stmt.(type) {
		case *vclparser.Backend:
			// Replace backend with mock address
			instrumentedBackend := instrumentBackend(s, config.BackendAddress)
			instrumented.WriteString(instrumentedBackend)
			instrumented.WriteString("\n")
		case *vclparser.Subroutine:
			// Instrument subroutine
			instrumentedSub := instrumentSubroutine(s)
			instrumented.WriteString(instrumentedSub)
			instrumented.WriteString("\n")
		case *vclparser.Import:
			// Keep import as-is
			instrumented.WriteString(fmt.Sprintf("import %s;\n", s.Name))
		case *vclparser.ACL:
			// Keep ACL as-is
			instrumented.WriteString(s.String())
			instrumented.WriteString("\n")
		default:
			// Keep other statements as-is
			instrumented.WriteString(s.String())
			instrumented.WriteString("\n")
		}
	}

	return &Result{
		VCL:       instrumented.String(),
		LineMap:   lineMap,
		VCLSource: vclSource,
		Executed:  make(map[int]bool),
	}, nil
}

// instrumentBackend replaces backend definition with mock address
func instrumentBackend(backend *vclparser.Backend, mockAddress string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("backend %s {\n", backend.Name))

	// Split address into host and port
	parts := strings.Split(mockAddress, ":")
	if len(parts) == 2 {
		b.WriteString(fmt.Sprintf("  .host = \"%s\";\n", parts[0]))
		b.WriteString(fmt.Sprintf("  .port = \"%s\";\n", parts[1]))
	} else {
		b.WriteString(fmt.Sprintf("  .host = \"%s\";\n", mockAddress))
		b.WriteString("  .port = \"8080\";\n")
	}

	b.WriteString("}")
	return b.String()
}

// instrumentSubroutine adds trace logging to subroutine
func instrumentSubroutine(sub *vclparser.Subroutine) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("sub %s {\n", sub.Name))

	// Add initial trace
	if sub.Line > 0 {
		b.WriteString(fmt.Sprintf("  std.log(\"TRACE:%d:%s\");\n", sub.Line, sub.Name))
	}

	// Process statements (simplified for now - proper AST walking would be better)
	for _, stmt := range sub.Statements {
		b.WriteString("  ")

		// Add trace before statement
		if stmt.GetLine() > 0 {
			b.WriteString(fmt.Sprintf("std.log(\"TRACE:%d:%s\"); ", stmt.GetLine(), sub.Name))
		}

		b.WriteString(stmt.String())
		b.WriteString("\n")
	}

	b.WriteString("}")
	return b.String()
}

// MarkExecuted marks a line as executed based on trace output
func (r *Result) MarkExecuted(line int) {
	r.Executed[line] = true
}
