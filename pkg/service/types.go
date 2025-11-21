package service

import (
	"log/slog"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/cache"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/varnishadm"
	"github.com/perbu/vcltest/pkg/vcl"
)

// Config holds the configuration for the service manager
type Config struct {
	// VarnishadmPort is the port the varnishadm server listens on
	VarnishadmPort uint16
	// Secret is the shared secret for varnishadm authentication
	Secret string
	// VarnishCmd is the path to the varnishd executable (empty for PATH lookup)
	VarnishCmd string
	// VCLPath is the path to the VCL file to load
	VCLPath string
	// VarnishConfig contains the varnish-specific configuration
	VarnishConfig *varnish.Config
	// Logger for structured logging
	Logger *slog.Logger
}

// Manager orchestrates the lifecycle of varnishadm and varnish services
type Manager struct {
	config         *Config
	broker         *broker.Broker
	varnishadm     varnishadm.VarnishadmInterface
	varnishManager *varnish.Manager
	vclLoader      *vcl.Loader
	cacheStarter   *cache.Starter
	logger         *slog.Logger
}
