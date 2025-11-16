package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/perbu/vcltest/pkg/assertion"
	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/instrument"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnish"
)

// TestResult represents the result of a single test execution
type TestResult struct {
	Name          string
	Passed        bool
	Duration      time.Duration
	Assertions    *assertion.Results
	Response      *http.Response
	ResponseBody  string
	ExecutedLines map[int]bool
	VCLSource     []string
	Error         error
}

// Runner executes VCL tests
type Runner struct {
	varnishMgr *varnish.Manager
}

// New creates a new test runner
func New() *Runner {
	return &Runner{}
}

// RunTest executes a single test
func (r *Runner) RunTest(ctx context.Context, test *testspec.TestSpec) *TestResult {
	startTime := time.Now()

	result := &TestResult{
		Name:       test.Name,
		Assertions: assertion.NewResults(),
	}

	// 1. Instrument VCL
	instrConfig := instrument.Config{
		VCLPath: test.VCL,
	}

	// 2. Start mock backend
	backendConfig := backend.Config{
		Status:  test.Backend.Status,
		Headers: test.Backend.Headers,
		Body:    test.Backend.Body,
	}
	mockBackend := backend.New(backendConfig)
	if err := mockBackend.Start(); err != nil {
		result.Error = fmt.Errorf("failed to start mock backend: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer mockBackend.Shutdown(ctx)

	// Update instrumentation config with backend address
	instrConfig.BackendAddress = mockBackend.Address()

	// Instrument the VCL
	instrResult, err := instrument.Instrument(instrConfig)
	if err != nil {
		result.Error = fmt.Errorf("failed to instrument VCL: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	result.VCLSource = instrResult.VCLSource

	// 3. Start varnishd (or reuse if already running)
	if r.varnishMgr == nil {
		varnishConfig := varnish.Config{
			Name:       fmt.Sprintf("test-%d", time.Now().Unix()),
			VCLContent: instrResult.VCL,
		}
		mgr, err := varnish.New(varnishConfig)
		if err != nil {
			result.Error = fmt.Errorf("failed to create varnish manager: %w", err)
			result.Duration = time.Since(startTime)
			return result
		}
		r.varnishMgr = mgr

		if err := r.varnishMgr.Start(ctx); err != nil {
			result.Error = fmt.Errorf("failed to start varnish: %w", err)
			result.Duration = time.Since(startTime)
			return result
		}
	} else {
		// Reload VCL for existing instance
		if err := r.varnishMgr.LoadVCL(instrResult.VCL); err != nil {
			result.Error = fmt.Errorf("failed to load VCL: %w", err)
			result.Duration = time.Since(startTime)
			return result
		}
	}

	// 4. Start varnishlog
	logParser := varnish.NewLogParser(r.varnishMgr.ListenAddress())
	if err := logParser.Start(ctx); err != nil {
		result.Error = fmt.Errorf("failed to start varnishlog: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer logParser.Stop()

	// Give varnishlog time to start
	time.Sleep(100 * time.Millisecond)

	// 5. Execute HTTP request
	req, err := http.NewRequestWithContext(ctx, test.Request.Method, test.Request.URL, strings.NewReader(test.Request.Body))
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Add headers
	for key, value := range test.Request.Headers {
		req.Header.Set(key, value)
	}

	// Make request to Varnish
	// TODO: Use varnish listen address instead of hardcoded
	req.URL.Host = r.varnishMgr.ListenAddress()
	req.URL.Scheme = "http"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("failed to execute request: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to read response body: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	result.Response = resp
	result.ResponseBody = string(bodyBytes)

	// Give varnishlog time to process
	time.Sleep(200 * time.Millisecond)

	// 6. Collect execution trace
	result.ExecutedLines = logParser.GetExecutedLines()

	// 7. Evaluate assertions
	// Status (required)
	result.Assertions.Add(assertion.AssertStatus(test.Expect.Status, resp.StatusCode))

	// Backend calls (optional)
	if test.Expect.BackendCalls != nil {
		actualCalls := mockBackend.RequestCount()
		result.Assertions.Add(assertion.AssertBackendCalls(*test.Expect.BackendCalls, actualCalls))
	}

	// Headers (optional)
	if test.Expect.Headers != nil {
		headerResults := assertion.AssertHeaders(test.Expect.Headers, resp)
		for _, hr := range headerResults {
			result.Assertions.Add(hr)
		}
	}

	// Body contains (optional)
	if test.Expect.BodyContains != nil {
		result.Assertions.Add(assertion.AssertBodyContains(*test.Expect.BodyContains, result.ResponseBody))
	}

	result.Passed = result.Assertions.Passed()
	result.Duration = time.Since(startTime)

	return result
}

// RunTests executes multiple tests from a file
func (r *Runner) RunTests(ctx context.Context, testFile string) ([]*TestResult, error) {
	// Parse tests
	file, err := os.Open(testFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open test file: %w", err)
	}
	defer file.Close()

	tests, err := testspec.ParseTests(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tests: %w", err)
	}

	// Execute tests sequentially
	results := make([]*TestResult, 0, len(tests))
	for _, test := range tests {
		result := r.RunTest(ctx, test)
		results = append(results, result)
	}

	return results, nil
}

// Cleanup cleans up runner resources
func (r *Runner) Cleanup() error {
	if r.varnishMgr != nil {
		return r.varnishMgr.StopAndCleanup()
	}
	return nil
}
