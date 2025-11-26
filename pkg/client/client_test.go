package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/perbu/vcltest/pkg/testspec"
)

func TestMakeRequest_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/test",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.Status, http.StatusOK)
	}

	if resp.Body != "Hello, World!" {
		t.Errorf("Body = %q, want %q", resp.Body, "Hello, World!")
	}
}

func TestMakeRequest_WithHeaders(t *testing.T) {
	// Create test server that echoes headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request headers
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("Expected header X-Custom-Header=custom-value, got %q", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Expected header Authorization=Bearer token123, got %q", r.Header.Get("Authorization"))
		}

		// Send response with headers
		w.Header().Set("X-Response-Header", "response-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/api/test",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.Status, http.StatusOK)
	}

	if resp.Headers.Get("X-Response-Header") != "response-value" {
		t.Errorf("Response header X-Response-Header = %q, want %q",
			resp.Headers.Get("X-Response-Header"), "response-value")
	}
}

func TestMakeRequest_WithBody(t *testing.T) {
	// Create test server that reads body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		expectedBody := `{"name":"test","value":123}`
		if string(body) != expectedBody {
			t.Errorf("Request body = %q, want %q", string(body), expectedBody)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "POST",
		URL:    "/api/resource",
		Body:   `{"name":"test","value":123}`,
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Status != http.StatusCreated {
		t.Errorf("Status = %d, want %d", resp.Status, http.StatusCreated)
	}

	if resp.Body != "Created" {
		t.Errorf("Body = %q, want %q", resp.Body, "Created")
	}
}

func TestMakeRequest_NoRedirect(t *testing.T) {
	// Create test server that redirects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/original" {
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Final destination"))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/original",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	// Should return the redirect response, not follow it
	if resp.Status != http.StatusFound {
		t.Errorf("Status = %d, want %d (should not follow redirect)", resp.Status, http.StatusFound)
	}

	// Check Location header is present
	if resp.Headers.Get("Location") == "" {
		t.Error("Expected Location header in redirect response")
	}
}

func TestMakeRequest_DifferentMethods(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedMethod string
	}{
		{"GET request", "GET", "GET"},
		{"POST request", "POST", "POST"},
		{"PUT request", "PUT", "PUT"},
		{"DELETE request", "DELETE", "DELETE"},
		{"PATCH request", "PATCH", "PATCH"},
		{"HEAD request", "HEAD", "HEAD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that verifies method
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.expectedMethod {
					t.Errorf("Request method = %q, want %q", r.Method, tt.expectedMethod)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			req := testspec.RequestSpec{
				Method: tt.method,
				URL:    "/test",
			}

			resp, err := MakeRequest(nil, server.URL, req)
			if err != nil {
				t.Fatalf("MakeRequest() error = %v", err)
			}

			if resp.Status != http.StatusOK {
				t.Errorf("Status = %d, want %d", resp.Status, http.StatusOK)
			}
		})
	}
}

func TestMakeRequest_StatusCodes(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
	}{
		{"200 OK", http.StatusOK, "Success"},
		{"201 Created", http.StatusCreated, "Created"},
		{"204 No Content", http.StatusNoContent, ""},
		{"400 Bad Request", http.StatusBadRequest, "Bad Request"},
		{"401 Unauthorized", http.StatusUnauthorized, "Unauthorized"},
		{"403 Forbidden", http.StatusForbidden, "Forbidden"},
		{"404 Not Found", http.StatusNotFound, "Not Found"},
		{"500 Internal Server Error", http.StatusInternalServerError, "Server Error"},
		{"502 Bad Gateway", http.StatusBadGateway, "Bad Gateway"},
		{"503 Service Unavailable", http.StatusServiceUnavailable, "Unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			req := testspec.RequestSpec{
				Method: "GET",
				URL:    "/test",
			}

			resp, err := MakeRequest(nil, server.URL, req)
			if err != nil {
				t.Fatalf("MakeRequest() error = %v", err)
			}

			if resp.Status != tt.statusCode {
				t.Errorf("Status = %d, want %d", resp.Status, tt.statusCode)
			}

			if resp.Body != tt.responseBody {
				t.Errorf("Body = %q, want %q", resp.Body, tt.responseBody)
			}
		})
	}
}

func TestMakeRequest_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/test",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Body != "" {
		t.Errorf("Body = %q, want empty string", resp.Body)
	}
}

func TestMakeRequest_LargeBody(t *testing.T) {
	// Test with a larger response body
	largeBody := strings.Repeat("A", 10000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/large",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if len(resp.Body) != len(largeBody) {
		t.Errorf("Body length = %d, want %d", len(resp.Body), len(largeBody))
	}

	if resp.Body != largeBody {
		t.Error("Body content does not match expected large body")
	}
}

func TestMakeRequest_URLConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the path
		if r.URL.Path != "/api/v1/resource" {
			t.Errorf("Request path = %q, want %q", r.URL.Path, "/api/v1/resource")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/api/v1/resource",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.Status, http.StatusOK)
	}
}

func TestMakeRequest_ServerError(t *testing.T) {
	// Test with invalid URL (server not listening)
	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/test",
	}

	_, err := MakeRequest(nil, "http://localhost:1", req)
	if err == nil {
		t.Error("MakeRequest() expected error when server is not reachable")
	}

	if !strings.Contains(err.Error(), "making request") {
		t.Errorf("Error message = %v, want error containing 'making request'", err)
	}
}

func TestMakeRequest_MultipleHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set multiple response headers
		w.Header().Set("X-Header-1", "value1")
		w.Header().Set("X-Header-2", "value2")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	req := testspec.RequestSpec{
		Method: "GET",
		URL:    "/test",
	}

	resp, err := MakeRequest(nil, server.URL, req)
	if err != nil {
		t.Fatalf("MakeRequest() error = %v", err)
	}

	if resp.Headers.Get("X-Header-1") != "value1" {
		t.Errorf("X-Header-1 = %q, want %q", resp.Headers.Get("X-Header-1"), "value1")
	}

	if resp.Headers.Get("X-Header-2") != "value2" {
		t.Errorf("X-Header-2 = %q, want %q", resp.Headers.Get("X-Header-2"), "value2")
	}

	if resp.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", resp.Headers.Get("Content-Type"), "application/json")
	}
}
