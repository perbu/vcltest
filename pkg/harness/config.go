package harness

import (
	"log/slog"

	"github.com/perbu/vcltest/pkg/runner"
)

// Config holds configuration for the test harness.
type Config struct {
	// TestFile is the path to the YAML test specification file.
	TestFile string

	// VCLPath is an optional explicit path to the VCL file.
	// If empty, the harness will auto-detect based on the test file name.
	VCLPath string

	// Verbose enables debug logging.
	Verbose bool

	// DebugDump preserves all artifacts in /tmp for debugging.
	DebugDump bool

	// Logger is the structured logger to use. If nil, a default is created.
	Logger *slog.Logger
}

// Result holds the outcome of running all tests.
type Result struct {
	// Passed is the count of tests that passed.
	Passed int

	// Failed is the count of tests that failed.
	Failed int

	// Total is the total number of tests run.
	Total int

	// Results contains detailed results for each test.
	Results []runner.TestResult

	// DebugDumpPath is the path to debug artifacts, if DebugDump was enabled.
	DebugDumpPath string
}
