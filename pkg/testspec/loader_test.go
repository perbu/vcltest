package testspec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBackendSpec_InvalidFailureMode(t *testing.T) {
	tests := []struct {
		name        string
		failureMode string
		wantErr     bool
	}{
		{"empty failure mode is valid", "", false},
		{"failed is valid", "failed", false},
		{"frozen is valid", "frozen", false},
		{"invalid mode", "invalid", true},
		{"typo fail", "fail", true},
		{"typo freeze", "freeze", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := BackendSpec{
				Status:      200,
				FailureMode: tt.failureMode,
			}
			err := validateBackendSpec(spec, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBackendSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_InvalidFailureMode(t *testing.T) {
	// Create a temporary test file with invalid failure_mode
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.yaml")

	content := `name: Test with invalid failure mode
request:
  url: /test
backends:
  default:
    failure_mode: invalid
expectations:
  response:
    status: 200
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = Load(testFile)
	if err == nil {
		t.Error("Expected error for invalid failure_mode, got nil")
	}
}

func TestLoad_ValidFailureMode(t *testing.T) {
	// Create a temporary test file with valid failure_mode
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.yaml")

	content := `name: Test with valid failure mode
request:
  url: /test
backends:
  default:
    failure_mode: failed
expectations:
  response:
    status: 503
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests, err := Load(testFile)
	if err != nil {
		t.Fatalf("Unexpected error for valid failure_mode: %v", err)
	}

	if len(tests) != 1 {
		t.Fatalf("Expected 1 test, got %d", len(tests))
	}

	if tests[0].Backends["default"].FailureMode != "failed" {
		t.Errorf("Expected failure_mode 'failed', got %q", tests[0].Backends["default"].FailureMode)
	}
}
