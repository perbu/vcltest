package assertion

import (
	"net/http"
	"strings"
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
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

	result := Check(expectations, response, backendCalls, nil, nil)
	if !result.Passed {
		t.Errorf("expected test to pass, got errors: %v", result.Errors)
	}
}

func TestCheck_ResponseExpectations(t *testing.T) {
	tests := []struct {
		name           string
		responseExp    testspec.ResponseExpectations
		response       *client.Response
		expectPass     bool
		expectErrorStr string
	}{
		// Status expectations
		{
			name:        "status match",
			responseExp: testspec.ResponseExpectations{Status: 200},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{},
				Body:    "",
			},
			expectPass: true,
		},
		{
			name:        "status mismatch",
			responseExp: testspec.ResponseExpectations{Status: 200},
			response: &client.Response{
				Status:  404,
				Headers: http.Header{},
				Body:    "",
			},
			expectPass:     false,
			expectErrorStr: "Response status: expected 200, got 404",
		},

		// Header expectations
		{
			name: "header match",
			responseExp: testspec.ResponseExpectations{
				Status:  200,
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    "",
			},
			expectPass: true,
		},
		{
			name: "header mismatch",
			responseExp: testspec.ResponseExpectations{
				Status:  200,
				Headers: map[string]string{"X-Custom": "foo"},
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{"X-Custom": []string{"bar"}},
				Body:    "",
			},
			expectPass:     false,
			expectErrorStr: `Response header "X-Custom": expected "foo", got "bar"`,
		},
		{
			name: "header missing",
			responseExp: testspec.ResponseExpectations{
				Status:  200,
				Headers: map[string]string{"X-Custom": "foo"},
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{},
				Body:    "",
			},
			expectPass:     false,
			expectErrorStr: `Response header "X-Custom": expected "foo", got ""`,
		},
		{
			name: "multiple headers all match",
			responseExp: testspec.ResponseExpectations{
				Status: 200,
				Headers: map[string]string{
					"Content-Type":  "text/html",
					"Cache-Control": "max-age=3600",
				},
			},
			response: &client.Response{
				Status: 200,
				Headers: http.Header{
					"Content-Type":  []string{"text/html"},
					"Cache-Control": []string{"max-age=3600"},
				},
				Body: "",
			},
			expectPass: true,
		},

		// BodyContains expectations
		{
			name: "body contains match",
			responseExp: testspec.ResponseExpectations{
				Status:       200,
				BodyContains: "hello world",
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{},
				Body:    "This is a hello world example.",
			},
			expectPass: true,
		},
		{
			name: "body contains mismatch",
			responseExp: testspec.ResponseExpectations{
				Status:       200,
				BodyContains: "foobar",
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{},
				Body:    "This body has no match.",
			},
			expectPass:     false,
			expectErrorStr: `Response body should contain "foobar", but doesn't`,
		},
		{
			name: "body contains empty string (always passes)",
			responseExp: testspec.ResponseExpectations{
				Status:       200,
				BodyContains: "",
			},
			response: &client.Response{
				Status:  200,
				Headers: http.Header{},
				Body:    "anything",
			},
			expectPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectations := testspec.ExpectationsSpec{
				Response: tt.responseExp,
			}

			result := Check(expectations, tt.response, nil, nil, nil)

			if tt.expectPass && !result.Passed {
				t.Errorf("expected test to pass, got errors: %v", result.Errors)
			}
			if !tt.expectPass && result.Passed {
				t.Error("expected test to fail, but it passed")
			}
			if tt.expectErrorStr != "" && !result.Passed {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(err, tt.expectErrorStr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.expectErrorStr, result.Errors)
				}
			}
		})
	}
}

