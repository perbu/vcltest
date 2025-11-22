package varnish

// Config holds the configuration for building Varnish command-line arguments
type Config struct {
	WorkDir     string
	VarnishDir  string
	StorageArgs []string
	VCLPath     string // Optional: VCL file to load on startup (for -f flag)

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
	Time      TimeConfig
}

// TimeConfig controls optional time manipulation using libfaketime
type TimeConfig struct {
	Enabled bool   // Enable faketime (default: false for normal operation)
	LibPath string // Optional: override libfaketime library path (auto-detected if empty)
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
