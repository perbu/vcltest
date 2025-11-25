package runner

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/perbu/vcltest/pkg/backend"
	"github.com/perbu/vcltest/pkg/testspec"
	"github.com/perbu/vcltest/pkg/varnishadm"
	"github.com/perbu/vcltest/pkg/vclloader"
)

// Phase 1: Pure functions tests

func TestSanitizeVCLName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "mytest",
			expected: "test-mytest",
		},
		{
			name:     "name with spaces",
			input:    "my test name",
			expected: "test-my-test-name",
		},
		{
			name:     "name with special characters",
			input:    "test@#$%^&*()",
			expected: "test-test",
		},
		{
			name:     "mixed case",
			input:    "MyTestName",
			expected: "test-mytestname",
		},
		{
			name:     "name with leading/trailing special chars",
			input:    "---test---",
			expected: "test-test",
		},
		{
			name:     "complex name",
			input:    "Test: Basic cache hit (200 OK)",
			expected: "test-test-basic-cache-hit-200-ok",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "test-",
		},
		{
			name:     "only special characters",
			input:    "@#$%",
			expected: "test-",
		},
		{
			name:     "unicode characters",
			input:    "test_café_☕",
			expected: "test-test-caf",
		},
		{
			name:     "multiple consecutive spaces",
			input:    "test    multiple    spaces",
			expected: "test-test-multiple-spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeVCLName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeVCLName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  time.Duration
		shouldErr bool
	}{
		{
			name:      "zero seconds",
			input:     "0s",
			expected:  0,
			shouldErr: false,
		},
		{
			name:      "30 seconds",
			input:     "30s",
			expected:  30 * time.Second,
			shouldErr: false,
		},
		{
			name:      "2 minutes",
			input:     "2m",
			expected:  2 * time.Minute,
			shouldErr: false,
		},
		{
			name:      "1 hour",
			input:     "1h",
			expected:  time.Hour,
			shouldErr: false,
		},
		{
			name:      "complex duration",
			input:     "1h30m45s",
			expected:  time.Hour + 30*time.Minute + 45*time.Second,
			shouldErr: false,
		},
		{
			name:      "milliseconds",
			input:     "500ms",
			expected:  500 * time.Millisecond,
			shouldErr: false,
		},
		{
			name:      "microseconds",
			input:     "100us",
			expected:  100 * time.Microsecond,
			shouldErr: false,
		},
		{
			name:      "invalid format",
			input:     "invalid",
			expected:  0,
			shouldErr: true,
		},
		{
			name:      "empty string",
			input:     "",
			expected:  0,
			shouldErr: true,
		},
		{
			name:      "negative duration",
			input:     "-30s",
			expected:  -30 * time.Second,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("parseDuration(%q) = %v, want %v", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestExtractVCLFiles(t *testing.T) {
	tests := []struct {
		name         string
		vclShow      *varnishadm.VCLShowResult
		execByConfig map[int][]int
		expected     []VCLFileInfo
	}{
		{
			name: "single file with execution",
			vclShow: func() *varnishadm.VCLShowResult {
				source := "vcl 4.1;\n\nbackend default {\n  .host = \"127.0.0.1\";\n}\n"
				return &varnishadm.VCLShowResult{
					VCLSource: source,
					Entries: []varnishadm.VCLConfigEntry{
						{
							ConfigID: 1,
							Filename: "/tmp/test.vcl",
							Size:     len(source),
						},
					},
					ConfigMap: map[int]string{
						1: "/tmp/test.vcl",
					},
				}
			}(),
			execByConfig: map[int][]int{
				1: {1, 3, 4},
			},
			expected: []VCLFileInfo{
				{
					ConfigID:      1,
					Filename:      "/tmp/test.vcl",
					Source:        "vcl 4.1;\n\nbackend default {\n  .host = \"127.0.0.1\";\n}\n",
					ExecutedLines: []int{1, 3, 4},
				},
			},
		},
		{
			name: "multiple files",
			vclShow: func() *varnishadm.VCLShowResult {
				file1 := "vcl 4.1;\n\ninclude \"lib.vcl\";\n"
				file2 := "// File: lib.vcl\nsub vcl_recv { return (pass); }\n"
				return &varnishadm.VCLShowResult{
					VCLSource: file1 + file2,
					Entries: []varnishadm.VCLConfigEntry{
						{
							ConfigID: 1,
							Filename: "/tmp/main.vcl",
							Size:     len(file1),
						},
						{
							ConfigID: 2,
							Filename: "/tmp/lib.vcl",
							Size:     len(file2),
						},
					},
					ConfigMap: map[int]string{
						1: "/tmp/main.vcl",
						2: "/tmp/lib.vcl",
					},
				}
			}(),
			execByConfig: map[int][]int{
				1: {1, 3},
				2: {1},
			},
			expected: []VCLFileInfo{
				{
					ConfigID:      1,
					Filename:      "/tmp/main.vcl",
					Source:        "vcl 4.1;\n\ninclude \"lib.vcl\";\n",
					ExecutedLines: []int{1, 3},
				},
				{
					ConfigID:      2,
					Filename:      "/tmp/lib.vcl",
					Source:        "// File: lib.vcl\nsub vcl_recv { return (pass); }\n",
					ExecutedLines: []int{1},
				},
			},
		},
		{
			name: "file with no execution",
			vclShow: func() *varnishadm.VCLShowResult {
				source := "vcl 4.1;\n"
				return &varnishadm.VCLShowResult{
					VCLSource: source,
					Entries: []varnishadm.VCLConfigEntry{
						{
							ConfigID: 1,
							Filename: "/tmp/empty.vcl",
							Size:     len(source),
						},
					},
					ConfigMap: map[int]string{
						1: "/tmp/empty.vcl",
					},
				}
			}(),
			execByConfig: map[int][]int{},
			expected: []VCLFileInfo{
				{
					ConfigID:      1,
					Filename:      "/tmp/empty.vcl",
					Source:        "vcl 4.1;\n",
					ExecutedLines: nil,
				},
			},
		},
		{
			name: "builtin VCL is skipped",
			vclShow: func() *varnishadm.VCLShowResult {
				file1 := "vcl 4.1;\n"
				builtin := "// builtin content\n"
				file2 := "backend default {}\n"
				return &varnishadm.VCLShowResult{
					VCLSource: file1 + builtin + file2,
					Entries: []varnishadm.VCLConfigEntry{
						{
							ConfigID: 1,
							Filename: "/tmp/test.vcl",
							Size:     len(file1),
						},
						{
							ConfigID: 0,
							Filename: "<builtin>",
							Size:     len(builtin),
						},
						{
							ConfigID: 2,
							Filename: "/tmp/other.vcl",
							Size:     len(file2),
						},
					},
					ConfigMap: map[int]string{
						1: "/tmp/test.vcl",
						2: "/tmp/other.vcl",
					},
				}
			}(),
			execByConfig: map[int][]int{
				1: {1},
				2: {1},
			},
			expected: []VCLFileInfo{
				{
					ConfigID:      1,
					Filename:      "/tmp/test.vcl",
					Source:        "vcl 4.1;\n",
					ExecutedLines: []int{1},
				},
				{
					ConfigID:      2,
					Filename:      "/tmp/other.vcl",
					Source:        "backend default {}\n",
					ExecutedLines: []int{1},
				},
			},
		},
		{
			name: "empty VCL show result",
			vclShow: &varnishadm.VCLShowResult{
				VCLSource: "",
				Entries:   []varnishadm.VCLConfigEntry{},
				ConfigMap: map[int]string{},
			},
			execByConfig: map[int][]int{},
			expected:     []VCLFileInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a runner with minimal setup
			r := &Runner{
				logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}
			result := r.extractVCLFiles(tt.vclShow, tt.execByConfig)

			if len(result) != len(tt.expected) {
				t.Fatalf("extractVCLFiles() returned %d files, want %d", len(result), len(tt.expected))
			}

			for i, expectedFile := range tt.expected {
				actualFile := result[i]

				if actualFile.ConfigID != expectedFile.ConfigID {
					t.Errorf("file[%d].ConfigID = %d, want %d", i, actualFile.ConfigID, expectedFile.ConfigID)
				}
				if actualFile.Filename != expectedFile.Filename {
					t.Errorf("file[%d].Filename = %q, want %q", i, actualFile.Filename, expectedFile.Filename)
				}
				if actualFile.Source != expectedFile.Source {
					t.Errorf("file[%d].Source = %q, want %q", i, actualFile.Source, expectedFile.Source)
				}

				// Compare executed lines
				if len(actualFile.ExecutedLines) != len(expectedFile.ExecutedLines) {
					t.Errorf("file[%d].ExecutedLines length = %d, want %d",
						i, len(actualFile.ExecutedLines), len(expectedFile.ExecutedLines))
					continue
				}
				for j, line := range expectedFile.ExecutedLines {
					if actualFile.ExecutedLines[j] != line {
						t.Errorf("file[%d].ExecutedLines[%d] = %d, want %d",
							i, j, actualFile.ExecutedLines[j], line)
					}
				}
			}
		})
	}
}

// Phase 2: Backend management tests

func TestStartBackends_SingleBackend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	tests := []struct {
		name     string
		testSpec testspec.TestSpec
		wantErr  bool
	}{
		{
			name: "single backend with default status",
			testSpec: testspec.TestSpec{
				Name: "test",
				Backend: testspec.BackendSpec{
					Body: "Hello World",
				},
			},
			wantErr: false,
		},
		{
			name: "single backend with custom status",
			testSpec: testspec.TestSpec{
				Name: "test",
				Backend: testspec.BackendSpec{
					Status: 404,
					Body:   "Not Found",
				},
			},
			wantErr: false,
		},
		{
			name: "single backend with headers",
			testSpec: testspec.TestSpec{
				Name: "test",
				Backend: testspec.BackendSpec{
					Status: 200,
					Headers: map[string]string{
						"X-Custom": "value",
					},
					Body: "test body",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm, addresses, err := r.startBackends(tt.testSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("startBackends() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			defer bm.stopAll()

			// Verify single backend was created
			if len(bm.backends) != 1 {
				t.Errorf("startBackends() created %d backends, want 1", len(bm.backends))
			}

			// Verify default backend exists
			if _, ok := bm.backends["default"]; !ok {
				t.Error("startBackends() did not create 'default' backend")
			}

			// Verify address was returned
			if len(addresses) != 1 {
				t.Errorf("startBackends() returned %d addresses, want 1", len(addresses))
			}
			addr, ok := addresses["default"]
			if !ok {
				t.Error("startBackends() did not return address for 'default' backend")
			}
			if addr.Host == "" || addr.Port == "" {
				t.Errorf("startBackends() returned invalid address: host=%q, port=%q", addr.Host, addr.Port)
			}
		})
	}
}

func TestStartBackends_MultiBackend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	testSpec := testspec.TestSpec{
		Name: "multi-backend test",
		Backends: map[string]testspec.BackendSpec{
			"api": {
				Status: 200,
				Body:   "API response",
			},
			"web": {
				Status: 201,
				Body:   "Web response",
			},
			"cache": {
				Status: 500,
				Body:   "Error",
			},
		},
	}

	bm, addresses, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() unexpected error: %v", err)
	}
	defer bm.stopAll()

	// Verify all backends were created
	if len(bm.backends) != 3 {
		t.Errorf("startBackends() created %d backends, want 3", len(bm.backends))
	}

	// Verify each named backend exists
	for name := range testSpec.Backends {
		if _, ok := bm.backends[name]; !ok {
			t.Errorf("startBackends() did not create backend %q", name)
		}
	}

	// Verify addresses were returned
	if len(addresses) != 3 {
		t.Errorf("startBackends() returned %d addresses, want 3", len(addresses))
	}
	for name := range testSpec.Backends {
		addr, ok := addresses[name]
		if !ok {
			t.Errorf("startBackends() did not return address for backend %q", name)
			continue
		}
		if addr.Host == "" || addr.Port == "" {
			t.Errorf("startBackends() returned invalid address for %q: host=%q, port=%q",
				name, addr.Host, addr.Port)
		}
	}
}

func TestBackendManager_GetCallCounts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"backend1": {Status: 200},
			"backend2": {Status: 200},
		},
	}

	bm, _, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}
	defer bm.stopAll()

	// Initial call counts should be zero
	counts := bm.getCallCounts()
	if len(counts) != 2 {
		t.Errorf("getCallCounts() returned %d backends, want 2", len(counts))
	}
	for name, count := range counts {
		if count != 0 {
			t.Errorf("initial call count for %q = %d, want 0", name, count)
		}
	}

	// Call counts should reflect actual requests
	// Note: We can't easily make real HTTP requests in this test without
	// additional setup, so we're testing the structure rather than behavior
}

