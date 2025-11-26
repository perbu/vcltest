package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/formatter"
	"github.com/perbu/vcltest/pkg/recorder"
	"github.com/perbu/vcltest/pkg/runner"
	"github.com/perbu/vcltest/pkg/service"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/vclloader"
)

// convertRoutes converts testspec routes to backend routes
func convertRoutes(routes map[string]testspec.RouteSpec) map[string]backend.RouteConfig {
	if routes == nil {
		return nil
	}
	result := make(map[string]backend.RouteConfig, len(routes))
	for path, spec := range routes {
		result[path] = backend.RouteConfig{
			Status:      spec.Status,
			Headers:     spec.Headers,
			Body:        spec.Body,
			FailureMode: spec.FailureMode,
		}
	}
	return result
}

// runTests runs the test file
func runTests(ctx context.Context, testFile string, verbose bool, cliVCL string, debugDump bool) error {
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
	if !debugDump {
		defer os.RemoveAll(workDir)
	}

	varnishDir, err := os.MkdirTemp("", "vcltest-varnish-*")
	if err != nil {
		return fmt.Errorf("creating varnish dir: %w", err)
	}
	if !debugDump {
		defer os.RemoveAll(varnishDir)
	}

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

	// Create test runner (use workDir since vcl_path points there for include resolution)
	testRunner := runner.New(varnishadm, "http://127.0.0.1:8080", workDir, logger, rec)

	// Set time controller for scenario-based tests
	testRunner.SetTimeController(manager)

	// Determine backends needed across all tests
	backendAddresses, mockBackends, err := startAllBackends(tests, logger)
	if err != nil {
		return fmt.Errorf("starting backends: %w", err)
	}
	defer stopAllBackends(mockBackends, logger)

	// Set mock backends for dynamic reconfiguration in scenario tests
	testRunner.SetMockBackends(mockBackends)

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
			if result.VCLTrace != nil && len(result.VCLTrace.Files) > 0 {
				// Convert runner.VCLFileInfo to formatter.VCLFileInfo
				var files []formatter.VCLFileInfo
				for _, f := range result.VCLTrace.Files {
					files = append(files, formatter.VCLFileInfo{
						ConfigID:      f.ConfigID,
						Filename:      f.Filename,
						Source:        f.Source,
						ExecutedLines: f.ExecutedLines,
					})
				}

				output := formatter.FormatTestFailure(
					result.TestName,
					result.Errors,
					files,
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

	// Create debug dump if enabled
	if debugDump {
		dumpDir, err := createDebugDump(testFile, vclPath, workDir, varnishDir, testRunner, tests, passed, failed, logger)
		if err != nil {
			logger.Warn("Failed to create debug dump", "error", err)
		} else {
			fmt.Printf("\nDebug artifacts saved to: %s\n", dumpDir)
		}
	}

	if failed > 0 {
		fmt.Printf("Tests failed: %d/%d\n", failed, len(tests))
		return fmt.Errorf("some tests failed")
	}

	return nil
}

// startAllBackends starts all mock backends needed across all tests
func startAllBackends(tests []testspec.TestSpec, logger *slog.Logger) (map[string]vclloader.BackendAddress, map[string]*backend.MockBackend, error) {
	addresses := make(map[string]vclloader.BackendAddress)
	mockBackends := make(map[string]*backend.MockBackend)

	// Collect backend configurations from all tests
	// For shared VCL mode, we use the configuration from the FIRST test that defines each backend
	backendConfigs := make(map[string]testspec.BackendSpec)

	for _, test := range tests {
		for name, spec := range test.Backends {
			if _, exists := backendConfigs[name]; !exists {
				backendConfigs[name] = spec
			}
		}
	}

	// If no backends were found in tests, create a default one
	if len(backendConfigs) == 0 {
		backendConfigs["default"] = testspec.BackendSpec{
			Status: 200,
		}
	}

	// Start a mock backend for each configuration
	for name, spec := range backendConfigs {
		cfg := backend.Config{
			Status:      spec.Status,
			Headers:     spec.Headers,
			Body:        spec.Body,
			FailureMode: spec.FailureMode,
			Routes:      convertRoutes(spec.Routes),
		}
		// Apply default status if not set
		if cfg.Status == 0 {
			cfg.Status = 200
		}

		mock := backend.New(cfg)
		addr, err := mock.Start()
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("starting backend %q: %w", name, err)
		}

		host, port, err := vclloader.ParseAddress(addr)
		if err != nil {
			stopAllBackends(mockBackends, logger)
			return nil, nil, fmt.Errorf("parsing address for backend %q: %w", name, err)
		}

		mockBackends[name] = mock
		addresses[name] = vclloader.BackendAddress{Host: host, Port: port}
		logger.Debug("Started shared backend", "name", name, "address", addr, "body_len", len(spec.Body))
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

// createDebugDump creates a debug dump directory with all test artifacts
func createDebugDump(testFile, vclPath, workDir, varnishDir string, testRunner *runner.Runner, tests []testspec.TestSpec, passed, failed int, logger *slog.Logger) (string, error) {
	// Create dump directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	testBasename := filepath.Base(testFile)
	testBasename = strings.TrimSuffix(testBasename, filepath.Ext(testBasename))
	dumpDir := filepath.Join("/tmp", fmt.Sprintf("vcltest-debug-%s-%s", testBasename, timestamp))

	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		return "", fmt.Errorf("creating dump directory: %w", err)
	}

	logger.Debug("Creating debug dump", "dir", dumpDir)

	// Copy test YAML file
	if err := copyFile(testFile, filepath.Join(dumpDir, "test.yaml")); err != nil {
		logger.Warn("Failed to copy test file", "error", err)
	}

	// Copy original VCL file
	if err := copyFile(vclPath, filepath.Join(dumpDir, "original.vcl")); err != nil {
		logger.Warn("Failed to copy VCL file", "error", err)
	}

	// Save modified VCL (from runner)
	if modifiedVCL := testRunner.GetLoadedVCLSource(); modifiedVCL != "" {
		if err := os.WriteFile(filepath.Join(dumpDir, "modified.vcl"), []byte(modifiedVCL), 0644); err != nil {
			logger.Warn("Failed to save modified VCL", "error", err)
		}
	}

	// Copy varnishlog output
	varnishLogPath := filepath.Join(varnishDir, "varnish.log")
	if err := copyFile(varnishLogPath, filepath.Join(dumpDir, "varnish.log")); err != nil {
		logger.Warn("Failed to copy varnishlog", "error", err)
	}

	// Copy faketime control file and document its mtime
	faketimePath := filepath.Join(workDir, "faketime.control")
	if err := copyFile(faketimePath, filepath.Join(dumpDir, "faketime.control")); err != nil {
		// This is expected if faketime wasn't used, so don't warn
		logger.Debug("Faketime control file not found (expected if not using time scenarios)", "error", err)
	} else {
		// Document the faketime control file's mtime (this is how libfaketime tracks time)
		if info, err := os.Stat(faketimePath); err == nil {
			faketimeInfo := fmt.Sprintf("Faketime Control File Information\n"+
				"===================================\n\n"+
				"The faketime.control file uses its modification time (mtime) to control the fake time.\n"+
				"Varnish reads this file's mtime to determine what time it thinks it is.\n\n"+
				"Final mtime: %s\n"+
				"File size: %d bytes (always empty, only mtime matters)\n\n"+
				"When vcltest calls AdvanceTimeBy(offset), it runs:\n"+
				"  os.Chtimes(faketime.control, t0+offset, t0+offset)\n\n"+
				"This changes the file's mtime, which libfaketime intercepts when Varnish\n"+
				"calls stat() or similar syscalls, making Varnish think time has advanced.\n",
				info.ModTime().Format("2006-01-02 15:04:05.000"),
				info.Size(),
			)
			os.WriteFile(filepath.Join(dumpDir, "faketime-info.txt"), []byte(faketimeInfo), 0644)
		}
	}

	// Copy varnish secret file
	secretPath := filepath.Join(workDir, "secret")
	if err := copyFile(secretPath, filepath.Join(dumpDir, "secret")); err != nil {
		logger.Warn("Failed to copy secret file", "error", err)
	}

	// Create README with test run information
	readme := fmt.Sprintf(`VCLTest Debug Dump
==================

Generated: %s
Test file: %s
VCL file: %s

Test Results:
- Passed: %d/%d
- Failed: %d/%d

Files in this directory:
- test.yaml: The original test specification
- original.vcl: The original VCL file before modification
- modified.vcl: The VCL file with backend addresses replaced
- varnish.log: The varnishlog output from test execution
- faketime.control: The libfaketime control file (if time scenarios used)
- faketime-info.txt: Explanation of how faketime works (if time scenarios used)
- secret: The varnishadm authentication secret
- README.txt: This file

Temporary Directories (preserved):
- Work dir: %s
- Varnish dir: %s

To manually inspect Varnish state, you can use:
  varnishadm -T localhost:6082 -S %s <command>

Example commands:
  varnishadm -T localhost:6082 -S %s vcl.list
  varnishadm -T localhost:6082 -S %s vcl.show shared-vcl
  varnishadm -T localhost:6082 -S %s backend.list

Note: Varnish is no longer running, these directories are for forensic analysis only.
`,
		time.Now().Format("2006-01-02 15:04:05"),
		testFile,
		vclPath,
		passed, len(tests),
		failed, len(tests),
		workDir,
		varnishDir,
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
	)

	if err := os.WriteFile(filepath.Join(dumpDir, "README.txt"), []byte(readme), 0644); err != nil {
		logger.Warn("Failed to create README", "error", err)
	}

	return dumpDir, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
