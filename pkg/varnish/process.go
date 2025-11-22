package varnish

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Manager manages the varnishd process lifecycle
type Manager struct {
	workDir         string
	varnishDir      string
	secret          string
	logger          *slog.Logger
	timeControlFile string    // Path to faketime control file
	lastTimestamp   time.Time // Track last set time for forward-only validation
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

	m.logger.Debug("Wrote Varnish Enterprise license file", "path", licensePath)
	return nil
}

// Start starts the varnishd process with the given arguments
func (m *Manager) Start(ctx context.Context, varnishCmd string, args []string, timeConfig *TimeConfig) error {
	// Find varnishd executable if not specified
	if varnishCmd == "" {
		var err error
		varnishCmd, err = exec.LookPath("varnishd")
		if err != nil {
			return fmt.Errorf("varnishd not found in PATH: %w", err)
		}
	}

	m.logger.Debug("Starting varnishd", "cmd", varnishCmd, "args", args)

	// Create the command, ctx lets us cancel and kill varnishd
	cmd := exec.CommandContext(ctx, varnishCmd, args...)
	cmd.Dir = m.varnishDir

	// Inherit environment variables so VMOD otel can read OTEL_* configuration
	cmd.Env = os.Environ()

	// Setup faketime if enabled
	if timeConfig != nil && timeConfig.Enabled {
		if err := m.setupFaketime(cmd, timeConfig); err != nil {
			return fmt.Errorf("failed to setup faketime: %w", err)
		}
	}

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
		m.logger.Debug("Varnish process exited successfully")
	}

	return nil
}

// setupFaketime configures the command environment for libfaketime
func (m *Manager) setupFaketime(cmd *exec.Cmd, timeConfig *TimeConfig) error {
	// Detect library path
	libPath, err := detectLibfaketimePath(timeConfig.LibPath)
	if err != nil {
		return err
	}

	// Setup control file
	controlFile := timeConfig.ControlFile
	if controlFile == "" {
		controlFile, err = m.SetupTimeControl(timeConfig.StartTime)
		if err != nil {
			return err
		}
	} else {
		m.timeControlFile = controlFile
		// Parse StartTime to set lastTimestamp for validation
		t, err := time.Parse("2006-01-02 15:04:05", timeConfig.StartTime)
		if err != nil {
			return fmt.Errorf("invalid start time: %w", err)
		}
		m.lastTimestamp = t
	}

	// Add faketime environment variables
	cmd.Env = append(cmd.Env,
		"FAKETIME=%",
		fmt.Sprintf("FAKETIME_FOLLOW_FILE=%s", controlFile),
		"FAKETIME_DONT_RESET=1",
		"FAKETIME_NO_CACHE=1",
	)

	// Platform-specific library injection
	switch runtime.GOOS {
	case "darwin":
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("DYLD_INSERT_LIBRARIES=%s", libPath),
			"DYLD_FORCE_FLAT_NAMESPACE=1",
		)
	case "linux":
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("LD_PRELOAD=%s", libPath),
		)
	default:
		return fmt.Errorf("faketime not supported on %s", runtime.GOOS)
	}

	m.logger.Debug("Faketime enabled", "lib_path", libPath, "control_file", controlFile, "start_time", timeConfig.StartTime)

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

// SetupTimeControl initializes the faketime control file with the start time
// Returns the control file path or error
func (m *Manager) SetupTimeControl(startTime string) (string, error) {
	// Parse startTime string to time.Time
	t, err := time.Parse("2006-01-02 15:04:05", startTime)
	if err != nil {
		return "", fmt.Errorf("invalid start time format (expected 'YYYY-MM-DD HH:MM:SS'): %w", err)
	}

	// Create control file path
	controlFile := filepath.Join(m.workDir, "faketime.control")

	// Create empty file
	f, err := os.Create(controlFile)
	if err != nil {
		return "", fmt.Errorf("failed to create control file: %w", err)
	}
	f.Close()

	// Set file modification time using os.Chtimes
	if err := os.Chtimes(controlFile, t, t); err != nil {
		return "", fmt.Errorf("failed to set control file time: %w", err)
	}

	m.timeControlFile = controlFile
	m.lastTimestamp = t
	m.logger.Debug("Faketime control file created", "path", controlFile, "time", startTime)

	return controlFile, nil
}

// AdvanceTime updates the control file to advance time forward
// timeStr format: "2006-01-02 15:04:05" (e.g., "2026-05-22 08:30:30")
// Returns error if time would go backwards
func (m *Manager) AdvanceTime(timeStr string) error {
	if m.timeControlFile == "" {
		return fmt.Errorf("time control not initialized")
	}

	// Parse new time
	newTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return fmt.Errorf("invalid time format (expected 'YYYY-MM-DD HH:MM:SS'): %w", err)
	}

	// Enforce forward-only time movement
	if !newTime.After(m.lastTimestamp) {
		return fmt.Errorf("time must move forward: current=%s, requested=%s",
			m.lastTimestamp.Format("2006-01-02 15:04:05"), timeStr)
	}

	// Update control file mtime
	if err := os.Chtimes(m.timeControlFile, newTime, newTime); err != nil {
		return fmt.Errorf("failed to update control file time: %w", err)
	}

	m.lastTimestamp = newTime
	m.logger.Debug("Advanced fake time", "new_time", timeStr)

	return nil
}

// GetCurrentFakeTime reads the current fake time from control file mtime
// Returns zero time if not using faketime
func (m *Manager) GetCurrentFakeTime() time.Time {
	if m.timeControlFile == "" {
		return time.Time{}
	}

	info, err := os.Stat(m.timeControlFile)
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}

// detectLibfaketimePath finds the libfaketime library path
// Returns custom path if provided, otherwise auto-detects based on OS
func detectLibfaketimePath(customPath string) (string, error) {
	if customPath != "" {
		// Verify custom path exists
		if _, err := os.Stat(customPath); err != nil {
			return "", fmt.Errorf("custom libfaketime path not found: %w", err)
		}
		return customPath, nil
	}

	// Auto-detect based on OS
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/opt/homebrew/lib/faketime/libfaketime.1.dylib",
			"/usr/local/lib/faketime/libfaketime.1.dylib",
		}
	case "linux":
		candidates = []string{
			"/usr/lib/x86_64-linux-gnu/faketime/libfaketime.so.1",
			"/usr/lib/faketime/libfaketime.so.1",
		}
	default:
		return "", fmt.Errorf("libfaketime auto-detection not supported on %s", runtime.GOOS)
	}

	// Try each candidate
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("libfaketime not found in standard locations")
}