func TestBackendManager_GetTotalCallCount(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"backend1": {Status: 200},
			"backend2": {Status: 200},
		},
	}

	bm, _, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}
	defer bm.stopAll()

	// Initial total should be zero
	total := bm.getTotalCallCount()
	if total != 0 {
		t.Errorf("getTotalCallCount() = %d, want 0", total)
	}
}

func TestBackendManager_ResetCallCounts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"backend1": {Status: 200},
		},
	}

	bm, _, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}
	defer bm.stopAll()

	// Reset should not panic and should work with zero counts
	bm.resetCallCounts()

	counts := bm.getCallCounts()
	for name, count := range counts {
		if count != 0 {
			t.Errorf("call count for %q after reset = %d, want 0", name, count)
		}
	}
}

func TestBackendManager_StopAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"backend1": {Status: 200},
			"backend2": {Status: 200},
		},
	}

	bm, addresses, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}

	// Verify backends are running (addresses are populated)
	if len(addresses) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(addresses))
	}

	// Stop all backends
	bm.stopAll()

	// Calling stopAll again should not panic
	bm.stopAll()
}

// Phase 3: Runner constructor and setters

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	varnishadmMock := varnishadm.NewMock(6082, "secret", logger)

	tests := []struct {
		name       string
		varnishadm varnishadm.VarnishadmInterface
		varnishURL string
		workDir    string
		logger     *slog.Logger
	}{
		{
			name:       "with all parameters",
			varnishadm: varnishadmMock,
			varnishURL: "http://localhost:8080",
			workDir:    "/tmp/work",
			logger:     logger,
		},
		{
			name:       "with nil logger defaults to slog.Default",
			varnishadm: varnishadmMock,
			varnishURL: "http://localhost:8080",
			workDir:    "/tmp/work",
			logger:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.varnishadm, tt.varnishURL, tt.workDir, tt.logger, nil)

			if r == nil {
				t.Fatal("New() returned nil")
			}
			if r.varnishadm != tt.varnishadm {
				t.Error("varnishadm not set correctly")
			}
			if r.varnishURL != tt.varnishURL {
				t.Errorf("varnishURL = %q, want %q", r.varnishURL, tt.varnishURL)
			}
			if r.workDir != tt.workDir {
				t.Errorf("workDir = %q, want %q", r.workDir, tt.workDir)
			}
			if r.logger == nil {
				t.Error("logger should not be nil (defaults to slog.Default)")
			}
		})
	}
}

