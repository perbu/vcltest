package runner

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/perbu/vcltest/pkg/assertion"
	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/client"
	"github.com/perbu/vcltest/pkg/recorder"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnishadm"
	"github.com/perbu/vcltest/pkg/vcl"
	"github.com/perbu/vcltest/pkg/vclmod"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// sanitizeVCLName converts a test name into a valid VCL name
// Removes spaces and special characters, converts to lowercase
func sanitizeVCLName(name string) string {
	// Replace spaces and special chars with hyphens
	sanitized := nonAlphanumeric.ReplaceAllString(name, "-")
	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")
	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)
	// Prepend "test-" prefix
	return "test-" + sanitized
}

// TestResult represents the outcome of a single test
type TestResult struct {
	TestName  string
	Passed    bool
	Errors    []string
	VCLTrace  *VCLTraceInfo // VCL execution trace (only populated on failure)
	VCLSource string        // Original VCL source code
}

// VCLTraceInfo contains VCL execution trace information
type VCLTraceInfo struct {
	ExecutedLines []int
	BackendCalls  int
	VCLFlow       []string
}

// TimeController interface for time manipulation in tests
type TimeController interface {
	AdvanceTimeBy(offset time.Duration) error
}

// Runner orchestrates test execution
type Runner struct {
	varnishadm     varnishadm.VarnishadmInterface
	varnishURL     string
	workDir        string
	logger         *slog.Logger
	recorder       *recorder.Recorder
	timeController TimeController // Optional: for temporal testing

	// VCL state for shared VCL across tests
	loadedVCLName   string
	loadedVCLSource string

	// Mock backends for dynamic reconfiguration in scenario tests
	mockBackends map[string]*backend.MockBackend
}

// New creates a new test runner with a recorder
func New(varnishadm varnishadm.VarnishadmInterface, varnishURL, workDir string, logger *slog.Logger, rec *recorder.Recorder) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		varnishadm: varnishadm,
		varnishURL: varnishURL,
		workDir:    workDir,
		logger:     logger,
		recorder:   rec,
	}
}

// SetTimeController sets the time controller for temporal testing
func (r *Runner) SetTimeController(tc TimeController) {
	r.timeController = tc
}

// SetMockBackends sets the mock backend references for dynamic reconfiguration
func (r *Runner) SetMockBackends(backends map[string]*backend.MockBackend) {
	r.mockBackends = backends
}

// parseDuration parses a duration string like "0s", "30s", "2m" into time.Duration
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// backendManager manages multiple mock backends for a test
type backendManager struct {
	backends map[string]*backend.MockBackend
	logger   *slog.Logger
}

// startBackends initializes and starts all mock backends
// Returns a map of backend names to their addresses
func (r *Runner) startBackends(test testspec.TestSpec) (*backendManager, map[string]vcl.BackendAddress, error) {
	bm := &backendManager{
		backends: make(map[string]*backend.MockBackend),
		logger:   r.logger,
	}
	addresses := make(map[string]vcl.BackendAddress)

	// Handle multi-backend tests
	if len(test.Backends) > 0 {
		for name, spec := range test.Backends {
			cfg := backend.Config{
				Status:  spec.Status,
				Headers: spec.Headers,
				Body:    spec.Body,
			}
			// Apply default status if not set
			if cfg.Status == 0 {
				cfg.Status = 200
			}
			mock := backend.New(cfg)
			addr, err := mock.Start()
			if err != nil {
				bm.stopAll()
				return nil, nil, fmt.Errorf("starting backend %q: %w", name, err)
			}

			host, port, err := vcl.ParseAddress(addr)
			if err != nil {
				bm.stopAll()
				return nil, nil, fmt.Errorf("parsing address for backend %q: %w", name, err)
			}

			bm.backends[name] = mock
			addresses[name] = vcl.BackendAddress{Host: host, Port: port}
			r.logger.Debug("Started backend", "name", name, "address", addr)
		}
	} else {
		// Legacy single-backend mode - get backend config from test.Backend or first scenario step
		var backendSpec testspec.BackendSpec
		if test.Backend.Status != 0 || len(test.Backend.Headers) > 0 || test.Backend.Body != "" {
			// Use test-level backend config
			backendSpec = test.Backend
		} else if len(test.Scenario) > 0 {
			// Use backend config from first scenario step that has one
			for _, step := range test.Scenario {
				if step.Backend.Status != 0 || len(step.Backend.Headers) > 0 || step.Backend.Body != "" {
					backendSpec = step.Backend
					break
				}
			}
		}

		cfg := backend.Config{
			Status:  backendSpec.Status,
			Headers: backendSpec.Headers,
			Body:    backendSpec.Body,
		}
		// Apply default status if not set
		if cfg.Status == 0 {
			cfg.Status = 200
		}

		mock := backend.New(cfg)
		addr, err := mock.Start()
		if err != nil {
			return nil, nil, fmt.Errorf("starting mock backend: %w", err)
		}

		host, port, err := vcl.ParseAddress(addr)
		if err != nil {
			bm.stopAll()
			return nil, nil, fmt.Errorf("parsing backend address: %w", err)
		}

		bm.backends["default"] = mock
		addresses["default"] = vcl.BackendAddress{Host: host, Port: port}
	}

	return bm, addresses, nil
}

