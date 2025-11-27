package varnishadm

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Standard Varnish commands

// Ping sends a ping command to varnishadm
func (v *Server) Ping() (VarnishResponse, error) {
	return v.Exec("ping")
}

// Status returns the status of the Varnish child process
func (v *Server) Status() (VarnishResponse, error) {
	return v.Exec("status")
}

// Start starts the Varnish child process
func (v *Server) Start() (VarnishResponse, error) {
	return v.Exec("start")
}

// Stop stops the Varnish child process
func (v *Server) Stop() (VarnishResponse, error) {
	return v.Exec("stop")
}

// PanicShow shows the panic message if available
func (v *Server) PanicShow() (VarnishResponse, error) {
	return v.Exec("panic.show")
}

// PanicClear clears the panic message
func (v *Server) PanicClear() (VarnishResponse, error) {
	return v.Exec("panic.clear")
}

// VCL commands

// VCLLoad loads a VCL configuration from a file
func (v *Server) VCLLoad(name, path string) (VarnishResponse, error) {
	start := time.Now()
	defer func() {
		v.logger.Debug("VCLLoad completed", "name", name, "duration_ms", time.Since(start).Milliseconds())
	}()
	cmd := fmt.Sprintf("vcl.load %s %s", name, path)
	return v.Exec(cmd)
}

// VCLUse switches to using the specified VCL configuration
func (v *Server) VCLUse(name string) (VarnishResponse, error) {
	start := time.Now()
	defer func() {
		v.logger.Debug("VCLUse completed", "name", name, "duration_ms", time.Since(start).Milliseconds())
	}()
	cmd := fmt.Sprintf("vcl.use %s", name)
	return v.Exec(cmd)
}

// VCLDiscard discards a VCL configuration
func (v *Server) VCLDiscard(name string) (VarnishResponse, error) {
	start := time.Now()
	defer func() {
		v.logger.Debug("VCLDiscard completed", "name", name, "duration_ms", time.Since(start).Milliseconds())
	}()
	cmd := fmt.Sprintf("vcl.discard %s", name)
	return v.Exec(cmd)
}

// VCLList lists all VCL configurations
func (v *Server) VCLList() (VarnishResponse, error) {
	return v.Exec("vcl.list")
}

// VCLListStructured lists all VCL configurations and returns parsed results
func (v *Server) VCLListStructured() (*VCLListResult, error) {
	resp, err := v.Exec("vcl.list")
	if err != nil {
		return nil, err
	}

	if resp.statusCode != ClisOk {
		return nil, fmt.Errorf("vcl.list command failed with status %d: %s", resp.statusCode, resp.payload)
	}

	return parseVCLList(resp.payload)
}

// VCLShow shows the VCL source with verbose output including config headers
func (v *Server) VCLShow(name string) (VarnishResponse, error) {
	cmd := fmt.Sprintf("vcl.show -v %s", name)
	return v.Exec(cmd)
}

// VCLShowStructured shows VCL source and returns parsed config mapping
// This is useful for mapping trace log config IDs to filenames
func (v *Server) VCLShowStructured(name string) (*VCLShowResult, error) {
	resp, err := v.VCLShow(name)
	if err != nil {
		return nil, err
	}

	if resp.statusCode != ClisOk {
		return nil, fmt.Errorf("vcl.show -v command failed with status %d: %s", resp.statusCode, resp.payload)
	}

	return parseVCLShow(resp.payload)
}

// Parameter commands

// ParamShow shows the value of a parameter
func (v *Server) ParamShow(name string) (VarnishResponse, error) {
	if name == "" {
		return v.Exec("param.show")
	}
	cmd := fmt.Sprintf("param.show %s", name)
	return v.Exec(cmd)
}

// ParamSet sets the value of a parameter
func (v *Server) ParamSet(name, value string) (VarnishResponse, error) {
	cmd := fmt.Sprintf("param.set %s %s", name, value)
	resp, err := v.Exec(cmd)
	if err != nil {
		return resp, err
	}
	if resp.statusCode != ClisOk {
		return resp, fmt.Errorf("param.set %s failed with status %d: %s", name, resp.statusCode, resp.payload)
	}
	return resp, nil
}

// ParamValue defines acceptable parameter value types
type ParamValue interface {
	int | bool | float64 | string | time.Duration | Size
}

// ParamSetter is a minimal interface for types that can set parameters
type ParamSetter interface {
	ParamSet(name, value string) (VarnishResponse, error)
}

