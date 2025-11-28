package harness

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/vclmod"
)

// convertRoutes converts testspec routes to backend routes.
func convertRoutes(routes map[string]testspec.RouteSpec) map[string]backend.RouteConfig {
	if routes == nil {
		return nil
	}
	result := make(map[string]backend.RouteConfig, len(routes))
	for path, spec := range routes {
		result[path] = backend.RouteConfig{
			Status:      spec.Status,
			Headers:     spec.Headers,
			Body:        spec.Body,
			FailureMode: spec.FailureMode,
			EchoRequest: spec.EchoRequest,
		}
	}
	return result
}

// startAllBackends starts all mock backends needed across all tests.
// It collects backend configurations from all tests and starts a mock backend
// for each unique backend name (using the first test's configuration for that backend).
func startAllBackends(tests []testspec.TestSpec, logger *slog.Logger) (map[string]vclmod.BackendAddress, map[string]*backend.MockBackend, error) {
	addresses := make(map[string]vclmod.BackendAddress)
	mockBackends := make(map[string]*backend.MockBackend)

	// Collect backend configurations from all tests
	// For shared VCL mode, we use the configuration from the FIRST test that defines each backend
	backendConfigs := make(map[string]testspec.BackendSpec)

	for _, test := range tests {
		for name, spec := range test.Backends {
			if _, exists := backendConfigs[name]; !exists {
				backendConfigs[name] = spec
			}
		}
	}

	// If no backends were found in tests, create a default one
	if len(backendConfigs) == 0 {
		backendConfigs["default"] = testspec.BackendSpec{
			Status: 200,
		}
	}

	// Start a mock backend for each configuration
	for name, spec := range backendConfigs {
		cfg := backend.Config{
			Status:      spec.Status,
			Headers:     spec.Headers,
			Body:        spec.Body,
			FailureMode: spec.FailureMode,
			Routes:      convertRoutes(spec.Routes),
			EchoRequest: spec.EchoRequest,
		}
		// Apply default status if not set
		if cfg.Status == 0 {
			cfg.Status = 200
		}

		mock := backend.New(cfg)
		addr, err := mock.Start()
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("starting backend %q: %w", name, err)
		}

		host, port, err := parseAddress(addr)
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("parsing address for backend %q: %w", name, err)
		}

		mockBackends[name] = mock
		addresses[name] = vclmod.BackendAddress{Host: host, Port: port}
		logger.Debug("Started shared backend", "name", name, "address", addr, "body_len", len(spec.Body), "echo_request", spec.EchoRequest)
	}

	return addresses, mockBackends, nil
}

// parseAddress parses a "host:port" string into host and port components.
func parseAddress(addr string) (string, string, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid address %q: %w", addr, err)
	}
	// Validate port is a number
	if _, err := strconv.Atoi(portStr); err != nil {
		return "", "", fmt.Errorf("invalid port in address %q: %w", addr, err)
	}
	return host, portStr, nil
}

// stopAllBackends stops all mock backends.
func stopAllBackends(mockBackends map[string]*backend.MockBackend, logger *slog.Logger) {
	for name, mock := range mockBackends {
		if err := mock.Stop(); err != nil {
			logger.Warn("Failed to stop backend", "name", name, "error", err)
		}
	}
}
