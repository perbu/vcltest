package assertion

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
// backendCalls is a map of backend name -> call count
// cookieJar and requestURL are optional (can be nil) - used for cookie expectations in scenarios
func Check(expectations testspec.ExpectationsSpec, response *client.Response, backendCalls map[string]int, cookieJar http.CookieJar, requestURL *url.URL) *Result {
	result := &Result{
		Passed: true,
		Errors: []string{},
	}

	// Response expectations (required)
	checkResponseExpectations(&expectations.Response, response, result)

	// Backend expectations (optional)
	if expectations.Backend != nil {
		checkBackendExpectations(expectations.Backend, backendCalls, result)
	}

	// Cache expectations (optional)
	if expectations.Cache != nil {
		checkCacheExpectations(expectations.Cache, response, result)
	}

	// Cookie expectations (optional)
	if len(expectations.Cookies) > 0 {
		checkCookieExpectations(expectations.Cookies, cookieJar, requestURL, result)
	}

	return result
}

func checkResponseExpectations(exp *testspec.ResponseExpectations, response *client.Response, result *Result) {
	if response.Status != exp.Status {
		result.Passed = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Response status: expected %d, got %d", exp.Status, response.Status))
	}

	for key, expectedValue := range exp.Headers {
		actualValue := response.Headers.Get(key)
		if actualValue != expectedValue {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Response header %q: expected %q, got %q", key, expectedValue, actualValue))
		}
	}

	if exp.BodyContains != "" {
		if !strings.Contains(response.Body, exp.BodyContains) {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Response body should contain %q, but doesn't", exp.BodyContains))
		}
	}
}

func checkBackendExpectations(exp *testspec.BackendExpectations, backendCalls map[string]int, result *Result) {
	// Format 1: Simple string (backend: "api_server")
	// Asserts that this backend was called at least once
	if exp.Name != "" {
		calls, found := backendCalls[exp.Name]
		if !found || calls == 0 {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Backend %q: expected to be called, but was not", exp.Name))
		}
		return
	}

	// Format 2: Object with Used and/or Calls
	if exp.Used != "" {
		calls, found := backendCalls[exp.Used]
		if !found || calls == 0 {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Backend %q: expected to be called, but was not", exp.Used))
		}
	}

	// Check total call count across all backends
	if exp.Calls != nil {
		totalCalls := 0
		for _, count := range backendCalls {
			totalCalls += count
		}
		if totalCalls != *exp.Calls {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Backend calls: expected %d total, got %d", *exp.Calls, totalCalls))
		}
	}

	// Format 3: Per-backend call counts (backends: {api_server: {calls: 1}})
	if len(exp.PerBackend) > 0 {
		for backendName, expectation := range exp.PerBackend {
			actualCalls := backendCalls[backendName]
			if actualCalls != expectation.Calls {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Backend %q calls: expected %d, got %d", backendName, expectation.Calls, actualCalls))
			}
		}
	}
}

func checkCacheExpectations(exp *testspec.CacheExpectations, response *client.Response, result *Result) {
	if exp.Hit != nil {
		isCached := checkIfCached(response)
		if isCached != *exp.Hit {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Cache hit: expected %v, got %v", *exp.Hit, isCached))
		}
	}

	if exp.AgeGt != nil || exp.AgeLt != nil {
		ageStr := response.Headers.Get("Age")
		if ageStr == "" {
			result.Passed = false
			result.Errors = append(result.Errors, "Age header is missing but age constraint specified")
		} else {
			age, err := strconv.Atoi(ageStr)
			if err != nil {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Age header is not a valid number: %q", ageStr))
			} else {
				if exp.AgeGt != nil {
					if age <= *exp.AgeGt {
						result.Passed = false
						result.Errors = append(result.Errors,
							fmt.Sprintf("Age: expected > %d, got %d", *exp.AgeGt, age))
					}
				}
				if exp.AgeLt != nil {
					if age >= *exp.AgeLt {
						result.Passed = false
						result.Errors = append(result.Errors,
							fmt.Sprintf("Age: expected < %d, got %d", *exp.AgeLt, age))
					}
				}
			}
		}
	}

}

// checkIfCached determines if a response was served from cache
// Uses X-Varnish header format: "VXID VXID" indicates cache hit (two VXIDs)
// and Age header presence (Age > 0 typically indicates cached)
func checkIfCached(response *client.Response) bool {
	// Check X-Varnish header
	xVarnish := response.Headers.Get("X-Varnish")
	if xVarnish != "" {
		// Format is "reqid" for miss, "reqid objid" for hit
		parts := strings.Fields(xVarnish)
		if len(parts) == 2 {
			return true // Cache hit
		}
	}

	// Check Age header as secondary indicator
	ageStr := response.Headers.Get("Age")
	if ageStr != "" {
		age, err := strconv.Atoi(ageStr)
		if err == nil && age > 0 {
			return true
		}
	}

	return false
}

// checkCookieExpectations validates expected cookies against the cookie jar
func checkCookieExpectations(expected map[string]string, jar http.CookieJar, requestURL *url.URL, result *Result) {
	if jar == nil {
		result.Passed = false
		result.Errors = append(result.Errors, "cookie expectations specified but no cookie jar available")
		return
	}

	if requestURL == nil {
		result.Passed = false
		result.Errors = append(result.Errors, "cookie expectations specified but no request URL available")
		return
	}

	// Get cookies from jar for this URL
	jarCookies := jar.Cookies(requestURL)
	jarMap := make(map[string]string)
	for _, c := range jarCookies {
		jarMap[c.Name] = c.Value
	}

	// Check each expected cookie
	for name, expectedValue := range expected {
		if actualValue, ok := jarMap[name]; !ok {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("cookie %q: expected in jar, but not present", name))
		} else if actualValue != expectedValue {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("cookie %q: expected %q, got %q", name, expectedValue, actualValue))
		}
	}
}