func TestSetTimeController(t *testing.T) {
	r := &Runner{}

	// Mock time controller
	tc := &mockTimeController{}

	r.SetTimeController(tc)

	if r.timeController != tc {
		t.Error("SetTimeController() did not set timeController")
	}
}

func TestSetMockBackends(t *testing.T) {
	r := &Runner{}

	// Create mock backends map
	backends := map[string]*backend.MockBackend{
		"api": backend.New(backend.Config{Status: 200}),
	}

	r.SetMockBackends(backends)

	if r.mockBackends == nil {
		t.Fatal("SetMockBackends() did not set mockBackends")
	}
	if len(r.mockBackends) != 1 {
		t.Errorf("mockBackends has %d entries, want 1", len(r.mockBackends))
	}
}

func TestGetLoadedVCLSource(t *testing.T) {
	r := &Runner{}

	// Initially no VCL loaded
	if source := r.GetLoadedVCLSource(); source != "" {
		t.Errorf("GetLoadedVCLSource() = %q, want empty string", source)
	}

	// Set VCL show result
	r.vclShowResult = &varnishadm.VCLShowResult{
		VCLSource: "vcl 4.1;",
	}

	if source := r.GetLoadedVCLSource(); source != "vcl 4.1;" {
		t.Errorf("GetLoadedVCLSource() = %q, want %q", source, "vcl 4.1;")
	}
}

