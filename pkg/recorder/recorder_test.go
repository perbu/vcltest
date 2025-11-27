package recorder

import (
	"log/slog"
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name      string
		workDir   string
		logger    *slog.Logger
		wantError bool
	}{
		{
			name:      "valid config",
			workDir:   "/tmp/varnish",
			logger:    logger,
			wantError: false,
		},
		{
			name:      "empty workDir",
			workDir:   "",
			logger:    logger,
			wantError: true,
		},
		{
			name:      "nil logger",
			workDir:   "/tmp/varnish",
			logger:    nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := New(tt.workDir, tt.logger)
			if tt.wantError {
				if err == nil {
					t.Errorf("New() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("New() unexpected error: %v", err)
				return
			}
			if rec == nil {
				t.Errorf("New() returned nil recorder")
			}
			if rec.IsRunning() {
				t.Errorf("New recorder should not be running")
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	rec, _ := New("/tmp/test", logger)

	tests := []struct {
		name        string
		line        string
		wantType    MessageType
		wantContent string
	}{
		{
			name:        "VCL_trace",
			line:        "-   VCL_trace      boot 1 0.10.5",
			wantType:    MessageTypeVCLTrace,
			wantContent: "boot 1 0.10.5",
		},
		{
			name:        "VCL_call",
			line:        "-   VCL_call       RECV",
			wantType:    MessageTypeVCLCall,
			wantContent: "RECV",
		},
		{
			name:        "VCL_return",
			line:        "-   VCL_return     synth",
			wantType:    MessageTypeVCLReturn,
			wantContent: "synth",
		},
		{
			name:        "BackendOpen",
			line:        "-   BackendOpen    22 default 127.0.0.1 8080 127.0.0.1 56783 connect",
			wantType:    MessageTypeBackendOpen,
			wantContent: "22 default 127.0.0.1 8080 127.0.0.1 56783 connect",
		},
		{
			name:        "ReqURL",
			line:        "-   ReqURL         /admin",
			wantType:    MessageTypeReqURL,
			wantContent: "/admin",
		},
		{
			name:        "RespStatus",
			line:        "-   RespStatus     403",
			wantType:    MessageTypeRespStatus,
			wantContent: "403",
		},
		{
			name:     "empty line",
			line:     "",
			wantType: MessageTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := rec.parseLine(tt.line)
			if msg.Type != tt.wantType {
				t.Errorf("parseLine() type = %v, want %v", msg.Type, tt.wantType)
			}
			if tt.wantContent != "" && msg.Content != tt.wantContent {
				t.Errorf("parseLine() content = %v, want %v", msg.Content, tt.wantContent)
			}
		})
	}
}

func TestParseVCLTrace(t *testing.T) {
	tests := []struct {
		name       string
		msg        Message
		wantConfig int
		wantLine   int
		wantColumn int
		wantOk     bool
	}{
		{
			name: "valid trace",
			msg: Message{
				Type:   MessageTypeVCLTrace,
				Fields: []string{"-", "VCL_trace", "boot", "1", "0.10.5"},
			},
			wantConfig: 0,
			wantLine:   10,
			wantColumn: 5,
			wantOk:     true,
		},
		{
			name: "builtin VCL",
			msg: Message{
				Type:   MessageTypeVCLTrace,
				Fields: []string{"-", "VCL_trace", "boot", "2", "42.100.0"},
			},
			wantConfig: 42,
			wantLine:   100,
			wantColumn: 0,
			wantOk:     true,
		},
		{
			name: "wrong message type",
			msg: Message{
				Type:   MessageTypeVCLCall,
				Fields: []string{"-", "VCL_call", "RECV"},
			},
			wantOk: false,
		},
		{
			name: "insufficient fields",
			msg: Message{
				Type:   MessageTypeVCLTrace,
				Fields: []string{"-", "VCL_trace", "boot"},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace, ok := ParseVCLTrace(tt.msg)
			if ok != tt.wantOk {
				t.Errorf("ParseVCLTrace() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if !tt.wantOk {
				return
			}
			if trace.Config != tt.wantConfig {
				t.Errorf("ParseVCLTrace() config = %v, want %v", trace.Config, tt.wantConfig)
			}
			if trace.Line != tt.wantLine {
				t.Errorf("ParseVCLTrace() line = %v, want %v", trace.Line, tt.wantLine)
			}
			if trace.Column != tt.wantColumn {
				t.Errorf("ParseVCLTrace() column = %v, want %v", trace.Column, tt.wantColumn)
			}
		})
	}
}

func TestParseBackendCall(t *testing.T) {
	tests := []struct {
		name            string
		msg             Message
		wantBackendName string
		wantHost        string
		wantPort        string
		wantOk          bool
	}{
		{
			name: "valid backend call",
			msg: Message{
				Type:   MessageTypeBackendOpen,
				Fields: []string{"-", "BackendOpen", "22", "default", "127.0.0.1", "8080", "127.0.0.1", "56783", "connect"},
			},
			wantBackendName: "default",
			wantHost:        "127.0.0.1",
			wantPort:        "8080",
			wantOk:          true,
		},
		{
			name: "wrong message type",
			msg: Message{
				Type:   MessageTypeVCLTrace,
				Fields: []string{"-", "VCL_trace", "boot", "1", "0.10.5"},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, ok := ParseBackendCall(tt.msg)
			if ok != tt.wantOk {
				t.Errorf("ParseBackendCall() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if !tt.wantOk {
				return
			}
			if call.BackendName != tt.wantBackendName {
				t.Errorf("ParseBackendCall() backend = %v, want %v", call.BackendName, tt.wantBackendName)
			}
			if call.Host != tt.wantHost {
				t.Errorf("ParseBackendCall() host = %v, want %v", call.Host, tt.wantHost)
			}
			if call.Port != tt.wantPort {
				t.Errorf("ParseBackendCall() port = %v, want %v", call.Port, tt.wantPort)
			}
		})
	}
}

func TestGetExecutedLines(t *testing.T) {
	messages := []Message{
		{
			Type:   MessageTypeVCLTrace,
			Fields: []string{"-", "VCL_trace", "boot", "1", "0.10.5"},
		},
		{
			Type:   MessageTypeVCLTrace,
			Fields: []string{"-", "VCL_trace", "boot", "2", "0.15.9"},
		},
		{
			Type:   MessageTypeVCLTrace,
			Fields: []string{"-", "VCL_trace", "boot", "3", "0.10.5"}, // Duplicate line 10
		},
		{
			Type:   MessageTypeVCLTrace,
			Fields: []string{"-", "VCL_trace", "boot", "4", "42.100.0"}, // Built-in VCL, should be filtered
		},
	}

	lines := GetExecutedLines(messages)

	expectedLines := []int{10, 15}
	if len(lines) != len(expectedLines) {
		t.Errorf("GetExecutedLines() returned %d lines, want %d", len(lines), len(expectedLines))
	}

	for i, line := range lines {
		if line != expectedLines[i] {
			t.Errorf("GetExecutedLines()[%d] = %d, want %d", i, line, expectedLines[i])
		}
	}
}

func TestCountBackendCalls(t *testing.T) {
	messages := []Message{
		{Type: MessageTypeBackendOpen},
		{Type: MessageTypeVCLTrace},
		{Type: MessageTypeBackendOpen},
		{Type: MessageTypeVCLCall},
		{Type: MessageTypeBackendOpen},
	}

	count := CountBackendCalls(messages)
	if count != 3 {
		t.Errorf("CountBackendCalls() = %d, want 3", count)
	}
}

func TestGetTraceSummary(t *testing.T) {
	messages := []Message{
		{
			Type:    MessageTypeVCLCall,
			Content: "RECV",
		},
		{
			Type:   MessageTypeVCLTrace,
			Fields: []string{"-", "VCL_trace", "boot", "1", "0.10.5"},
		},
		{
			Type:    MessageTypeVCLReturn,
			Content: "synth",
		},
		{
			Type: MessageTypeBackendOpen,
		},
		{
			Type: MessageTypeBackendOpen,
		},
	}

	summary := GetTraceSummary(messages)

	if len(summary.ExecutedLines) != 1 || summary.ExecutedLines[0] != 10 {
		t.Errorf("GetTraceSummary() ExecutedLines = %v, want [10]", summary.ExecutedLines)
	}
	if summary.BackendCalls != 2 {
		t.Errorf("GetTraceSummary() BackendCalls = %d, want 2", summary.BackendCalls)
	}
}
