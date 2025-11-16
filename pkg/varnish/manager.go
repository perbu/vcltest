package varnish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Manager manages a varnishd instance
type Manager struct {
	name       string
	workdir    string
	vclContent string
	cmd        *exec.Cmd
	listenAddr string
	adminAddr  string
}

// Config holds configuration for Varnish manager
type Config struct {
	Name       string // Unique name for this instance
	VCLContent string // VCL code to load
}

// New creates a new Varnish manager
func New(config Config) (*Manager, error) {
	// Create temporary work directory
	workdir, err := os.MkdirTemp("", fmt.Sprintf("vcltest-%s-*", config.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create workdir: %w", err)
	}

	return &Manager{
		name:       config.Name,
		workdir:    workdir,
		vclContent: config.VCLContent,
		listenAddr: "127.0.0.1:0", // Random port
		adminAddr:  "127.0.0.1:0", // Random admin port
	}, nil
}

// Start starts the varnishd process
func (m *Manager) Start(ctx context.Context) error {
	// Write VCL to file
	vclPath := filepath.Join(m.workdir, "test.vcl")
	if err := os.WriteFile(vclPath, []byte(m.vclContent), 0644); err != nil {
		return fmt.Errorf("failed to write VCL file: %w", err)
	}

	// Start varnishd
	args := []string{
		"-n", m.workdir,
		"-f", vclPath,
		"-a", m.listenAddr,
		"-T", m.adminAddr,
		"-s", "malloc,64M",
		"-F", // Run in foreground
	}

	m.cmd = exec.CommandContext(ctx, "varnishd", args...)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start varnishd: %w", err)
	}

	// Wait for varnishd to be ready
	time.Sleep(500 * time.Millisecond)

	// Get actual listen address (if we used :0 for random port)
	// For now, we'll keep it simple and require explicit ports
	// TODO: Parse varnishd output to get actual listening address

	return nil
}

// LoadVCL loads new VCL configuration
func (m *Manager) LoadVCL(vclContent string) error {
	vclPath := filepath.Join(m.workdir, "test.vcl")
	if err := os.WriteFile(vclPath, []byte(vclContent), 0644); err != nil {
		return fmt.Errorf("failed to write VCL file: %w", err)
	}

	// Use varnishadm to reload VCL
	cmd := exec.Command("varnishadm", "-n", m.workdir, "vcl.load", "test_reload", vclPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load VCL: %w", err)
	}

	cmd = exec.Command("varnishadm", "-n", m.workdir, "vcl.use", "test_reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to activate VCL: %w", err)
	}

	m.vclContent = vclContent
	return nil
}

// ListenAddress returns the address where varnishd is listening
func (m *Manager) ListenAddress() string {
	return m.listenAddr
}

// Stop stops the varnishd process and cleans up
func (m *Manager) Stop() error {
	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill varnishd: %w", err)
		}
		m.cmd.Wait()
	}
	return nil
}

// Cleanup removes the work directory
func (m *Manager) Cleanup() error {
	if m.workdir != "" {
		return os.RemoveAll(m.workdir)
	}
	return nil
}

// StopAndCleanup stops varnishd and cleans up resources
func (m *Manager) StopAndCleanup() error {
	if err := m.Stop(); err != nil {
		// Continue with cleanup even if stop fails
		m.Cleanup()
		return err
	}
	return m.Cleanup()
}
