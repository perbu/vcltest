package assertion

import (
	"fmt"
	"net/http"
	"strings"
)

// Result represents the result of an assertion
type Result struct {
	Type     string // status, backend_calls, headers, body_contains
	Passed   bool
	Expected string
	Actual   string
	Message  string
}

// Results is a collection of assertion results
type Results struct {
	results []Result
}

// NewResults creates a new assertion results collection
func NewResults() *Results {
	return &Results{
		results: make([]Result, 0),
	}
}

// Add adds a result to the collection
func (r *Results) Add(result Result) {
	r.results = append(r.results, result)
}

// All returns all results
func (r *Results) All() []Result {
	return r.results
}

// Passed returns true if all assertions passed
func (r *Results) Passed() bool {
	for _, result := range r.results {
		if !result.Passed {
			return false
		}
	}
	return true
}

// Failed returns all failed assertion results
func (r *Results) Failed() []Result {
	var failed []Result
	for _, result := range r.results {
		if !result.Passed {
			failed = append(failed, result)
		}
	}
	return failed
}

// AssertStatus checks if the HTTP status code matches the expected value
func AssertStatus(expected, actual int) Result {
	passed := expected == actual
	return Result{
		Type:     "status",
		Passed:   passed,
		Expected: fmt.Sprintf("%d", expected),
		Actual:   fmt.Sprintf("%d", actual),
		Message:  fmt.Sprintf("expected status %d, got %d", expected, actual),
	}
}

// AssertBackendCalls checks if the number of backend calls matches the expected value
func AssertBackendCalls(expected, actual int) Result {
	passed := expected == actual
	return Result{
		Type:     "backend_calls",
		Passed:   passed,
		Expected: fmt.Sprintf("%d", expected),
		Actual:   fmt.Sprintf("%d", actual),
		Message:  fmt.Sprintf("expected %d backend call(s), got %d", expected, actual),
	}
}

// AssertHeader checks if a specific header has the expected value
func AssertHeader(name, expected, actual string) Result {
	passed := expected == actual
	return Result{
		Type:     "header",
		Passed:   passed,
		Expected: fmt.Sprintf("%s: %s", name, expected),
		Actual:   fmt.Sprintf("%s: %s", name, actual),
		Message:  fmt.Sprintf("expected header '%s' to be '%s', got '%s'", name, expected, actual),
	}
}

// AssertHeaders checks if all expected headers match their values
func AssertHeaders(expected map[string]string, response *http.Response) []Result {
	var results []Result
	for name, expectedValue := range expected {
		actualValue := response.Header.Get(name)
		results = append(results, AssertHeader(name, expectedValue, actualValue))
	}
	return results
}

// AssertBodyContains checks if the response body contains the expected substring
func AssertBodyContains(expected, body string) Result {
	passed := strings.Contains(body, expected)
	message := fmt.Sprintf("expected body to contain '%s'", expected)
	if !passed {
		if len(body) > 100 {
			message = fmt.Sprintf("expected body to contain '%s', but body was: %s...", expected, body[:100])
		} else {
			message = fmt.Sprintf("expected body to contain '%s', but body was: %s", expected, body)
		}
	}
	return Result{
		Type:     "body_contains",
		Passed:   passed,
		Expected: expected,
		Actual:   body,
		Message:  message,
	}
}
