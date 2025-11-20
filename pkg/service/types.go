package service

import (
	"log/slog"

	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/varnishadm"
)

// Config holds the configuration for the service manager
type Config struct {
	// VarnishadmPort is the port the varnishadm server listens on
	VarnishadmPort uint16
	// Secret is the shared secret for varnishadm authentication
	Secret string
	// VarnishCmd is the path to the varnishd executable (empty for PATH lookup)
	VarnishCmd string
	// VarnishConfig contains the varnish-specific configuration
	VarnishConfig *varnish.Config
	// Logger for structured logging
	Logger *slog.Logger
}

// Manager orchestrates the lifecycle of varnishadm and varnish services
type Manager struct {
	config         *Config
	varnishadm     varnishadm.VarnishadmInterface
	varnishManager *varnish.Manager
	logger         *slog.Logger
}
