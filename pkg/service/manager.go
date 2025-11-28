package service

import (
	"context"
	"fmt"
	"time"

	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/varnishadm"
)

// NewManager creates a new service manager with the given configuration
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if config.VarnishConfig == nil {
		return nil, fmt.Errorf("varnish config cannot be nil")
	}
	if config.Secret == "" {
		return nil, fmt.Errorf("secret cannot be empty")
	}
	if config.VCLPath == "" {
		return nil, fmt.Errorf("VCL path cannot be empty")
	}

	// Create varnishadm server (no broker needed - simplified flow)
	varnishadmServer := varnishadm.New(config.VarnishadmPort, config.Secret, config.Logger, nil)

	// Create varnish manager
	varnishManager := varnish.New(
		config.VarnishConfig.WorkDir,
		config.Logger,
		config.VarnishConfig.VarnishDir,
	)

	return &Manager{
		config:         config,
		varnishadm:     varnishadmServer,
		varnishManager: varnishManager,
		logger:         config.Logger,
	}, nil
}

// Start starts the varnishadm server and varnish daemon in order.
// VCL must be prepared (with backend addresses modified) and passed via config.VCLPath.
// Varnishd will load the VCL at boot time - no event-driven loading needed.
// This method blocks until either service fails or the context is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	// Create error channel to receive errors from goroutines
	errCh := make(chan error, 2)

	// Create varnishadm listener first (this binds to the port immediately)
	m.logger.Debug("Creating varnishadm listener", "requested_port", m.config.VarnishadmPort)
	actualPort, err := m.varnishadm.Listen()
	if err != nil {
		return fmt.Errorf("varnishadm listen failed: %w", err)
	}
	m.logger.Debug("varnishadm listening", "port", actualPort)

	// Update varnish config with actual admin port (important for dynamic port assignment)
	m.config.VarnishConfig.Varnish.AdminPort = int(actualPort)

	// Start varnishadm server in a goroutine (listener is already ready)
	go func() {
		if err := m.varnishadm.Run(ctx); err != nil {
			errCh <- fmt.Errorf("varnishadm server failed: %w", err)
		}
	}()

	// Prepare varnish workspace (directories, secret file, license)
	m.logger.Debug("Preparing varnish workspace")
	if err := m.varnishManager.PrepareWorkspace(m.config.Secret, m.config.VarnishConfig.License.Text); err != nil {
		return fmt.Errorf("failed to prepare varnish workspace: %w", err)
	}

	// Build varnish command-line arguments
	// VCL is loaded at boot time via -f flag (no dynamic loading)
	args := varnish.BuildArgs(m.config.VarnishConfig)

	// Start varnish in a goroutine
	m.logger.Debug("Starting varnish daemon", "cmd", m.config.VarnishCmd, "vcl", m.config.VCLPath)
	go func() {
		if err := m.varnishManager.Start(ctx, m.config.VarnishCmd, args, &m.config.VarnishConfig.Varnish.Time); err != nil {
			errCh <- fmt.Errorf("varnish daemon failed: %w", err)
		}
	}()

	// Wait for either an error or context cancellation
	select {
	case err := <-errCh:
		// One of the services failed
		m.logger.Error("Service failed", "error", err)
		return err
	case <-ctx.Done():
		// Context was cancelled, graceful shutdown
		m.logger.Debug("Context cancelled, shutting down services")
		return ctx.Err()
	}
}

// GetVarnishadm returns the varnishadm interface for issuing commands
func (m *Manager) GetVarnishadm() varnishadm.VarnishadmInterface {
	return m.varnishadm
}

// GetVarnishManager returns the varnish manager
func (m *Manager) GetVarnishManager() *varnish.Manager {
	return m.varnishManager
}

// AdvanceTimeBy advances the fake time to testStartTime + offset (if faketime is enabled)
// offset is relative to test start (t0), e.g., 5*time.Second means "5 seconds after test start"
// Returns error if time control is not enabled
func (m *Manager) AdvanceTimeBy(offset time.Duration) error {
	return m.varnishManager.AdvanceTimeBy(offset)
}

// GetHTTPPort queries varnishd for the actual HTTP listen port.
// This is useful when varnishd was started with -a :0 for dynamic port assignment.
// Must be called after varnishd has connected and is accepting connections.
func (m *Manager) GetHTTPPort() (int, error) {
	addresses, err := m.varnishadm.DebugListenAddressStructured()
	if err != nil {
		return 0, fmt.Errorf("failed to get listen addresses: %w", err)
	}

	// When Varnish binds to :0 (dynamic port), it creates separate IPv4 and IPv6 listeners
	// with DIFFERENT ports. Since we connect to 127.0.0.1 (IPv4), we must use the IPv4 port.
	// IPv4 addresses: 0.0.0.0 or specific IPv4 like 127.0.0.1
	// IPv6 addresses: :: or specific IPv6 like ::1
	var ipv4Port, fallbackPort int
	for _, addr := range addresses {
		if addr.Port <= 0 {
			continue // Skip Unix sockets
		}
		// Check for IPv4 - does not contain ':' (IPv6 addresses always have colons)
		if !containsColon(addr.Address) {
			ipv4Port = addr.Port
			break // Prefer first IPv4 address
		}
		if fallbackPort == 0 {
			fallbackPort = addr.Port // Keep first TCP port as fallback
		}
	}

	if ipv4Port > 0 {
		return ipv4Port, nil
	}
	if fallbackPort > 0 {
		return fallbackPort, nil
	}
	return 0, fmt.Errorf("no HTTP listen address found in %d addresses", len(addresses))
}

// containsColon checks if a string contains a colon character
func containsColon(s string) bool {
	for _, c := range s {
		if c == ':' {
			return true
		}
	}
	return false
}
