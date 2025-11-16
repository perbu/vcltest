package backend

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMockServer_StartAndShutdown(t *testing.T) {
	config := Config{
		Status: 200,
		Body:   "test",
	}

	server := New(config)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	addr := server.Address()
	if addr == "" {
		t.Fatal("server address is empty")
	}

	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("expected address to start with '127.0.0.1:', got '%s'", addr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("failed to shutdown server: %v", err)
	}
}

func TestMockServer_Response(t *testing.T) {
	config := Config{
		Status: 201,
		Headers: map[string]string{
			"X-Custom":     "value",
			"Content-Type": "application/json",
		},
		Body: `{"status":"ok"}`,
	}

	server := New(config)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	// Make request
	resp, err := http.Get("http://" + server.Address() + "/test")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	// Check headers
	if resp.Header.Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header to be 'value', got '%s'", resp.Header.Get("X-Custom"))
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type header to be 'application/json', got '%s'", resp.Header.Get("Content-Type"))
	}

	// Check body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected body '{\"status\":\"ok\"}', got '%s'", string(body))
	}
}

func TestMockServer_RequestCount(t *testing.T) {
	config := Config{
		Status: 200,
	}

	server := New(config)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	// Initial count should be 0
	if count := server.RequestCount(); count != 0 {
		t.Errorf("expected initial count 0, got %d", count)
	}

	// Make requests
	for i := 0; i < 5; i++ {
		resp, err := http.Get("http://" + server.Address() + "/")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()
	}

	// Check count
	if count := server.RequestCount(); count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

func TestMockServer_EmptyBody(t *testing.T) {
	config := Config{
		Status: 204,
	}

	server := New(config)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	resp, err := http.Get("http://" + server.Address() + "/")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body, got '%s'", string(body))
	}
}

func TestMockServer_MultipleRequests(t *testing.T) {
	config := Config{
		Status: 200,
		Body:   "response",
	}

	server := New(config)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Shutdown(context.Background())

	// Make multiple requests to different paths
	paths := []string{"/", "/api", "/test", "/path/to/resource"}
	for _, path := range paths {
		resp, err := http.Get("http://" + server.Address() + path)
		if err != nil {
			t.Fatalf("failed to make request to %s: %v", path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("path %s: expected status 200, got %d", path, resp.StatusCode)
		}
	}

	// All paths should be counted
	if count := server.RequestCount(); count != len(paths) {
		t.Errorf("expected count %d, got %d", len(paths), count)
	}
}

func TestMockServer_DoubleStart(t *testing.T) {
	config := Config{Status: 200}
	server := New(config)

	if err := server.Start(); err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer server.Shutdown(context.Background())

	// Second start should fail
	if err := server.Start(); err == nil {
		t.Error("expected error on second start")
	}
}

func TestMockServer_ShutdownWithoutStart(t *testing.T) {
	config := Config{Status: 200}
	server := New(config)

	// Should not error when shutting down without starting
	if err := server.Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown without start returned error: %v", err)
	}
}
