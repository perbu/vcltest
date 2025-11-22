package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/formatter"
	"github.com/perbu/vcltest/pkg/recorder"
	"github.com/perbu/vcltest/pkg/runner"
	"github.com/perbu/vcltest/pkg/service"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/vcl"
)

// runTests runs the test file
func runTests(ctx context.Context, testFile string, verbose bool, cliVCL string) error {
	// Setup logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Resolve VCL file path
	vclPath, err := testspec.ResolveVCL(testFile, cliVCL)
	if err != nil {
		return fmt.Errorf("resolving VCL file: %w", err)
	}
	logger.Debug("Resolved VCL file", "path", vclPath)

	// Load test specifications
	logger.Debug("Loading test file", "file", testFile)
	tests, err := testspec.Load(testFile)
	if err != nil {
		return fmt.Errorf("loading test file: %w", err)
	}

	logger.Debug("Loaded tests", "count", len(tests))

	// Check if any tests are scenario-based (require time control)
	hasScenarioTests := false
	for _, test := range tests {
		if test.IsScenario() {
			hasScenarioTests = true
			break
		}
	}

	// Create temporary directories for Varnish
	workDir, err := os.MkdirTemp("", "vcltest-work-*")
	if err != nil {
		return fmt.Errorf("creating work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	varnishDir, err := os.MkdirTemp("", "vcltest-varnish-*")
	if err != nil {
		return fmt.Errorf("creating varnish dir: %w", err)
	}
	defer os.RemoveAll(varnishDir)

	// Create absolute path to empty.vcl
	emptyVCLPath, err := filepath.Abs("examples/empty.vcl")
	if err != nil {
		return fmt.Errorf("resolving empty VCL path: %w", err)
	}

	// Create service configuration
	serviceCfg := &service.Config{
		VarnishadmPort: 6082,
		Secret:         "test-secret",
		VarnishCmd:     "varnishd",
		VCLPath:        emptyVCLPath, // Dummy VCL for service manager
		VarnishConfig: &varnish.Config{
			WorkDir:    workDir,
			VarnishDir: varnishDir,
			VCLPath:    emptyVCLPath, // VCL file for -f flag
			Varnish: varnish.VarnishConfig{
				AdminPort: 6082,
				HTTP: []varnish.HTTPConfig{
					{Address: "127.0.0.1", Port: 8080},
				},
				Time: varnish.TimeConfig{
					Enabled: hasScenarioTests, // Enable faketime only if needed
				},
			},
		},
		Logger: logger,
	}

	// Create service manager
	manager, err := service.NewManager(serviceCfg)
	if err != nil {
		return fmt.Errorf("creating service manager: %w", err)
	}

	// Start services in background
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		if err := manager.Start(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("service error: %w", err)
		}
	}()

	// Wait for services to be ready
	logger.Debug("Waiting for Varnish to be ready...")
	select {
	case err := <-errChan:
		return fmt.Errorf("varnish failed to start: %w", err)
	case <-time.After(2 * time.Second):
		// Services appear to be running, continue
	}

	// Get varnishadm interface
	varnishadm := manager.GetVarnishadm()
	if varnishadm == nil {
		return fmt.Errorf("varnishadm not available")
	}

	// Create and start varnishlog recorder
	rec, err := recorder.New(varnishDir, logger)
	if err != nil {
		return fmt.Errorf("creating recorder: %w", err)
	}

	if err := rec.Start(); err != nil {
		return fmt.Errorf("starting recorder: %w", err)
	}
	defer rec.Stop()

	// Give varnishlog time to connect to VSM
	time.Sleep(500 * time.Millisecond)

	// Create test runner
	testRunner := runner.New(varnishadm, "http://127.0.0.1:8080", varnishDir, logger, rec)

	// Set time controller for scenario-based tests
	testRunner.SetTimeController(manager)

	// Determine backends needed across all tests
	backendAddresses, mockBackends, err := startAllBackends(tests, logger)
	if err != nil {
		return fmt.Errorf("starting backends: %w", err)
	}
	defer stopAllBackends(mockBackends, logger)

	// Load VCL once with all backends
	logger.Debug("Loading shared VCL", "path", vclPath)
	if err := testRunner.LoadVCL(vclPath, backendAddresses); err != nil {
		return fmt.Errorf("loading shared VCL: %w", err)
	}
	defer testRunner.UnloadVCL()

	// Run tests
	passed := 0
	failed := 0
	useColor := formatter.ShouldUseColor()

	for i, test := range tests {
		fmt.Printf("\nTest %d: %s\n", i+1, test.Name)

		// Nuke the cache before each test to ensure clean state
		logger.Debug("Nuking cache before test", "test", test.Name)
		if _, err := varnishadm.BanNukeCache(); err != nil {
			return fmt.Errorf("failed to nuke cache before test %q: %w", test.Name, err)
		}

		testStart := time.Now()
		result, err := testRunner.RunTestWithSharedVCL(test)
		testDuration := time.Since(testStart)

		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			logger.Debug("Test failed with error", "test", test.Name, "duration_ms", testDuration.Milliseconds())
			failed++
			continue
		}

		if result.Passed {
			if useColor {
				fmt.Printf("  %s✓ PASSED%s\n", formatter.ColorGreen, formatter.ColorReset)
			} else {
				fmt.Printf("  ✓ PASSED\n")
			}
			logger.Debug("Test passed", "test", test.Name, "duration_ms", testDuration.Milliseconds())
			passed++
		} else {
			// Display enhanced error output with VCL trace
			if result.VCLTrace != nil && len(result.VCLTrace.ExecutedLines) > 0 {
				output := formatter.FormatTestFailure(
					result.TestName,
					result.Errors,
					result.VCLSource,
					result.VCLTrace.ExecutedLines,
					result.VCLTrace.BackendCalls,
					result.VCLTrace.VCLFlow,
					useColor,
				)
				fmt.Print(output)
			} else {
				// Fallback to simple error output if trace is not available
				if useColor {
					fmt.Printf("  %s✗ FAILED%s\n", formatter.ColorRed, formatter.ColorReset)
				} else {
					fmt.Printf("  ✗ FAILED\n")
				}
				for _, errMsg := range result.Errors {
					fmt.Printf("    - %s\n", errMsg)
				}
			}
			logger.Debug("Test failed", "test", test.Name, "duration_ms", testDuration.Milliseconds())
			failed++
		}
	}

	// Print summary
	fmt.Printf("\n")
	fmt.Printf("====================\n")
	fmt.Printf("Tests passed: %d/%d\n", passed, len(tests))
	if failed > 0 {
		fmt.Printf("Tests failed: %d/%d\n", failed, len(tests))
		return fmt.Errorf("some tests failed")
	}

	return nil
}

