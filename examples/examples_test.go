package examples_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perbu/vcltest/pkg/harness"
)

// TestExamples runs all YAML test files in the examples directory as integration tests.
// Tests with "fail" in the filename are expected to fail.
// Tests are skipped if varnishd or varnishlog are not available.
func TestExamples(t *testing.T) {
	// Skip if varnishd is not available
	if _, err := exec.LookPath("varnishd"); err != nil {
		t.Skip("varnishd not found in PATH, skipping integration tests")
	}

	// Skip if varnishlog is not available
	if _, err := exec.LookPath("varnishlog"); err != nil {
		t.Skip("varnishlog not found in PATH, skipping integration tests")
	}

	// Find all YAML test files
	yamlFiles, err := filepath.Glob("*.yaml")
	if err != nil {
		t.Fatalf("failed to glob YAML files: %v", err)
	}

	if len(yamlFiles) == 0 {
		t.Fatal("no YAML test files found in examples directory")
	}

	// Create a quiet logger for tests
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors during tests
	}))

	for _, yamlFile := range yamlFiles {
		// Capture the variable for the closure
		yamlFile := yamlFile
		testName := strings.TrimSuffix(yamlFile, ".yaml")

		t.Run(testName, func(t *testing.T) {
			t.Parallel() // Safe now that harness uses dynamic port assignment

			// Determine if this test is expected to fail.
			// Convention: files with "failing" in the name are expected to fail.
			// Note: "failure" (as in backend-failure.yaml) does NOT trigger this.
			expectFailure := strings.Contains(strings.ToLower(yamlFile), "failing")

			// Get absolute path to the test file
			absPath, err := filepath.Abs(yamlFile)
			if err != nil {
				t.Fatalf("failed to get absolute path: %v", err)
			}

			// Create harness configuration
			cfg := &harness.Config{
				TestFile:  absPath,
				Verbose:   false,
				DebugDump: false,
				Logger:    logger,
			}

			// Create and run harness
			h := harness.New(cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := h.Run(ctx)
			if err != nil {
				if expectFailure {
					// Expected to fail, but failed at harness level - that's ok
					t.Logf("test failed as expected at harness level: %v", err)
					return
				}
				t.Fatalf("harness failed: %v", err)
			}

			// Check results
			if expectFailure {
				if result.Failed == 0 {
					t.Errorf("expected test to fail, but all %d tests passed", result.Total)
				} else {
					t.Logf("test failed as expected: %d/%d failed", result.Failed, result.Total)
				}
			} else {
				if result.Failed > 0 {
					// Collect error details for better test output
					var errors []string
					for _, r := range result.Results {
						if !r.Passed {
							errors = append(errors, r.TestName+": "+strings.Join(r.Errors, "; "))
						}
					}
					t.Errorf("expected all tests to pass, but %d/%d failed:\n%s",
						result.Failed, result.Total, strings.Join(errors, "\n"))
				}
			}
		})
	}
}
