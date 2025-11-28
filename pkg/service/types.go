package service

import (
	"io"
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
	// VCLPath is the path to the VCL file to load (must be prepared with backend addresses)
	VCLPath string
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

// SetVarnishadmTranscript sets a writer for recording varnishadm CLI traffic.
// This is only effective if the underlying varnishadm is a *Server (not a mock).
// Call this before Start() to capture all traffic including authentication.
func (m *Manager) SetVarnishadmTranscript(w io.Writer) {
	if server, ok := m.varnishadm.(*varnishadm.Server); ok {
		server.SetTranscriptWriter(w)
	}
}
