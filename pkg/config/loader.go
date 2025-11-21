package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if err := applyDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("applying defaults: %w", err)
	}

	// Load license from file if specified
	if cfg.License.File != "" && cfg.License.Text == "" {
		licenseData, err := os.ReadFile(cfg.License.File)
		if err != nil {
			return nil, fmt.Errorf("reading license file %q: %w", cfg.License.File, err)
		}
		cfg.License.Text = string(licenseData)
	}

	return &cfg, nil
}

// validate checks that required fields are present and valid
func validate(cfg *Config) error {
	if cfg.VarnishadmPort == 0 {
		return fmt.Errorf("varnishadm_port is required")
	}
	if cfg.WorkDir == "" {
		return fmt.Errorf("work_dir is required")
	}
	if cfg.VarnishDir == "" {
		return fmt.Errorf("varnish_dir is required")
	}
	if cfg.Varnish.AdminPort == 0 {
		return fmt.Errorf("varnish.admin_port is required")
	}
	if cfg.VarnishadmPort != uint16(cfg.Varnish.AdminPort) {
		return fmt.Errorf("varnish.admin_port must match varnishadm_port (varnishd connects to varnishadm server)")
	}

	return nil
}

// applyDefaults sets default values for optional fields
func applyDefaults(cfg *Config) error {
	// Generate secret if not provided
	if cfg.Secret == "" {
		secret, err := generateSecret()
		if err != nil {
			return fmt.Errorf("generating secret: %w", err)
		}
		cfg.Secret = secret
	}

	// Default varnish command to "varnishd" for PATH lookup
	if cfg.VarnishCmd == "" {
		cfg.VarnishCmd = "varnishd"
	}

	return nil
}

// generateSecret creates a random 32-byte hex-encoded secret
func generateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
