package cache

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/borud/broker"
	"github.com/perbu/vcltest/pkg/events"
	"github.com/perbu/vcltest/pkg/varnishadm"
)

func TestNew_Constructor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)
	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	if starter == nil {
		t.Fatal("New() returned nil")
	}

	if starter.varnishadm == nil {
		t.Error("varnishadm is nil")
	}

	if starter.broker == nil {
		t.Error("broker is nil")
	}

	if starter.logger == nil {
		t.Error("logger is nil")
	}
}

func TestStart_SubscribesCorrectly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)
	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Give the goroutine time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Publish a non-VCL event - should be ignored
	err := b.Publish("/process", events.EventVarnishdConnected{}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Give time for event processing
	time.Sleep(50 * time.Millisecond)

	// Verify start command was not called
	history := mock.GetCallHistory()
	if len(history) != 0 {
		t.Errorf("Expected 0 commands for non-VCL event, got %d: %v", len(history), history)
	}
}

func TestStartCache_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)

	// Set up mock response for start command
	mock.SetResponse("start", varnishadm.NewVarnishResponse(
		varnishadm.ClisOk,
		"Child process started",
	))

	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Subscribe to events to verify EventCacheStarted and EventReady are published
	sub, err := b.Subscribe("/process")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give the starter time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Clear call history before test
	mock.ClearCallHistory()

	// Create VCL mapping for the event
	vclMapping := &varnishadm.VCLShowResult{
		VCLSource: "vcl 4.1;",
		ConfigMap: map[int]string{0: "test.vcl"},
	}

	// Publish VCLLoaded event
	err = b.Publish("/process", events.EventVCLLoaded{
		Name:    "test",
		Path:    "/tmp/test.vcl",
		Mapping: vclMapping,
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish VCLLoaded event: %v", err)
	}

	// Wait for events to be processed
	var gotCacheStarted, gotReady bool
	timeout := time.After(1 * time.Second)

	for !gotCacheStarted || !gotReady {
		select {
		case msg := <-sub.Messages():
			switch msg.Payload.(type) {
			case events.EventCacheStarted:
				gotCacheStarted = true
			case events.EventReady:
				gotReady = true
			}
		case <-timeout:
			t.Fatal("Timeout waiting for EventCacheStarted and EventReady")
		}
	}

	// Verify start command was called
	history := mock.GetCallHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 command (start), got %d: %v", len(history), history)
	}
	if len(history) > 0 && history[0] != "start" {
		t.Errorf("Expected 'start' command, got %q", history[0])
	}

	// Verify VCL mapping was stored
	storedMapping := starter.GetVCLMapping()
	if storedMapping == nil {
		t.Fatal("VCL mapping not stored")
	}
	if storedMapping.VCLSource != vclMapping.VCLSource {
		t.Errorf("VCL source = %q, want %q", storedMapping.VCLSource, vclMapping.VCLSource)
	}
}

func TestStartCache_AlreadyRunning(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)

	// Set up mock response for start command (status 300 = already running)
	mock.SetResponse("start", varnishadm.NewVarnishResponse(
		300,
		"Child process is already running",
	))

	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Subscribe to events
	sub, err := b.Subscribe("/process")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give the starter time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Publish VCLLoaded event
	err = b.Publish("/process", events.EventVCLLoaded{
		Name:    "test",
		Path:    "/tmp/test.vcl",
		Mapping: &varnishadm.VCLShowResult{},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish VCLLoaded event: %v", err)
	}

	// Should still publish EventCacheStarted and EventReady even when already running
	var gotCacheStarted, gotReady bool
	timeout := time.After(1 * time.Second)

	for !gotCacheStarted || !gotReady {
		select {
		case msg := <-sub.Messages():
			switch msg.Payload.(type) {
			case events.EventCacheStarted:
				gotCacheStarted = true
			case events.EventReady:
				gotReady = true
			}
		case <-timeout:
			t.Fatal("Timeout waiting for EventCacheStarted and EventReady")
		}
	}
}

func TestStartCache_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)

	// Set up mock response for start command (failure)
	mock.SetResponse("start", varnishadm.NewVarnishResponse(
		500,
		"Failed to start child process",
	))

	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Subscribe to events
	sub, err := b.Subscribe("/process")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give the starter time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Publish VCLLoaded event
	err = b.Publish("/process", events.EventVCLLoaded{
		Name:    "test",
		Path:    "/tmp/test.vcl",
		Mapping: &varnishadm.VCLShowResult{},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish VCLLoaded event: %v", err)
	}

	// Should publish EventProcessError
	timeout := time.After(1 * time.Second)
	gotError := false

	for !gotError {
		select {
		case msg := <-sub.Messages():
			if errEvent, ok := msg.Payload.(events.EventProcessError); ok {
				gotError = true
				if errEvent.Component != "cache-starter" {
					t.Errorf("Error component = %q, want %q", errEvent.Component, "cache-starter")
				}
				if errEvent.Error == nil {
					t.Error("Error event has nil error")
				}
			}
			// Skip other events (like EventVCLLoaded)
		case <-timeout:
			t.Fatal("Timeout waiting for EventProcessError")
		}
	}
}

