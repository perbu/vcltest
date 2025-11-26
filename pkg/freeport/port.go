package freeport

import (
	"fmt"
	"net"
)

// FindFreePort finds an available TCP port on the specified address.
// If address is empty, it defaults to "127.0.0.1".
// Returns the port number or an error if no port is available.
//
// Note: There is a small race window between closing the listener and
// another process binding to the same port. This is acceptable for
// sequential test execution.
func FindFreePort(address string) (int, error) {
	if address == "" {
		address = "127.0.0.1"
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:0", address))
	if err != nil {
		return 0, fmt.Errorf("finding free port: %w", err)
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	return port, nil
}

// FindFreePorts finds multiple available TCP ports on the specified address.
// Returns a slice of port numbers or an error.
func FindFreePorts(address string, count int) ([]int, error) {
	if address == "" {
		address = "127.0.0.1"
	}

	listeners := make([]net.Listener, 0, count)
	ports := make([]int, 0, count)

	// Open all listeners first to ensure we get unique ports
	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:0", address))
		if err != nil {
			// Close any listeners we already opened
			for _, listener := range listeners {
				listener.Close()
			}
			return nil, fmt.Errorf("finding free port %d of %d: %w", i+1, count, err)
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}

	// Close all listeners
	for _, l := range listeners {
		l.Close()
	}

	return ports, nil
}
