package vclmod

import (
	"strings"
	"testing"
)

// TestValidateAndModifyBackends_Success tests the combined operation with valid backends
func TestValidateAndModifyBackends_Success(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}

backend web {
    .host = "web.example.com";
    .port = "80";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
		"web": {Host: "127.0.0.1", Port: "8002"},
	}

	modified, result, err := ValidateAndModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ValidateAndModifyBackends failed: %v", err)
	}

	// Check validation result
	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
	if len(result.Warnings) > 0 {
		t.Errorf("Expected no warnings, got: %v", result.Warnings)
	}

	// Check that both backends were modified
	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host")
	}
	if !strings.Contains(modified, `"8001"`) {
		t.Errorf("Modified VCL doesn't contain expected port 8001")
	}
	if !strings.Contains(modified, `"8002"`) {
		t.Errorf("Modified VCL doesn't contain expected port 8002")
	}

	// Ensure original hosts are gone
	if strings.Contains(modified, "api.example.com") {
		t.Errorf("Modified VCL still contains original api host")
	}
	if strings.Contains(modified, "web.example.com") {
		t.Errorf("Modified VCL still contains original web host")
	}
}

// TestValidateAndModifyBackends_ValidationError tests error handling for missing backends
func TestValidateAndModifyBackends_ValidationError(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}
`

	backends := map[string]BackendAddress{
		"api":         {Host: "127.0.0.1", Port: "8001"},
		"nonexistent": {Host: "127.0.0.1", Port: "9999"},
	}

	_, result, err := ValidateAndModifyBackends(vclContent, "test.vcl", backends)
	if err == nil {
		t.Fatal("Expected error for nonexistent backend, got nil")
	}

	if result == nil {
		t.Fatal("Expected validation result even on error")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors, got none")
	}

	// Check error message contains helpful info
	errorStr := strings.Join(result.Errors, " ")
	if !strings.Contains(errorStr, "nonexistent") {
		t.Errorf("Error should mention the missing backend name")
	}
}

// TestValidateAndModifyBackends_UnusedVCLBackend tests warning for unused backends
func TestValidateAndModifyBackends_UnusedVCLBackend(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}

backend unused {
    .host = "unused.example.com";
    .port = "80";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
	}

	modified, result, err := ValidateAndModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ValidateAndModifyBackends failed: %v", err)
	}

	// Check that we got a warning about unused backend
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning about unused backend, got %d", len(result.Warnings))
	}

	if len(result.Warnings) > 0 && !strings.Contains(result.Warnings[0], "unused") {
		t.Errorf("Warning should mention the unused backend: %v", result.Warnings[0])
	}

	// Check that api backend was modified
	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host")
	}
	if !strings.Contains(modified, `"8001"`) {
		t.Errorf("Modified VCL doesn't contain expected port")
	}

	// Unused backend should remain unchanged
	if !strings.Contains(modified, "unused.example.com") {
		t.Errorf("Unused backend should remain in VCL with original host")
	}
}

// TestModifyBackends_PerfectMatch tests modifying all backends with perfect match
func TestModifyBackends_PerfectMatch(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}

backend web {
    .host = "web.example.com";
    .port = "80";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
		"web": {Host: "127.0.0.1", Port: "8002"},
	}

	modified, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ModifyBackends failed: %v", err)
	}

	// Check that both backends were modified
	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host")
	}
	if !strings.Contains(modified, `"8001"`) {
		t.Errorf("Modified VCL doesn't contain expected port 8001")
	}
	if !strings.Contains(modified, `"8002"`) {
		t.Errorf("Modified VCL doesn't contain expected port 8002")
	}

	// Ensure original hosts are gone
	if strings.Contains(modified, "api.example.com") {
		t.Errorf("Modified VCL still contains original api host")
	}
	if strings.Contains(modified, "web.example.com") {
		t.Errorf("Modified VCL still contains original web host")
	}
}

// TestModifyBackends_SingleBackend tests modifying a single backend
func TestModifyBackends_SingleBackend(t *testing.T) {
	vclContent := `vcl 4.1;

backend default {
    .host = "origin.example.com";
    .port = "80";
}
`

	backends := map[string]BackendAddress{
		"default": {Host: "127.0.0.1", Port: "9000"},
	}

	modified, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ModifyBackends failed: %v", err)
	}

	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host")
	}
	if !strings.Contains(modified, `"9000"`) {
		t.Errorf("Modified VCL doesn't contain expected port")
	}
	if strings.Contains(modified, "origin.example.com") {
		t.Errorf("Modified VCL still contains original host")
	}
}

// TestModifyBackends_MissingPort tests adding port when it doesn't exist
func TestModifyBackends_MissingPort(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8080"},
	}

	modified, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ModifyBackends failed: %v", err)
	}

	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host")
	}
	if !strings.Contains(modified, `"8080"`) {
		t.Errorf("Modified VCL doesn't contain expected port (should be added)")
	}
}

// TestModifyBackends_PartialModification tests modifying only some backends
func TestModifyBackends_PartialModification(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}

backend legacy {
    .host = "legacy.example.com";
    .port = "8080";
}
`

	// Only modify api backend, not legacy
	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "9001"},
	}

	modified, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ModifyBackends failed: %v", err)
	}

	// api should be modified
	if !strings.Contains(modified, `"127.0.0.1"`) {
		t.Errorf("Modified VCL doesn't contain expected host for api")
	}
	if !strings.Contains(modified, `"9001"`) {
		t.Errorf("Modified VCL doesn't contain expected port for api")
	}

	// legacy should remain unchanged
	if !strings.Contains(modified, "legacy.example.com") {
		t.Errorf("Modified VCL should still contain legacy backend host")
	}
}

