package formatter

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorGray   = "\033[90m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBold   = "\033[1m"
)

// FormatVCLWithTrace formats VCL source code with execution highlights
// executedLines contains line numbers that were executed (1-indexed)
func FormatVCLWithTrace(vclSource string, executedLines []int, useColor bool) string {
	lines := strings.Split(vclSource, "\n")

	// Create a map for quick lookup of executed lines
	executedMap := make(map[int]bool)
	for _, lineNum := range executedLines {
		executedMap[lineNum] = true
	}

	var output strings.Builder

	for i, line := range lines {
		lineNum := i + 1
		executed := executedMap[lineNum]

		if useColor {
			if executed {
				// Executed line: green checkmark and content
				fmt.Fprintf(&output, "%s✓ %3d | %s%s\n", ColorGreen, lineNum, line, ColorReset)
			} else {
				// Non-executed line: gray and dimmed
				fmt.Fprintf(&output, "%s  %3d | %s%s\n", ColorGray, lineNum, line, ColorReset)
			}
		} else {
			// Plain text fallback
			if executed {
				fmt.Fprintf(&output, "✓ %3d | %s\n", lineNum, line)
			} else {
				fmt.Fprintf(&output, "  %3d | %s\n", lineNum, line)
			}
		}
	}

	return output.String()
}

// FormatTestFailure formats a complete test failure message with VCL trace
func FormatTestFailure(testName string, errors []string, vclSource string, executedLines []int, backendCalls int, vclFlow []string, useColor bool) string {
	var output strings.Builder

	// Test name
	if useColor {
		fmt.Fprintf(&output, "\n%s%sFAILED:%s %s\n", ColorBold, ColorRed, ColorReset, testName)
	} else {
		fmt.Fprintf(&output, "\nFAILED: %s\n", testName)
	}

	// Error messages
	for _, err := range errors {
		if useColor {
			fmt.Fprintf(&output, "  %s✗%s %s\n", ColorRed, ColorReset, err)
		} else {
			fmt.Fprintf(&output, "  ✗ %s\n", err)
		}
	}

	// VCL execution trace
	if len(executedLines) > 0 {
		if useColor {
			fmt.Fprintf(&output, "\n%s%sVCL Execution Trace:%s\n", ColorBold, ColorYellow, ColorReset)
		} else {
			fmt.Fprintf(&output, "\nVCL Execution Trace:\n")
		}

		output.WriteString(FormatVCLWithTrace(vclSource, executedLines, useColor))

		// Additional trace info
		if useColor {
			fmt.Fprintf(&output, "\n%sBackend Calls:%s %d\n", ColorBold, ColorReset, backendCalls)
		} else {
			fmt.Fprintf(&output, "\nBackend Calls: %d\n", backendCalls)
		}

		if len(vclFlow) > 0 {
			if useColor {
				fmt.Fprintf(&output, "%sVCL Flow:%s %s\n", ColorBold, ColorReset, strings.Join(vclFlow, " → "))
			} else {
				fmt.Fprintf(&output, "VCL Flow: %s\n", strings.Join(vclFlow, " -> "))
			}
		}
	}

	return output.String()
}

// ShouldUseColor determines if color output should be used
// Returns true if output is a terminal (not piped)
func ShouldUseColor() bool {
	// For now, default to true. Can be enhanced to check if stdout is a TTY
	// using os.Stdout.Fd() and syscall.IoctlGetTermios or similar
	return true
}
