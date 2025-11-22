package testspec

import (
	"bytes"
	"fmt"
	"io"
	"os"

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
	if test.VCL == "" {
		return fmt.Errorf("vcl field is required")
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
		if test.Expect.Status == 0 {
			return fmt.Errorf("expect.status is required")
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
			if step.Expect.Status == 0 {
				return fmt.Errorf("scenario step %d: expect.status is required", i+1)
			}
		}
	}

	return nil
}
