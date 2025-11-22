package vcl

import (
	"fmt"
	"os"
	"strings"
)

// BackendAddress represents a backend's host and port
type BackendAddress struct {
	Host string
	Port string
}

// ReplaceBackend replaces backend definitions in VCL with a mock backend address
// This is a simple text-based replacement for testing purposes (legacy single-backend)
func ReplaceBackend(vclContent, mockHost, mockPort string) (string, error) {
	backends := map[string]BackendAddress{
		"default": {Host: mockHost, Port: mockPort},
	}
	return ReplaceBackends(vclContent, backends)
}

// ReplaceBackends replaces multiple named backend placeholders in VCL
// Placeholders follow the pattern: __BACKEND_HOST_BACKENDNAME__ and __BACKEND_PORT_BACKENDNAME__
// where BACKENDNAME is the backend name in uppercase
// For the "default" backend, also replaces legacy __BACKEND_HOST__ and __BACKEND_PORT__
func ReplaceBackends(vclContent string, backends map[string]BackendAddress) (string, error) {
	result := vclContent

	for name, addr := range backends {
		// Convert backend name to uppercase for placeholder matching
		nameUpper := strings.ToUpper(name)

		// Replace named placeholders
		hostPlaceholder := fmt.Sprintf("__BACKEND_HOST_%s__", nameUpper)
		result = strings.ReplaceAll(result, hostPlaceholder, addr.Host)

		portPlaceholder := fmt.Sprintf("__BACKEND_PORT_%s__", nameUpper)
		result = strings.ReplaceAll(result, portPlaceholder, addr.Port)

		// For "default" backend, also replace legacy unnamed placeholders
		if name == "default" {
			result = strings.ReplaceAll(result, "__BACKEND_HOST__", addr.Host)
			result = strings.ReplaceAll(result, "__BACKEND_PORT__", addr.Port)
		}
	}

	return result, nil
}

// LoadAndReplace loads a VCL file, replaces backend address, and returns the modified content
func LoadAndReplace(vclPath, mockHost, mockPort string) (string, error) {
	data, err := os.ReadFile(vclPath)
	if err != nil {
		return "", fmt.Errorf("reading VCL file: %w", err)
	}

	content := string(data)
	return ReplaceBackend(content, mockHost, mockPort)
}

// ParseAddress parses a "host:port" address into separate host and port strings
func ParseAddress(addr string) (host string, port string, err error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid address format %q, expected host:port", addr)
	}
	return parts[0], parts[1], nil
}
