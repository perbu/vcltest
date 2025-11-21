package backend

import (
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
)

// MockBackend is a simple HTTP server that returns configured responses
type MockBackend struct {
	server    *http.Server
	listener  net.Listener
	callCount atomic.Int32
	config    Config
}

// Config defines the mock backend response configuration
type Config struct {
	Status  int
	Headers map[string]string
	Body    string
}

// New creates a new mock backend with the given configuration
func New(config Config) *MockBackend {
	return &MockBackend{
		config: config,
	}
}

// Start starts the mock backend on a random available port
// Returns the address (127.0.0.1:port) that the backend is listening on
func (m *MockBackend) Start() (string, error) {
	// Create listener on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to create listener: %w", err)
	}
	m.listener = listener

	// Create HTTP server
	m.server = &http.Server{
		Handler: http.HandlerFunc(m.handleRequest),
	}

	// Start server in background
	go func() {
		_ = m.server.Serve(listener)
	}()

	return listener.Addr().String(), nil
}

// handleRequest handles incoming HTTP requests
func (m *MockBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Increment call counter
	m.callCount.Add(1)

	// Set response headers
	for key, value := range m.config.Headers {
		w.Header().Set(key, value)
	}

	// Write status code
	w.WriteHeader(m.config.Status)

	// Write body
	if m.config.Body != "" {
		_, _ = w.Write([]byte(m.config.Body))
	}
}

// GetCallCount returns the number of times the backend has been called
func (m *MockBackend) GetCallCount() int {
	return int(m.callCount.Load())
}

// Stop gracefully stops the mock backend
func (m *MockBackend) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}