// mockTimeController is a simple mock for TimeController interface
type mockTimeController struct {
	advancedBy time.Duration
	advanceErr error
}

func (m *mockTimeController) AdvanceTimeBy(offset time.Duration) error {
	m.advancedBy = offset
	return m.advanceErr
}

// Phase 4: LoadVCL/UnloadVCL tests

func TestUnloadVCL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	varnishadmMock := varnishadm.NewMock(6082, "secret", logger)

	tests := []struct {
		name           string
		setupRunner    func(*Runner)
		expectCommands bool
	}{
		{
			name: "nothing loaded",
			setupRunner: func(r *Runner) {
				// No VCL loaded
			},
			expectCommands: false,
		},
		{
			name: "VCL loaded",
			setupRunner: func(r *Runner) {
				r.loadedVCLName = "test-vcl"
				r.vclShowResult = &varnishadm.VCLShowResult{
					VCLSource: "vcl 4.1;",
				}
			},
			expectCommands: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				varnishadm: varnishadmMock,
				logger:     logger,
			}
			tt.setupRunner(r)

			err := r.UnloadVCL()
			if err != nil {
				t.Errorf("UnloadVCL() unexpected error: %v", err)
			}

			// Verify state is cleared
			if r.loadedVCLName != "" {
				t.Error("loadedVCLName should be empty after UnloadVCL()")
			}
			if r.vclShowResult != nil {
				t.Error("vclShowResult should be nil after UnloadVCL()")
			}
		})
	}
}

