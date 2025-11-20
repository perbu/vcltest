package varnish

// Config holds the configuration for building Varnish command-line arguments
type Config struct {
	WorkDir     string
	VarnishDir  string
	StorageArgs []string

	License LicenseConfig
	Varnish VarnishConfig
}

// LicenseConfig holds Varnish Enterprise license configuration
type LicenseConfig struct {
	Text string // License text content
	File string // Path to license file
}

// VarnishConfig holds Varnish daemon configuration
type VarnishConfig struct {
	AdminPort int
	HTTP      []HTTPConfig
	HTTPS     []HTTPSConfig
	ExtraArgs []string
}

// HTTPConfig defines an HTTP listening address
type HTTPConfig struct {
	Address string // IP address to bind to (empty for all interfaces)
	Port    int    // Port number
}

// HTTPSConfig defines an HTTPS listening address with TLS termination
type HTTPSConfig struct {
	Address string // IP address to bind to (empty for all interfaces)
	Port    int    // Port number
}
