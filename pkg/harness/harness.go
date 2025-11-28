package harness

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/recorder"
	"github.com/perbu/vcltest/pkg/runner"
	"github.com/perbu/vcltest/pkg/service"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/vclmod"
)

// Harness orchestrates VCL test execution.
type Harness struct {
	cfg    *Config
	logger *slog.Logger

	// Runtime state
	workDir        string
	varnishDir     string
	httpPort       int // Dynamically assigned HTTP port for Varnish
	manager        *service.Manager
	recorder       *recorder.Recorder
	testRunner     *runner.Runner
	mockBackends   map[string]*backend.MockBackend
	cancelServices context.CancelFunc // Cancels the service context to stop varnishd
	transcriptFile *os.File           // varnishadm traffic log (when DebugDump enabled)
}

// New creates a new test harness with the given configuration.
func New(cfg *Config) *Harness {
	logger := cfg.Logger
	if logger == nil {
		logLevel := slog.LevelInfo
		if cfg.Verbose {
			logLevel = slog.LevelDebug
		}
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		}))
	}

	return &Harness{
		cfg:    cfg,
		logger: logger,
	}
}

// Run executes all tests and returns the results.
func (h *Harness) Run(ctx context.Context) (*Result, error) {
	// Resolve VCL file path
	vclPath, err := testspec.ResolveVCL(h.cfg.TestFile, h.cfg.VCLPath)
	if err != nil {
		return nil, fmt.Errorf("resolving VCL file: %w", err)
	}
	h.logger.Debug("Resolved VCL file", "path", vclPath)

	// Load test specifications
	h.logger.Debug("Loading test file", "file", h.cfg.TestFile)
	tests, err := testspec.Load(h.cfg.TestFile)
	if err != nil {
		return nil, fmt.Errorf("loading test file: %w", err)
	}
	h.logger.Debug("Loaded tests", "count", len(tests))

	// Check if any tests are scenario-based (require time control)
	hasScenarioTests := false
	for _, test := range tests {
		if test.IsScenario() {
			hasScenarioTests = true
			break
		}
	}

	// Create temporary directories
	if err := h.createTempDirs(); err != nil {
		return nil, err
	}
	if !h.cfg.DebugDump {
		defer h.cleanupTempDirs()
	}

	// === NEW SIMPLIFIED STARTUP FLOW ===
	// 1. Start backends FIRST (need addresses for VCL modification)
	backendAddresses, err := h.startBackendsEarly(tests)
	if err != nil {
		return nil, err
	}
	defer stopAllBackends(h.mockBackends, h.logger)

	// 2. Prepare VCL with modified backend addresses and write to workdir
	modifiedVCLPath, err := h.prepareVCL(vclPath, backendAddresses)
	if err != nil {
		return nil, err
	}

	// 3. Start services with the modified VCL
	if err := h.startServices(ctx, modifiedVCLPath, hasScenarioTests); err != nil {
		return nil, err
	}
	defer h.stopServices() // Stop varnishd and recorder when done

	// Run tests (VCL is already loaded at startup, no need for LoadVCL/UnloadVCL)
	result := h.runTests(tests)

	// Create debug dump if enabled
	if h.cfg.DebugDump {
		dumpPath, err := createDebugDump(
			h.cfg.TestFile, vclPath, h.workDir, h.varnishDir,
			h.testRunner, tests, result.Passed, result.Failed, h.logger,
		)
		if err != nil {
			h.logger.Warn("Failed to create debug dump", "error", err)
		} else {
			result.DebugDumpPath = dumpPath
		}
	}

	return result, nil
}

// createTempDirs creates temporary directories for Varnish.
func (h *Harness) createTempDirs() error {
	var err error

	h.workDir, err = os.MkdirTemp("", "vcltest-work-*")
	if err != nil {
		return fmt.Errorf("creating work dir: %w", err)
	}

	h.varnishDir, err = os.MkdirTemp("", "vcltest-varnish-*")
	if err != nil {
		os.RemoveAll(h.workDir)
		return fmt.Errorf("creating varnish dir: %w", err)
	}

	return nil
}

// cleanupTempDirs removes temporary directories.
func (h *Harness) cleanupTempDirs() {
	if h.workDir != "" {
		os.RemoveAll(h.workDir)
	}
	if h.varnishDir != "" {
		os.RemoveAll(h.varnishDir)
	}
}

// stopServices stops varnishd and the recorder.
func (h *Harness) stopServices() {
	// Stop recorder first (it reads from varnish shared memory)
	if h.recorder != nil {
		h.recorder.Stop()
	}

	// Cancel context to trigger varnishd shutdown.
	// This kills the entire process group (manager + child) via SIGKILL.
	// We don't use varnishadm "stop" command because it can timeout waiting
	// for VCL references to be released.
	if h.cancelServices != nil {
		h.logger.Debug("Canceling varnish context")
		h.cancelServices()
	}

	// Close transcript file if open
	if h.transcriptFile != nil {
		h.transcriptFile.Close()
		h.transcriptFile = nil
	}

	// Brief wait to allow process to terminate
	time.Sleep(100 * time.Millisecond)
}

