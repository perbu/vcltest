package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
)

//go:embed .version
var embeddedVersion string

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")
}

func run(ctx context.Context, args []string) error {
	// Parse flags
	flags := flag.NewFlagSet("vcltest", flag.ExitOnError)
	verbose := flags.Bool("verbose", false, "verbose output")
	noColor := flags.Bool("no-color", false, "disable color output")
	showVersion := flags.Bool("version", false, "show version")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if verbose != nil {
		fmt.Println("Verbose output enabled")
	}

	if noColor != nil {
		fmt.Println("Color output disabled")
	}

	// Handle version flag
	if *showVersion {
		fmt.Printf("vcltest version %s\n", embeddedVersion)
		return nil
	}

	// Check for test file argument
	if flags.NArg() == 0 {
		return fmt.Errorf("missing test file argument")
	}

	testFile := flags.Arg(0)

	// Check if file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		return fmt.Errorf("test file %q does not exist", testFile)
	}
	return nil
}
