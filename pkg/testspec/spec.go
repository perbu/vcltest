package testspec

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// TestSpec represents a single test specification
type TestSpec struct {
	Name    string      `yaml:"name"`
	VCL     string      `yaml:"vcl"`
	Request RequestSpec `yaml:"request"`
	Backend BackendSpec `yaml:"backend"`
	Expect  ExpectSpec  `yaml:"expect"`
}

// RequestSpec defines the HTTP request configuration
type RequestSpec struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

// BackendSpec defines the mock backend configuration
type BackendSpec struct {
	Status  int               `yaml:"status"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

// ExpectSpec defines test assertions
type ExpectSpec struct {
	Status       int               `yaml:"status"`
	BackendCalls *int              `yaml:"backend_calls,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	BodyContains *string           `yaml:"body_contains,omitempty"`
}

// ApplyDefaults applies default values to the test specification
func (t *TestSpec) ApplyDefaults() {
	// Request defaults
	if t.Request.Method == "" {
		t.Request.Method = "GET"
	}
	if t.Request.Headers == nil {
		t.Request.Headers = make(map[string]string)
	}

	// Backend defaults
	if t.Backend.Status == 0 {
		t.Backend.Status = 200
	}
	if t.Backend.Headers == nil {
		t.Backend.Headers = make(map[string]string)
	}
}

// Validate checks if the test specification is valid
func (t *TestSpec) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("test name is required")
	}
	if t.VCL == "" {
		return fmt.Errorf("VCL file path is required")
	}
	if t.Request.URL == "" {
		return fmt.Errorf("request URL is required")
	}
	if t.Expect.Status == 0 {
		return fmt.Errorf("expect.status is required")
	}
	return nil
}

// ParseTests parses multiple test specifications from a YAML reader
// Supports both single documents and multi-document YAML (separated by ---)
func ParseTests(r io.Reader) ([]*TestSpec, error) {
	decoder := yaml.NewDecoder(r)
	var tests []*TestSpec

	for {
		var test TestSpec
		err := decoder.Decode(&test)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		test.ApplyDefaults()
		if err := test.Validate(); err != nil {
			return nil, fmt.Errorf("invalid test specification for '%s': %w", test.Name, err)
		}

		tests = append(tests, &test)
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("no tests found in YAML")
	}

	return tests, nil
}

// ParseTest parses a single test specification from a YAML reader
func ParseTest(r io.Reader) (*TestSpec, error) {
	tests, err := ParseTests(r)
	if err != nil {
		return nil, err
	}
	if len(tests) > 1 {
		return nil, fmt.Errorf("expected single test but found %d tests", len(tests))
	}
	return tests[0], nil
}
