package formatter

import (
	"strings"
	"testing"

	"github.com/perbu/vcltest/pkg/coverage"
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
}

func TestFormatVCLWithBlocks(t *testing.T) {
	vclSource := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}`

	// Create blocks structure
	fb, err := coverage.AnalyzeVCL(vclSource, "/test.vcl")
	if err != nil {
		t.Fatalf("Failed to analyze VCL: %v", err)
	}

	// Mark sub as entered, if as NOT entered
	fb.Blocks[0].Entered = true
	fb.Blocks[0].Children[0].Entered = false

	// Test without color (plain text)
	result := FormatVCLWithBlocks(vclSource, fb, false)

	// All lines should be present
	lines := strings.Split(result, "\n")
	if len(lines) < 8 {
		t.Errorf("Expected at least 8 lines, got %d", len(lines))
	}

	// Check that entered block lines have * marker
	// Line 3 is "sub vcl_recv {" - should have * since sub block is entered
	if !strings.Contains(result, "*    3 | sub vcl_recv") {
		t.Errorf("Expected line 3 to have * marker (entered block)")
	}

	// Line 7 is "return (hash);" inside sub but outside if - should have *
	if !strings.Contains(result, "*    7 |     return (hash)") {
		t.Errorf("Expected line 7 to have * marker (inside entered sub)")
	}

	// Line 5 is inside the non-entered if block - should NOT have *
	if strings.Contains(result, "*    5 |") {
		t.Errorf("Expected line 5 to NOT have * marker (inside non-entered if)")
	}

	// Line 1 is outside any block - should NOT have *
	if strings.Contains(result, "*    1 |") {
		t.Errorf("Expected line 1 to NOT have * marker (outside blocks)")
	}
}

func TestFormatVCLWithBlocksColor(t *testing.T) {
	vclSource := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}`

	fb, err := coverage.AnalyzeVCL(vclSource, "/test.vcl")
	if err != nil {
		t.Fatalf("Failed to analyze VCL: %v", err)
	}

	// Mark sub as entered, if as NOT entered
	fb.Blocks[0].Entered = true
	fb.Blocks[0].Children[0].Entered = false

	// Test with color
	result := FormatVCLWithBlocks(vclSource, fb, true)

	// Check that color codes are present
	if !strings.Contains(result, ColorGreen) {
		t.Errorf("Expected green color for entered block lines")
	}

	if !strings.Contains(result, ColorGray) {
		t.Errorf("Expected gray color for non-entered block lines")
	}

	if !strings.Contains(result, ColorReset) {
		t.Errorf("Expected reset color code")
	}
}

func TestFormatTestFailureWithBlocks(t *testing.T) {
	vclSource := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}`

	fb, err := coverage.AnalyzeVCL(vclSource, "/test.vcl")
	if err != nil {
		t.Fatalf("Failed to analyze VCL: %v", err)
	}

	// Mark sub as entered, if as NOT entered
	fb.Blocks[0].Entered = true
	fb.Blocks[0].Children[0].Entered = false

	errors := []string{"Status code: expected 404, got 200"}

	files := []VCLFileInfoWithBlocks{
		{
			ConfigID: 0,
			Filename: "/path/to/main.vcl",
			Source:   vclSource,
			Blocks:   fb,
		},
	}

	result := FormatTestFailureWithBlocks(
		"Test name",
		errors,
		files,
		1,
		false,
	)

	// Check that all components are present
	if !strings.Contains(result, "FAILED: Test name") {
		t.Errorf("Expected test name in output")
	}

	if !strings.Contains(result, "Status code: expected 404, got 200") {
		t.Errorf("Expected error message in output")
	}

	if !strings.Contains(result, "VCL Block Coverage:") {
		t.Errorf("Expected block coverage header in output")
	}

	if !strings.Contains(result, "Backend Calls: 1") {
		t.Errorf("Expected backend calls in output")
	}

	// Check for blocks entered summary
	if !strings.Contains(result, "Blocks entered:") {
		t.Errorf("Expected blocks entered summary in output")
	}

	if !strings.Contains(result, "vcl_recv") {
		t.Errorf("Expected vcl_recv in blocks entered")
	}

	// Check for blocks not entered
	if !strings.Contains(result, "Blocks not entered:") {
		t.Errorf("Expected blocks not entered summary in output")
	}

	if !strings.Contains(result, "if@4") {
		t.Errorf("Expected if@4 in blocks not entered")
	}
}