func TestRunTestWithSharedVCL_NoVCLLoaded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	varnishadmMock := varnishadm.NewMock(6082, "secret", logger)

	r := &Runner{
		varnishadm: varnishadmMock,
		varnishURL: "http://localhost:8080",
		logger:     logger,
	}

	testSpec := testspec.TestSpec{
		Name: "test",
		Request: testspec.RequestSpec{
			Method: "GET",
			URL:    "/",
		},
	}

	_, err := r.RunTestWithSharedVCL(testSpec)
	if err == nil {
		t.Error("RunTestWithSharedVCL() should return error when no VCL loaded")
	}
	if err != nil && err.Error() != "no VCL loaded - call LoadVCL first" {
		t.Errorf("RunTestWithSharedVCL() wrong error: %v", err)
	}
}

func TestReplaceBackendsInVCL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	tests := []struct {
		name       string
		vclContent string
		vclPath    string
		backends   map[string]vclloader.BackendAddress
		wantErr    bool
	}{
		{
			name: "valid VCL with single backend",
			vclContent: `vcl 4.1;
backend default {
	.host = "example.com";
	.port = "80";
}`,
			vclPath: "/tmp/test.vcl",
			backends: map[string]vclloader.BackendAddress{
				"default": {Host: "127.0.0.1", Port: "8080"},
			},
			wantErr: false,
		},
		{
			name: "invalid VCL syntax",
			vclContent: `vcl 4.1;
backend {
	invalid syntax here
}`,
			vclPath:  "/tmp/invalid.vcl",
			backends: map[string]vclloader.BackendAddress{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.replaceBackendsInVCL(tt.vclContent, tt.vclPath, tt.backends)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceBackendsInVCL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == "" {
				t.Error("replaceBackendsInVCL() returned empty result")
			}
		})
	}
}

