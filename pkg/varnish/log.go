package varnish

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// LogParser parses varnishlog output
type LogParser struct {
	workdir       string
	cmd           *exec.Cmd
	executedLines map[int]bool
	backendCalls  int
	tracePattern  *regexp.Regexp
}

// NewLogParser creates a new varnishlog parser
func NewLogParser(workdir string) *LogParser {
	return &LogParser{
		workdir:       workdir,
		executedLines: make(map[int]bool),
		tracePattern:  regexp.MustCompile(`TRACE:(\d+):(\w+)`),
	}
}

// Start starts capturing varnishlog output for a specific transaction
func (lp *LogParser) Start(ctx context.Context) error {
	// Start varnishlog to capture all transactions
	args := []string{
		"-n", lp.workdir,
		"-g", "request", // Group by request
		"-i", "VCL_Log,Backend_health,BackendOpen,BackendClose", // Only capture relevant tags
	}

	lp.cmd = exec.CommandContext(ctx, "varnishlog", args...)

	stdout, err := lp.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := lp.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start varnishlog: %w", err)
	}

	// Parse output in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			lp.parseLine(line)
		}
	}()

	return nil
}

// parseLine parses a single line of varnishlog output
func (lp *LogParser) parseLine(line string) {
	// Look for TRACE log entries
	if strings.Contains(line, "VCL_Log") && strings.Contains(line, "TRACE:") {
		// Extract TRACE information
		matches := lp.tracePattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			lineNum, err := strconv.Atoi(matches[1])
			if err == nil {
				lp.executedLines[lineNum] = true
			}
		}
	}

	// Count backend connections
	if strings.Contains(line, "BackendOpen") || strings.Contains(line, "Backend_health") {
		lp.backendCalls++
	}
}

// GetExecutedLines returns the set of executed line numbers
func (lp *LogParser) GetExecutedLines() map[int]bool {
	return lp.executedLines
}

// GetBackendCalls returns the number of backend calls
func (lp *LogParser) GetBackendCalls() int {
	return lp.backendCalls
}

// Stop stops the varnishlog process
func (lp *LogParser) Stop() error {
	if lp.cmd != nil && lp.cmd.Process != nil {
		if err := lp.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill varnishlog: %w", err)
		}
		lp.cmd.Wait()
	}
	return nil
}
