package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/perbu/vcltest/pkg/testspec"
)

//go:embed .version
var embeddedVersion string

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	// Parse flags
	flags := flag.NewFlagSet("vcltest", flag.ExitOnError)
	verbose := flags.Bool("verbose", false, "verbose output")
	flags.BoolVar(verbose, "v", false, "verbose output (shorthand)")
	showVersion := flags.Bool("version", false, "show version")
	vclFileFlag := flags.String("vcl", "", "VCL file to use for tests (overrides auto-detection)")
	debugDump := flags.Bool("debug-dump", false, "preserve all artifacts in /tmp for debugging (no cleanup)")
	generateSchema := flags.Bool("generate-schema", false, "generate JSON schema for test specification")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	// Handle version flag
	if *showVersion {
		fmt.Printf("vcltest version %s\n", embeddedVersion)
		return nil
	}

	// Handle schema generation flag
	if *generateSchema {
		return generateJSONSchema()
	}

	// Check for test spec file argument
	if flags.NArg() == 0 {
		return fmt.Errorf("missing test spec file argument\nUsage: vcltest [options] <test-spec.yaml>")
	}

	testSpecFile := flags.Arg(0)

	// Run tests
	return runTests(ctx, testSpecFile, *verbose, *vclFileFlag, *debugDump)
}

func generateJSONSchema() error {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}

	schema := reflector.Reflect(&testspec.TestSpec{})
	schema.Title = "VCLTest Test Specification"
	schema.Description = "Schema for VCLTest YAML test specification files"
	schema.Version = "https://json-schema.org/draft/2020-12/schema"

	output, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
