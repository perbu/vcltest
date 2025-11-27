package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/perbu/vcltest/pkg/formatter"
	"github.com/perbu/vcltest/pkg/harness"
)

// runTests runs the test file using the harness.
func runTests(ctx context.Context, testFile string, verbose bool, cliVCL string, debugDump bool) error {
	// Setup logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Create harness configuration
	cfg := &harness.Config{
		TestFile:  testFile,
		VCLPath:   cliVCL,
		Verbose:   verbose,
		DebugDump: debugDump,
		Logger:    logger,
	}

	// Create and run harness
	h := harness.New(cfg)
	result, err := h.Run(ctx)
	if err != nil {
		return err
	}

	// Display results
	displayResults(result)

	// Report debug dump location if created
	if result.DebugDumpPath != "" {
		fmt.Printf("\nDebug artifacts saved to: %s\n", result.DebugDumpPath)
	}

	if result.Failed > 0 {
		return fmt.Errorf("some tests failed")
	}

	return nil
}

// displayResults prints test results to stdout.
func displayResults(result *harness.Result) {
	useColor := formatter.ShouldUseColor()

	for i, testResult := range result.Results {
		fmt.Printf("\nTest %d: %s\n", i+1, testResult.TestName)

		if testResult.Passed {
			if useColor {
				fmt.Printf("  %s✓ PASSED%s\n", formatter.ColorGreen, formatter.ColorReset)
			} else {
				fmt.Printf("  ✓ PASSED\n")
			}
		} else {
			// Display enhanced error output with VCL trace
			if testResult.VCLTrace != nil && len(testResult.VCLTrace.Files) > 0 {
				// Check if we have block-level coverage data
				hasBlocks := false
				for _, f := range testResult.VCLTrace.Files {
					if f.Blocks != nil {
						hasBlocks = true
						break
					}
				}

				if hasBlocks {
					// Use new block-level coverage formatting
					var files []formatter.VCLFileInfoWithBlocks
					for _, f := range testResult.VCLTrace.Files {
						files = append(files, formatter.VCLFileInfoWithBlocks{
							ConfigID: f.ConfigID,
							Filename: f.Filename,
							Source:   f.Source,
							Blocks:   f.Blocks,
						})
					}

					output := formatter.FormatTestFailureWithBlocks(
						testResult.TestName,
						testResult.Errors,
						files,
						testResult.VCLTrace.BackendCalls,
						useColor,
					)
					fmt.Print(output)
				} else {
					// Fallback to legacy line-based formatting
					var files []formatter.VCLFileInfo
					for _, f := range testResult.VCLTrace.Files {
						files = append(files, formatter.VCLFileInfo{
							ConfigID:      f.ConfigID,
							Filename:      f.Filename,
							Source:        f.Source,
							ExecutedLines: f.ExecutedLines,
						})
					}

					output := formatter.FormatTestFailure(
						testResult.TestName,
						testResult.Errors,
						files,
						testResult.VCLTrace.BackendCalls,
						useColor,
					)
					fmt.Print(output)
				}
			} else {
				// Fallback to simple error output if trace is not available
				if useColor {
					fmt.Printf("  %s✗ FAILED%s\n", formatter.ColorRed, formatter.ColorReset)
				} else {
					fmt.Printf("  ✗ FAILED\n")
				}
				for _, errMsg := range testResult.Errors {
					fmt.Printf("    - %s\n", errMsg)
				}
			}
		}
	}

	// Print summary
	fmt.Printf("\n")
	fmt.Printf("====================\n")
	fmt.Printf("Tests passed: %d/%d\n", result.Passed, result.Total)

	if result.Failed > 0 {
		fmt.Printf("Tests failed: %d/%d\n", result.Failed, result.Total)
	}
}
