package instrument

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Config holds configuration for VCL instrumentation
type Config struct {
	VCLPath        string
	BackendAddress string // host:port for mock backend
}

// Result contains the instrumented VCL and metadata
type Result struct {
	VCL       string       // Instrumented VCL code
	LineMap   map[int]int  // Maps instrumented line to original line
	VCLSource []string     // Original VCL source lines
	Executed  map[int]bool // Tracks which lines were executed (for output)
}

// Instrument reads a VCL file, instruments it with trace logs, and replaces backends
func Instrument(config Config) (*Result, error) {
	// Read VCL file
	vclContent, err := os.ReadFile(config.VCLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read VCL file: %w", err)
	}

	// Store original source lines
	vclSource := strings.Split(string(vclContent), "\n")

	// Instrument the VCL
	instrumented := instrumentVCL(string(vclContent), config.BackendAddress)

	return &Result{
		VCL:       instrumented,
		LineMap:   make(map[int]int), // Simplified: 1:1 mapping
		VCLSource: vclSource,
		Executed:  make(map[int]bool),
	}, nil
}

// instrumentVCL performs simple regex-based instrumentation
func instrumentVCL(vcl string, backendAddr string) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(vcl))

	lineNum := 0
	currentSub := ""
	inSub := false
	hasStdImport := strings.Contains(vcl, "import std")

	// Regular expressions
	subStartRe := regexp.MustCompile(`^sub\s+(\w+)\s*\{`)
	subEndRe := regexp.MustCompile(`^\s*\}`)
	backendStartRe := regexp.MustCompile(`^backend\s+(\w+)\s*\{`)
	backendHostRe := regexp.MustCompile(`^\s*\.host\s*=\s*"[^"]*"\s*;`)
	backendPortRe := regexp.MustCompile(`^\s*\.port\s*=\s*"[^"]*"\s*;`)
	emptyLineRe := regexp.MustCompile(`^\s*$`)
	commentRe := regexp.MustCompile(`^\s*#`)
	vclVersionRe := regexp.MustCompile(`^vcl\s+[\d.]+\s*;`)

	// Add std import after vcl version if not present
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Write VCL version line
		if vclVersionRe.MatchString(line) {
			result.WriteString(line)
			result.WriteString("\n")
			if !hasStdImport {
				result.WriteString("\nimport std;\n")
			}
			continue
		}

		// Skip import std if we already added it
		if strings.Contains(line, "import std") && !hasStdImport {
			continue
		}

		// Handle backend definitions
		if match := backendStartRe.FindStringSubmatch(line); match != nil {
			result.WriteString(line)
			result.WriteString("\n")

			// Replace backend host and port
			parts := strings.Split(backendAddr, ":")
			host := parts[0]
			port := "8080"
			if len(parts) > 1 {
				port = parts[1]
			}

			// Read until end of backend block
			for scanner.Scan() {
				lineNum++
				backendLine := scanner.Text()

				if backendHostRe.MatchString(backendLine) {
					result.WriteString(fmt.Sprintf("    .host = \"%s\";\n", host))
				} else if backendPortRe.MatchString(backendLine) {
					result.WriteString(fmt.Sprintf("    .port = \"%s\";\n", port))
				} else {
					result.WriteString(backendLine)
					result.WriteString("\n")
					if strings.TrimSpace(backendLine) == "}" {
						break
					}
				}
			}
			continue
		}

		// Handle subroutine start
		if match := subStartRe.FindStringSubmatch(line); match != nil {
			currentSub = match[1]
			inSub = true
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Handle subroutine end
		if inSub && subEndRe.MatchString(line) {
			inSub = false
			currentSub = ""
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Instrument statements inside subroutines
		if inSub && !emptyLineRe.MatchString(line) && !commentRe.MatchString(line) {
			indent := getIndent(line)
			// Add trace before the statement
			result.WriteString(fmt.Sprintf("%sstd.log(\"TRACE:%d:%s\");\n", indent, lineNum, currentSub))
			result.WriteString(line)
			result.WriteString("\n")
		} else {
			// Write line as-is
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// getIndent returns the leading whitespace of a line
func getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return line
}

// MarkExecuted marks a line as executed based on trace output
func (r *Result) MarkExecuted(line int) {
	r.Executed[line] = true
}
