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

// RunTest executes a single test case
func (r *Runner) RunTest(test testspec.TestSpec) (*TestResult, error) {
	// Check if this is a scenario-based test
	if test.IsScenario() {
		return r.runScenarioTest(test)
	}
	return r.runSingleRequestTest(test)
}

// runSingleRequestTest executes a traditional single-request test
func (r *Runner) runSingleRequestTest(test testspec.TestSpec) (*TestResult, error) {
	// Start mock backends
	bm, addresses, err := r.startBackends(test)
	if err != nil {
		return nil, err
	}
	defer bm.stopAll()

	// Load VCL file
	vclData, err := os.ReadFile(test.VCL)
	if err != nil {
		return nil, fmt.Errorf("reading VCL file: %w", err)
	}

	// Replace backend placeholders
	modifiedVCL, err := vcl.ReplaceBackends(string(vclData), addresses)
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

	// Mark current log position before making request
	var logOffset int64
	if r.recorder != nil {
		logOffset, err = r.recorder.MarkPosition()
		if err != nil {
			r.logger.Warn("Failed to mark log position", "error", err)
		}
	}

	// Make HTTP request to Varnish
	response, err := client.MakeRequest(r.varnishURL, test.Request)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	// Give varnishlog time to process and flush writes
	time.Sleep(500 * time.Millisecond)

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

// runScenarioTest executes a scenario-based temporal test
func (r *Runner) runScenarioTest(test testspec.TestSpec) (*TestResult, error) {
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
	vclData, err := os.ReadFile(test.VCL)
	if err != nil {
		return nil, fmt.Errorf("reading VCL file: %w", err)
	}

	// Replace backend placeholders
	modifiedVCL, err := vcl.ReplaceBackends(string(vclData), addresses)
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

		// Give varnishlog time to process and flush writes
		time.Sleep(500 * time.Millisecond)

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
