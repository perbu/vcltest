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
	"github.com/perbu/vcltest/pkg/freeport"
	"github.com/perbu/vcltest/pkg/varnish"
	"github.com/perbu/vcltest/pkg/vclloader"
)

// Harness orchestrates VCL test execution.
type Harness struct {
	cfg    *Config
	logger *slog.Logger

	// Runtime state
	workDir        string
	varnishDir     string
	httpPort       int                            // Dynamically assigned HTTP port for Varnish
	manager        *service.Manager
	recorder       *recorder.Recorder
	testRunner     *runner.Runner
	mockBackends   map[string]*backend.MockBackend
	cancelServices context.CancelFunc // Cancels the service context to stop varnishd
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

	// Start services
	if err := h.startServices(ctx, hasScenarioTests); err != nil {
		return nil, err
	}
	defer h.stopServices() // Stop varnishd and recorder when done

	// Start backends
	backendAddresses, err := h.startBackends(tests)
	if err != nil {
		return nil, err
	}
	defer stopAllBackends(h.mockBackends, h.logger)

	// Load VCL
	if err := h.loadVCL(vclPath, backendAddresses); err != nil {
		return nil, err
	}
	defer h.testRunner.UnloadVCL()

	// Run tests
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

	// Brief wait to allow process to terminate
	time.Sleep(100 * time.Millisecond)
}

// startServices starts varnishd and varnishadm.
func (h *Harness) startServices(ctx context.Context, hasScenarioTests bool) error {
	// Find empty.vcl for initial boot
	emptyVCLPath, err := h.findEmptyVCL()
	if err != nil {
		return err
	}

	// Find a free port for HTTP (small race window, but acceptable for tests)
	httpPort, err := freeport.FindFreePort("127.0.0.1")
	if err != nil {
		return fmt.Errorf("finding free HTTP port: %w", err)
	}
	h.httpPort = httpPort
	h.logger.Debug("Using dynamic HTTP port", "port", httpPort)

	// Create service configuration
	// VarnishadmPort: 0 means "use any available port" (dynamic assignment)
	// AdminPort: 0 will be updated by service.Manager after Listen()
	serviceCfg := &service.Config{
		VarnishadmPort: 0, // Dynamic port assignment
		Secret:         "test-secret",
		VarnishCmd:     "varnishd",
		VCLPath:        emptyVCLPath,
		VarnishConfig: &varnish.Config{
			WorkDir:    h.workDir,
			VarnishDir: h.varnishDir,
			VCLPath:    emptyVCLPath,
			Varnish: varnish.VarnishConfig{
				AdminPort: 0, // Will be set by service.Manager
				HTTP: []varnish.HTTPConfig{
					{Address: "127.0.0.1", Port: httpPort},
				},
				Time: varnish.TimeConfig{
					Enabled: hasScenarioTests,
				},
			},
		},
		Logger: h.logger,
	}

	// Create service manager
	h.manager, err = service.NewManager(serviceCfg)
	if err != nil {
		return fmt.Errorf("creating service manager: %w", err)
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

	// Wait for services to be ready
	h.logger.Debug("Waiting for Varnish to be ready...")
	select {
	case err := <-errChan:
		return fmt.Errorf("varnish failed to start: %w", err)
	case <-time.After(2 * time.Second):
		// Services appear to be running, continue
	}

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

	// Create test runner with dynamic HTTP port
	varnishURL := fmt.Sprintf("http://127.0.0.1:%d", h.httpPort)
	h.testRunner = runner.New(varnishadm, varnishURL, h.workDir, h.logger, h.recorder)
	h.testRunner.SetTimeController(h.manager)

	return nil
}

// findEmptyVCL locates the empty.vcl file needed for initial boot.
func (h *Harness) findEmptyVCL() (string, error) {
	// Try relative to current directory first
	candidates := []string{
		"examples/empty.vcl",
		"../examples/empty.vcl",
		"../../examples/empty.vcl",
	}

	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	// Try relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, "examples", "empty.vcl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cannot find examples/empty.vcl - run from project root or install properly")
}

// startBackends starts all mock backends.
func (h *Harness) startBackends(tests []testspec.TestSpec) (map[string]vclloader.BackendAddress, error) {
	addresses, mockBackends, err := startAllBackends(tests, h.logger)
	if err != nil {
		return nil, fmt.Errorf("starting backends: %w", err)
	}
	h.mockBackends = mockBackends
	h.testRunner.SetMockBackends(mockBackends)
	return addresses, nil
}

// loadVCL loads VCL into Varnish.
func (h *Harness) loadVCL(vclPath string, backendAddresses map[string]vclloader.BackendAddress) error {
	h.logger.Debug("Loading shared VCL", "path", vclPath)
	if err := h.testRunner.LoadVCL(vclPath, backendAddresses); err != nil {
		return fmt.Errorf("loading shared VCL: %w", err)
	}
	return nil
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
			}
			if cfg.Status == 0 {
				cfg.Status = 200
			}
			mock.UpdateConfig(cfg)
			h.logger.Debug("Updated backend config for test", "backend", name, "test", test.Name, "failureMode", spec.FailureMode)
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
