package vcl

import (
	"testing"
)

func TestParseAddress_Valid(t *testing.T) {
	tests := []struct {
		name         string
		addr         string
		expectedHost string
		expectedPort string
	}{
		{
			name:         "Standard IPv4 address",
			addr:         "127.0.0.1:8080",
			expectedHost: "127.0.0.1",
			expectedPort: "8080",
		},
		{
			name:         "Hostname with port",
			addr:         "localhost:3000",
			expectedHost: "localhost",
			expectedPort: "3000",
		},
		{
			name:         "FQDN with port",
			addr:         "api.example.com:443",
			expectedHost: "api.example.com",
			expectedPort: "443",
		},
		{
			name:         "IPv4 with low port",
			addr:         "192.168.1.1:80",
			expectedHost: "192.168.1.1",
			expectedPort: "80",
		},
		{
			name:         "IPv4 with high port",
			addr:         "10.0.0.1:65535",
			expectedHost: "10.0.0.1",
			expectedPort: "65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := ParseAddress(tt.addr)
			if err != nil {
				t.Fatalf("ParseAddress(%q) unexpected error: %v", tt.addr, err)
			}
			if host != tt.expectedHost {
				t.Errorf("ParseAddress(%q) host = %q, want %q", tt.addr, host, tt.expectedHost)
			}
			if port != tt.expectedPort {
				t.Errorf("ParseAddress(%q) port = %q, want %q", tt.addr, port, tt.expectedPort)
			}
		})
	}
}

func TestParseAddress_Invalid(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "Missing port",
			addr: "127.0.0.1",
		},
		{
			name: "Missing host",
			addr: ":8080",
		},
		{
			name: "Empty string",
			addr: "",
		},
		{
			name: "Multiple colons",
			addr: "127.0.0.1:8080:9090",
		},
		{
			name: "Only colon",
			addr: ":",
		},
		{
			name: "URL format not supported",
			addr: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := ParseAddress(tt.addr)
			if err == nil {
				t.Errorf("ParseAddress(%q) expected error, got host=%q port=%q", tt.addr, host, port)
			}
		})
	}
}

func TestParseAddress_IPv6(t *testing.T) {
	// Note: IPv6 addresses with ports need brackets: [::1]:8080
	// But the current implementation splits on ":", so it won't handle IPv6 correctly
	// This test documents the current behavior
	tests := []struct {
		name        string
		addr        string
		expectError bool
	}{
		{
			name:        "IPv6 loopback without brackets",
			addr:        "::1:8080",
			expectError: true, // Multiple colons will fail
		},
		{
			name:        "IPv6 with brackets - not supported",
			addr:        "[::1]:8080",
			expectError: true, // Brackets won't be handled correctly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseAddress(tt.addr)
			if tt.expectError && err == nil {
				t.Errorf("ParseAddress(%q) expected error but got none", tt.addr)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ParseAddress(%q) unexpected error: %v", tt.addr, err)
			}
		})
	}
}
