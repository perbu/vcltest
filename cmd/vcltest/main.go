package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
		return 1
	}

	testFile := flags.Arg(0)

	// Check if file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: test file not found: %s\n", testFile)
		return 1
	}

}