// stopAll stops all managed backends
func (bm *backendManager) stopAll() {
	for name, backend := range bm.backends {
		if err := backend.Stop(); err != nil {
			bm.logger.Warn("Failed to stop backend", "name", name, "error", err)
		}
	}
}

// getTotalCallCount returns the sum of all backend call counts
func (bm *backendManager) getTotalCallCount() int {
	total := 0
	for _, backend := range bm.backends {
		total += backend.GetCallCount()
	}
	return total
}

// replaceBackendsInVCL performs backend replacement using AST-based modification
func (r *Runner) replaceBackendsInVCL(vclContent string, backends map[string]vcl.BackendAddress) (string, error) {
	// Convert to vclmod.BackendAddress type
	vclmodBackends := make(map[string]vclmod.BackendAddress)
	for name, addr := range backends {
		vclmodBackends[name] = vclmod.BackendAddress{
			Host: addr.Host,
			Port: addr.Port,
		}
	}

	// Validate backends exist in VCL
	validationResult, err := vclmod.ValidateBackends(vclContent, vclmodBackends)
	if err != nil {
		// Log validation errors
		if validationResult != nil {
			for _, errMsg := range validationResult.Errors {
				r.logger.Error("Backend validation failed", "error", errMsg)
			}
		}
		return "", err
	}

	// Log warnings about unused backends
	for _, warning := range validationResult.Warnings {
		r.logger.Warn("Backend validation", "warning", warning)
	}

	// Perform AST-based modification
	modifiedVCL, err := vclmod.ModifyBackends(vclContent, vclmodBackends)
	if err != nil {
		return "", fmt.Errorf("modifying backends in VCL: %w", err)
	}

	r.logger.Debug("VCL backends modified using AST parser")
	return modifiedVCL, nil
}

