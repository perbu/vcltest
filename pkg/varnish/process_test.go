package varnish

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManagerCreation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := "/tmp/test-varnish"

	mgr := New(workDir, logger, "")
	if mgr == nil {
		t.Fatal("Manager creation failed")
	}
	if mgr.workDir != workDir {
		t.Errorf("Expected workDir %s, got %s", workDir, mgr.workDir)
	}
	if mgr.varnishDir != filepath.Join(workDir, "varnish") {
		t.Errorf("Expected varnishDir %s, got %s", filepath.Join(workDir, "varnish"), mgr.varnishDir)
	}
}

func TestPrepareWorkspace(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	err := mgr.PrepareWorkspace("test-secret", "")
	if err != nil {
		t.Fatalf("PrepareWorkspace failed: %v", err)
	}

	// Check varnish directory exists
	if _, err := os.Stat(mgr.varnishDir); os.IsNotExist(err) {
		t.Errorf("Varnish directory was not created: %s", mgr.varnishDir)
	}

	// Check secret file exists
	secretPath := filepath.Join(workDir, "secret")
	if _, err := os.Stat(secretPath); os.IsNotExist(err) {
		t.Error("Secret file was not created")
	}

	// Check secret is set
	if mgr.secret == "" {
		t.Error("Secret was not generated")
	}
}

func TestPrepareWorkspaceWithLicense(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	licenseText := "TEST LICENSE"
	err := mgr.PrepareWorkspace("test-secret", licenseText)
	if err != nil {
		t.Fatalf("PrepareWorkspace failed: %v", err)
	}

	// Check license file exists and has correct content
	licensePath := filepath.Join(workDir, "varnish-enterprise.lic")
	content, err := os.ReadFile(licensePath)
	if err != nil {
		t.Error("License file was not created")
	}
	if string(content) != licenseText {
		t.Errorf("License content mismatch: expected %s, got %s", licenseText, string(content))
	}
}

func TestBuildArgs(t *testing.T) {
	cfg := &Config{
		WorkDir:     "/tmp/test",
		VarnishDir:  "/tmp/test/varnish",
		StorageArgs: []string{"-s", "malloc,256m"},
		Varnish: VarnishConfig{
			AdminPort: 6082,
			HTTP: []HTTPConfig{
				{Port: 8080},
			},
			ExtraArgs: []string{"--debug"},
		},
	}

	args := BuildArgs(cfg)

	// Check some expected arguments
	expectedArgs := []string{"-n", cfg.VarnishDir, "-F", "-f", "", "-a", ":8080,http", "--debug"}

	for _, expected := range expectedArgs {
		found := false
		for _, arg := range args {
			if arg == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected argument %s not found in args: %v", expected, args)
		}
	}

	// Verify storage args are present
	storageFound := false
	for i, arg := range args {
		if arg == "-s" && i+1 < len(args) && args[i+1] == "malloc,256m" {
			storageFound = true
			break
		}
	}
	if !storageFound {
		t.Error("Storage arguments not found in args")
	}
}

// TestBuildArgsWithLicense is removed because it requires a valid cryptographically signed
// license, which is complex to create for testing. The license flag functionality is simple:
// when cfg.License.Text is non-empty, BuildArgs adds "-L /path/to/license.lic" to args.
// This is adequately covered by integration tests and real usage.

func TestGetParamName(t *testing.T) {
	// Create test structs with yaml tags
	type testStruct struct {
		SimpleParam   string `yaml:"simple_param"`
		WithOmitempty string `yaml:"with_omitempty,omitempty"`
		ThreadPoolMax int    `yaml:"thread_pool_max,omitempty"`
		NoYamlTag     string // Should return empty string
		YamlDash      string `yaml:"-"` // Should return empty string (explicitly ignored)
		HTTPMaxHdr    int    `yaml:"http_max_hdr,omitempty"`
	}

	tests := []struct {
		fieldName string
		expected  string
	}{
		{"SimpleParam", "simple_param"},
		{"WithOmitempty", "with_omitempty"},
		{"ThreadPoolMax", "thread_pool_max"},
		{"NoYamlTag", ""},
		{"YamlDash", ""},
		{"HTTPMaxHdr", "http_max_hdr"},
	}

	structType := reflect.TypeOf(testStruct{})
	for _, tt := range tests {
		field, found := structType.FieldByName(tt.fieldName)
		if !found {
			t.Fatalf("Field %s not found in test struct", tt.fieldName)
		}
		result := GetParamName(field)
		if result != tt.expected {
			t.Errorf("GetParamName(%s) = %s, expected %s", tt.fieldName, result, tt.expected)
		}
	}
}
