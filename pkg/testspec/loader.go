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
	if test.Request.URL == "" {
		return fmt.Errorf("request.url is required")
	}
	if test.Expect.Status == 0 {
		return fmt.Errorf("expect.status is required")
	}

	return nil
}
