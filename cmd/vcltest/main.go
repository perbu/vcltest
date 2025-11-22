package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/perbu/vcltest/pkg/config"
	"github.com/perbu/vcltest/pkg/service"
	"github.com/perbu/vcltest/pkg/varnish"
)

//go:embed .version
var embeddedVersion string

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	// Parse flags
	flags := flag.NewFlagSet("vcltest", flag.ExitOnError)
	verbose := flags.Bool("verbose", false, "verbose output")
	flags.BoolVar(verbose, "v", false, "verbose output (shorthand)")
	configFile := flags.String("config", "vcltest.yaml", "configuration file")
	showVersion := flags.Bool("version", false, "show version")
	vclFileFlag := flags.String("vcl", "", "VCL file to use for tests (overrides auto-detection)")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	// Handle version flag
	if *showVersion {
		fmt.Printf("vcltest version %s\n", embeddedVersion)
		return nil
	}

	// Check for file argument
	if flags.NArg() == 0 {
		return fmt.Errorf("missing file argument\nUsage: vcltest [options] <test-file.yaml|vcl-file>")
	}

	inputFile := flags.Arg(0)

	// Determine if input is a test file (.yaml) or VCL file (.vcl)
	if strings.HasSuffix(inputFile, ".yaml") || strings.HasSuffix(inputFile, ".yml") {
		// Run tests
		return runTests(ctx, inputFile, *verbose, *vclFileFlag)
	}

	// Otherwise, treat as VCL file (old behavior)
	vclFile := inputFile

	// Check if VCL file exists
	if _, err := os.Stat(vclFile); os.IsNotExist(err) {
		return fmt.Errorf("VCL file %q does not exist", vclFile)
	}

	// Setup logger
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Load configuration
	logger.Info("Loading configuration", "file", *configFile)
	cfg, err := config.Load(*configFile)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Create service configuration
	serviceCfg := &service.Config{
		VarnishadmPort: cfg.VarnishadmPort,
		Secret:         cfg.Secret,
		VarnishCmd:     cfg.VarnishCmd,
		VCLPath:        vclFile,
		VarnishConfig: &varnish.Config{
			WorkDir:     cfg.WorkDir,
			VarnishDir:  cfg.VarnishDir,
			StorageArgs: cfg.StorageArgs,
			License: varnish.LicenseConfig{
				Text: cfg.License.Text,
				File: cfg.License.File,
			},
			Varnish: varnish.VarnishConfig{
				AdminPort: cfg.Varnish.AdminPort,
				HTTP:      convertHTTPConfig(cfg.Varnish.HTTP),
				HTTPS:     convertHTTPSConfig(cfg.Varnish.HTTPS),
				ExtraArgs: cfg.Varnish.ExtraArgs,
			},
		},
		Logger: logger,
	}

	// Create service manager
	manager, err := service.NewManager(serviceCfg)
	if err != nil {
		return fmt.Errorf("creating service manager: %w", err)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start services
	logger.Info("Starting vcltest services")
	if err := manager.Start(ctx); err != nil {
		if err == context.Canceled {
			logger.Info("Services stopped gracefully")
			return nil
		}
		return fmt.Errorf("service error: %w", err)
	}

	return nil
}

func convertHTTPConfig(configs []config.HTTPConfig) []varnish.HTTPConfig {
	result := make([]varnish.HTTPConfig, len(configs))
	for i, cfg := range configs {
		result[i] = varnish.HTTPConfig{
			Address: cfg.Address,
			Port:    cfg.Port,
		}
	}
	return result
}

func convertHTTPSConfig(configs []config.HTTPSConfig) []varnish.HTTPSConfig {
	result := make([]varnish.HTTPSConfig, len(configs))
	for i, cfg := range configs {
		result[i] = varnish.HTTPSConfig{
			Address: cfg.Address,
			Port:    cfg.Port,
		}
	}
	return result
}
