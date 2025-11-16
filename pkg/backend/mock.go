package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

// Config defines the mock backend configuration
type Config struct {
	Status  int
	Headers map[string]string
	Body    string
}

// MockServer is a simple HTTP server that returns configured responses
type MockServer struct {
	config       Config
	server       *http.Server
	listener     net.Listener
	requestCount atomic.Int32
	mu           sync.Mutex
}

// New creates a new mock backend server
func New(config Config) *MockServer {
	return &MockServer{
		config: config,
	}
}

// Start starts the mock server on a random available port
func (m *MockServer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener != nil {
		return fmt.Errorf("server already started")
	}

	// Listen on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	m.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleRequest)

	m.server = &http.Server{
		Handler: mux,
	}

	// Start serving in background
	go func() {
		if err := m.server.Serve(m.listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't panic - server might be shutting down
		}
	}()

	return nil
}

// handleRequest handles all incoming HTTP requests
func (m *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.requestCount.Add(1)

	// Set headers
	for key, value := range m.config.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(m.config.Status)

	// Write body
	if m.config.Body != "" {
		w.Write([]byte(m.config.Body))
	}
}

// Address returns the server's listening address (host:port)
func (m *MockServer) Address() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener == nil {
		return ""
	}
	return m.listener.Addr().String()
}

// RequestCount returns the number of requests received
func (m *MockServer) RequestCount() int {
	return int(m.requestCount.Load())
}

// Shutdown gracefully shuts down the server
func (m *MockServer) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.server == nil {
		return nil
	}

	err := m.server.Shutdown(ctx)
	m.server = nil
	m.listener = nil
	return err
}
