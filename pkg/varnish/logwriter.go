package varnish

import (
	"bufio"
	"context"
	"log/slog"
	"strings"
)

// logWriter is an io.Writer adapter that routes varnishd output through structured logging
type logWriter struct {
	logger *slog.Logger
	source string
}

// newLogWriter creates a new log writer for varnishd output
func newLogWriter(logger *slog.Logger, source string) *logWriter {
	return &logWriter{
		logger: logger,
		source: source,
	}
}

// Write implements io.Writer interface and logs each line through slog
func (lw *logWriter) Write(p []byte) (n int, err error) {
	// Split the input by newlines and log each line
	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Determine log level based on line prefix
		var level slog.Level
		switch {
		case strings.HasPrefix(line, "Debug:"):
			level = slog.LevelDebug
			line = strings.TrimSpace(strings.TrimPrefix(line, "Debug:"))
		case strings.HasPrefix(line, "Info:"):
			level = slog.LevelDebug
			line = strings.TrimSpace(strings.TrimPrefix(line, "Info:"))
		case strings.HasPrefix(line, "Warning:") || strings.HasPrefix(line, "Warn:"):
			level = slog.LevelWarn
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Warning:"), "Warn:"))
		case strings.HasPrefix(line, "Error:"):
			level = slog.LevelError
			line = strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
		case strings.HasPrefix(line, "Child ") && (strings.Contains(line, "Started") || strings.Contains(line, "said")):
			// Varnish child process status - treat as debug
			level = slog.LevelDebug
		default:
			// Default to debug level for other varnishd output
			level = slog.LevelDebug
		}
		// Log with source attribution
		lw.logger.Log(context.Background(), level, line, "source", lw.source)
	}
	// Always return the full length written to satisfy io.Writer interface
	return len(p), nil
}
