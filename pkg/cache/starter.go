package cache

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/events"
	"github.com/perbu/vcltest/pkg/varnishadm"
)

const publishTimeout = 1 * time.Second

// Starter manages cache process startup and publishes events
type Starter struct {
	varnishadm varnishadm.VarnishadmInterface
	broker     *broker.Broker
	logger     *slog.Logger
	vclMapping *varnishadm.VCLShowResult
}

// New creates a new cache starter
func New(varnishadm varnishadm.VarnishadmInterface, broker *broker.Broker, logger *slog.Logger) *Starter {
	return &Starter{
		varnishadm: varnishadm,
		broker:     broker,
		logger:     logger,
	}
}

// Start begins listening for EventVCLLoaded
func (s *Starter) Start() {
	subscriber, err := s.broker.Subscribe("/process")
	if err != nil {
		s.logger.Error("Failed to subscribe to /process", "error", err)
		return
	}

	go func() {
		for msg := range subscriber.Messages() {
			if evt, ok := msg.Payload.(events.EventVCLLoaded); ok {
				s.logger.Info("VCL loaded, starting cache process")
				// Store the VCL mapping for later use (type assert from any)
				if mapping, ok := evt.Mapping.(*varnishadm.VCLShowResult); ok {
					s.vclMapping = mapping
				}
				if err := s.startCache(); err != nil {
					_ = s.broker.Publish("/process", events.EventProcessError{
						Component: "cache-starter",
						Error:     err,
					}, publishTimeout)
				}
			}
		}
	}()
}

// startCache starts the cache process and publishes EventCacheStarted
func (s *Starter) startCache() error {
	s.logger.Info("Starting cache process")

	resp, err := s.varnishadm.Start()
	if err != nil {
		return fmt.Errorf("failed to start cache: %w", err)
	}

	if resp.StatusCode() != varnishadm.ClisOk {
		return fmt.Errorf("cache start failed (status %d): %s", resp.StatusCode(), resp.Payload())
	}

	s.logger.Info("Cache process started successfully")

	// Publish EventCacheStarted
	_ = s.broker.Publish("/process", events.EventCacheStarted{}, publishTimeout)

	// Also publish EventReady to signal system is fully initialized
	_ = s.broker.Publish("/process", events.EventReady{}, publishTimeout)

	return nil
}

// GetVCLMapping returns the stored VCL mapping
func (s *Starter) GetVCLMapping() *varnishadm.VCLShowResult {
	return s.vclMapping
}
