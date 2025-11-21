package recorder

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// New creates a new varnishlog recorder
// workDir is the varnish working directory (-n parameter)
func New(workDir string, logger *slog.Logger) (*Recorder, error) {
	if workDir == "" {
		return nil, fmt.Errorf("workDir cannot be empty")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Check if varnishlog is available
	if _, err := exec.LookPath("varnishlog"); err != nil {
		return nil, fmt.Errorf("varnishlog not found in PATH: %w", err)
	}

	outputFile := filepath.Join(workDir, "varnish.log")

	return &Recorder{
		workDir:    workDir,
		outputFile: outputFile,
		logger:     logger,
		running:    false,
	}, nil
}

// Start begins recording varnishlog output to a binary file
func (r *Recorder) Start() error {
	if r.running {
		return fmt.Errorf("recorder is already running")
	}

	// Remove old log file if it exists
	if err := os.Remove(r.outputFile); err != nil && !os.IsNotExist(err) {
		r.logger.Warn("Failed to remove old log file", "error", err)
	}

	// Start varnishlog in background
	// -n: varnish working directory
	// -g request: group by request
	// Write to stdout instead of binary file (no -w flag)
	// Binary format has buffering issues with single requests
	r.cmd = exec.Command("varnishlog",
		"-n", r.workDir,
		"-g", "request",
	)

	// Capture stdout to file
	var err error
	r.outfile, err = os.Create(r.outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	r.cmd.Stdout = r.outfile
	r.cmd.Stderr = os.Stderr

	r.logger.Info("Starting varnishlog recorder", "output_file", r.outputFile)

	if err := r.cmd.Start(); err != nil {
		r.outfile.Close()
		return fmt.Errorf("failed to start varnishlog: %w", err)
	}

	r.running = true

	// Give varnishlog a moment to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop stops the recording and waits for varnishlog to finish
func (r *Recorder) Stop() error {
	if !r.running {
		return fmt.Errorf("recorder is not running")
	}

	r.logger.Info("Stopping varnishlog recorder")

	// Send interrupt signal to gracefully stop varnishlog
	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to signal varnishlog: %w", err)
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- r.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			r.logger.Warn("varnishlog exited with error", "error", err)
		}
	case <-time.After(5 * time.Second):
		r.logger.Warn("varnishlog did not exit in time, killing process")
		if err := r.cmd.Process.Kill(); err != nil {
			r.outfile.Close()
			return fmt.Errorf("failed to kill varnishlog: %w", err)
		}
	}

	// Close output file
	if r.outfile != nil {
		r.outfile.Close()
	}

	r.running = false
	return nil
}

// IsRunning returns whether the recorder is currently recording
func (r *Recorder) IsRunning() bool {
	return r.running
}

// GetMessages reads the recorded log file and returns all parsed messages
func (r *Recorder) GetMessages() ([]Message, error) {
	if r.running {
		return nil, fmt.Errorf("cannot read messages while recording is active, call Stop() first")
	}

	// Check if log file exists
	if _, err := os.Stat(r.outputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("log file does not exist: %s", r.outputFile)
	}

	// Read the text log file directly (no longer binary)
	data, err := os.ReadFile(r.outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	return r.parseMessages(string(data)), nil
}

// GetVCLMessages returns only VCL-related messages (VCL_trace, VCL_call, VCL_return)
func (r *Recorder) GetVCLMessages() ([]Message, error) {
	messages, err := r.GetMessages()
	if err != nil {
		return nil, err
	}

	vclMessages := make([]Message, 0)
	for _, msg := range messages {
		switch msg.Type {
		case MessageTypeVCLTrace, MessageTypeVCLCall, MessageTypeVCLReturn:
			vclMessages = append(vclMessages, msg)
		}
	}

	return vclMessages, nil
}

// parseMessages parses raw varnishlog output into structured messages
func (r *Recorder) parseMessages(output string) []Message {
	messages := make([]Message, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		msg := r.parseLine(line)
		if msg.Type != MessageTypeOther {
			messages = append(messages, msg)
		}
	}

	return messages
}

// parseLine parses a single varnishlog line into a Message
// Example line: "-   VCL_trace      boot 1 0.10.5"
func (r *Recorder) parseLine(line string) Message {
	msg := Message{
		Raw:  line,
		Type: MessageTypeOther,
	}

	// Split on whitespace and filter empty fields
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return msg
	}

	// First field after whitespace is the message type
	msgType := fields[1]

	// Store all fields
	msg.Fields = fields

	// Determine message type and extract content
	switch msgType {
	case "VCL_trace":
		msg.Type = MessageTypeVCLTrace
		if len(fields) >= 5 {
			msg.Content = strings.Join(fields[2:], " ")
		}
	case "VCL_call":
		msg.Type = MessageTypeVCLCall
		if len(fields) >= 3 {
			msg.Content = fields[2]
		}
	case "VCL_return":
		msg.Type = MessageTypeVCLReturn
		if len(fields) >= 3 {
			msg.Content = fields[2]
		}
	case "BackendOpen":
		msg.Type = MessageTypeBackendOpen
		if len(fields) >= 3 {
			msg.Content = strings.Join(fields[2:], " ")
		}
	case "ReqURL":
		msg.Type = MessageTypeReqURL
		if len(fields) >= 3 {
			msg.Content = fields[2]
		}
	case "RespStatus":
		msg.Type = MessageTypeRespStatus
		if len(fields) >= 3 {
			msg.Content = fields[2]
		}
	case "ReqHeader":
		msg.Type = MessageTypeReqHeader
		if len(fields) >= 3 {
			msg.Content = strings.Join(fields[2:], " ")
		}
	case "RespHeader":
		msg.Type = MessageTypeRespHeader
		if len(fields) >= 3 {
			msg.Content = strings.Join(fields[2:], " ")
		}
	}

	return msg
}

// GetOutputFile returns the path to the log output file
func (r *Recorder) GetOutputFile() string {
	return r.outputFile
}
