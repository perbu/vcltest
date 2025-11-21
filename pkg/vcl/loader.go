package vcl

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/events"
	"github.com/perbu/vcltest/pkg/varnishadm"
)

const publishTimeout = 1 * time.Second

// Loader manages VCL loading and publishes events
type Loader struct {
	varnishadm varnishadm.VarnishadmInterface
	broker     *broker.Broker
	vclPath    string
	vclName    string
	logger     *slog.Logger
}

// New creates a new VCL loader
func New(varnishadm varnishadm.VarnishadmInterface, broker *broker.Broker, vclPath string, logger *slog.Logger) *Loader {
	return &Loader{
		varnishadm: varnishadm,
		broker:     broker,
		vclPath:    vclPath,
		vclName:    "boot", // Default VCL name
		logger:     logger,
	}
}

// Start begins listening for EventVarnishdConnected
func (l *Loader) Start() {
	subscriber, err := l.broker.Subscribe("/process")
	if err != nil {
		l.logger.Error("Failed to subscribe to /process", "error", err)
		return
	}

	go func() {
		for msg := range subscriber.Messages() {
			if _, ok := msg.Payload.(events.EventVarnishdConnected); ok {
				l.logger.Info("Varnishd connected, loading VCL", "path", l.vclPath)
				if err := l.loadVCL(); err != nil {
					_ = l.broker.Publish("/process", events.EventProcessError{
						Component: "vcl-loader",
						Error:     err,
					}, publishTimeout)
				}
			}
		}
	}()
}

// loadVCL loads the VCL file and publishes EventVCLLoaded
func (l *Loader) loadVCL() error {
	// Resolve absolute path to VCL file
	absPath, err := filepath.Abs(l.vclPath)
	if err != nil {
		return fmt.Errorf("failed to resolve VCL path: %w", err)
	}

	// Verify VCL file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("VCL file does not exist: %s", absPath)
	}

	// Get VCL directory for includes
	vclDir := filepath.Dir(absPath)

	// Change CWD to VCL directory so includes work
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	l.logger.Debug("Changing CWD for VCL includes", "from", originalDir, "to", vclDir)
	if err := os.Chdir(vclDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", vclDir, err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			l.logger.Error("Failed to restore original directory", "error", err)
		}
	}()

	// Load VCL
	l.logger.Info("Loading VCL", "name", l.vclName, "path", absPath)
	resp, err := l.varnishadm.VCLLoad(l.vclName, absPath)
	if err != nil {
		return fmt.Errorf("failed to load VCL: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return fmt.Errorf("VCL compilation failed (status %d): %s", resp.StatusCode(), resp.Payload())
	}

	l.logger.Info("VCL loaded successfully", "name", l.vclName)

	// Activate VCL
	resp, err = l.varnishadm.VCLUse(l.vclName)
	if err != nil {
		return fmt.Errorf("failed to activate VCL: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return fmt.Errorf("VCL activation failed (status %d): %s", resp.StatusCode(), resp.Payload())
	}

	l.logger.Info("VCL activated", "name", l.vclName)

	// Get VCL mapping (config ID to filename)
	mapping, err := l.varnishadm.VCLShowStructured(l.vclName)
	if err != nil {
		return fmt.Errorf("failed to get VCL mapping: %w", err)
	}

	l.logger.Debug("VCL mapping retrieved", "entries", len(mapping.Entries))

	// Publish EventVCLLoaded
	_ = l.broker.Publish("/process", events.EventVCLLoaded{
		Name:    l.vclName,
		Path:    absPath,
		Mapping: mapping,
	}, publishTimeout)

	return nil
}
