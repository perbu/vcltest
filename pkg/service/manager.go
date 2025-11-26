package service

import (
	"context"
	"fmt"
	"time"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/cache"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/varnishadm"
	"github.com/perbu/vcltest/pkg/vclloader"
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

	// Create event broker
	broker := broker.New(broker.Config{})

	// Create varnishadm server with broker
	varnishadmServer := varnishadm.New(config.VarnishadmPort, config.Secret, config.Logger, broker)

	// Create varnish manager
	varnishManager := varnish.New(
		config.VarnishConfig.WorkDir,
		config.Logger,
		config.VarnishConfig.VarnishDir,
	)

	// Create VCL loader
	vclLoader := vclloader.New(varnishadmServer, broker, config.VCLPath, config.Logger)

	// Create cache starter
	cacheStarter := cache.New(varnishadmServer, broker, config.Logger)

	return &Manager{
		config:         config,
		broker:         broker,
		varnishadm:     varnishadmServer,
		varnishManager: varnishManager,
		vclLoader:      vclLoader,
		cacheStarter:   cacheStarter,
		logger:         config.Logger,
	}, nil
}

// Start starts the varnishadm server and varnish daemon in order
// This method blocks until either service fails or the context is cancelled
func (m *Manager) Start(ctx context.Context) error {
	// Start event listeners
	m.logger.Debug("Starting VCL loader event listener")
	m.vclLoader.Start()

	m.logger.Debug("Starting cache starter event listener")
	m.cacheStarter.Start()

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
	args := varnish.BuildArgs(m.config.VarnishConfig)

	// Start varnish in a goroutine
	m.logger.Debug("Starting varnish daemon", "cmd", m.config.VarnishCmd)
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

// GetCacheStarter returns the cache starter (for accessing VCL mapping)
func (m *Manager) GetCacheStarter() *cache.Starter {
	return m.cacheStarter
}

// AdvanceTimeBy advances the fake time to testStartTime + offset (if faketime is enabled)
// offset is relative to test start (t0), e.g., 5*time.Second means "5 seconds after test start"
// Returns error if time control is not enabled
func (m *Manager) AdvanceTimeBy(offset time.Duration) error {
	return m.varnishManager.AdvanceTimeBy(offset)
}
