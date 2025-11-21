package vcl

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ReplaceBackend replaces backend definitions in VCL with a mock backend address
// This is a simple text-based replacement for testing purposes
func ReplaceBackend(vclContent, mockHost, mockPort string) (string, error) {
	// Replace .host = "..." with .host = mockHost
	hostRegex := regexp.MustCompile(`(\.host\s*=\s*)"[^"]+"`)
	vclContent = hostRegex.ReplaceAllString(vclContent, fmt.Sprintf(`$1"%s"`, mockHost))

	// Replace .port = "..." with .port = mockPort
	portRegex := regexp.MustCompile(`(\.port\s*=\s*)"[^"]+"`)
	vclContent = portRegex.ReplaceAllString(vclContent, fmt.Sprintf(`$1"%s"`, mockPort))

	return vclContent, nil
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