// startServices starts varnishd and varnishadm with the prepared VCL.
func (h *Harness) startServices(ctx context.Context, vclPath string, hasScenarioTests bool) error {
	// Create service configuration
	// VarnishadmPort: 0 means "use any available port" (dynamic assignment)
	// AdminPort: 0 will be updated by service.Manager after Listen()
	// HTTP Port: 0 means kernel assigns port, discovered via debug.listen_address
	serviceCfg := &service.Config{
		VarnishadmPort: 0, // Dynamic port assignment
		Secret:         "test-secret",
		VarnishCmd:     "varnishd",
		VCLPath:        vclPath, // Use the prepared VCL with modified backends
		VarnishConfig: &varnish.Config{
			WorkDir:    h.workDir,
			VarnishDir: h.varnishDir,
			VCLPath:    vclPath, // VCL is ready at boot time
			Varnish: varnish.VarnishConfig{
				AdminPort: 0, // Will be set by service.Manager
				HTTP: []varnish.HTTPConfig{
					{Port: 0}, // Dynamic port - kernel assigns, we discover via debug.listen_address
				},
				Time: varnish.TimeConfig{
					Enabled: hasScenarioTests,
				},
			},
		},
		Logger: h.logger,
	}

	// Create service manager
	var err error
	h.manager, err = service.NewManager(serviceCfg)
	if err != nil {
		return fmt.Errorf("creating service manager: %w", err)
	}

	// Set up varnishadm transcript logging if debug dump is enabled
	if h.cfg.DebugDump {
		transcriptPath := filepath.Join(h.workDir, "varnishadm-traffic.log")
		h.transcriptFile, err = os.Create(transcriptPath)
		if err != nil {
			h.logger.Warn("Failed to create varnishadm transcript file", "error", err)
		} else {
			h.manager.SetVarnishadmTranscript(h.transcriptFile)
			h.logger.Debug("Varnishadm transcript logging enabled", "path", transcriptPath)
		}
	}

	// Start services in background
	ctx, cancel := context.WithCancel(ctx)
	h.cancelServices = cancel // Store so we can cancel on cleanup
	errChan := make(chan error, 1)
	go func() {
		if err := h.manager.Start(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("service error: %w", err)
		}
	}()

	// Wait for varnish to be ready and discover HTTP port
	// debug.listen_address blocks until pool_accepting is true
	h.logger.Debug("Waiting for Varnish to be ready...")
	if err := h.waitForVarnishReady(ctx, errChan); err != nil {
		return err
	}
	h.logger.Debug("Discovered HTTP port", "port", h.httpPort)

	// Get varnishadm interface
	varnishadm := h.manager.GetVarnishadm()
	if varnishadm == nil {
		return fmt.Errorf("varnishadm not available")
	}

	// Create and start varnishlog recorder
	h.recorder, err = recorder.New(h.varnishDir, h.logger)
	if err != nil {
		return fmt.Errorf("creating recorder: %w", err)
	}

	if err := h.recorder.Start(); err != nil {
		return fmt.Errorf("starting recorder: %w", err)
	}

	// Give varnishlog time to connect to VSM
	time.Sleep(500 * time.Millisecond)

	// Create test runner with discovered HTTP port
	varnishURL := fmt.Sprintf("http://127.0.0.1:%d", h.httpPort)
	h.testRunner = runner.New(varnishadm, varnishURL, h.workDir, h.logger, h.recorder)
	h.testRunner.SetTimeController(h.manager)

	// Set mock backends on the runner (they were started before services)
	if h.mockBackends != nil {
		h.testRunner.SetMockBackends(h.mockBackends)
	}

	// Set the VCL show result on the runner so it has trace info
	// The VCL was loaded at boot time with name "boot"
	vclShowResult, err := varnishadm.VCLShowStructured("boot")
	if err != nil {
		h.logger.Warn("Failed to get VCL structure", "error", err)
	} else {
		h.testRunner.SetVCLShowResult(vclShowResult)
	}

	return nil
}

