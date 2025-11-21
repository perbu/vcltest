package recorder

import (
	"log/slog"
	"os"
	"os/exec"
)

// MessageType represents the type of varnishlog message
type MessageType string

const (
	MessageTypeVCLTrace    MessageType = "VCL_trace"
	MessageTypeVCLCall     MessageType = "VCL_call"
	MessageTypeVCLReturn   MessageType = "VCL_return"
	MessageTypeBackendOpen MessageType = "BackendOpen"
	MessageTypeReqURL      MessageType = "ReqURL"
	MessageTypeRespStatus  MessageType = "RespStatus"
	MessageTypeReqHeader   MessageType = "ReqHeader"
	MessageTypeRespHeader  MessageType = "RespHeader"
	MessageTypeOther       MessageType = "Other"
)

// Message represents a parsed varnishlog message
type Message struct {
	Type    MessageType
	Content string
	Fields  []string
	Raw     string
}

// VCLTrace represents a parsed VCL_trace log entry
type VCLTrace struct {
	VCLName        string // e.g., "boot"
	TraceID        string // Sequential trace point ID
	SourceLocation string // e.g., "0.10.5" (config.line.column)
	Config         int    // Config number (0 = main VCL)
	Line           int    // Line number in VCL file
	Column         int    // Column position
}

// BackendCall represents a parsed BackendOpen log entry
type BackendCall struct {
	ID          string
	BackendName string
	Host        string
	Port        string
}

// Recorder manages varnishlog recording for capturing VCL execution traces
type Recorder struct {
	workDir    string
	outputFile string
	cmd        *exec.Cmd
	logger     *slog.Logger
	running    bool
	outfile    *os.File // stdout file handle
}
