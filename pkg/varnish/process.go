package varnish

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// Manager manages the varnishd process lifecycle
type Manager struct {
	workDir    string
	varnishDir string
	secret     string
	logger     *slog.Logger
}

// New creates a new Varnish manager
// If customVarnishDir is empty, defaults to workDir/varnish
func New(workDir string, logger *slog.Logger, customVarnishDir string) *Manager {
	varnishDir := customVarnishDir
	if varnishDir == "" {
		varnishDir = filepath.Join(workDir, "varnish")
	}
	return &Manager{
		workDir:    workDir,
		varnishDir: varnishDir,
		logger:     logger,
	}
}

// PrepareWorkspace sets up the varnish directory, secret file, and license file
func (m *Manager) PrepareWorkspace(secret, licenseText string) error {
	// Create work directory for secret and license files
	if err := os.MkdirAll(m.workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory %s: %w", m.workDir, err)
	}

	// Create VCL directory for vcl_path parameter
	vclDir := filepath.Join(m.workDir, "vcl")
	if err := os.MkdirAll(vclDir, 0755); err != nil {
		return fmt.Errorf("failed to create VCL directory %s: %w", vclDir, err)
	}

	// Create varnish directory with permissions that allow Varnish to read after dropping privileges
	if err := os.MkdirAll(m.varnishDir, 0755); err != nil {
		return fmt.Errorf("failed to create varnish directory %s: %w", m.varnishDir, err)
	}
	if err := os.Chmod(m.varnishDir, 0755); err != nil {
		return fmt.Errorf("failed to set permissions on varnish directory %s: %w", m.varnishDir, err)
	}

	// Write secret file for varnishadm authentication
	if err := m.writeSecretFile(secret); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	// Write Varnish Enterprise license file if present
	if err := m.writeLicenseFile(licenseText); err != nil {
		return fmt.Errorf("failed to write license file: %w", err)
	}

	m.logger.Debug("Varnish workspace prepared", "varnish_dir", m.varnishDir)
	return nil
}

// writeSecretFile writes the provided secret to the secret file
func (m *Manager) writeSecretFile(secret string) error {
	// Store the secret for later use
	m.secret = secret

	// Write secret to file with restrictive permissions
	secretPath := filepath.Join(m.workDir, "secret")
	if err := os.WriteFile(secretPath, []byte(secret), 0600); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	m.logger.Debug("Wrote varnishadm secret file", "path", secretPath)
	return nil
}

// writeLicenseFile writes the Varnish Enterprise license to disk if present
func (m *Manager) writeLicenseFile(licenseText string) error {
	if licenseText == "" {
		m.logger.Debug("No license text provided, skipping license file creation")
		return nil
	}
	licensePath := filepath.Join(m.workDir, "varnish-enterprise.lic")
	if err := os.WriteFile(licensePath, []byte(licenseText), 0644); err != nil {
		return fmt.Errorf("failed to write license file: %w", err)
	}

	m.logger.Info("Wrote Varnish Enterprise license file", "path", licensePath)
	return nil
}

// Start starts the varnishd process with the given arguments
func (m *Manager) Start(ctx context.Context, varnishCmd string, args []string) error {
	// Find varnishd executable if not specified
	if varnishCmd == "" {
		var err error
		varnishCmd, err = exec.LookPath("varnishd")
		if err != nil {
			return fmt.Errorf("varnishd not found in PATH: %w", err)
		}
	}

	m.logger.Info("Starting varnishd", "cmd", varnishCmd, "args", args)

	// Create the command, ctx lets us cancel and kill varnishd
	cmd := exec.CommandContext(ctx, varnishCmd, args...)
	cmd.Dir = m.varnishDir

	// Inherit environment variables so VMOD otel can read OTEL_* configuration
	cmd.Env = os.Environ()

	// Route varnishd output through our structured logging
	cmd.Stdout = newLogWriter(m.logger, "varnishd")
	cmd.Stderr = newLogWriter(m.logger, "varnishd")

	// Start Varnish
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}

	// Wait for Varnish to exit
	err := cmd.Wait()
	if err != nil {
		return fmt.Errorf("varnish process failed: %w", err)
	} else {
		m.logger.Info("Varnish process exited successfully")
	}

	return nil
}

// GetSecret returns the varnishadm authentication secret
func (m *Manager) GetSecret() string {
	return m.secret
}

// GetVarnishDir returns the varnish directory path
func (m *Manager) GetVarnishDir() string {
	return m.varnishDir
}

// GetSecretPath returns the path to the secret file
func (m *Manager) GetSecretPath() string {
	return filepath.Join(m.workDir, "secret")
}

// GetLicensePath returns the path to the license file
func (m *Manager) GetLicensePath() string {
	return filepath.Join(m.workDir, "varnish-enterprise.lic")
}