func TestGetVCLMapping_Empty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)
	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)

	// Should return nil when no VCL has been loaded
	mapping := starter.GetVCLMapping()
	if mapping != nil {
		t.Errorf("GetVCLMapping() = %v, want nil", mapping)
	}
}

func TestGetVCLMapping_AfterLoad(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)

	mock.SetResponse("start", varnishadm.NewVarnishResponse(
		varnishadm.ClisOk,
		"OK",
	))

	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Subscribe to wait for completion
	sub, err := b.Subscribe("/process")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give the starter time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Create expected VCL mapping
	expectedMapping := &varnishadm.VCLShowResult{
		VCLSource: "vcl 4.1;\nbackend default { .host = \"localhost\"; }",
		ConfigMap: map[int]string{
			0: "main.vcl",
			1: "includes/common.vcl",
		},
		Entries: []varnishadm.VCLConfigEntry{
			{ConfigID: 0, Filename: "main.vcl", Size: 100},
			{ConfigID: 1, Filename: "includes/common.vcl", Size: 50},
		},
	}

	// Publish VCLLoaded event with mapping
	err = b.Publish("/process", events.EventVCLLoaded{
		Name:    "test-vcl",
		Path:    "/tmp/test.vcl",
		Mapping: expectedMapping,
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish VCLLoaded event: %v", err)
	}

	// Wait for EventCacheStarted to ensure processing is complete
	timeout := time.After(1 * time.Second)
	select {
	case msg := <-sub.Messages():
		if _, ok := msg.Payload.(events.EventCacheStarted); !ok {
			// Skip non-EventCacheStarted events
			select {
			case msg = <-sub.Messages():
				if _, ok := msg.Payload.(events.EventCacheStarted); !ok {
					t.Fatalf("Expected EventCacheStarted, got %T", msg.Payload)
				}
			case <-timeout:
				t.Fatal("Timeout waiting for EventCacheStarted")
			}
		}
	case <-timeout:
		t.Fatal("Timeout waiting for events")
	}

	// Verify mapping was stored
	mapping := starter.GetVCLMapping()
	if mapping == nil {
		t.Fatal("GetVCLMapping() returned nil")
	}

	if mapping.VCLSource != expectedMapping.VCLSource {
		t.Errorf("VCLSource = %q, want %q", mapping.VCLSource, expectedMapping.VCLSource)
	}

	if len(mapping.ConfigMap) != len(expectedMapping.ConfigMap) {
		t.Errorf("ConfigMap length = %d, want %d", len(mapping.ConfigMap), len(expectedMapping.ConfigMap))
	}

	for id, filename := range expectedMapping.ConfigMap {
		if mapping.ConfigMap[id] != filename {
			t.Errorf("ConfigMap[%d] = %q, want %q", id, mapping.ConfigMap[id], filename)
		}
	}

	if len(mapping.Entries) != len(expectedMapping.Entries) {
		t.Errorf("Entries length = %d, want %d", len(mapping.Entries), len(expectedMapping.Entries))
	}
}

func TestMultipleVCLLoadEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := varnishadm.NewMock(6082, "secret", logger)

	mock.SetResponse("start", varnishadm.NewVarnishResponse(
		varnishadm.ClisOk,
		"OK",
	))

	b := broker.New(broker.Config{
		DownStreamChanLen:  10,
		PublishChanLen:     10,
		SubscribeChanLen:   10,
		UnsubscribeChanLen: 10,
		DeliveryTimeout:    100 * time.Millisecond,
	})

	starter := New(mock, b, logger)
	starter.Start()

	// Give the starter time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Clear call history
	mock.ClearCallHistory()

	// Publish first VCL load event
	err := b.Publish("/process", events.EventVCLLoaded{
		Name:    "vcl1",
		Path:    "/tmp/vcl1.vcl",
		Mapping: &varnishadm.VCLShowResult{VCLSource: "vcl1"},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish first event: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish second VCL load event
	err = b.Publish("/process", events.EventVCLLoaded{
		Name:    "vcl2",
		Path:    "/tmp/vcl2.vcl",
		Mapping: &varnishadm.VCLShowResult{VCLSource: "vcl2"},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to publish second event: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Both events should trigger start commands
	history := mock.GetCallHistory()
	if len(history) != 2 {
		t.Errorf("Expected 2 start commands, got %d: %v", len(history), history)
	}

	// Latest mapping should be stored
	mapping := starter.GetVCLMapping()
	if mapping == nil {
		t.Fatal("GetVCLMapping() returned nil")
	}
	if mapping.VCLSource != "vcl2" {
		t.Errorf("VCLSource = %q, want %q (should be latest)", mapping.VCLSource, "vcl2")
	}
}