// Additional coverage tests for error paths and edge cases

func TestRunTestWithSharedVCL_ScenarioRequiresTimeController(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	varnishadmMock := varnishadm.NewMock(6082, "secret", logger)

	r := &Runner{
		varnishadm:     varnishadmMock,
		varnishURL:     "http://localhost:8080",
		logger:         logger,
		loadedVCLName:  "test-vcl",
		timeController: nil, // No time controller set
	}

	testSpec := testspec.TestSpec{
		Name: "scenario test",
		Scenario: []testspec.ScenarioStep{
			{
				At: "0s",
				Request: testspec.RequestSpec{
					Method: "GET",
					URL:    "/",
				},
			},
		},
	}

	_, err := r.RunTestWithSharedVCL(testSpec)
	if err == nil {
		t.Error("RunTestWithSharedVCL() should return error for scenario without time controller")
	}
	if err != nil && err.Error() != "scenario-based tests require time controller to be set" {
		t.Errorf("RunTestWithSharedVCL() wrong error: %v", err)
	}
}

func TestStartBackends_ScenarioBackendConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	// Test with scenario that has backend config in first step
	testSpec := testspec.TestSpec{
		Name: "scenario test",
		Scenario: []testspec.ScenarioStep{
			{
				At: "0s",
				Backend: testspec.BackendSpec{
					Status: 404,
					Body:   "Not Found",
				},
			},
		},
	}

	bm, addresses, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}
	defer bm.stopAll()

	if len(bm.backends) != 1 {
		t.Errorf("startBackends() created %d backends, want 1", len(bm.backends))
	}

	if _, ok := addresses["default"]; !ok {
		t.Error("startBackends() did not create default backend for scenario")
	}
}

func TestStartBackends_DefaultStatusApplied(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	// Test that default status (200) is applied when status is 0
	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"backend1": {
				Status: 0, // Should default to 200
				Body:   "test",
			},
		},
	}

	bm, _, err := r.startBackends(testSpec)
	if err != nil {
		t.Fatalf("startBackends() error: %v", err)
	}
	defer bm.stopAll()

	// We can't directly check the status without making an HTTP request,
	// but we can verify the backend was created
	if len(bm.backends) != 1 {
		t.Errorf("startBackends() created %d backends, want 1", len(bm.backends))
	}
}

func TestParseDuration_WrapperFunction(t *testing.T) {
	// Test the exported parseDuration function wrapper
	d, err := parseDuration("1h30m")
	if err != nil {
		t.Fatalf("parseDuration() error: %v", err)
	}
	expected := time.Hour + 30*time.Minute
	if d != expected {
		t.Errorf("parseDuration(\"1h30m\") = %v, want %v", d, expected)
	}
}

func TestTestResult_Structure(t *testing.T) {
	// Test TestResult structure
	result := &TestResult{
		TestName: "test",
		Passed:   false,
		Errors:   []string{"error 1", "error 2"},
		VCLTrace: &VCLTraceInfo{
			Files: []VCLFileInfo{
				{
					ConfigID:      1,
					Filename:      "/tmp/test.vcl",
					Source:        "vcl 4.1;",
					ExecutedLines: []int{1, 2, 3},
				},
			},
			BackendCalls: 2,
			VCLFlow:      []string{"vcl_recv", "vcl_backend_fetch"},
		},
	}

	if result.TestName != "test" {
		t.Errorf("TestName = %q, want %q", result.TestName, "test")
	}
	if result.Passed {
		t.Error("Passed should be false")
	}
	if len(result.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(result.Errors))
	}
	if result.VCLTrace == nil {
		t.Fatal("VCLTrace should not be nil")
	}
	if result.VCLTrace.BackendCalls != 2 {
		t.Errorf("BackendCalls = %d, want 2", result.VCLTrace.BackendCalls)
	}
}

