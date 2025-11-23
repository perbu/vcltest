package formatter

import (
	"strings"
	"testing"
)

func TestFormatVCLWithTrace(t *testing.T) {
	vclSource := `vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    return (pass);
}`

	executedLines := []int{1, 3, 8, 9}

	// Test without color
	result := FormatVCLWithTrace(vclSource, executedLines, false)

	// Check that executed lines have checkmarks
	if !strings.Contains(result, "✓   1 | vcl 4.1;") {
		t.Errorf("Expected line 1 to be marked as executed")
	}

	// Check that non-executed lines don't have checkmarks
	if !strings.Contains(result, "    2 | ") {
		t.Errorf("Expected line 2 to not be marked as executed")
	}

	// Check that all lines are present
	lines := strings.Split(result, "\n")
	// 9 lines of VCL + trailing newline creates empty element
	if len(lines) < 9 {
		t.Errorf("Expected at least 9 lines, got %d", len(lines))
	}
}

func TestFormatVCLWithTraceColor(t *testing.T) {
	vclSource := `vcl 4.1;
backend default {}`

	executedLines := []int{1}

	// Test with color
	result := FormatVCLWithTrace(vclSource, executedLines, true)

	// Check that color codes are present
	if !strings.Contains(result, ColorGreen) {
		t.Errorf("Expected green color code for executed line")
	}

	if !strings.Contains(result, ColorGray) {
		t.Errorf("Expected gray color code for non-executed line")
	}

	if !strings.Contains(result, ColorReset) {
		t.Errorf("Expected reset color code")
	}
}

func TestFormatTestFailure(t *testing.T) {
	vclSource := `sub vcl_recv {
    return (pass);
}`

	errors := []string{"Status code: expected 404, got 200"}
	executedLines := []int{1, 2}
	vclFlow := []string{"vcl_recv", "pass"}

	files := []VCLFileInfo{
		{
			ConfigID:      0,
			Filename:      "/path/to/main.vcl",
			Source:        vclSource,
			ExecutedLines: executedLines,
		},
	}

	result := FormatTestFailure(
		"Test name",
		errors,
		files,
		1,
		vclFlow,
		false,
	)

	// Check that all components are present
	if !strings.Contains(result, "FAILED: Test name") {
		t.Errorf("Expected test name in output")
	}

	if !strings.Contains(result, "Status code: expected 404, got 200") {
		t.Errorf("Expected error message in output")
	}

	if !strings.Contains(result, "VCL Execution Trace:") {
		t.Errorf("Expected trace header in output")
	}

	if !strings.Contains(result, "Backend Calls: 1") {
		t.Errorf("Expected backend calls in output")
	}

	if !strings.Contains(result, "VCL Flow: vcl_recv -> pass") {
		t.Errorf("Expected VCL flow in output")
	}
}
