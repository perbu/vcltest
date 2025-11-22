package recorder

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

	// Create output file
	outFile, err := os.Create(r.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	// Start varnishlog with request grouping to capture backend connections
	r.cmd = exec.Command("varnishlog", "-n", r.workDir, "-g", "request")
	r.cmd.Stdout = outFile
	r.cmd.Stderr = outFile

	r.logger.Debug("Starting varnishlog recorder", "output_file", r.outputFile, "work_dir", r.workDir)

	if err := r.cmd.Start(); err != nil {
		outFile.Close()
		return fmt.Errorf("failed to start varnishlog: %w", err)
	}

	r.running = true
	r.outFile = outFile

	// Give varnishlog a moment to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop stops the recording and waits for varnishlog to finish
func (r *Recorder) Stop() error {
	if !r.running {
		return fmt.Errorf("recorder is not running")
	}

	r.logger.Debug("Stopping varnishlog recorder")

	// Send interrupt signal to process group to ensure varnishlog receives it
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
	case <-time.After(1 * time.Second):
		r.logger.Warn("varnishlog did not exit in time, killing process")
		if err := r.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill varnishlog: %w", err)
		}
		// Wait for kill to complete
		<-done
	}

	r.running = false

	// Close output file
	if r.outFile != nil {
		if err := r.outFile.Close(); err != nil {
			r.logger.Warn("Failed to close output file", "error", err)
		}
	}

	// Give file system a moment to flush writes
	time.Sleep(100 * time.Millisecond)

	return nil
}

// IsRunning returns whether the recorder is currently recording
func (r *Recorder) IsRunning() bool {
	return r.running
}

// Flush forces varnishlog to flush its buffer by sending SIGUSR1
func (r *Recorder) Flush() error {
	if !r.running {
		return fmt.Errorf("recorder is not running")
	}

	if r.cmd == nil || r.cmd.Process == nil {
		return fmt.Errorf("no process to flush")
	}

	// Send SIGUSR1 to force varnishlog to flush
	if err := r.cmd.Process.Signal(os.Signal(syscall.SIGUSR1)); err != nil {
		return fmt.Errorf("failed to send SIGUSR1 to varnishlog: %w", err)
	}

	r.logger.Debug("Flushed varnishlog buffer")

	// Give it a tiny moment to flush to disk
	time.Sleep(10 * time.Millisecond)

	return nil
}

// MarkPosition records the current log file position for later reading
func (r *Recorder) MarkPosition() (int64, error) {
	stat, err := os.Stat(r.outputFile)
	if err != nil {
		return 0, fmt.Errorf("failed to stat log file: %w", err)
	}
	return stat.Size(), nil
}

// GetMessagesSince reads log entries from a specific file offset
func (r *Recorder) GetMessagesSince(offset int64) ([]Message, error) {
	// Check if log file exists
	if _, err := os.Stat(r.outputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("log file does not exist: %s", r.outputFile)
	}

	// Open file and seek to offset
	file, err := os.Open(r.outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to offset %d: %w", offset, err)
	}

	// Read from offset to end
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	return r.parseMessages(string(data)), nil
}

// GetMessages reads the entire recorded log file and returns all parsed messages
func (r *Recorder) GetMessages() ([]Message, error) {
	return r.GetMessagesSince(0)
}

// GetVCLMessagesSince returns only VCL-related messages from a specific offset
func (r *Recorder) GetVCLMessagesSince(offset int64) ([]Message, error) {
	messages, err := r.GetMessagesSince(offset)
	if err != nil {
		return nil, err
	}

	vclMessages := make([]Message, 0)
	for _, msg := range messages {
		switch msg.Type {
		case MessageTypeVCLTrace, MessageTypeVCLCall, MessageTypeVCLReturn, MessageTypeBackendOpen:
			vclMessages = append(vclMessages, msg)
		}
	}

	return vclMessages, nil
}

// GetVCLMessages returns only VCL-related messages (VCL_trace, VCL_call, VCL_return)
func (r *Recorder) GetVCLMessages() ([]Message, error) {
	return r.GetVCLMessagesSince(0)
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
