package harness

import (
	"log/slog"
	"os"
	"testing"

	"github.com/perbu/vcltest/pkg/testspec"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantLog bool
	}{
		{
			name: "with logger",
			cfg: &Config{
				TestFile: "test.yaml",
				Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
			},
			wantLog: true,
		},
		{
			name: "without logger creates default",
			cfg: &Config{
				TestFile: "test.yaml",
			},
			wantLog: true,
		},
		{
			name: "verbose mode",
			cfg: &Config{
				TestFile: "test.yaml",
				Verbose:  true,
			},
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(tt.cfg)
			if h == nil {
				t.Fatal("New() returned nil")
			}
			if h.cfg != tt.cfg {
				t.Error("config not set correctly")
			}
			if tt.wantLog && h.logger == nil {
				t.Error("logger should not be nil")
			}
		})
	}
}

func TestConvertRoutes(t *testing.T) {
	tests := []struct {
		name   string
		routes map[string]testspec.RouteSpec
		want   int // expected number of routes
	}{
		{
			name:   "nil routes",
			routes: nil,
			want:   0,
		},
		{
			name:   "empty routes",
			routes: map[string]testspec.RouteSpec{},
			want:   0,
		},
		{
			name: "single route",
			routes: map[string]testspec.RouteSpec{
				"/api": {
					Status: 200,
					Body:   "OK",
				},
			},
			want: 1,
		},
		{
			name: "multiple routes",
			routes: map[string]testspec.RouteSpec{
				"/api":    {Status: 200},
				"/health": {Status: 204},
				"/error":  {Status: 500},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertRoutes(tt.routes)
			if tt.routes == nil {
				if got != nil {
					t.Errorf("convertRoutes(nil) = %v, want nil", got)
				}
				return
			}
			if len(got) != tt.want {
				t.Errorf("convertRoutes() returned %d routes, want %d", len(got), tt.want)
			}

			// Verify route content is preserved
			for path, spec := range tt.routes {
				if route, ok := got[path]; ok {
					if route.Status != spec.Status {
						t.Errorf("route %s status = %d, want %d", path, route.Status, spec.Status)
					}
					if route.Body != spec.Body {
						t.Errorf("route %s body = %q, want %q", path, route.Body, spec.Body)
					}
				} else {
					t.Errorf("route %s not found in result", path)
				}
			}
		})
	}
}

func TestStartAllBackends(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		tests   []testspec.TestSpec
		wantErr bool
		wantBe  int // expected number of backends
	}{
		{
			name:    "empty tests creates default backend",
			tests:   []testspec.TestSpec{},
			wantErr: false,
			wantBe:  1,
		},
		{
			name: "single test with single backend",
			tests: []testspec.TestSpec{
				{
					Name: "test1",
					Backends: map[string]testspec.BackendSpec{
						"default": {Status: 200},
					},
				},
			},
			wantErr: false,
			wantBe:  1,
		},
		{
			name: "multiple tests with different backends",
			tests: []testspec.TestSpec{
				{
					Name: "test1",
					Backends: map[string]testspec.BackendSpec{
						"api": {Status: 200},
					},
				},
				{
					Name: "test2",
					Backends: map[string]testspec.BackendSpec{
						"web": {Status: 201},
					},
				},
			},
			wantErr: false,
			wantBe:  2,
		},
		{
			name: "duplicate backend names use first config",
			tests: []testspec.TestSpec{
				{
					Name: "test1",
					Backends: map[string]testspec.BackendSpec{
						"default": {Status: 200},
					},
				},
				{
					Name: "test2",
					Backends: map[string]testspec.BackendSpec{
						"default": {Status: 404}, // Should be ignored
					},
				},
			},
			wantErr: false,
			wantBe:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addresses, backends, err := startAllBackends(tt.tests, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("startAllBackends() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			defer stopAllBackends(backends, logger)

			if len(backends) != tt.wantBe {
				t.Errorf("startAllBackends() created %d backends, want %d", len(backends), tt.wantBe)
			}
			if len(addresses) != tt.wantBe {
				t.Errorf("startAllBackends() returned %d addresses, want %d", len(addresses), tt.wantBe)
			}

			// Verify addresses are valid
			for name, addr := range addresses {
				if addr.Host == "" {
					t.Errorf("backend %s has empty host", name)
				}
				if addr.Port == "" {
					t.Errorf("backend %s has empty port", name)
				}
			}
		})
	}
}

func TestStopAllBackends(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Start some backends first
	tests := []testspec.TestSpec{
		{
			Name: "test",
			Backends: map[string]testspec.BackendSpec{
				"be1": {Status: 200},
				"be2": {Status: 200},
			},
		},
	}

	_, backends, err := startAllBackends(tests, logger)
	if err != nil {
		t.Fatalf("startAllBackends() error: %v", err)
	}

	// Stop should not panic
	stopAllBackends(backends, logger)

	// Stopping again should not panic
	stopAllBackends(backends, logger)

	// Stopping nil should not panic
	stopAllBackends(nil, logger)
}

func TestResult(t *testing.T) {
	r := &Result{
		Passed: 5,
		Failed: 2,
		Total:  7,
	}

	if r.Passed != 5 {
		t.Errorf("Passed = %d, want 5", r.Passed)
	}
	if r.Failed != 2 {
		t.Errorf("Failed = %d, want 2", r.Failed)
	}
	if r.Total != 7 {
		t.Errorf("Total = %d, want 7", r.Total)
	}
}

func TestConfig(t *testing.T) {
	cfg := &Config{
		TestFile:  "/path/to/test.yaml",
		VCLPath:   "/path/to/test.vcl",
		Verbose:   true,
		DebugDump: true,
	}

	if cfg.TestFile != "/path/to/test.yaml" {
		t.Errorf("TestFile = %q, want /path/to/test.yaml", cfg.TestFile)
	}
	if cfg.VCLPath != "/path/to/test.vcl" {
		t.Errorf("VCLPath = %q, want /path/to/test.vcl", cfg.VCLPath)
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
	if !cfg.DebugDump {
		t.Error("DebugDump should be true")
	}
}
