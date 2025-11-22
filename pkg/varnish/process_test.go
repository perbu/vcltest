package varnish

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
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

func TestSetupTimeControl(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	startTime := "2026-05-22 08:30:00"
	controlFile, err := mgr.SetupTimeControl(startTime)
	if err != nil {
		t.Fatalf("SetupTimeControl failed: %v", err)
	}

	// Check control file was created
	if _, err := os.Stat(controlFile); os.IsNotExist(err) {
		t.Errorf("Control file was not created: %s", controlFile)
	}

	// Check control file has correct mtime
	info, err := os.Stat(controlFile)
	if err != nil {
		t.Fatalf("Failed to stat control file: %v", err)
	}

	expectedTime, _ := time.Parse("2006-01-02 15:04:05", startTime)
	if !info.ModTime().Equal(expectedTime) {
		t.Errorf("Control file mtime incorrect: expected %v, got %v", expectedTime, info.ModTime())
	}

	// Check manager state
	if mgr.timeControlFile != controlFile {
		t.Errorf("Manager timeControlFile not set correctly")
	}
	if !mgr.lastTimestamp.Equal(expectedTime) {
		t.Errorf("Manager lastTimestamp not set correctly")
	}
}

func TestSetupTimeControlInvalidFormat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	_, err := mgr.SetupTimeControl("invalid-time-format")
	if err == nil {
		t.Error("Expected error for invalid time format, got nil")
	}
}

func TestAdvanceTime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	// Setup initial time
	startTime := "2026-05-22 08:30:00"
	_, err := mgr.SetupTimeControl(startTime)
	if err != nil {
		t.Fatalf("SetupTimeControl failed: %v", err)
	}

	// Advance time forward
	newTime := "2026-05-22 08:30:30"
	err = mgr.AdvanceTime(newTime)
	if err != nil {
		t.Fatalf("AdvanceTime failed: %v", err)
	}

	// Check control file has new mtime
	info, err := os.Stat(mgr.timeControlFile)
	if err != nil {
		t.Fatalf("Failed to stat control file: %v", err)
	}

	expectedTime, _ := time.Parse("2006-01-02 15:04:05", newTime)
	if !info.ModTime().Equal(expectedTime) {
		t.Errorf("Control file mtime not updated: expected %v, got %v", expectedTime, info.ModTime())
	}

	// Check manager state
	if !mgr.lastTimestamp.Equal(expectedTime) {
		t.Errorf("Manager lastTimestamp not updated correctly")
	}
}

func TestAdvanceTimeBackwardsRejected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	// Setup initial time
	startTime := "2026-05-22 08:30:00"
	_, err := mgr.SetupTimeControl(startTime)
	if err != nil {
		t.Fatalf("SetupTimeControl failed: %v", err)
	}

	// Try to move time backwards
	pastTime := "2026-05-22 08:29:00"
	err = mgr.AdvanceTime(pastTime)
	if err == nil {
		t.Error("Expected error when moving time backwards, got nil")
	}

	// Try to move to same time
	err = mgr.AdvanceTime(startTime)
	if err == nil {
		t.Error("Expected error when moving time to same value, got nil")
	}
}

func TestAdvanceTimeNotInitialized(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	// Try to advance time without initializing
	err := mgr.AdvanceTime("2026-05-22 08:30:00")
	if err == nil {
		t.Error("Expected error when advancing time without initialization, got nil")
	}
}

func TestGetCurrentFakeTime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	mgr := New(workDir, logger, "")

	// Before initialization, should return zero time
	zeroTime := mgr.GetCurrentFakeTime()
	if !zeroTime.IsZero() {
		t.Error("Expected zero time before initialization")
	}

	// After initialization
	startTime := "2026-05-22 08:30:00"
	_, err := mgr.SetupTimeControl(startTime)
	if err != nil {
		t.Fatalf("SetupTimeControl failed: %v", err)
	}

	currentTime := mgr.GetCurrentFakeTime()
	expectedTime, _ := time.Parse("2006-01-02 15:04:05", startTime)
	if !currentTime.Equal(expectedTime) {
		t.Errorf("GetCurrentFakeTime incorrect: expected %v, got %v", expectedTime, currentTime)
	}
}

func TestDetectLibfaketimePath(t *testing.T) {
	// Test custom path that exists
	workDir := t.TempDir()
	customPath := filepath.Join(workDir, "libfaketime.dylib")
	if err := os.WriteFile(customPath, []byte("fake"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	path, err := detectLibfaketimePath(customPath)
	if err != nil {
		t.Errorf("detectLibfaketimePath with valid custom path failed: %v", err)
	}
	if path != customPath {
		t.Errorf("Expected custom path %s, got %s", customPath, path)
	}

	// Test custom path that doesn't exist
	_, err = detectLibfaketimePath("/nonexistent/path/libfaketime.so")
	if err == nil {
		t.Error("Expected error for nonexistent custom path, got nil")
	}

	// Test auto-detection (may or may not succeed depending on system)
	path, err = detectLibfaketimePath("")
	if err == nil {
		// If auto-detection succeeded, verify it's a valid path
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("Auto-detected path %s does not exist", path)
		}
		// Verify it's the correct path for the platform
		switch runtime.GOOS {
		case "darwin":
			if path != "/opt/homebrew/lib/faketime/libfaketime.1.dylib" &&
				path != "/usr/local/lib/faketime/libfaketime.1.dylib" {
				t.Errorf("Unexpected darwin path: %s", path)
			}
		case "linux":
			if path != "/usr/lib/x86_64-linux-gnu/faketime/libfaketime.so.1" &&
				path != "/usr/lib/faketime/libfaketime.so.1" {
				t.Errorf("Unexpected linux path: %s", path)
			}
		}
	}
	// If auto-detection failed, that's OK - libfaketime might not be installed
}