func TestVCLFileInfo_Structure(t *testing.T) {
	// Test VCLFileInfo structure
	info := VCLFileInfo{
		ConfigID:      1,
		Filename:      "/tmp/test.vcl",
		Source:        "vcl 4.1;\nbackend default {}",
		ExecutedLines: []int{1, 2},
	}

	if info.ConfigID != 1 {
		t.Errorf("ConfigID = %d, want 1", info.ConfigID)
	}
	if info.Filename != "/tmp/test.vcl" {
		t.Errorf("Filename = %q, want %q", info.Filename, "/tmp/test.vcl")
	}
	if len(info.ExecutedLines) != 2 {
		t.Errorf("len(ExecutedLines) = %d, want 2", len(info.ExecutedLines))
	}
}

func TestTimeController_Interface(t *testing.T) {
	// Verify mockTimeController implements TimeController
	var _ TimeController = (*mockTimeController)(nil)

	tc := &mockTimeController{}
	err := tc.AdvanceTimeBy(5 * time.Second)
	if err != nil {
		t.Errorf("AdvanceTimeBy() unexpected error: %v", err)
	}
	if tc.advancedBy != 5*time.Second {
		t.Errorf("advancedBy = %v, want %v", tc.advancedBy, 5*time.Second)
	}
}

func TestBackendManager_EmptyBackends(t *testing.T) {
	bm := &backendManager{
		backends: make(map[string]*backend.MockBackend),
		logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Test with empty backends
	bm.stopAll()
	bm.resetCallCounts()

	counts := bm.getCallCounts()
	if len(counts) != 0 {
		t.Errorf("getCallCounts() with empty backends returned %d entries, want 0", len(counts))
	}

	total := bm.getTotalCallCount()
	if total != 0 {
		t.Errorf("getTotalCallCount() with empty backends = %d, want 0", total)
	}
}

// Test extractVCLFiles edge cases

func TestExtractVCLFiles_SizeMismatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	// Test with size that exceeds available source
	vclShow := &varnishadm.VCLShowResult{
		VCLSource: "short",
		Entries: []varnishadm.VCLConfigEntry{
			{
				ConfigID: 1,
				Filename: "/tmp/test.vcl",
				Size:     1000, // Way too big
			},
		},
		ConfigMap: map[int]string{
			1: "/tmp/test.vcl",
		},
	}

	result := r.extractVCLFiles(vclShow, map[int][]int{})

	// Should handle gracefully and not panic
	if len(result) != 0 {
		t.Errorf("extractVCLFiles() with size mismatch returned %d files, want 0", len(result))
	}
}

// Test startBackends error handling

func TestStartBackends_ErrorOnStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Runner{
		logger: logger,
	}

	// Test with many backends to increase chance of port conflicts
	// (though in practice this is hard to force)
	testSpec := testspec.TestSpec{
		Name: "test",
		Backends: map[string]testspec.BackendSpec{
			"b1": {Status: 200},
			"b2": {Status: 200},
			"b3": {Status: 200},
		},
	}

	bm, addresses, err := r.startBackends(testSpec)
	if err != nil {
		// Error is possible but shouldn't crash
		if bm != nil {
			bm.stopAll()
		}
		return
	}
	defer bm.stopAll()

	// Success case
	if len(addresses) != 3 {
		t.Errorf("startBackends() created %d backends, want 3", len(addresses))
	}
}
