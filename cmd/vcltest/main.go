package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/perbu/vcltest/pkg/runner"
)

const version = "0.1.0-alpha"

func main() {
	ctx := context.Background()
	code := run(ctx, os.Args[1:])
	os.Exit(code)
}

func run(ctx context.Context, args []string) int {
	// Parse flags
	flags := flag.NewFlagSet("vcltest", flag.ExitOnError)
	verbose := flags.Bool("v", false, "verbose output")
	verboseLong := flags.Bool("verbose", false, "verbose output")
	noColor := flags.Bool("no-color", false, "disable color output")
	showVersion := flags.Bool("version", false, "show version")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	// Handle version flag
	if *showVersion {
		fmt.Printf("vcltest version %s\n", version)
		return 0
	}

	// Check for test file argument
	if flags.NArg() == 0 {
		printUsage()
		return 1
	}

	testFile := flags.Arg(0)

	// Check if file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: test file not found: %s\n", testFile)
		return 1
	}

	// Set verbose from either flag
	isVerbose := *verbose || *verboseLong

	// Set color (enabled by default unless --no-color)
	useColor := !*noColor

	// Create runner
	testRunner := runner.New()
	defer testRunner.Cleanup()

	// Run tests
	results, err := testRunner.RunTests(ctx, testFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running tests: %v\n", err)
		return 1
	}

	// Format output
	formatter := runner.NewFormatter(runner.OutputOptions{
		Verbose: isVerbose,
		Color:   useColor,
	}, os.Stdout)

	// Print individual test results
	for _, result := range results {
		formatter.FormatResult(result)
		fmt.Println() // Blank line between tests
	}

	// Print summary if multiple tests
	if len(results) > 1 {
		formatter.FormatSummary(results)
	}

	// Return exit code
	return runner.ExitCode(results)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `vcltest - VCL testing framework

Usage:
  vcltest [options] <test-file>

Options:
  -v, --verbose     Show verbose output including VCL execution trace
  --no-color        Disable color output
  --version         Show version information

Examples:
  vcltest test.yaml
  vcltest -v test.yaml
  vcltest --no-color test.yaml

For more information, see: https://github.com/perbu/vcltest
`)
}
