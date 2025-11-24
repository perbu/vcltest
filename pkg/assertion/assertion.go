package assertion

import (
	"fmt"
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
// backendsUsed is optional - only needed if backend.used assertion is specified
func Check(expectations testspec.ExpectationsSpec, response *client.Response, backendCalls int, backendsUsed ...[]string) *Result {
	result := &Result{
		Passed: true,
		Errors: []string{},
	}

	var backends []string
	if len(backendsUsed) > 0 {
		backends = backendsUsed[0]
	}

	// Response expectations (required)
	checkResponseExpectations(&expectations.Response, response, result)

	// Backend expectations (optional)
	if expectations.Backend != nil {
		checkBackendExpectations(expectations.Backend, backendCalls, backends, result)
	}

	// Cache expectations (optional)
	if expectations.Cache != nil {
		checkCacheExpectations(expectations.Cache, response, result)
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

func checkBackendExpectations(exp *testspec.BackendExpectations, backendCalls int, backendsUsed []string, result *Result) {
	if exp.Calls != nil {
		if backendCalls != *exp.Calls {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Backend calls: expected %d, got %d", *exp.Calls, backendCalls))
		}
	}

	if exp.Used != "" {
		found := false
		for _, backend := range backendsUsed {
			if backend == exp.Used {
				found = true
				break
			}
		}
		if !found {
			if len(backendsUsed) == 0 {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Backend used: expected %q, but no backend was called", exp.Used))
			} else {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Backend used: expected %q, got %v", exp.Used, backendsUsed))
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

	if exp.Stale != nil {
		isStale := checkIfStale(response)
		if isStale != *exp.Stale {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Stale: expected %v, got %v", *exp.Stale, isStale))
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

// checkIfStale determines if stale content was served
// Checks for X-Varnish-Stale header or Warning: 110 header
func checkIfStale(response *client.Response) bool {
	// Check for X-Varnish-Stale header (custom header that VCL might set)
	if response.Headers.Get("X-Varnish-Stale") != "" {
		return true
	}

	// Check for Warning: 110 (Response is Stale)
	warning := response.Headers.Get("Warning")
	if strings.Contains(warning, "110") {
		return true
	}

	return false
}
