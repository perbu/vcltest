package backend

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

// MockBackend is a simple HTTP server that returns configured responses
type MockBackend struct {
	server     *http.Server
	listener   net.Listener
	callCount  atomic.Int32
	config     Config
	configMu   sync.RWMutex  // Protects config field
	shutdownCh chan struct{} // Closed on Stop() to unblock frozen handlers
}

// Config defines the mock backend response configuration
type Config struct {
	Status      int
	Headers     map[string]string
	Body        string
	FailureMode string // "failed" = connection reset, "frozen" = never responds, "" = normal
}

// New creates a new mock backend with the given configuration
func New(config Config) *MockBackend {
	return &MockBackend{
		config:     config,
		shutdownCh: make(chan struct{}),
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

	// Read config with lock
	m.configMu.RLock()
	status := m.config.Status
	headers := m.config.Headers
	body := m.config.Body
	failureMode := m.config.FailureMode
	m.configMu.RUnlock()

	// Handle failure modes
	switch failureMode {
	case "frozen":
		// Block until either backend is stopped or client disconnects
		select {
		case <-m.shutdownCh:
		case <-r.Context().Done():
		}
		// Connection closes without response, triggering timeout in Varnish
		return

	case "failed":
		// Hijack connection and close it immediately to simulate connection reset
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		conn.Close()
		return
	}

	// Set response headers
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	// Set Content-Length if body is present
	// This must be done BEFORE WriteHeader() to ensure it's sent with correct length
	if body != "" {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}

	// Write status code
	w.WriteHeader(status)

	// Write body
	if body != "" {
		_, _ = w.Write([]byte(body))
	}
}

// GetCallCount returns the number of times the backend has been called
func (m *MockBackend) GetCallCount() int {
	return int(m.callCount.Load())
}

// ResetCallCount resets the call counter to zero
// This is useful for resetting state between tests in shared VCL mode
func (m *MockBackend) ResetCallCount() {
	m.callCount.Store(0)
}

// UpdateConfig atomically updates the backend response configuration
// This allows changing the backend's behavior without restarting it
func (m *MockBackend) UpdateConfig(newConfig Config) {
	m.configMu.Lock()
	defer m.configMu.Unlock()
	m.config = newConfig
}

// Stop gracefully stops the mock backend
func (m *MockBackend) Stop() error {
	// Signal frozen handlers to unblock
	select {
	case <-m.shutdownCh:
		// Already closed
	default:
		close(m.shutdownCh)
	}

	if m.server != nil {
		return m.server.Close()
	}
	return nil
}
