package varnish

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

// BuildArgs constructs the complete varnishd command line arguments
func BuildArgs(cfg *Config) []string {
	args := make([]string, 0)

	// Add license file if present (either as direct text or file path)
	if cfg.License.Text != "" || cfg.License.File != "" {
		licensePath := filepath.Join(cfg.WorkDir, "varnish-enterprise.lic")
		args = append(args, "-L", licensePath)
	}

	// Basic arguments
	secretPath := filepath.Join(cfg.WorkDir, "secret")
	args = append(args, "-S", secretPath) // Enable auth on the CLI with secret file
	args = append(args, "-M", fmt.Sprintf("localhost:%d", cfg.Varnish.AdminPort))
	args = append(args, "-n", cfg.VarnishDir)
	args = append(args, "-F")     // Run in foreground
	args = append(args, "-f", "") // Empty VCL - VCL will be loaded via varnishadm CLI after startup

	// HTTP listening addresses
	for _, http := range cfg.Varnish.HTTP {
		if http.Address != "" {
			args = append(args, "-a", fmt.Sprintf("%s:%d,http", http.Address, http.Port))
		} else {
			args = append(args, "-a", fmt.Sprintf(":%d,http", http.Port))
		}
	}

	// HTTPS listening addresses with TLS termination
	// Format: ":443,https" enables TLS termination on port 443
	// Certificates will be loaded via varnishadm after startup
	for _, https := range cfg.Varnish.HTTPS {
		var listenSpec string
		if https.Address != "" {
			listenSpec = fmt.Sprintf("%s:%d,https", https.Address, https.Port)
		} else {
			listenSpec = fmt.Sprintf(":%d,https", https.Port)
		}
		args = append(args, "-a", listenSpec)
	}

	// Add storage arguments
	args = append(args, cfg.StorageArgs...)

	// Add extra args (these take precedence as they're appended last)
	args = append(args, cfg.Varnish.ExtraArgs...)

	// Set non-user-controllable parameters
	args = append(args, "-p", "vcl_path="+filepath.Join(cfg.WorkDir, "vcl")) // vcl_path points to the generated VCL directory

	return args
}

// GetParamName extracts the Varnish parameter name from the yaml struct tag.
// Returns the parameter name (without ",omitempty" suffix) or empty string if no yaml tag exists.
func GetParamName(field reflect.StructField) string {
	yamlTag := field.Tag.Get("yaml")
	if yamlTag == "" || yamlTag == "-" {
		return ""
	}
	// Remove ",omitempty" or other tag options
	if idx := strings.Index(yamlTag, ","); idx != -1 {
		return yamlTag[:idx]
	}
	return yamlTag
}
