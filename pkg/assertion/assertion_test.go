package assertion

import (
	"net/http"
	"testing"

	"github.com/perbu/vcltest/pkg/client"
	"github.com/perbu/vcltest/pkg/testspec"
)

func TestCheck_BackendSimpleString(t *testing.T) {
	// Test simple string format: backend: "api_server"
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Name: "api_server",
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// Backend was called
	backendCalls := map[string]int{
		"api_server": 1,
	}

	result := Check(expectations, response, backendCalls)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}

func TestCheck_BackendSimpleString_NotCalled(t *testing.T) {
	// Test simple string format when backend was not called
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Name: "api_server",
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// Backend was not called
	backendCalls := map[string]int{
		"api_server": 0,
	}

	result := Check(expectations, response, backendCalls)
	if result.Passed {
		t.Error("expected test to fail when backend was not called")
	}

	if len(result.Errors) == 0 {
		t.Error("expected error message")
	}
}

func TestCheck_BackendUsed(t *testing.T) {
	// Test object format with "used" field
	calls := 2
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Used:  "api_server",
			Calls: &calls,
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// Backend was called twice
	backendCalls := map[string]int{
		"api_server": 2,
	}

	result := Check(expectations, response, backendCalls)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}

func TestCheck_BackendCalls_TotalCount(t *testing.T) {
	// Test total call count across all backends
	calls := 3
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Calls: &calls,
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// Total of 3 calls across backends
	backendCalls := map[string]int{
		"api_server": 2,
		"web_server": 1,
	}

	result := Check(expectations, response, backendCalls)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}

func TestCheck_BackendCalls_WrongCount(t *testing.T) {
	// Test when total count doesn't match
	calls := 2
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Calls: &calls,
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// Actually 3 calls, but expected 2
	backendCalls := map[string]int{
		"api_server": 2,
		"web_server": 1,
	}

	result := Check(expectations, response, backendCalls)
	if result.Passed {
		t.Error("expected test to fail when call count doesn't match")
	}
}

func TestCheck_BackendPerBackend(t *testing.T) {
	// Test per-backend call counts
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			PerBackend: map[string]testspec.BackendCallExpectation{
				"api_server": {Calls: 1},
				"web_server": {Calls: 0},
			},
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	backendCalls := map[string]int{
		"api_server": 1,
		"web_server": 0,
	}

	result := Check(expectations, response, backendCalls)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}

func TestCheck_BackendPerBackend_Mismatch(t *testing.T) {
	// Test per-backend call counts with mismatch
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			PerBackend: map[string]testspec.BackendCallExpectation{
				"api_server": {Calls: 2},
				"web_server": {Calls: 0},
			},
		},
	}

	response := &client.Response{
		Status:  200,
		Headers: http.Header{},
		Body:    "",
	}

	// api_server only called once, not twice
	backendCalls := map[string]int{
		"api_server": 1,
		"web_server": 0,
	}

	result := Check(expectations, response, backendCalls)
	if result.Passed {
		t.Error("expected test to fail when per-backend count doesn't match")
	}

	if len(result.Errors) == 0 {
		t.Error("expected error message")
	}
}

func TestCheck_BackendCacheHit_ZeroCalls(t *testing.T) {
	// Test cache hit scenario with zero backend calls
	calls := 0
	hit := true
	expectations := testspec.ExpectationsSpec{
		Response: testspec.ResponseExpectations{
			Status: 200,
		},
		Backend: &testspec.BackendExpectations{
			Calls: &calls,
		},
		Cache: &testspec.CacheExpectations{
			Hit: &hit,
		},
	}

	response := &client.Response{
		Status: 200,
		Headers: http.Header{
			"X-Varnish": []string{"123 456"}, // Two VXIDs = cache hit
			"Age":       []string{"10"},
		},
		Body: "",
	}

	// No backend calls
	backendCalls := map[string]int{
		"api_server": 0,
	}

	result := Check(expectations, response, backendCalls)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}
