package recorder

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseVCLTrace parses a VCL_trace message into structured data
// Example: "VCL_trace boot 1 0.10.5"
// Returns VCLTrace and true if successful, empty VCLTrace and false otherwise
func ParseVCLTrace(msg Message) (VCLTrace, bool) {
	if msg.Type != MessageTypeVCLTrace {
		return VCLTrace{}, false
	}

	// Fields: ["-", "VCL_trace", "boot", "1", "0.10.5"]
	if len(msg.Fields) < 5 {
		return VCLTrace{}, false
	}

	trace := VCLTrace{
		VCLName:        msg.Fields[2],
		TraceID:        msg.Fields[3],
		SourceLocation: msg.Fields[4],
	}

	// Parse source location: "config.line.column"
	parts := strings.Split(trace.SourceLocation, ".")
	if len(parts) != 3 {
		return VCLTrace{}, false
	}

	var err error
	trace.Config, err = strconv.Atoi(parts[0])
	if err != nil {
		return VCLTrace{}, false
	}

	trace.Line, err = strconv.Atoi(parts[1])
	if err != nil {
		return VCLTrace{}, false
	}

	trace.Column, err = strconv.Atoi(parts[2])
	if err != nil {
		return VCLTrace{}, false
	}

	return trace, true
}

// ParseBackendCall parses a BackendOpen message into structured data
// Example: "BackendOpen 22 default 127.0.0.1 8080 127.0.0.1 56783 connect"
// Returns BackendCall and true if successful, empty BackendCall and false otherwise
func ParseBackendCall(msg Message) (BackendCall, bool) {
	if msg.Type != MessageTypeBackendOpen {
		return BackendCall{}, false
	}

	// Fields: ["-", "BackendOpen", "22", "default", "127.0.0.1", "8080", ...]
	if len(msg.Fields) < 6 {
		return BackendCall{}, false
	}

	return BackendCall{
		ID:          msg.Fields[2],
		BackendName: msg.Fields[3],
		Host:        msg.Fields[4],
		Port:        msg.Fields[5],
	}, true
}

// GetExecutedLinesByConfig extracts line numbers from VCL trace messages per config ID
// Only includes config IDs present in configMap (filters out built-in VCL)
// Returns map of config ID to sorted list of executed line numbers
func GetExecutedLinesByConfig(messages []Message, configMap map[int]string) map[int][]int {
	// map[configID]map[lineNumber]bool for deduplication
	execByConfig := make(map[int]map[int]bool)

	for _, msg := range messages {
		if trace, ok := ParseVCLTrace(msg); ok {
			// Only include if config ID is in user's ConfigMap
			if _, isUserVCL := configMap[trace.Config]; isUserVCL {
				if execByConfig[trace.Config] == nil {
					execByConfig[trace.Config] = make(map[int]bool)
				}
				execByConfig[trace.Config][trace.Line] = true
			}
		}
	}

	// Convert to sorted slices
	result := make(map[int][]int)
	for configID, lineMap := range execByConfig {
		lines := make([]int, 0, len(lineMap))
		for line := range lineMap {
			lines = append(lines, line)
		}

		// Sort the lines
		for i := 0; i < len(lines); i++ {
			for j := i + 1; j < len(lines); j++ {
				if lines[i] > lines[j] {
					lines[i], lines[j] = lines[j], lines[i]
				}
			}
		}

		result[configID] = lines
	}

	return result
}

// GetExecutedLines extracts line numbers from VCL trace messages
// Only returns lines from user VCL (config=0), filters out built-in VCL
// Removes duplicates and returns sorted unique line numbers
// This is a legacy function that only returns config=0 lines for backward compatibility
func GetExecutedLines(messages []Message) []int {
	configMap := map[int]string{0: "main"}
	byConfig := GetExecutedLinesByConfig(messages, configMap)
	if lines, ok := byConfig[0]; ok {
		return lines
	}
	return []int{}
}

// CountBackendCalls counts the number of BackendOpen messages
func CountBackendCalls(messages []Message) int {
	count := 0
	for _, msg := range messages {
		if msg.Type == MessageTypeBackendOpen {
			count++
		}
	}
	return count
}

// GetBackendsUsed extracts unique backend names from BackendOpen messages
func GetBackendsUsed(messages []Message) []string {
	backendSet := make(map[string]bool)

	for _, msg := range messages {
		if backend, ok := ParseBackendCall(msg); ok {
			backendSet[backend.BackendName] = true
		}
	}

	// Convert to slice
	backends := make([]string, 0, len(backendSet))
	for name := range backendSet {
		backends = append(backends, name)
	}

	return backends
}

// GetVCLTraceSummary returns a summary of VCL execution
type VCLTraceSummary struct {
	ExecutedLines []int
	BackendCalls  int
	BackendsUsed  []string // Names of backends that were called
	VCLCalls      []string
	VCLReturns    []string
}

// GetTraceSummary analyzes messages and returns execution summary
func GetTraceSummary(messages []Message) VCLTraceSummary {
	summary := VCLTraceSummary{
		ExecutedLines: GetExecutedLines(messages),
		BackendCalls:  CountBackendCalls(messages),
		BackendsUsed:  GetBackendsUsed(messages),
		VCLCalls:      make([]string, 0),
		VCLReturns:    make([]string, 0),
	}

	for _, msg := range messages {
		switch msg.Type {
		case MessageTypeVCLCall:
			summary.VCLCalls = append(summary.VCLCalls, msg.Content)
		case MessageTypeVCLReturn:
			summary.VCLReturns = append(summary.VCLReturns, msg.Content)
		}
	}

	return summary
}

// FormatVCLTrace formats a VCL trace for display
func (t VCLTrace) String() string {
	return fmt.Sprintf("line %d (config %d, col %d)", t.Line, t.Config, t.Column)
}

// FormatBackendCall formats a backend call for display
func (b BackendCall) String() string {
	return fmt.Sprintf("%s @ %s:%s", b.BackendName, b.Host, b.Port)
}