// startAllBackends starts all mock backends needed across all tests
func startAllBackends(tests []testspec.TestSpec, logger *slog.Logger) (map[string]vcl.BackendAddress, map[string]*backend.MockBackend, error) {
	addresses := make(map[string]vcl.BackendAddress)
	mockBackends := make(map[string]*backend.MockBackend)

	// Collect all unique backend names
	backendNames := make(map[string]bool)
	for _, test := range tests {
		if len(test.Backends) > 0 {
			for name := range test.Backends {
				backendNames[name] = true
			}
		} else {
			// Single backend mode
			backendNames["default"] = true
		}
	}

	// Start a mock backend for each name
	for name := range backendNames {
		cfg := backend.Config{
			Status:  200,
			Headers: make(map[string]string),
			Body:    "",
		}
		mock := backend.New(cfg)
		addr, err := mock.Start()
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("starting backend %q: %w", name, err)
		}

		host, port, err := vcl.ParseAddress(addr)
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("parsing address for backend %q: %w", name, err)
		}

		mockBackends[name] = mock
		addresses[name] = vcl.BackendAddress{Host: host, Port: port}
		logger.Debug("Started shared backend", "name", name, "address", addr)
	}

	return addresses, mockBackends, nil
}

// stopAllBackends stops all mock backends
func stopAllBackends(mockBackends map[string]*backend.MockBackend, logger *slog.Logger) {
	for name, mock := range mockBackends {
		if err := mock.Stop(); err != nil {
			logger.Warn("Failed to stop backend", "name", name, "error", err)
		}
	}
}
