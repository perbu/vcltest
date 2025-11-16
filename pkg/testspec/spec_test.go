package testspec

import (
	"strings"
	"testing"
)

func TestParseTest_Valid(t *testing.T) {
	yaml := `
name: Basic test
vcl: basic.vcl
request:
  url: /test
expect:
  status: 200
`
	test, err := ParseTest(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("ParseTest failed: %v", err)
	}

	if test.Name != "Basic test" {
		t.Errorf("expected name 'Basic test', got '%s'", test.Name)
	}
	if test.VCL != "basic.vcl" {
		t.Errorf("expected vcl 'basic.vcl', got '%s'", test.VCL)
	}
	if test.Request.URL != "/test" {
		t.Errorf("expected url '/test', got '%s'", test.Request.URL)
	}
	if test.Expect.Status != 200 {
		t.Errorf("expected status 200, got %d", test.Expect.Status)
	}
}

func TestParseTest_WithDefaults(t *testing.T) {
	yaml := `
name: Test with defaults
vcl: test.vcl
request:
  url: /
expect:
  status: 404
`
	test, err := ParseTest(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("ParseTest failed: %v", err)
	}

	// Check request defaults
	if test.Request.Method != "GET" {
		t.Errorf("expected default method 'GET', got '%s'", test.Request.Method)
	}
	if test.Request.Headers == nil {
		t.Error("expected headers map to be initialized")
	}

	// Check backend defaults
	if test.Backend.Status != 200 {
		t.Errorf("expected default backend status 200, got %d", test.Backend.Status)
	}
	if test.Backend.Headers == nil {
		t.Error("expected backend headers map to be initialized")
	}
}

func TestParseTest_CustomValues(t *testing.T) {
	yaml := `
name: Custom test
vcl: custom.vcl
request:
  method: POST
  url: /api/data
  headers:
    Content-Type: application/json
  body: '{"key":"value"}'
backend:
  status: 201
  headers:
    X-Custom: header-value
  body: 'backend response'
expect:
  status: 201
  backend_calls: 1
  headers:
    X-Custom: header-value
  body_contains: 'response'
`
	test, err := ParseTest(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("ParseTest failed: %v", err)
	}

	// Check request
	if test.Request.Method != "POST" {
		t.Errorf("expected method 'POST', got '%s'", test.Request.Method)
	}
	if test.Request.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header")
	}
	if test.Request.Body != `{"key":"value"}` {
		t.Errorf("unexpected request body: %s", test.Request.Body)
	}

	// Check backend
	if test.Backend.Status != 201 {
		t.Errorf("expected backend status 201, got %d", test.Backend.Status)
	}
	if test.Backend.Headers["X-Custom"] != "header-value" {
		t.Errorf("expected X-Custom header in backend")
	}
	if test.Backend.Body != "backend response" {
		t.Errorf("unexpected backend body: %s", test.Backend.Body)
	}

	// Check expectations
	if test.Expect.Status != 201 {
		t.Errorf("expected status 201, got %d", test.Expect.Status)
	}
	if test.Expect.BackendCalls == nil || *test.Expect.BackendCalls != 1 {
		t.Errorf("expected backend_calls to be 1")
	}
	if test.Expect.Headers["X-Custom"] != "header-value" {
		t.Errorf("expected X-Custom header in expect")
	}
	if test.Expect.BodyContains == nil || *test.Expect.BodyContains != "response" {
		t.Errorf("expected body_contains 'response'")
	}
}

func TestParseTests_MultiDocument(t *testing.T) {
	yaml := `
name: Test 1
vcl: test1.vcl
request:
  url: /test1
expect:
  status: 200
---
name: Test 2
vcl: test2.vcl
request:
  url: /test2
expect:
  status: 404
---
name: Test 3
vcl: test3.vcl
request:
  url: /test3
expect:
  status: 301
`
	tests, err := ParseTests(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("ParseTests failed: %v", err)
	}

	if len(tests) != 3 {
		t.Fatalf("expected 3 tests, got %d", len(tests))
	}

	if tests[0].Name != "Test 1" || tests[0].Expect.Status != 200 {
		t.Errorf("test 1 not parsed correctly")
	}
	if tests[1].Name != "Test 2" || tests[1].Expect.Status != 404 {
		t.Errorf("test 2 not parsed correctly")
	}
	if tests[2].Name != "Test 3" || tests[2].Expect.Status != 301 {
		t.Errorf("test 3 not parsed correctly")
	}
}

func TestParseTest_MissingName(t *testing.T) {
	yaml := `
vcl: test.vcl
request:
  url: /
expect:
  status: 200
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseTest_MissingVCL(t *testing.T) {
	yaml := `
name: Test
request:
  url: /
expect:
  status: 200
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error for missing VCL")
	}
	if !strings.Contains(err.Error(), "VCL file path is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseTest_MissingURL(t *testing.T) {
	yaml := `
name: Test
vcl: test.vcl
expect:
  status: 200
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "request URL is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseTest_MissingExpectStatus(t *testing.T) {
	yaml := `
name: Test
vcl: test.vcl
request:
  url: /
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error for missing expect status")
	}
	if !strings.Contains(err.Error(), "expect.status is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseTest_InvalidYAML(t *testing.T) {
	yaml := `
name: Test
vcl: test.vcl
request: [this is not valid
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseTest_EmptyInput(t *testing.T) {
	_, err := ParseTest(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseTest_MultipleTestsError(t *testing.T) {
	yaml := `
name: Test 1
vcl: test.vcl
request:
  url: /
expect:
  status: 200
---
name: Test 2
vcl: test.vcl
request:
  url: /
expect:
  status: 404
`
	_, err := ParseTest(strings.NewReader(yaml))
	if err == nil {
		t.Error("expected error when multiple tests provided to ParseTest")
	}
	if !strings.Contains(err.Error(), "expected single test but found 2 tests") {
		t.Errorf("unexpected error message: %v", err)
	}
}
