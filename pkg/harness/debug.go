package harness

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/perbu/vcltest/pkg/runner"
	"github.com/perbu/vcltest/pkg/testspec"
)

// createDebugDump creates a debug dump directory with all test artifacts.
func createDebugDump(testFile, vclPath, workDir, varnishDir string, testRunner *runner.Runner, tests []testspec.TestSpec, passed, failed int, logger *slog.Logger) (string, error) {
	// Create dump directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	testBasename := filepath.Base(testFile)
	testBasename = strings.TrimSuffix(testBasename, filepath.Ext(testBasename))
	dumpDir := filepath.Join("/tmp", fmt.Sprintf("vcltest-debug-%s-%s", testBasename, timestamp))

	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		return "", fmt.Errorf("creating dump directory: %w", err)
	}

	logger.Debug("Creating debug dump", "dir", dumpDir)

	// Copy test YAML file
	if err := copyFile(testFile, filepath.Join(dumpDir, "test.yaml")); err != nil {
		logger.Warn("Failed to copy test file", "error", err)
	}

	// Copy original VCL file
	if err := copyFile(vclPath, filepath.Join(dumpDir, "original.vcl")); err != nil {
		logger.Warn("Failed to copy VCL file", "error", err)
	}

	// Save modified VCL (from runner)
	if modifiedVCL := testRunner.GetLoadedVCLSource(); modifiedVCL != "" {
		if err := os.WriteFile(filepath.Join(dumpDir, "modified.vcl"), []byte(modifiedVCL), 0644); err != nil {
			logger.Warn("Failed to save modified VCL", "error", err)
		}
	}

	// Copy varnishlog output
	varnishLogPath := filepath.Join(varnishDir, "varnish.log")
	if err := copyFile(varnishLogPath, filepath.Join(dumpDir, "varnish.log")); err != nil {
		logger.Warn("Failed to copy varnishlog", "error", err)
	}

	// Copy faketime control file and document its mtime
	faketimePath := filepath.Join(workDir, "faketime.control")
	if err := copyFile(faketimePath, filepath.Join(dumpDir, "faketime.control")); err != nil {
		// This is expected if faketime wasn't used, so don't warn
		logger.Debug("Faketime control file not found (expected if not using time scenarios)", "error", err)
	} else {
		// Document the faketime control file's mtime (this is how libfaketime tracks time)
		if info, err := os.Stat(faketimePath); err == nil {
			faketimeInfo := fmt.Sprintf("Faketime Control File Information\n"+
				"===================================\n\n"+
				"The faketime.control file uses its modification time (mtime) to control the fake time.\n"+
				"Varnish reads this file's mtime to determine what time it thinks it is.\n\n"+
				"Final mtime: %s\n"+
				"File size: %d bytes (always empty, only mtime matters)\n\n"+
				"When vcltest calls AdvanceTimeBy(offset), it runs:\n"+
				"  os.Chtimes(faketime.control, t0+offset, t0+offset)\n\n"+
				"This changes the file's mtime, which libfaketime intercepts when Varnish\n"+
				"calls stat() or similar syscalls, making Varnish think time has advanced.\n",
				info.ModTime().Format("2006-01-02 15:04:05.000"),
				info.Size(),
			)
			os.WriteFile(filepath.Join(dumpDir, "faketime-info.txt"), []byte(faketimeInfo), 0644)
		}
	}

	// Copy varnish secret file
	secretPath := filepath.Join(workDir, "secret")
	if err := copyFile(secretPath, filepath.Join(dumpDir, "secret")); err != nil {
		logger.Warn("Failed to copy secret file", "error", err)
	}

	// Copy varnishadm traffic log
	transcriptPath := filepath.Join(workDir, "varnishadm-traffic.log")
	if err := copyFile(transcriptPath, filepath.Join(dumpDir, "varnishadm-traffic.log")); err != nil {
		logger.Debug("Varnishadm traffic log not found", "error", err)
	}

	// Create README with test run information
	readme := fmt.Sprintf(`VCLTest Debug Dump
==================

Generated: %s
Test file: %s
VCL file: %s

Test Results:
- Passed: %d/%d
- Failed: %d/%d

Files in this directory:
- test.yaml: The original test specification
- original.vcl: The original VCL file before modification
- modified.vcl: The VCL file with backend addresses replaced
- varnish.log: The varnishlog output from test execution
- varnishadm-traffic.log: Transcript of varnishadm CLI commands and responses
- faketime.control: The libfaketime control file (if time scenarios used)
- faketime-info.txt: Explanation of how faketime works (if time scenarios used)
- secret: The varnishadm authentication secret
- README.txt: This file

Temporary Directories (preserved):
- Work dir: %s
- Varnish dir: %s

To manually inspect Varnish state, you can use:
  varnishadm -T localhost:6082 -S %s <command>

Example commands:
  varnishadm -T localhost:6082 -S %s vcl.list
  varnishadm -T localhost:6082 -S %s vcl.show shared-vcl
  varnishadm -T localhost:6082 -S %s backend.list

Note: Varnish is no longer running, these directories are for forensic analysis only.
`,
		time.Now().Format("2006-01-02 15:04:05"),
		testFile,
		vclPath,
		passed, len(tests),
		failed, len(tests),
		workDir,
		varnishDir,
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
		filepath.Join(dumpDir, "secret"),
	)

	if err := os.WriteFile(filepath.Join(dumpDir, "README.txt"), []byte(readme), 0644); err != nil {
		logger.Warn("Failed to create README", "error", err)
	}

	return dumpDir, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