// ParamSetTyped sets a parameter with type-safe value conversion.
// Note: This is a package function (not a method) because Go doesn't allow type parameters on methods.
func ParamSetTyped[T ParamValue](v ParamSetter, name string, value T) (VarnishResponse, error) {
	var strValue string

	switch val := any(value).(type) {
	case int:
		strValue = strconv.Itoa(val)
	case bool:
		if val {
			strValue = "on"
		} else {
			strValue = "off"
		}
	case float64:
		strValue = strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		strValue = val
	case time.Duration:
		strValue = fmt.Sprintf("%.0fs", val.Seconds())
	case Size:
		strValue = val.String()
	}

	return v.ParamSet(name, strValue)
}

// Varnish Enterprise TLS commands

// TLSCertList lists all TLS certificates
func (v *Server) TLSCertList() (VarnishResponse, error) {
	return v.Exec("tls.cert.list")
}

// TLSCertListStructured lists all TLS certificates and returns parsed results
func (v *Server) TLSCertListStructured() (*TLSCertListResult, error) {
	resp, err := v.Exec("tls.cert.list")
	if err != nil {
		return nil, err
	}

	if resp.statusCode != ClisOk {
		return nil, fmt.Errorf("tls.cert.list command failed with status %d: %s", resp.statusCode, resp.payload)
	}

	return parseTLSCertList(resp.payload)
}

// TLSCertLoad loads a combined TLS certificate+key PEM file
func (v *Server) TLSCertLoad(name, certFile string) (VarnishResponse, error) {
	cmd := fmt.Sprintf("tls.cert.load %s %s", name, certFile)
	return v.Exec(cmd)
}

// TLSCertDiscard discards a TLS certificate by ID
func (v *Server) TLSCertDiscard(id string) (VarnishResponse, error) {
	cmd := fmt.Sprintf("tls.cert.discard %s", id)
	return v.Exec(cmd)
}

// TLSCertCommit commits the loaded TLS certificates
func (v *Server) TLSCertCommit() (VarnishResponse, error) {
	return v.Exec("tls.cert.commit")
}

// TLSCertRollback rolls back the TLS certificate changes
func (v *Server) TLSCertRollback() (VarnishResponse, error) {
	return v.Exec("tls.cert.rollback")
}

// TLSCertReload reloads all TLS certificates
func (v *Server) TLSCertReload() (VarnishResponse, error) {
	return v.Exec("tls.cert.reload")
}

// Ban commands

// BanNukeCache nukes the entire cache by issuing a ban that matches everything
func (v *Server) BanNukeCache() (VarnishResponse, error) {
	return v.Exec("ban req.url ~ .")
}

// Debug commands

// DebugListenAddress returns the actual listen addresses bound by varnishd.
// This is an undocumented debug command that waits until the acceptor is ready.
// Output format: "<name> <address> <port>" for TCP, "<name> <path> -" for Unix sockets.
func (v *Server) DebugListenAddress() (VarnishResponse, error) {
	return v.Exec("debug.listen_address")
}

// ListenAddress represents a single listen socket bound by varnishd
type ListenAddress struct {
	Name    string // Socket name (e.g., "a0", "http", or custom from -a name=:port)
	Address string // IP address (for TCP) or path (for Unix sockets)
	Port    int    // Port number (-1 for Unix sockets)
}

// DebugListenAddressStructured returns parsed listen addresses.
// This is useful for discovering dynamically assigned ports when using -a :0.
func (v *Server) DebugListenAddressStructured() ([]ListenAddress, error) {
	resp, err := v.DebugListenAddress()
	if err != nil {
		return nil, err
	}
	if resp.statusCode != ClisOk {
		return nil, fmt.Errorf("debug.listen_address failed (status %d): %s",
			resp.statusCode, resp.payload)
	}
	return parseListenAddresses(resp.payload)
}

// parseListenAddresses parses the debug.listen_address output.
// Format: "<name> <address> <port>" per line (port is "-" for Unix sockets)
func parseListenAddresses(payload string) ([]ListenAddress, error) {
	var addresses []ListenAddress

	lines := splitLines(payload)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid listen_address line: %q (expected 3 fields)", line)
		}

		addr := ListenAddress{
			Name:    fields[0],
			Address: fields[1],
		}

		if fields[2] == "-" {
			// Unix socket
			addr.Port = -1
		} else {
			port, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, fmt.Errorf("invalid port in listen_address: %q", fields[2])
			}
			addr.Port = port
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

// splitLines splits a string into lines, handling both \n and \r\n
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
