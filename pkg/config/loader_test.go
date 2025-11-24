package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configYAML := `
varnishadm_port: 6082
secret: "mysecret"
varnish_cmd: "/usr/local/bin/varnishd"
work_dir: "/tmp/vcltest"
varnish_dir: "/var/lib/varnish/test"
varnish:
  admin_port: 6082
  http:
    - port: 8080
    - address: "127.0.0.1"
      port: 8081
`

	if err := os.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.VarnishadmPort != 6082 {
		t.Errorf("VarnishadmPort = %d, want 6082", cfg.VarnishadmPort)
	}
	if cfg.Secret != "mysecret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "mysecret")
	}
	if cfg.VarnishCmd != "/usr/local/bin/varnishd" {
		t.Errorf("VarnishCmd = %q, want %q", cfg.VarnishCmd, "/usr/local/bin/varnishd")
	}
	if cfg.WorkDir != "/tmp/vcltest" {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, "/tmp/vcltest")
	}
	if cfg.VarnishDir != "/var/lib/varnish/test" {
		t.Errorf("VarnishDir = %q, want %q", cfg.VarnishDir, "/var/lib/varnish/test")
	}
	if cfg.Varnish.AdminPort != 6082 {
		t.Errorf("Varnish.AdminPort = %d, want 6082", cfg.Varnish.AdminPort)
	}
	if len(cfg.Varnish.HTTP) != 2 {
		t.Errorf("len(Varnish.HTTP) = %d, want 2", len(cfg.Varnish.HTTP))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "reading config file") {
		t.Errorf("Load() error = %v, want 'reading config file' error", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
varnishadm_port: 6082
secret: "unclosed string
varnish_cmd: /usr/bin/varnishd
`

	if err := os.WriteFile(configFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := Load(configFile)
	if err == nil {
		t.Error("Load() expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing config file") {
		t.Errorf("Load() error = %v, want 'parsing config file' error", err)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError string
	}{
		{
			name: "Missing varnishadm_port",
			config: Config{
				WorkDir:    "/tmp",
				VarnishDir: "/var/lib/varnish",
				Varnish: VarnishConfig{
					AdminPort: 6082,
				},
			},
			expectError: "varnishadm_port is required",
		},
		{
			name: "Missing work_dir",
			config: Config{
				VarnishadmPort: 6082,
				VarnishDir:     "/var/lib/varnish",
				Varnish: VarnishConfig{
					AdminPort: 6082,
				},
			},
			expectError: "work_dir is required",
		},
		{
			name: "Missing varnish_dir",
			config: Config{
				VarnishadmPort: 6082,
				WorkDir:        "/tmp",
				Varnish: VarnishConfig{
					AdminPort: 6082,
				},
			},
			expectError: "varnish_dir is required",
		},
		{
			name: "Missing varnish.admin_port",
			config: Config{
				VarnishadmPort: 6082,
				WorkDir:        "/tmp",
				VarnishDir:     "/var/lib/varnish",
			},
			expectError: "varnish.admin_port is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.config)
			if err == nil {
				t.Error("validate() expected error")
			}
			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("validate() error = %v, want error containing %q", err, tt.expectError)
			}
		})
	}
}

func TestValidate_PortMismatch(t *testing.T) {
	cfg := Config{
		VarnishadmPort: 6082,
		WorkDir:        "/tmp",
		VarnishDir:     "/var/lib/varnish",
		Varnish: VarnishConfig{
			AdminPort: 7082, // Different port
		},
	}

	err := validate(&cfg)
	if err == nil {
		t.Error("validate() expected error for port mismatch")
	}
	if !strings.Contains(err.Error(), "admin_port must match varnishadm_port") {
		t.Errorf("validate() error = %v, want 'admin_port must match' error", err)
	}
}

func TestApplyDefaults_GeneratesSecret(t *testing.T) {
	cfg := Config{
		VarnishadmPort: 6082,
		WorkDir:        "/tmp",
		VarnishDir:     "/var/lib/varnish",
		Varnish: VarnishConfig{
			AdminPort: 6082,
		},
	}

	err := applyDefaults(&cfg)
	if err != nil {
		t.Fatalf("applyDefaults() error = %v", err)
	}

	if cfg.Secret == "" {
		t.Error("applyDefaults() should generate a secret when empty")
	}

	// Secret should be hex-encoded 32 bytes = 64 hex chars
	if len(cfg.Secret) != 64 {
		t.Errorf("applyDefaults() secret length = %d, want 64", len(cfg.Secret))
	}
}

func TestApplyDefaults_PreservesExistingSecret(t *testing.T) {
	cfg := Config{
		VarnishadmPort: 6082,
		Secret:         "existing-secret",
		WorkDir:        "/tmp",
		VarnishDir:     "/var/lib/varnish",
		Varnish: VarnishConfig{
			AdminPort: 6082,
		},
	}

	err := applyDefaults(&cfg)
	if err != nil {
		t.Fatalf("applyDefaults() error = %v", err)
	}

	if cfg.Secret != "existing-secret" {
		t.Errorf("applyDefaults() changed existing secret to %q", cfg.Secret)
	}
}

func TestApplyDefaults_VarnishCmd(t *testing.T) {
	cfg := Config{
		VarnishadmPort: 6082,
		WorkDir:        "/tmp",
		VarnishDir:     "/var/lib/varnish",
		Varnish: VarnishConfig{
			AdminPort: 6082,
		},
	}

	err := applyDefaults(&cfg)
	if err != nil {
		t.Fatalf("applyDefaults() error = %v", err)
	}

	if cfg.VarnishCmd != "varnishd" {
		t.Errorf("applyDefaults() VarnishCmd = %q, want %q", cfg.VarnishCmd, "varnishd")
	}
}

func TestApplyDefaults_PreservesExistingVarnishCmd(t *testing.T) {
	cfg := Config{
		VarnishadmPort: 6082,
		VarnishCmd:     "/custom/path/varnishd",
		WorkDir:        "/tmp",
		VarnishDir:     "/var/lib/varnish",
		Varnish: VarnishConfig{
			AdminPort: 6082,
		},
	}

	err := applyDefaults(&cfg)
	if err != nil {
		t.Fatalf("applyDefaults() error = %v", err)
	}

	if cfg.VarnishCmd != "/custom/path/varnishd" {
		t.Errorf("applyDefaults() changed existing VarnishCmd to %q", cfg.VarnishCmd)
	}
}

func TestLoad_LicenseFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	licenseFile := filepath.Join(tmpDir, "license.txt")

	// Write license file
	licenseContent := "VARNISH-ENTERPRISE-LICENSE-1234567890"
	if err := os.WriteFile(licenseFile, []byte(licenseContent), 0644); err != nil {
		t.Fatalf("Failed to write license file: %v", err)
	}

	configYAML := `
varnishadm_port: 6082
work_dir: "/tmp/vcltest"
varnish_dir: "/var/lib/varnish/test"
license:
  file: "` + licenseFile + `"
varnish:
  admin_port: 6082
`

	if err := os.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.License.Text != licenseContent {
		t.Errorf("License.Text = %q, want %q", cfg.License.Text, licenseContent)
	}
}

func TestLoad_LicenseFromText(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	licenseContent := "INLINE-LICENSE-CONTENT"
	configYAML := `
varnishadm_port: 6082
work_dir: "/tmp/vcltest"
varnish_dir: "/var/lib/varnish/test"
license:
  text: "` + licenseContent + `"
varnish:
  admin_port: 6082
`

	if err := os.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.License.Text != licenseContent {
		t.Errorf("License.Text = %q, want %q", cfg.License.Text, licenseContent)
	}
}

func TestLoad_LicenseFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configYAML := `
varnishadm_port: 6082
work_dir: "/tmp/vcltest"
varnish_dir: "/var/lib/varnish/test"
license:
  file: "/nonexistent/license.txt"
varnish:
  admin_port: 6082
`

	if err := os.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := Load(configFile)
	if err == nil {
		t.Error("Load() expected error for missing license file")
	}
	if !strings.Contains(err.Error(), "reading license file") {
		t.Errorf("Load() error = %v, want 'reading license file' error", err)
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}

	// Should be 64 hex characters (32 bytes)
	if len(secret1) != 64 {
		t.Errorf("generateSecret() length = %d, want 64", len(secret1))
	}

	// Should be different on each call
	secret2, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}

	if secret1 == secret2 {
		t.Error("generateSecret() should generate different secrets on each call")
	}

	// Should be valid hex
	for _, c := range secret1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateSecret() contains invalid hex character: %c", c)
		}
	}
}

func TestLoad_CompleteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configYAML := `
varnishadm_port: 6082
secret: "test-secret"
varnish_cmd: "/usr/bin/varnishd"
work_dir: "/tmp/vcltest"
varnish_dir: "/var/lib/varnish/test"
storage_args:
  - "malloc,256m"
  - "file,/tmp/cache.bin,1G"
license:
  text: "LICENSE-DATA"
varnish:
  admin_port: 6082
  http:
    - port: 8080
    - address: "127.0.0.1"
      port: 8081
  https:
    - port: 8443
  extra_args:
    - "-p"
    - "feature=+trace"
`

	if err := os.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all fields
	if cfg.VarnishadmPort != 6082 {
		t.Errorf("VarnishadmPort = %d, want 6082", cfg.VarnishadmPort)
	}
	if cfg.Secret != "test-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "test-secret")
	}
	if len(cfg.StorageArgs) != 2 {
		t.Errorf("len(StorageArgs) = %d, want 2", len(cfg.StorageArgs))
	}
	if len(cfg.Varnish.HTTP) != 2 {
		t.Errorf("len(Varnish.HTTP) = %d, want 2", len(cfg.Varnish.HTTP))
	}
	if len(cfg.Varnish.HTTPS) != 1 {
		t.Errorf("len(Varnish.HTTPS) = %d, want 1", len(cfg.Varnish.HTTPS))
	}
	if len(cfg.Varnish.ExtraArgs) != 2 {
		t.Errorf("len(Varnish.ExtraArgs) = %d, want 2", len(cfg.Varnish.ExtraArgs))
	}
	if cfg.License.Text != "LICENSE-DATA" {
		t.Errorf("License.Text = %q, want %q", cfg.License.Text, "LICENSE-DATA")
	}
}
