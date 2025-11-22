package service

import (
	"context"
	"fmt"
	"time"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/cache"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/varnishadm"
	"github.com/perbu/vcltest/pkg/vcl"
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
	vclLoader := vcl.New(varnishadmServer, broker, config.VCLPath, config.Logger)

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

	// Start varnishadm server in a goroutine
	m.logger.Debug("Starting varnishadm server", "port", m.config.VarnishadmPort)
	go func() {
		if err := m.varnishadm.Run(ctx); err != nil {
			errCh <- fmt.Errorf("varnishadm server failed: %w", err)
		}
	}()

	// Give varnishadm a moment to start listening
	// TODO: Consider implementing a proper health check instead of sleep
	m.logger.Debug("Waiting for varnishadm to initialize")
	time.Sleep(100 * time.Millisecond)

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

// AdvanceTime advances the fake time (if faketime is enabled)
// timeStr format: "2006-01-02 15:04:05" (e.g., "2026-05-22 08:30:30")
// Returns error if time control is not enabled or if time would go backwards
func (m *Manager) AdvanceTime(timeStr string) error {
	return m.varnishManager.AdvanceTime(timeStr)
}
