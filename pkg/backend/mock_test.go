package backend

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew_CreatesBackend(t *testing.T) {
	cfg := Config{
		Status:  200,
		Headers: map[string]string{"X-Test": "value"},
		Body:    "test body",
	}

	backend := New(cfg)
	if backend == nil {
		t.Fatal("New() returned nil")
	}

	if backend.config.Status != 200 {
		t.Errorf("config.Status = %d, want 200", backend.config.Status)
	}

	if backend.config.Body != "test body" {
		t.Errorf("config.Body = %q, want %q", backend.config.Body, "test body")
	}
}

func TestStart_RandomPort(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "OK",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	if addr == "" {
		t.Error("Start() returned empty address")
	}

	// Address should be in format "127.0.0.1:port"
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("Start() address = %q, want 127.0.0.1:port format", addr)
	}

	// Verify server is actually listening
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Failed to connect to backend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Response status = %d, want 200", resp.StatusCode)
	}
}

func TestStart_MultiplePorts(t *testing.T) {
	// Start multiple backends to ensure they get different ports
	backend1 := New(Config{Status: 200})
	backend2 := New(Config{Status: 200})

	addr1, err := backend1.Start()
	if err != nil {
		t.Fatalf("backend1.Start() error = %v", err)
	}
	defer backend1.Stop()

	addr2, err := backend2.Start()
	if err != nil {
		t.Fatalf("backend2.Start() error = %v", err)
	}
	defer backend2.Stop()

	if addr1 == addr2 {
		t.Errorf("Both backends got same address %q, want different ports", addr1)
	}
}

func TestHandleRequest_Status(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantStatus int
	}{
		{"200 OK", 200, 200},
		{"201 Created", 201, 201},
		{"204 No Content", 204, 204},
		{"400 Bad Request", 400, 400},
		{"404 Not Found", 404, 404},
		{"500 Internal Server Error", 500, 500},
		{"503 Service Unavailable", 503, 503},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := New(Config{
				Status: tt.status,
			})

			addr, err := backend.Start()
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			defer backend.Stop()

			resp, err := http.Get("http://" + addr)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestHandleRequest_Headers(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Content-Type":    "application/json",
			"X-Backend-ID":    "backend-1",
		},
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", resp.Header.Get("X-Custom-Header"), "custom-value")
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", resp.Header.Get("Content-Type"), "application/json")
	}

	if resp.Header.Get("X-Backend-ID") != "backend-1" {
		t.Errorf("X-Backend-ID = %q, want %q", resp.Header.Get("X-Backend-ID"), "backend-1")
	}
}

func TestHandleRequest_Body(t *testing.T) {
	body := "Response body content"
	backend := New(Config{
		Status: 200,
		Body:   body,
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(respBody) != body {
		t.Errorf("Body = %q, want %q", string(respBody), body)
	}
}

func TestHandleRequest_BodyWithContentLength(t *testing.T) {
	body := "Test response body"
	backend := New(Config{
		Status: 200,
		Body:   body,
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify Content-Length header is set correctly
	if resp.ContentLength != int64(len(body)) {
		t.Errorf("ContentLength = %d, want %d", resp.ContentLength, len(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(respBody) != len(body) {
		t.Errorf("Body length = %d, want %d", len(respBody), len(body))
	}
}

func TestHandleRequest_EmptyBody(t *testing.T) {
	backend := New(Config{
		Status: 204,
		Body:   "",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(respBody) != 0 {
		t.Errorf("Body length = %d, want 0", len(respBody))
	}
}

func TestGetCallCount_Increments(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "OK",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// Initial count should be 0
	if count := backend.GetCallCount(); count != 0 {
		t.Errorf("Initial call count = %d, want 0", count)
	}

	// Make first request
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request 1 failed: %v", err)
	}
	resp.Body.Close()

	if count := backend.GetCallCount(); count != 1 {
		t.Errorf("Call count after 1 request = %d, want 1", count)
	}

	// Make second request
	resp, err = http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request 2 failed: %v", err)
	}
	resp.Body.Close()

	if count := backend.GetCallCount(); count != 2 {
		t.Errorf("Call count after 2 requests = %d, want 2", count)
	}

	// Make third request
	resp, err = http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request 3 failed: %v", err)
	}
	resp.Body.Close()

	if count := backend.GetCallCount(); count != 3 {
		t.Errorf("Call count after 3 requests = %d, want 3", count)
	}
}

func TestResetCallCount(t *testing.T) {
	backend := New(Config{
		Status: 200,
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// Make some requests
	for i := 0; i < 5; i++ {
		resp, err := http.Get("http://" + addr)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		resp.Body.Close()
	}

	if count := backend.GetCallCount(); count != 5 {
		t.Errorf("Call count before reset = %d, want 5", count)
	}

	// Reset counter
	backend.ResetCallCount()

	if count := backend.GetCallCount(); count != 0 {
		t.Errorf("Call count after reset = %d, want 0", count)
	}

	// Make another request to verify counting still works
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request after reset failed: %v", err)
	}
	resp.Body.Close()

	if count := backend.GetCallCount(); count != 1 {
		t.Errorf("Call count after reset + 1 request = %d, want 1", count)
	}
}

func TestUpdateConfig_ChangesResponse(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "Initial body",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// First request with initial config
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request 1 failed: %v", err)
	}
	body1, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body1) != "Initial body" {
		t.Errorf("Initial body = %q, want %q", string(body1), "Initial body")
	}

	// Update config
	backend.UpdateConfig(Config{
		Status:  201,
		Headers: map[string]string{"X-Updated": "true"},
		Body:    "Updated body",
	})

	// Second request should use new config
	resp, err = http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request 2 failed: %v", err)
	}
	body2, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("Updated status = %d, want 201", resp.StatusCode)
	}

	if string(body2) != "Updated body" {
		t.Errorf("Updated body = %q, want %q", string(body2), "Updated body")
	}

	if resp.Header.Get("X-Updated") != "true" {
		t.Errorf("Updated header X-Updated = %q, want %q", resp.Header.Get("X-Updated"), "true")
	}
}

func TestStop_GracefulShutdown(t *testing.T) {
	backend := New(Config{
		Status: 200,
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify server is running
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request before stop failed: %v", err)
	}
	resp.Body.Close()

	// Stop the server
	err = backend.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Give it a moment to fully shut down
	time.Sleep(50 * time.Millisecond)

	// Verify server is no longer running
	_, err = http.Get("http://" + addr)
	if err == nil {
		t.Error("Expected error when connecting to stopped server")
	}
}

func TestConcurrentRequests(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "OK",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// Make concurrent requests
	numRequests := 100
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + addr)
			if err != nil {
				t.Errorf("Concurrent request failed: %v", err)
				return
			}
			resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Errorf("Concurrent request status = %d, want 200", resp.StatusCode)
			}
		}()
	}

	wg.Wait()

	// Verify call count is accurate
	if count := backend.GetCallCount(); count != numRequests {
		t.Errorf("Call count after %d concurrent requests = %d, want %d", numRequests, count, numRequests)
	}
}