// TestValidateBackends_AllMatch tests validation when all backends match
func TestValidateBackends_AllMatch(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
}

backend web {
    .host = "web.example.com";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
		"web": {Host: "127.0.0.1", Port: "8002"},
	}

	result, err := ValidateBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ValidateBackends should not error on perfect match: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
	if len(result.Warnings) > 0 {
		t.Errorf("Expected no warnings, got: %v", result.Warnings)
	}
}

// TestValidateBackends_YAMLBackendNotInVCL tests error when YAML backend doesn't exist in VCL
func TestValidateBackends_YAMLBackendNotInVCL(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
}
`

	backends := map[string]BackendAddress{
		"api":      {Host: "127.0.0.1", Port: "8001"},
		"nonexist": {Host: "127.0.0.1", Port: "8002"}, // This doesn't exist in VCL
	}

	result, err := ValidateBackends(vclContent, "test.vcl", backends)
	if err == nil {
		t.Fatal("ValidateBackends should error when YAML backend not in VCL")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation error for nonexistent backend")
	}

	// Check error message contains helpful info
	errorMsg := strings.Join(result.Errors, " ")
	if !strings.Contains(errorMsg, "nonexist") {
		t.Errorf("Error message should mention the missing backend: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "Available backends") {
		t.Errorf("Error message should list available backends: %s", errorMsg)
	}
}

// TestValidateBackends_VCLBackendNotInYAML tests warning when VCL backend not used
func TestValidateBackends_VCLBackendNotInYAML(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
}

backend unused {
    .host = "unused.example.com";
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
		// "unused" backend is not in YAML
	}

	result, err := ValidateBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ValidateBackends should not error when VCL backend unused: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Error("Expected warning for unused VCL backend")
	}

	// Check warning message
	warningMsg := strings.Join(result.Warnings, " ")
	if !strings.Contains(warningMsg, "unused") {
		t.Errorf("Warning should mention unused backend: %s", warningMsg)
	}
}

// TestModifyBackends_PreserveStructure tests that VCL structure is preserved
func TestModifyBackends_PreserveStructure(t *testing.T) {
	vclContent := `vcl 4.1;

backend api {
    .host = "api.example.com";
    .port = "443";
}

sub vcl_recv {
    set req.backend_hint = api;
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
	}

	modified, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err != nil {
		t.Fatalf("ModifyBackends failed: %v", err)
	}

	// Check that VCL structure is preserved
	if !strings.Contains(modified, "vcl 4.1") {
		t.Error("Modified VCL should preserve version declaration")
	}
	if !strings.Contains(modified, "sub vcl_recv") {
		t.Error("Modified VCL should preserve subroutine")
	}
	if !strings.Contains(modified, "req.backend_hint = api") {
		t.Error("Modified VCL should preserve backend assignment")
	}
}

// TestModifyBackends_InvalidVCL tests handling of invalid VCL
func TestModifyBackends_InvalidVCL(t *testing.T) {
	vclContent := `this is not valid VCL`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
	}

	_, err := ModifyBackends(vclContent, "test.vcl", backends)
	if err == nil {
		t.Fatal("ModifyBackends should error on invalid VCL")
	}
}

// TestValidateBackends_EmptyVCL tests validation with VCL that has no backends
func TestValidateBackends_EmptyVCL(t *testing.T) {
	vclContent := `vcl 4.1;

sub vcl_recv {
    return (pass);
}
`

	backends := map[string]BackendAddress{
		"api": {Host: "127.0.0.1", Port: "8001"},
	}

	result, err := ValidateBackends(vclContent, "test.vcl", backends)
	if err == nil {
		t.Fatal("ValidateBackends should error when no backends in VCL")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error about missing backends")
	}

	errorMsg := strings.Join(result.Errors, " ")
	if !strings.Contains(errorMsg, "No backends found") {
		t.Errorf("Error should mention no backends found: %s", errorMsg)
	}
}

// TestFindClosestMatch tests the suggestion algorithm
func TestFindClosestMatch(t *testing.T) {
	tests := []struct {
		target     string
		candidates []string
		expected   string
	}{
		{"api", []string{"api", "web"}, "api"},
		{"API", []string{"api", "web"}, "api"},
		{"api_server", []string{"api", "web"}, "api"},
		{"backend_api", []string{"api", "web"}, "api"},
		{"xyz", []string{"api", "web"}, ""}, // No match
	}

	for _, tt := range tests {
		result := findClosestMatch(tt.target, tt.candidates)
		if result != tt.expected {
			t.Errorf("findClosestMatch(%q, %v) = %q, want %q",
				tt.target, tt.candidates, result, tt.expected)
		}
	}
}
