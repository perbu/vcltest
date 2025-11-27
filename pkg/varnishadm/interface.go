package varnishadm

import "context"

// VarnishadmInterface defines the interface for varnishadm implementations
type VarnishadmInterface interface {
	// Listen creates a TCP listener and returns the actual port.
	// If the configured port is 0, a random available port is assigned.
	// This must be called before Run().
	Listen() (uint16, error)
	// GetPort returns the port the server is listening on.
	// Returns 0 if Listen() hasn't been called yet.
	GetPort() uint16
	// Run starts the varnishadm server and blocks until context is cancelled.
	// Listen() must be called before Run().
	Run(ctx context.Context) error
	// Exec executes a command and returns the response
	Exec(cmd string) (VarnishResponse, error)

	// Standard commands
	Ping() (VarnishResponse, error)
	Status() (VarnishResponse, error)
	Start() (VarnishResponse, error)
	Stop() (VarnishResponse, error)
	PanicShow() (VarnishResponse, error)
	PanicClear() (VarnishResponse, error)

	// VCL commands
	VCLLoad(name, path string) (VarnishResponse, error)
	VCLUse(name string) (VarnishResponse, error)
	VCLDiscard(name string) (VarnishResponse, error)
	VCLList() (VarnishResponse, error)
	VCLListStructured() (*VCLListResult, error)
	VCLShow(name string) (VarnishResponse, error)
	VCLShowStructured(name string) (*VCLShowResult, error)

	// Parameter commands
	ParamShow(name string) (VarnishResponse, error)
	ParamSet(name, value string) (VarnishResponse, error)

	// Ban commands
	BanNukeCache() (VarnishResponse, error)

	// Varnish Enterprise TLS commands
	TLSCertList() (VarnishResponse, error)
	TLSCertListStructured() (*TLSCertListResult, error)
	TLSCertLoad(name, certFile string) (VarnishResponse, error)
	TLSCertCommit() (VarnishResponse, error)
	TLSCertRollback() (VarnishResponse, error)
	TLSCertReload() (VarnishResponse, error)

	// Debug commands
	DebugListenAddress() (VarnishResponse, error)
	DebugListenAddressStructured() ([]ListenAddress, error)
}