// LoadVCL loads VCL file and prepares it for sharing across all tests
func (r *Runner) LoadVCL(vclPath string, backends map[string]vcl.BackendAddress) error {
	// Read VCL file
	vclData, err := os.ReadFile(vclPath)
	if err != nil {
		return fmt.Errorf("reading VCL file: %w", err)
	}

	// Replace backend addresses using AST-based modification
	modifiedVCL, err := r.replaceBackendsInVCL(string(vclData), backends)
	if err != nil {
		return fmt.Errorf("replacing backends in VCL: %w", err)
	}

	// Write modified VCL to temporary file
	tmpFile, err := os.CreateTemp("", "vcltest-shared-*.vcl")
	if err != nil {
		return fmt.Errorf("creating temp VCL file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(modifiedVCL); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp VCL file: %w", err)
	}
	tmpFile.Close()

	// Load VCL into Varnish
	vclName := "shared-vcl"
	vclLoadStart := time.Now()
	resp, err := r.varnishadm.VCLLoad(vclName, tmpFile.Name())
	if err != nil {
		return fmt.Errorf("loading VCL into Varnish: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return fmt.Errorf("VCL compilation failed: %s", resp.Payload())
	}
	r.logger.Debug("Shared VCL loaded", "name", vclName, "duration_ms", time.Since(vclLoadStart).Milliseconds())

	// Activate VCL
	vclUseStart := time.Now()
	resp, err = r.varnishadm.VCLUse(vclName)
	if err != nil {
		return fmt.Errorf("activating VCL: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return fmt.Errorf("VCL activation failed: %s", resp.Payload())
	}
	r.logger.Debug("Shared VCL activated", "name", vclName, "duration_ms", time.Since(vclUseStart).Milliseconds())

	// Store state
	r.loadedVCLName = vclName
	r.loadedVCLSource = modifiedVCL

	return nil
}

// UnloadVCL cleans up the shared VCL
func (r *Runner) UnloadVCL() error {
	if r.loadedVCLName == "" {
		return nil // Nothing to unload
	}

	// Switch to boot VCL
	if resp, err := r.varnishadm.VCLUse("boot"); err != nil {
		r.logger.Warn("Failed to switch to boot VCL", "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to switch to boot VCL", "status", resp.StatusCode(), "response", resp.Payload())
	}

	// Discard the shared VCL
	if resp, err := r.varnishadm.VCLDiscard(r.loadedVCLName); err != nil {
		r.logger.Warn("Failed to discard VCL", "vcl", r.loadedVCLName, "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to discard VCL", "vcl", r.loadedVCLName, "status", resp.StatusCode(), "response", resp.Payload())
	}

	r.loadedVCLName = ""
	r.loadedVCLSource = ""
	return nil
}

// GetLoadedVCLSource returns the currently loaded VCL source code (for debugging)
func (r *Runner) GetLoadedVCLSource() string {
	return r.loadedVCLSource
}

// RunTest executes a single test case (legacy method - loads VCL per test)
func (r *Runner) RunTest(test testspec.TestSpec, vclPath string) (*TestResult, error) {
	start := time.Now()
	r.logger.Debug("Starting test execution", "test", test.Name)

	// Check if this is a scenario-based test
	var result *TestResult
	var err error
	if test.IsScenario() {
		result, err = r.runScenarioTest(test, vclPath)
	} else {
		result, err = r.runSingleRequestTest(test, vclPath)
	}

	duration := time.Since(start)
	r.logger.Debug("Test execution completed", "test", test.Name, "passed", result != nil && result.Passed, "duration_ms", duration.Milliseconds())

	return result, err
}

// RunTestWithSharedVCL executes a single test using pre-loaded shared VCL
func (r *Runner) RunTestWithSharedVCL(test testspec.TestSpec) (*TestResult, error) {
	if r.loadedVCLName == "" {
		return nil, fmt.Errorf("no VCL loaded - call LoadVCL first")
	}

	start := time.Now()
	r.logger.Debug("Starting test execution with shared VCL", "test", test.Name)

	// Check if this is a scenario-based test
	var result *TestResult
	var err error
	if test.IsScenario() {
		result, err = r.runScenarioTestWithSharedVCL(test)
	} else {
		result, err = r.runSingleRequestTestWithSharedVCL(test)
	}

	duration := time.Since(start)
	r.logger.Debug("Test execution completed", "test", test.Name, "passed", result != nil && result.Passed, "duration_ms", duration.Milliseconds())

	return result, err
}

// runSingleRequestTest executes a traditional single-request test
func (r *Runner) runSingleRequestTest(test testspec.TestSpec, vclPath string) (*TestResult, error) {
	// Start mock backends
	bm, addresses, err := r.startBackends(test)
	if err != nil {
		return nil, err
	}
	defer bm.stopAll()

	// Load VCL file
	vclData, err := os.ReadFile(vclPath)
	if err != nil {
		return nil, fmt.Errorf("reading VCL file: %w", err)
	}

	// Replace backend addresses using AST-based modification
	modifiedVCL, err := r.replaceBackendsInVCL(string(vclData), addresses)
	if err != nil {
		return nil, fmt.Errorf("replacing backends in VCL: %w", err)
	}

	// Write modified VCL to temporary file
	tmpFile, err := os.CreateTemp("", "vcltest-*.vcl")
	if err != nil {
		return nil, fmt.Errorf("creating temp VCL file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(modifiedVCL); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing temp VCL file: %w", err)
	}
	tmpFile.Close()

	// Load VCL into Varnish
	// Sanitize VCL name - remove spaces and special characters
	vclName := sanitizeVCLName(test.Name)
	vclLoadStart := time.Now()
	resp, err := r.varnishadm.VCLLoad(vclName, tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("loading VCL into Varnish: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return nil, fmt.Errorf("VCL compilation failed: %s", resp.Payload())
	}
	r.logger.Debug("VCL loaded", "name", vclName, "duration_ms", time.Since(vclLoadStart).Milliseconds())

	// Activate VCL
	vclUseStart := time.Now()
	resp, err = r.varnishadm.VCLUse(vclName)
	if err != nil {
		return nil, fmt.Errorf("activating VCL: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return nil, fmt.Errorf("VCL activation failed: %s", resp.Payload())
	}
	r.logger.Debug("VCL activated", "name", vclName, "duration_ms", time.Since(vclUseStart).Milliseconds())

	// Mark current log position before making request
	var logOffset int64
	if r.recorder != nil {
		logOffset, err = r.recorder.MarkPosition()
		if err != nil {
			r.logger.Warn("Failed to mark log position", "error", err)
		}
	}

	// Make HTTP request to Varnish
	requestStart := time.Now()
	response, err := client.MakeRequest(r.varnishURL, test.Request)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	r.logger.Debug("HTTP request completed", "url", test.Request.URL, "status", response.Status, "duration_ms", time.Since(requestStart).Milliseconds())

	// Flush varnishlog to ensure logs are written
	if r.recorder != nil {
		flushStart := time.Now()
		if err := r.recorder.Flush(); err != nil {
			r.logger.Warn("Failed to flush varnishlog", "error", err)
		}
		r.logger.Debug("Varnishlog flushed", "duration_ms", time.Since(flushStart).Milliseconds())
	}

	// Get backends used from varnishlog (for backend_used assertion)
	var backendsUsed []string
	if r.recorder != nil && test.Expect.BackendUsed != "" {
		messages, err := r.recorder.GetVCLMessagesSince(logOffset)
		if err != nil {
			r.logger.Warn("Failed to get VCL messages for backend check", "error", err)
		} else {
			backendsUsed = recorder.GetBackendsUsed(messages)
		}
	}

	// Check assertions
	assertResult := assertion.Check(test.Expect, response, bm.getTotalCallCount(), backendsUsed)

	// Prepare test result
	result := &TestResult{
		TestName:  test.Name,
		Passed:    assertResult.Passed,
		Errors:    assertResult.Errors,
		VCLSource: modifiedVCL,
	}

	// If test failed, collect and attach trace information
	if !assertResult.Passed && r.recorder != nil {
		messages, err := r.recorder.GetVCLMessagesSince(logOffset)
		if err != nil {
			r.logger.Warn("Failed to get VCL messages", "error", err)
		} else {
			summary := recorder.GetTraceSummary(messages)
			result.VCLTrace = &VCLTraceInfo{
				ExecutedLines: summary.ExecutedLines,
				BackendCalls:  summary.BackendCalls,
				VCLFlow:       append(summary.VCLCalls, summary.VCLReturns...),
			}
		}
	}

	// Clean up VCL - must switch to boot before discarding active VCL
	if resp, err := r.varnishadm.VCLUse("boot"); err != nil {
		r.logger.Warn("Failed to switch to boot VCL", "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to switch to boot VCL", "status", resp.StatusCode(), "response", resp.Payload())
	}

	if resp, err := r.varnishadm.VCLDiscard(vclName); err != nil {
		r.logger.Warn("Failed to discard VCL", "vcl", vclName, "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to discard VCL", "vcl", vclName, "status", resp.StatusCode(), "response", resp.Payload())
	}

	return result, nil
}

// runSingleRequestTestWithSharedVCL executes a single-request test with pre-loaded VCL
func (r *Runner) runSingleRequestTestWithSharedVCL(test testspec.TestSpec) (*TestResult, error) {
	// Mark current log position before making request
	var logOffset int64
	var err error
	if r.recorder != nil {
		logOffset, err = r.recorder.MarkPosition()
		if err != nil {
			r.logger.Warn("Failed to mark log position", "error", err)
		}
	}

	// Make HTTP request to Varnish
	requestStart := time.Now()
	response, err := client.MakeRequest(r.varnishURL, test.Request)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	r.logger.Debug("HTTP request completed", "url", test.Request.URL, "status", response.Status, "duration_ms", time.Since(requestStart).Milliseconds())

	// Flush varnishlog to ensure logs are written
	if r.recorder != nil {
		flushStart := time.Now()
		if err := r.recorder.Flush(); err != nil {
			r.logger.Warn("Failed to flush varnishlog", "error", err)
		}
		r.logger.Debug("Varnishlog flushed", "duration_ms", time.Since(flushStart).Milliseconds())
	}

	// Get backends used from varnishlog (for backend_used assertion)
	var backendsUsed []string
	if r.recorder != nil && test.Expect.BackendUsed != "" {
		messages, err := r.recorder.GetVCLMessagesSince(logOffset)
		if err != nil {
			r.logger.Warn("Failed to get VCL messages for backend check", "error", err)
		} else {
			backendsUsed = recorder.GetBackendsUsed(messages)
		}
	}

	// Note: backend call count is not tracked in shared VCL mode (would accumulate across tests)
	// Users should avoid backend_calls assertions when using shared VCL
	assertResult := assertion.Check(test.Expect, response, 0, backendsUsed)

	// Prepare test result
	result := &TestResult{
		TestName:  test.Name,
		Passed:    assertResult.Passed,
		Errors:    assertResult.Errors,
		VCLSource: r.loadedVCLSource,
	}

	// If test failed, collect and attach trace information
	if !assertResult.Passed && r.recorder != nil {
		messages, err := r.recorder.GetVCLMessagesSince(logOffset)
		if err != nil {
			r.logger.Warn("Failed to get VCL messages", "error", err)
		} else {
			summary := recorder.GetTraceSummary(messages)
			result.VCLTrace = &VCLTraceInfo{
				ExecutedLines: summary.ExecutedLines,
				BackendCalls:  summary.BackendCalls,
				VCLFlow:       append(summary.VCLCalls, summary.VCLReturns...),
			}
		}
	}

	return result, nil
}

// runScenarioTest executes a scenario-based temporal test
func (r *Runner) runScenarioTest(test testspec.TestSpec, vclPath string) (*TestResult, error) {
	if r.timeController == nil {
		return nil, fmt.Errorf("scenario-based tests require time controller to be set")
	}

	// Start mock backends
	bm, addresses, err := r.startBackends(test)
	if err != nil {
		return nil, err
	}
	defer bm.stopAll()

	// Load VCL file
	vclData, err := os.ReadFile(vclPath)
	if err != nil {
		return nil, fmt.Errorf("reading VCL file: %w", err)
	}

	// Replace backend addresses using AST-based modification
	modifiedVCL, err := r.replaceBackendsInVCL(string(vclData), addresses)
	if err != nil {
		return nil, fmt.Errorf("replacing backends in VCL: %w", err)
	}

	// Write modified VCL to temporary file
	tmpFile, err := os.CreateTemp("", "vcltest-*.vcl")
	if err != nil {
		return nil, fmt.Errorf("creating temp VCL file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(modifiedVCL); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing temp VCL file: %w", err)
	}
	tmpFile.Close()

	// Load VCL into Varnish
	vclName := sanitizeVCLName(test.Name)
	resp, err := r.varnishadm.VCLLoad(vclName, tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("loading VCL into Varnish: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return nil, fmt.Errorf("VCL compilation failed: %s", resp.Payload())
	}

	// Activate VCL
	resp, err = r.varnishadm.VCLUse(vclName)
	if err != nil {
		return nil, fmt.Errorf("activating VCL: %w", err)
	}
	if resp.StatusCode() != varnishadm.ClisOk {
		return nil, fmt.Errorf("VCL activation failed: %s", resp.Payload())
	}

	// Execute scenario steps
	var allErrors []string
	var firstFailedStep int = -1

	for stepIdx, step := range test.Scenario {
		// Parse time offset
		offset, err := parseDuration(step.At)
		if err != nil {
			return nil, fmt.Errorf("step %d: invalid time offset %q: %w", stepIdx+1, step.At, err)
		}

		// Advance time to this step's offset (absolute from test start)
		if err := r.timeController.AdvanceTimeBy(offset); err != nil {
			return nil, fmt.Errorf("step %d: failed to advance time: %w", stepIdx+1, err)
		}

		r.logger.Debug("Executing scenario step", "step", stepIdx+1, "at", step.At)

		// Mark current log position before making request
		var stepLogOffset int64
		if r.recorder != nil {
			stepLogOffset, err = r.recorder.MarkPosition()
			if err != nil {
				r.logger.Warn("Failed to mark log position", "error", err)
			}
		}

		// Make HTTP request to Varnish
		response, err := client.MakeRequest(r.varnishURL, step.Request)
		if err != nil {
			return nil, fmt.Errorf("step %d: making request: %w", stepIdx+1, err)
		}

		// Flush varnishlog to ensure logs are written
		if r.recorder != nil {
			if err := r.recorder.Flush(); err != nil {
				r.logger.Warn("Failed to flush varnishlog", "error", err)
			}
		}

		// Get backends used from varnishlog (for backend_used assertion)
		var backendsUsed []string
		if r.recorder != nil && step.Expect.BackendUsed != "" {
			messages, err := r.recorder.GetVCLMessagesSince(stepLogOffset)
			if err != nil {
				r.logger.Warn("Failed to get VCL messages for backend check", "error", err)
			} else {
				backendsUsed = recorder.GetBackendsUsed(messages)
			}
		}

		// Check assertions for this step
		assertResult := assertion.Check(step.Expect, response, bm.getTotalCallCount(), backendsUsed)

		if !assertResult.Passed {
			if firstFailedStep == -1 {
				firstFailedStep = stepIdx
			}
			for _, err := range assertResult.Errors {
				allErrors = append(allErrors, fmt.Sprintf("Step %d (at %s): %s", stepIdx+1, step.At, err))
			}
		}
	}

	// Prepare test result
	result := &TestResult{
		TestName:  test.Name,
		Passed:    len(allErrors) == 0,
		Errors:    allErrors,
		VCLSource: modifiedVCL,
	}

	// If test failed, collect and attach trace information from first failed step
	if !result.Passed && r.recorder != nil && firstFailedStep >= 0 {
		// Get all messages for the entire test
		messages, err := r.recorder.GetVCLMessages()
		if err != nil {
			r.logger.Warn("Failed to get VCL messages", "error", err)
		} else {
			summary := recorder.GetTraceSummary(messages)
			result.VCLTrace = &VCLTraceInfo{
				ExecutedLines: summary.ExecutedLines,
				BackendCalls:  summary.BackendCalls,
				VCLFlow:       append(summary.VCLCalls, summary.VCLReturns...),
			}
		}
	}

	// Clean up VCL
	if resp, err := r.varnishadm.VCLUse("boot"); err != nil {
		r.logger.Warn("Failed to switch to boot VCL", "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to switch to boot VCL", "status", resp.StatusCode(), "response", resp.Payload())
	}

	if resp, err := r.varnishadm.VCLDiscard(vclName); err != nil {
		r.logger.Warn("Failed to discard VCL", "vcl", vclName, "error", err)
	} else if resp.StatusCode() != varnishadm.ClisOk {
		r.logger.Warn("Failed to discard VCL", "vcl", vclName, "status", resp.StatusCode(), "response", resp.Payload())
	}

	return result, nil
}

// runScenarioTestWithSharedVCL executes a scenario-based test with pre-loaded VCL
func (r *Runner) runScenarioTestWithSharedVCL(test testspec.TestSpec) (*TestResult, error) {
	if r.timeController == nil {
		return nil, fmt.Errorf("scenario-based tests require time controller to be set")
	}

	// Execute scenario steps
	var allErrors []string
	var firstFailedStep int = -1

	for stepIdx, step := range test.Scenario {
		// Parse time offset
		offset, err := parseDuration(step.At)
		if err != nil {
			return nil, fmt.Errorf("step %d: invalid time offset %q: %w", stepIdx+1, step.At, err)
		}

		// Advance time to this step's offset (absolute from test start)
		if err := r.timeController.AdvanceTimeBy(offset); err != nil {
			return nil, fmt.Errorf("step %d: failed to advance time: %w", stepIdx+1, err)
		}

		// Update backend configuration if specified in this step
		if step.Backend.Status != 0 || len(step.Backend.Headers) > 0 || step.Backend.Body != "" {
			if r.mockBackends != nil {
				// Determine which backend to update (default or specific name)
				backendName := "default"
				if len(test.Backends) > 0 {
					// For multi-backend tests, we would need a way to specify which backend
					// For now, update all backends with the same config (legacy behavior)
					for name := range test.Backends {
						backendName = name
						break
					}
				}

				if mock, ok := r.mockBackends[backendName]; ok {
					cfg := backend.Config{
						Status:  step.Backend.Status,
						Headers: step.Backend.Headers,
						Body:    step.Backend.Body,
					}
					// Apply default status if not set
					if cfg.Status == 0 {
						cfg.Status = 200
					}
					mock.UpdateConfig(cfg)
					r.logger.Debug("Updated backend config for step", "step", stepIdx+1, "backend", backendName, "status", cfg.Status)
				} else {
					r.logger.Warn("Backend not found for config update", "backend", backendName, "step", stepIdx+1)
				}
			}
		}

		r.logger.Debug("Executing scenario step", "step", stepIdx+1, "at", step.At)

		// Mark current log position before making request
		var stepLogOffset int64
		if r.recorder != nil {
			stepLogOffset, err = r.recorder.MarkPosition()
			if err != nil {
				r.logger.Warn("Failed to mark log position", "error", err)
			}
		}

		// Make HTTP request to Varnish
		response, err := client.MakeRequest(r.varnishURL, step.Request)
		if err != nil {
			return nil, fmt.Errorf("step %d: making request: %w", stepIdx+1, err)
		}

		// Flush varnishlog to ensure logs are written
		if r.recorder != nil {
			if err := r.recorder.Flush(); err != nil {
				r.logger.Warn("Failed to flush varnishlog", "error", err)
			}
		}

		// Get backends used from varnishlog (for backend_used assertion)
		var backendsUsed []string
		if r.recorder != nil && step.Expect.BackendUsed != "" {
			messages, err := r.recorder.GetVCLMessagesSince(stepLogOffset)
			if err != nil {
				r.logger.Warn("Failed to get VCL messages for backend check", "error", err)
			} else {
				backendsUsed = recorder.GetBackendsUsed(messages)
			}
		}

		// Note: backend call count is not tracked in shared VCL mode
		assertResult := assertion.Check(step.Expect, response, 0, backendsUsed)

		if !assertResult.Passed {
			if firstFailedStep == -1 {
				firstFailedStep = stepIdx
			}
			for _, err := range assertResult.Errors {
				allErrors = append(allErrors, fmt.Sprintf("Step %d (at %s): %s", stepIdx+1, step.At, err))
			}
		}
	}

	// Prepare test result
	result := &TestResult{
		TestName:  test.Name,
		Passed:    len(allErrors) == 0,
		Errors:    allErrors,
		VCLSource: r.loadedVCLSource,
	}

	// If test failed, collect and attach trace information
	if !result.Passed && r.recorder != nil && firstFailedStep >= 0 {
		messages, err := r.recorder.GetVCLMessages()
		if err != nil {
			r.logger.Warn("Failed to get VCL messages", "error", err)
		} else {
			summary := recorder.GetTraceSummary(messages)
			result.VCLTrace = &VCLTraceInfo{
				ExecutedLines: summary.ExecutedLines,
				BackendCalls:  summary.BackendCalls,
				VCLFlow:       append(summary.VCLCalls, summary.VCLReturns...),
			}
		}
	}

	return result, nil
}
