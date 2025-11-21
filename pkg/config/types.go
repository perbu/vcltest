package config

// Config represents the complete application configuration
type Config struct {
	// VarnishadmPort is the port the varnishadm server listens on
	VarnishadmPort uint16 `yaml:"varnishadm_port"`
	// Secret is the shared secret for varnishadm authentication
	// If empty, a random secret will be generated
	Secret string `yaml:"secret,omitempty"`
	// VarnishCmd is the path to the varnishd executable
	// If empty, "varnishd" will be used (PATH lookup)
	VarnishCmd string `yaml:"varnish_cmd,omitempty"`
	// WorkDir is the working directory for Varnish files
	WorkDir string `yaml:"work_dir"`
	// VarnishDir is the varnish -n directory
	VarnishDir string `yaml:"varnish_dir"`
	// StorageArgs are additional storage arguments for varnishd
	StorageArgs []string `yaml:"storage_args,omitempty"`
	// License holds Varnish Enterprise license configuration
	License LicenseConfig `yaml:"license,omitempty"`
	// Varnish holds varnish daemon configuration
	Varnish VarnishConfig `yaml:"varnish"`
}

// LicenseConfig holds Varnish Enterprise license configuration
type LicenseConfig struct {
	// Text is the license text content
	Text string `yaml:"text,omitempty"`
	// File is the path to a license file
	File string `yaml:"file,omitempty"`
}

// VarnishConfig holds Varnish daemon configuration
type VarnishConfig struct {
	// AdminPort is the varnishadm control port
	AdminPort int `yaml:"admin_port"`
	// HTTP listening addresses
	HTTP []HTTPConfig `yaml:"http,omitempty"`
	// HTTPS listening addresses with TLS termination
	HTTPS []HTTPSConfig `yaml:"https,omitempty"`
	// ExtraArgs are additional command-line arguments for varnishd
	ExtraArgs []string `yaml:"extra_args,omitempty"`
}

// HTTPConfig defines an HTTP listening address
type HTTPConfig struct {
	// Address is the IP address to bind to (empty for all interfaces)
	Address string `yaml:"address,omitempty"`
	// Port is the port number
	Port int `yaml:"port"`
}

// HTTPSConfig defines an HTTPS listening address with TLS termination
type HTTPSConfig struct {
	// Address is the IP address to bind to (empty for all interfaces)
	Address string `yaml:"address,omitempty"`
	// Port is the port number
	Port int `yaml:"port"`
}
