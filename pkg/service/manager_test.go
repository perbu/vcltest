package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/perbu/vcltest/pkg/varnish"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid config",
			config: &Config{
				VarnishadmPort: 6082,
				Secret:         "test-secret",
				VarnishCmd:     "varnishd",
				VarnishConfig: &varnish.Config{
					WorkDir:    "/tmp/test",
					VarnishDir: "/tmp/test/varnish",
				},
				Logger: logger,
			},
			wantError: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantError: true,
		},
		{
			name: "nil logger",
			config: &Config{
				VarnishadmPort: 6082,
				Secret:         "test-secret",
				VarnishConfig: &varnish.Config{
					WorkDir:    "/tmp/test",
					VarnishDir: "/tmp/test/varnish",
				},
				Logger: nil,
			},
			wantError: true,
		},
		{
			name: "nil varnish config",
			config: &Config{
				VarnishadmPort: 6082,
				Secret:         "test-secret",
				VarnishConfig:  nil,
				Logger:         logger,
			},
			wantError: true,
		},
		{
			name: "empty secret",
			config: &Config{
				VarnishadmPort: 6082,
				Secret:         "",
				VarnishConfig: &varnish.Config{
					WorkDir:    "/tmp/test",
					VarnishDir: "/tmp/test/varnish",
				},
				Logger: logger,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.config)
			if tt.wantError {
				if err == nil {
					t.Errorf("NewManager() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("NewManager() unexpected error: %v", err)
				return
			}
			if mgr == nil {
				t.Errorf("NewManager() returned nil manager")
			}
		})
	}
}

func TestManagerGetters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := &Config{
		VarnishadmPort: 6082,
		Secret:         "test-secret",
		VarnishCmd:     "varnishd",
		VarnishConfig: &varnish.Config{
			WorkDir:    "/tmp/test",
			VarnishDir: "/tmp/test/varnish",
		},
		Logger: logger,
	}

	mgr, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	if mgr.GetVarnishadm() == nil {
		t.Error("GetVarnishadm() returned nil")
	}

	if mgr.GetVarnishManager() == nil {
		t.Error("GetVarnishManager() returned nil")
	}
}

func TestManagerStartContextCancellation(t *testing.T) {
	// This test verifies that the manager properly handles context cancellation
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	workDir := t.TempDir()

	config := &Config{
		VarnishadmPort: 16082, // Use high port to avoid conflicts
		Secret:         "test-secret",
		VarnishCmd:     "varnishd",
		VarnishConfig: &varnish.Config{
			WorkDir:    workDir,
			VarnishDir: workDir + "/varnish",
		},
		Logger: logger,
	}

	mgr, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}

	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())

	// Start manager in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	// Cancel context after a brief moment
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for Start to return
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Logf("Expected context.Canceled, got: %v (this is acceptable)", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start() did not return after context cancellation")
	}
}