// waitForVarnishReady waits for varnishd to be ready to accept HTTP connections.
// It polls for varnishd crashes while waiting for debug.listen_address to succeed.
// The debug.listen_address command blocks until pool_accepting is true.
func (h *Harness) waitForVarnishReady(ctx context.Context, errChan <-chan error) error {
	// Check for early crash before attempting to get port
	select {
	case err := <-errChan:
		return fmt.Errorf("varnish failed to start: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Give varnishd a moment to crash if it's going to
	}

	// Now try to get the port - this blocks until varnish is ready
	// Run in goroutine so we can still catch crashes
	portChan := make(chan int, 1)
	portErrChan := make(chan error, 1)
	go func() {
		port, err := h.manager.GetHTTPPort()
		if err != nil {
			portErrChan <- err
		} else {
			portChan <- port
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return fmt.Errorf("varnish failed to start: %w", err)
	case err := <-portErrChan:
		return fmt.Errorf("failed to get HTTP port: %w", err)
	case port := <-portChan:
		h.httpPort = port
		return nil
	}
}

// startBackendsEarly starts all mock backends before VCL preparation.
// This is called early in the startup sequence so we have backend addresses for VCL modification.
func (h *Harness) startBackendsEarly(tests []testspec.TestSpec) (map[string]vclmod.BackendAddress, error) {
	addresses, mockBackends, err := startAllBackends(tests, h.logger)
	if err != nil {
		return nil, fmt.Errorf("starting backends: %w", err)
	}
	h.mockBackends = mockBackends
	// Note: testRunner is set later in startServices, so we'll set mockBackends there too
	return addresses, nil
}

// prepareVCL modifies the VCL with backend addresses and writes to workdir.
// Returns the path to the modified VCL file that varnishd should load at boot.
func (h *Harness) prepareVCL(vclPath string, backends map[string]vclmod.BackendAddress) (string, error) {
	h.logger.Debug("Preparing VCL with backend modifications", "path", vclPath)

	// Process VCL with includes - walks the include tree and modifies each file
	processedFiles, validationResult, err := vclmod.ProcessVCLWithIncludes(vclPath, backends)
	if err != nil {
		// Log validation errors
		if validationResult != nil {
			for _, errMsg := range validationResult.Errors {
				h.logger.Error("Backend validation failed", "error", errMsg)
			}
		}
		return "", fmt.Errorf("processing VCL with includes: %w", err)
	}

	// Log warnings about unused backends
	if validationResult != nil {
		for _, warning := range validationResult.Warnings {
			h.logger.Warn("Backend validation", "warning", warning)
		}
	}

	// Use the vcl subdirectory of workDir - this is where Varnish's vcl_path points
	// so relative includes will be resolved correctly
	vclDir := filepath.Join(h.workDir, "vcl")

	// Write each processed file to vclDir preserving directory structure
	var mainVCLFile string
	for _, file := range processedFiles {
		// Determine output path in vclDir
		outPath := filepath.Join(vclDir, file.RelativePath)

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return "", fmt.Errorf("creating directory for %s: %w", file.RelativePath, err)
		}

		// Write the modified content
		if err := os.WriteFile(outPath, []byte(file.Content), 0644); err != nil {
			return "", fmt.Errorf("writing modified VCL %s: %w", file.RelativePath, err)
		}

		h.logger.Debug("Wrote modified VCL file", "path", outPath, "relative", file.RelativePath)

		// Track main VCL file (the first one processed is the main file)
		if mainVCLFile == "" {
			mainVCLFile = outPath
		}
	}

	h.logger.Debug("VCL prepared", "main_file", mainVCLFile, "total_files", len(processedFiles))
	return mainVCLFile, nil
}

// configureBackendsForTest updates mock backend configurations for a specific test.
func (h *Harness) configureBackendsForTest(test testspec.TestSpec) {
	for name, spec := range test.Backends {
		if mock, ok := h.mockBackends[name]; ok {
			cfg := backend.Config{
				Status:      spec.Status,
				Headers:     spec.Headers,
				Body:        spec.Body,
				FailureMode: spec.FailureMode,
				Routes:      convertRoutes(spec.Routes),
				EchoRequest: spec.EchoRequest,
			}
			if cfg.Status == 0 {
				cfg.Status = 200
			}
			mock.UpdateConfig(cfg)
			h.logger.Debug("Updated backend config for test", "backend", name, "test", test.Name, "failureMode", spec.FailureMode, "echoRequest", spec.EchoRequest)
		}
	}
}

// runTests executes all tests and collects results.
func (h *Harness) runTests(tests []testspec.TestSpec) *Result {
	result := &Result{
		Total:   len(tests),
		Results: make([]runner.TestResult, 0, len(tests)),
	}

	varnishadm := h.manager.GetVarnishadm()

	for _, test := range tests {
		// Nuke the cache before each test to ensure clean state
		h.logger.Debug("Nuking cache before test", "test", test.Name)
		if _, err := varnishadm.BanNukeCache(); err != nil {
			h.logger.Error("Failed to nuke cache before test", "test", test.Name, "error", err)
			result.Failed++
			result.Results = append(result.Results, runner.TestResult{
				TestName: test.Name,
				Passed:   false,
				Errors:   []string{fmt.Sprintf("failed to nuke cache: %v", err)},
			})
			continue
		}

		// Reconfigure backends for this specific test
		h.configureBackendsForTest(test)

		testResult, err := h.testRunner.RunTestWithSharedVCL(test)
		if err != nil {
			h.logger.Debug("Test failed with error", "test", test.Name, "error", err)
			result.Failed++
			result.Results = append(result.Results, runner.TestResult{
				TestName: test.Name,
				Passed:   false,
				Errors:   []string{err.Error()},
			})
			continue
		}

		if testResult.Passed {
			result.Passed++
		} else {
			result.Failed++
		}
		result.Results = append(result.Results, *testResult)
	}

	return result
}

// Cleanup releases resources. Call this if you need to stop early.
func (h *Harness) Cleanup() {
	h.stopServices()
	stopAllBackends(h.mockBackends, h.logger)
	h.cleanupTempDirs()
}