func TestConcurrentConfigUpdates(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "Initial",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	var wg sync.WaitGroup
	numOperations := 50

	// Concurrent config updates
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(n int) {
			defer wg.Done()
			backend.UpdateConfig(Config{
				Status: 200 + n,
				Body:   "Body",
			})
		}(i)
	}

	// Concurrent requests
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + addr)
			if err != nil {
				t.Errorf("Concurrent request failed: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}

	wg.Wait()

	// Verify server is still functional
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request after concurrent operations failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Errorf("Status after concurrent updates = %d, want 2xx", resp.StatusCode)
	}
}

func TestStop_NilServer(t *testing.T) {
	// Test stopping a backend that was never started
	backend := New(Config{Status: 200})

	err := backend.Stop()
	if err != nil {
		t.Errorf("Stop() on unstarted backend error = %v, want nil", err)
	}
}

func TestFailureMode_Failed(t *testing.T) {
	backend := New(Config{
		Status:      200,
		FailureMode: "failed",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// Request should fail due to connection reset
	resp, err := http.Get("http://" + addr + "/test")
	if err == nil {
		resp.Body.Close()
		t.Fatal("Expected error due to connection reset, but request succeeded")
	}

	// Verify the call was still counted
	if count := backend.GetCallCount(); count != 1 {
		t.Errorf("Call count = %d, want 1 (even for failed requests)", count)
	}
}

func TestFailureMode_CanBeUpdated(t *testing.T) {
	backend := New(Config{
		Status: 200,
		Body:   "OK",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// First request should succeed
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Initial status = %d, want 200", resp.StatusCode)
	}

	// Update to failure mode
	backend.UpdateConfig(Config{
		FailureMode: "failed",
	})

	// Second request should fail
	resp, err = http.Get("http://" + addr)
	if err == nil {
		resp.Body.Close()
		t.Fatal("Expected error after updating to failure mode, but request succeeded")
	}

	// Update back to normal mode
	backend.UpdateConfig(Config{
		Status: 201,
		Body:   "Created",
	})

	// Third request should succeed again
	resp, err = http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request after reverting from failure mode failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("Reverted status = %d, want 201", resp.StatusCode)
	}
}

func TestFailureMode_Frozen(t *testing.T) {
	backend := New(Config{
		Status:      200,
		FailureMode: "frozen",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer backend.Stop()

	// Create client with short timeout
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	// Request should timeout (not complete)
	_, err = client.Get("http://" + addr + "/test")
	if err == nil {
		t.Fatal("Expected timeout error, but request succeeded")
	}

	// Verify it's a timeout error (context deadline exceeded)
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Verify the call was still counted
	if count := backend.GetCallCount(); count != 1 {
		t.Errorf("Call count = %d, want 1", count)
	}
}

func TestFailureMode_Frozen_UnblocksOnStop(t *testing.T) {
	backend := New(Config{
		Status:      200,
		FailureMode: "frozen",
	})

	addr, err := backend.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Start a request in a goroutine (it will block)
	done := make(chan struct{})
	go func() {
		// Use a client with no timeout - it should unblock when Stop() is called
		client := &http.Client{}
		_, _ = client.Get("http://" + addr + "/test")
		close(done)
	}()

	// Give the request time to start and block
	time.Sleep(50 * time.Millisecond)

	// Stop should unblock the frozen handler
	err = backend.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Wait for the request goroutine to finish (with timeout)
	select {
	case <-done:
		// Success - the goroutine unblocked
	case <-time.After(1 * time.Second):
		t.Fatal("Frozen handler did not unblock after Stop()")
	}
}
