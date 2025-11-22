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
// backendsUsed is optional - only needed if backend_used assertion is specified
func Check(expect testspec.ExpectSpec, response *client.Response, backendCalls int, backendsUsed ...[]string) *Result {
	result := &Result{
		Passed: true,
		Errors: []string{},
	}

	// Extract backends used list (optional)
	var backends []string
	if len(backendsUsed) > 0 {
		backends = backendsUsed[0]
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

	// Check backend used (if specified)
	if expect.BackendUsed != "" {
		found := false
		for _, backend := range backends {
			if backend == expect.BackendUsed {
				found = true
				break
			}
		}
		if !found {
			if len(backends) == 0 {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Backend used: expected %q, but no backend was called", expect.BackendUsed))
			} else {
				result.Passed = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Backend used: expected %q, got %v", expect.BackendUsed, backends))
			}
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

	// Check cached status (if specified)
	if expect.Cached != nil {
		isCached := checkIfCached(response)
		if isCached != *expect.Cached {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Cached: expected %v, got %v", *expect.Cached, isCached))
		}
	}

	// Check Age header constraints (if specified)
	if expect.AgeGt != nil || expect.AgeLt != nil {
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
				// Check age_gt
				if expect.AgeGt != nil {
					if age <= *expect.AgeGt {
						result.Passed = false
						result.Errors = append(result.Errors,
							fmt.Sprintf("Age: expected > %d, got %d", *expect.AgeGt, age))
					}
				}
				// Check age_lt
				if expect.AgeLt != nil {
					if age >= *expect.AgeLt {
						result.Passed = false
						result.Errors = append(result.Errors,
							fmt.Sprintf("Age: expected < %d, got %d", *expect.AgeLt, age))
					}
				}
			}
		}
	}

	// Check stale status (if specified)
	// A response is considered stale if X-Varnish-Stale header is present
	// or if Warning header contains "110" (Response is Stale)
	if expect.Stale != nil {
		isStale := checkIfStale(response)
		if isStale != *expect.Stale {
			result.Passed = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Stale: expected %v, got %v", *expect.Stale, isStale))
		}
	}

	return result
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
