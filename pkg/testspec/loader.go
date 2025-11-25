package testspec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML test file
// Supports multiple test documents separated by ---
func Load(filename string) ([]TestSpec, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}

	// Parse multiple YAML documents using yaml.v3 decoder
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Strict mode - fail on unknown fields

	var tests []TestSpec
	docNum := 0

	for {
		var test TestSpec
		err := decoder.Decode(&test)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing test document %d: %w", docNum+1, err)
		}

		docNum++

		// Validate required fields
		if err := validate(&test); err != nil {
			return nil, fmt.Errorf("test %d (%q): %w", docNum, test.Name, err)
		}

		// Apply defaults
		test.ApplyDefaults()

		tests = append(tests, test)
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("no test documents found in %s", filename)
	}

	return tests, nil
}

// validate checks that required fields are present
func validate(test *TestSpec) error {
	if test.Name == "" {
		return fmt.Errorf("test name is required")
	}

	// Check if this is a scenario-based test or single-request test
	isScenario := len(test.Scenario) > 0
	isSingleRequest := test.Request.URL != ""

	// Must be either scenario or single-request, not both
	if isScenario && isSingleRequest {
		return fmt.Errorf("test cannot have both 'scenario' and 'request' fields")
	}
	if !isScenario && !isSingleRequest {
		return fmt.Errorf("test must have either 'scenario' or 'request' field")
	}

	// Validate single-request test
	if isSingleRequest {
		if test.Expectations.Response.Status == 0 {
			return fmt.Errorf("expectations.response.status is required")
		}
		if err := validateBackendSpec(test.Backend, "backend"); err != nil {
			return err
		}
		for name, spec := range test.Backends {
			if err := validateBackendSpec(spec, fmt.Sprintf("backends.%s", name)); err != nil {
				return err
			}
		}
	}

	// Validate scenario-based test
	if isScenario {
		if len(test.Scenario) == 0 {
			return fmt.Errorf("scenario must have at least one step")
		}
		for i, step := range test.Scenario {
			if step.At == "" {
				return fmt.Errorf("scenario step %d: 'at' field is required", i+1)
			}
			if step.Request.URL == "" {
				return fmt.Errorf("scenario step %d: request.url is required", i+1)
			}
			if step.Expectations.Response.Status == 0 {
				return fmt.Errorf("scenario step %d: expectations.response.status is required", i+1)
			}
			if err := validateBackendSpec(step.Backend, fmt.Sprintf("scenario step %d: backend", i+1)); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateBackendSpec validates a backend specification
func validateBackendSpec(spec BackendSpec, context string) error {
	switch spec.FailureMode {
	case "", "failed", "frozen":
		// Valid
	default:
		return fmt.Errorf("%s: invalid failure_mode %q, must be 'failed', 'frozen', or empty", context, spec.FailureMode)
	}
	return nil
}

// ResolveVCL determines the VCL file path to use for tests.
// Priority: 1) CLI flag (-vcl), 2) Same-named .vcl file
func ResolveVCL(testFilePath string, cliVCL string) (string, error) {
	// Priority 1: CLI flag
	if cliVCL != "" {
		if _, err := os.Stat(cliVCL); err != nil {
			return "", fmt.Errorf("VCL file specified via -vcl flag not found: %s", cliVCL)
		}
		return cliVCL, nil
	}

	// Priority 2: Same-named .vcl file
	testDir := filepath.Dir(testFilePath)
	testBase := filepath.Base(testFilePath)
	testName := strings.TrimSuffix(testBase, filepath.Ext(testBase))
	vclPath := filepath.Join(testDir, testName+".vcl")

	if _, err := os.Stat(vclPath); err == nil {
		return vclPath, nil
	}

	// No VCL found
	return "", fmt.Errorf("no VCL file found: tried -vcl flag and %s", vclPath)
}
