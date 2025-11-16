package assertion

import (
	"net/http"
	"strings"
	"testing"
)

func TestAssertStatus_Pass(t *testing.T) {
	result := AssertStatus(200, 200)
	if !result.Passed {
		t.Error("expected assertion to pass")
	}
	if result.Type != "status" {
		t.Errorf("expected type 'status', got '%s'", result.Type)
	}
	if result.Expected != "200" {
		t.Errorf("expected Expected to be '200', got '%s'", result.Expected)
	}
	if result.Actual != "200" {
		t.Errorf("expected Actual to be '200', got '%s'", result.Actual)
	}
}

func TestAssertStatus_Fail(t *testing.T) {
	result := AssertStatus(200, 404)
	if result.Passed {
		t.Error("expected assertion to fail")
	}
	if !strings.Contains(result.Message, "expected status 200, got 404") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestAssertBackendCalls_Pass(t *testing.T) {
	result := AssertBackendCalls(3, 3)
	if !result.Passed {
		t.Error("expected assertion to pass")
	}
	if result.Type != "backend_calls" {
		t.Errorf("expected type 'backend_calls', got '%s'", result.Type)
	}
}

func TestAssertBackendCalls_Fail(t *testing.T) {
	result := AssertBackendCalls(1, 2)
	if result.Passed {
		t.Error("expected assertion to fail")
	}
	if !strings.Contains(result.Message, "expected 1 backend call(s), got 2") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestAssertHeader_Pass(t *testing.T) {
	result := AssertHeader("Content-Type", "application/json", "application/json")
	if !result.Passed {
		t.Error("expected assertion to pass")
	}
	if result.Type != "header" {
		t.Errorf("expected type 'header', got '%s'", result.Type)
	}
}

func TestAssertHeader_Fail(t *testing.T) {
	result := AssertHeader("X-Custom", "expected", "actual")
	if result.Passed {
		t.Error("expected assertion to fail")
	}
	if !strings.Contains(result.Message, "expected header 'X-Custom' to be 'expected', got 'actual'") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestAssertHeaders(t *testing.T) {
	response := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"text/html"},
			"X-Custom":     []string{"value"},
		},
	}

	expected := map[string]string{
		"Content-Type": "text/html",
		"X-Custom":     "value",
	}

	results := AssertHeaders(expected, response)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if !result.Passed {
			t.Errorf("expected header assertion to pass: %s", result.Message)
		}
	}
}

func TestAssertHeaders_Fail(t *testing.T) {
	response := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"text/html"},
		},
	}

	expected := map[string]string{
		"Content-Type": "application/json",
		"X-Missing":    "value",
	}

	results := AssertHeaders(expected, response)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	failCount := 0
	for _, result := range results {
		if !result.Passed {
			failCount++
		}
	}

	if failCount != 2 {
		t.Errorf("expected 2 failures, got %d", failCount)
	}
}

func TestAssertBodyContains_Pass(t *testing.T) {
	body := "This is a test response body"
	result := AssertBodyContains("test response", body)
	if !result.Passed {
		t.Error("expected assertion to pass")
	}
	if result.Type != "body_contains" {
		t.Errorf("expected type 'body_contains', got '%s'", result.Type)
	}
}

func TestAssertBodyContains_Fail(t *testing.T) {
	body := "This is a test"
	result := AssertBodyContains("missing", body)
	if result.Passed {
		t.Error("expected assertion to fail")
	}
	if !strings.Contains(result.Message, "expected body to contain 'missing'") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestAssertBodyContains_LongBody(t *testing.T) {
	body := strings.Repeat("a", 200)
	result := AssertBodyContains("z", body)
	if result.Passed {
		t.Error("expected assertion to fail")
	}
	// Should truncate body in message
	if len(result.Message) > 200 {
		t.Errorf("message should be truncated, got length %d", len(result.Message))
	}
}

func TestResults_Add(t *testing.T) {
	results := NewResults()
	results.Add(AssertStatus(200, 200))
	results.Add(AssertStatus(404, 404))

	if len(results.All()) != 2 {
		t.Errorf("expected 2 results, got %d", len(results.All()))
	}
}

func TestResults_Passed(t *testing.T) {
	results := NewResults()
	results.Add(AssertStatus(200, 200))
	results.Add(AssertStatus(404, 404))

	if !results.Passed() {
		t.Error("expected all results to pass")
	}
}

func TestResults_Passed_WithFailure(t *testing.T) {
	results := NewResults()
	results.Add(AssertStatus(200, 200))
	results.Add(AssertStatus(404, 200))

	if results.Passed() {
		t.Error("expected results to have failures")
	}
}

func TestResults_Failed(t *testing.T) {
	results := NewResults()
	results.Add(AssertStatus(200, 200))
	results.Add(AssertStatus(404, 200))
	results.Add(AssertStatus(301, 302))

	failed := results.Failed()
	if len(failed) != 2 {
		t.Errorf("expected 2 failed results, got %d", len(failed))
	}
}

func TestResults_Failed_None(t *testing.T) {
	results := NewResults()
	results.Add(AssertStatus(200, 200))
	results.Add(AssertStatus(404, 404))

	failed := results.Failed()
	if len(failed) != 0 {
		t.Errorf("expected 0 failed results, got %d", len(failed))
	}
}
