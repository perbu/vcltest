package assertion

import (
	"fmt"
	"strings"

	"github.com/perbu/vcltest/pkg/client"
	"github.com/perbu/vcltest/pkg/testspec"
)

// Result represents the outcome of assertion checking
type Result struct {
	Passed bool
	Errors []string
}

// Check verifies all expectations against actual results
func Check(expect testspec.ExpectSpec, response *client.Response, backendCalls int) *Result {
	result := &Result{
		Passed: true,
		Errors: []string{},
	}

	// Check status code
	if response.Status != expect.Status {
		result.Passed = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Status code: expected %d, got %d", expect.Status, response.Status))
	}

	// Check backend calls (if specified)
	if expect.BackendCalls != nil {
		if backendCalls != *expect.BackendCalls {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Backend calls: expected %d, got %d", *expect.BackendCalls, backendCalls))
		}
	}

	// Check headers (if specified)
	for key, expectedValue := range expect.Headers {
		actualValue := response.Headers.Get(key)
		if actualValue != expectedValue {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Header %q: expected %q, got %q", key, expectedValue, actualValue))
		}
	}

	// Check body contains (if specified)
	if expect.BodyContains != "" {
		if !strings.Contains(response.Body, expect.BodyContains) {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Body should contain %q, but doesn't", expect.BodyContains))
		}
	}

	return result
}
