package vcl

import (
	"fmt"
	"strings"
)

// BackendAddress represents a backend's host and port
type BackendAddress struct {
	Host string
	Port string
}

// ParseAddress parses a "host:port" address into separate host and port strings
func ParseAddress(addr string) (host string, port string, err error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid address format %q, expected host:port", addr)
	}
	return parts[0], parts[1], nil
}