func TestCheck_CacheExpectations(t *testing.T) {
	// Helper to create bool pointer
	boolPtr := func(b bool) *bool { return &b }
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name           string
		cacheExp       *testspec.CacheExpectations
		headers        http.Header
		expectPass     bool
		expectErrorStr string // substring to check in errors
	}{
		// Cache hit expectations
		{
			name:     "cache hit expected, X-Varnish has two VXIDs",
			cacheExp: &testspec.CacheExpectations{Hit: boolPtr(true)},
			headers: http.Header{
				"X-Varnish": []string{"123 456"},
			},
			expectPass: true,
		},
		{
			name:     "cache hit expected via Age header",
			cacheExp: &testspec.CacheExpectations{Hit: boolPtr(true)},
			headers: http.Header{
				"Age": []string{"10"},
			},
			expectPass: true,
		},
		{
			name:           "cache hit expected but miss",
			cacheExp:       &testspec.CacheExpectations{Hit: boolPtr(true)},
			headers:        http.Header{"X-Varnish": []string{"123"}}, // single VXID = miss
			expectPass:     false,
			expectErrorStr: "Cache hit: expected true, got false",
		},
		{
			name:     "cache miss expected, single VXID",
			cacheExp: &testspec.CacheExpectations{Hit: boolPtr(false)},
			headers: http.Header{
				"X-Varnish": []string{"123"},
			},
			expectPass: true,
		},
		{
			name:           "cache miss expected but got hit",
			cacheExp:       &testspec.CacheExpectations{Hit: boolPtr(false)},
			headers:        http.Header{"X-Varnish": []string{"123 456"}},
			expectPass:     false,
			expectErrorStr: "Cache hit: expected false, got true",
		},

		// Age greater than expectations
		{
			name:     "age_gt satisfied",
			cacheExp: &testspec.CacheExpectations{AgeGt: intPtr(5)},
			headers: http.Header{
				"Age": []string{"10"},
			},
			expectPass: true,
		},
		{
			name:           "age_gt not satisfied - equal",
			cacheExp:       &testspec.CacheExpectations{AgeGt: intPtr(10)},
			headers:        http.Header{"Age": []string{"10"}},
			expectPass:     false,
			expectErrorStr: "Age: expected > 10, got 10",
		},
		{
			name:           "age_gt not satisfied - less",
			cacheExp:       &testspec.CacheExpectations{AgeGt: intPtr(10)},
			headers:        http.Header{"Age": []string{"5"}},
			expectPass:     false,
			expectErrorStr: "Age: expected > 10, got 5",
		},

		// Age less than expectations
		{
			name:     "age_lt satisfied",
			cacheExp: &testspec.CacheExpectations{AgeLt: intPtr(10)},
			headers: http.Header{
				"Age": []string{"5"},
			},
			expectPass: true,
		},
		{
			name:           "age_lt not satisfied - equal",
			cacheExp:       &testspec.CacheExpectations{AgeLt: intPtr(5)},
			headers:        http.Header{"Age": []string{"5"}},
			expectPass:     false,
			expectErrorStr: "Age: expected < 5, got 5",
		},
		{
			name:           "age_lt not satisfied - greater",
			cacheExp:       &testspec.CacheExpectations{AgeLt: intPtr(5)},
			headers:        http.Header{"Age": []string{"10"}},
			expectPass:     false,
			expectErrorStr: "Age: expected < 5, got 10",
		},

		// Combined age expectations
		{
			name: "age in range (gt and lt both satisfied)",
			cacheExp: &testspec.CacheExpectations{
				AgeGt: intPtr(5),
				AgeLt: intPtr(15),
			},
			headers:    http.Header{"Age": []string{"10"}},
			expectPass: true,
		},
		{
			name: "age outside range - too low",
			cacheExp: &testspec.CacheExpectations{
				AgeGt: intPtr(5),
				AgeLt: intPtr(15),
			},
			headers:        http.Header{"Age": []string{"3"}},
			expectPass:     false,
			expectErrorStr: "Age: expected > 5, got 3",
		},

		// Age header edge cases
		{
			name:           "age constraint with missing Age header",
			cacheExp:       &testspec.CacheExpectations{AgeGt: intPtr(5)},
			headers:        http.Header{},
			expectPass:     false,
			expectErrorStr: "Age header is missing",
		},
		{
			name:           "age constraint with invalid Age header",
			cacheExp:       &testspec.CacheExpectations{AgeGt: intPtr(5)},
			headers:        http.Header{"Age": []string{"not-a-number"}},
			expectPass:     false,
			expectErrorStr: "Age header is not a valid number",
		},

		// No cache expectations (nil)
		{
			name:       "nil cache expectations",
			cacheExp:   nil,
			headers:    http.Header{},
			expectPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectations := testspec.ExpectationsSpec{
				Response: testspec.ResponseExpectations{
					Status: 200,
				},
				Cache: tt.cacheExp,
			}

			response := &client.Response{
				Status:  200,
				Headers: tt.headers,
				Body:    "",
			}

			result := Check(expectations, response, nil, nil, nil)

			if tt.expectPass && !result.Passed {
				t.Errorf("expected test to pass, got errors: %v", result.Errors)
			}
			if !tt.expectPass && result.Passed {
				t.Error("expected test to fail, but it passed")
			}
			if tt.expectErrorStr != "" && result.Passed {
				t.Errorf("expected error containing %q, but test passed", tt.expectErrorStr)
			}
			if tt.expectErrorStr != "" && !result.Passed {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(err, tt.expectErrorStr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.expectErrorStr, result.Errors)
				}
			}
		})
	}
}
