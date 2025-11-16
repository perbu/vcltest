package runner

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/perbu/vcltest/pkg/assertion"
)

// OutputOptions configures output formatting
type OutputOptions struct {
	Verbose bool
	Color   bool
}

// Formatter formats test results for output
type Formatter struct {
	options OutputOptions
	writer  io.Writer
}

// NewFormatter creates a new output formatter
func NewFormatter(options OutputOptions, writer io.Writer) *Formatter {
	if writer == nil {
		writer = os.Stdout
	}

	// Auto-detect color support
	if options.Color {
		// Check if output is a terminal
		if fileInfo, err := os.Stdout.Stat(); err == nil {
			if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
				// Not a terminal, disable colors
				options.Color = false
			}
		}

		// Check NO_COLOR environment variable
		if os.Getenv("NO_COLOR") != "" {
			options.Color = false
		}
	}

	return &Formatter{
		options: options,
		writer:  writer,
	}
}

// FormatResult formats a single test result
func (f *Formatter) FormatResult(result *TestResult) {
	if result.Error != nil {
		f.formatError(result)
		return
	}

	if result.Passed {
		f.formatPass(result)
	} else {
		f.formatFail(result)
	}
}

// formatPass formats a passing test
func (f *Formatter) formatPass(result *TestResult) {
	fmt.Fprintf(f.writer, "%sPASS%s: %s (%dms)\n",
		f.green(),
		f.reset(),
		result.Name,
		result.Duration.Milliseconds())

	if f.options.Verbose {
		f.formatVCLExecution(result)
	}
}

// formatFail formats a failing test
func (f *Formatter) formatFail(result *TestResult) {
	fmt.Fprintf(f.writer, "%sFAIL%s: %s (%dms)\n",
		f.red(),
		f.reset(),
		result.Name,
		result.Duration.Milliseconds())

	// Show failed assertions
	fmt.Fprintf(f.writer, "\nFailed assertions:\n")
	for _, failed := range result.Assertions.Failed() {
		f.formatFailedAssertion(failed)
	}

	// Show VCL execution trace
	fmt.Fprintf(f.writer, "\nVCL execution:\n")
	f.formatVCLExecution(result)
}

// formatError formats a test that encountered an error
func (f *Formatter) formatError(result *TestResult) {
	fmt.Fprintf(f.writer, "%sERROR%s: %s (%dms)\n",
		f.red(),
		f.reset(),
		result.Name,
		result.Duration.Milliseconds())
	fmt.Fprintf(f.writer, "  %s\n", result.Error)
}

// formatFailedAssertion formats a failed assertion
func (f *Formatter) formatFailedAssertion(failed assertion.Result) {
	fmt.Fprintf(f.writer, "  - %s\n", failed.Message)
	fmt.Fprintf(f.writer, "    Expected: %s\n", failed.Expected)
	fmt.Fprintf(f.writer, "    Actual:   %s\n", failed.Actual)
}

// formatVCLExecution formats VCL execution trace
func (f *Formatter) formatVCLExecution(result *TestResult) {
	if len(result.VCLSource) == 0 {
		return
	}

	for i, line := range result.VCLSource {
		lineNum := i + 1
		marker := "|"
		color := ""

		if result.ExecutedLines[lineNum] {
			marker = "*"
			if f.options.Color {
				color = f.green()
			}
		}

		fmt.Fprintf(f.writer, "  %s%s %4d  %s%s\n",
			color,
			marker,
			lineNum,
			line,
			f.reset())
	}
}

// FormatSummary formats a summary of multiple test results
func (f *Formatter) FormatSummary(results []*TestResult) {
	total := len(results)
	passed := 0
	failed := 0
	errors := 0

	for _, result := range results {
		if result.Error != nil {
			errors++
		} else if result.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Fprintf(f.writer, "\n")
	fmt.Fprintf(f.writer, "========================================\n")
	fmt.Fprintf(f.writer, "Test Summary\n")
	fmt.Fprintf(f.writer, "========================================\n")
	fmt.Fprintf(f.writer, "Total:  %d\n", total)
	fmt.Fprintf(f.writer, "%sPassed: %d%s\n", f.green(), passed, f.reset())

	if failed > 0 {
		fmt.Fprintf(f.writer, "%sFailed: %d%s\n", f.red(), failed, f.reset())
	} else {
		fmt.Fprintf(f.writer, "Failed: %d\n", failed)
	}

	if errors > 0 {
		fmt.Fprintf(f.writer, "%sErrors: %d%s\n", f.red(), errors, f.reset())
	}

	// List failed tests
	if failed > 0 || errors > 0 {
		fmt.Fprintf(f.writer, "\nFailed tests:\n")
		for _, result := range results {
			if result.Error != nil || !result.Passed {
				fmt.Fprintf(f.writer, "  - %s\n", result.Name)
			}
		}
	}
}

// Color helpers
func (f *Formatter) green() string {
	if f.options.Color {
		return "\033[32m"
	}
	return ""
}

func (f *Formatter) red() string {
	if f.options.Color {
		return "\033[31m"
	}
	return ""
}

func (f *Formatter) reset() string {
	if f.options.Color {
		return "\033[0m"
	}
	return ""
}

// ExitCode returns the appropriate exit code based on results
func ExitCode(results []*TestResult) int {
	for _, result := range results {
		if result.Error != nil || !result.Passed {
			return 1
		}
	}
	return 0
}
