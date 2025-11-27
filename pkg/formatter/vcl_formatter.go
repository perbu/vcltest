package formatter

import (
	"fmt"
	"os"
	"strings"

	"github.com/perbu/vcltest/pkg/coverage"
	"golang.org/x/term"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorGray   = "\033[90m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBold   = "\033[1m"
)

// FormatVCLWithTrace formats VCL source code with execution highlights
// executedLines contains line numbers that were executed (1-indexed)
func FormatVCLWithTrace(vclSource string, executedLines []int, useColor bool) string {
	lines := strings.Split(vclSource, "\n")

	// Create a map for quick lookup of executed lines
	executedMap := make(map[int]bool)
	for _, lineNum := range executedLines {
		executedMap[lineNum] = true
	}

	var output strings.Builder

	for i, line := range lines {
		lineNum := i + 1
		executed := executedMap[lineNum]

		if useColor {
			if executed {
				// Executed line: green checkmark and content
				fmt.Fprintf(&output, "%s✓ %3d | %s%s\n", ColorGreen, lineNum, line, ColorReset)
			} else {
				// Non-executed line: gray and dimmed
				fmt.Fprintf(&output, "%s  %3d | %s%s\n", ColorGray, lineNum, line, ColorReset)
			}
		} else {
			// Plain text fallback
			if executed {
				fmt.Fprintf(&output, "✓ %3d | %s\n", lineNum, line)
			} else {
				fmt.Fprintf(&output, "  %3d | %s\n", lineNum, line)
			}
		}
	}

	return output.String()
}

// VCLFileInfo contains source and execution trace for a single VCL file
type VCLFileInfo struct {
	ConfigID      int
	Filename      string
	Source        string
	ExecutedLines []int
}

// FormatTestFailure formats a complete test failure message with multi-file VCL trace
func FormatTestFailure(testName string, errors []string, files []VCLFileInfo, backendCalls int, useColor bool) string {
	var output strings.Builder

	// Test name
	if useColor {
		fmt.Fprintf(&output, "\n%s%sFAILED:%s %s\n", ColorBold, ColorRed, ColorReset, testName)
	} else {
		fmt.Fprintf(&output, "\nFAILED: %s\n", testName)
	}

	// Error messages
	for _, err := range errors {
		if useColor {
			fmt.Fprintf(&output, "  %s✗%s %s\n", ColorRed, ColorReset, err)
		} else {
			fmt.Fprintf(&output, "  ✗ %s\n", err)
		}
	}

	// VCL execution trace
	if len(files) > 0 {
		if useColor {
			fmt.Fprintf(&output, "\n%s%sVCL Execution Trace:%s\n", ColorBold, ColorYellow, ColorReset)
		} else {
			fmt.Fprintf(&output, "\nVCL Execution Trace:\n")
		}

		// Check if ANY user VCL was executed
		totalExecutedLines := 0
		for _, file := range files {
			totalExecutedLines += len(file.ExecutedLines)
		}

		if totalExecutedLines == 0 {
			// All execution was in built-in VCL
			if useColor {
				fmt.Fprintf(&output, "%s  (No user VCL executed - request handled entirely by built-in VCL)%s\n",
					ColorGray, ColorReset)
			} else {
				fmt.Fprintf(&output, "  (No user VCL executed - request handled entirely by built-in VCL)\n")
			}

			// Still show main VCL for context, but all gray
			if len(files) > 0 {
				file := files[0] // Show main VCL at least
				if useColor {
					fmt.Fprintf(&output, "\n%s%s (config %d):%s\n", ColorBold, file.Filename, file.ConfigID, ColorReset)
				} else {
					fmt.Fprintf(&output, "\n%s (config %d):\n", file.Filename, file.ConfigID)
				}
				output.WriteString(FormatVCLWithTrace(file.Source, []int{}, useColor))
			}
		} else {
			// Display each file with execution traces
			for i, file := range files {
				if i > 0 {
					output.WriteString("\n") // Separator between files
				}

				// File header
				if useColor {
					fmt.Fprintf(&output, "%s%s (config %d):%s\n",
						ColorBold, file.Filename, file.ConfigID, ColorReset)
				} else {
					fmt.Fprintf(&output, "%s (config %d):\n", file.Filename, file.ConfigID)
				}

				// VCL with trace markers
				output.WriteString(FormatVCLWithTrace(file.Source, file.ExecutedLines, useColor))
			}
		}

		// Additional trace info
		if useColor {
			fmt.Fprintf(&output, "\n%sBackend Calls:%s %d\n", ColorBold, ColorReset, backendCalls)
		} else {
			fmt.Fprintf(&output, "\nBackend Calls: %d\n", backendCalls)
		}
	}

	return output.String()
}

// ShouldUseColor determines if color output should be used.
// Returns true only if stdout is a terminal (not piped to a file or another program).
func ShouldUseColor() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// FormatVCLWithBlocks formats VCL source code with block-level coverage highlighting.
// Lines within entered blocks are shown with a * marker and green color.
// Lines within non-entered blocks are shown in gray/dimmed with no marker.
// Lines outside any block are shown in normal color with no marker.
// The * marker ensures both humans (via colors) and LLMs (via markers) can see coverage.
func FormatVCLWithBlocks(vclSource string, blocks *coverage.FileBlocks, useColor bool) string {
	lines := strings.Split(vclSource, "\n")

	// Get line status from blocks (line -> entered)
	var lineStatus map[int]bool
	if blocks != nil {
		lineStatus = blocks.GetLineStatus()
	} else {
		lineStatus = make(map[int]bool)
	}

	var output strings.Builder

	for i, line := range lines {
		lineNum := i + 1
		entered, inBlock := lineStatus[lineNum]

		if useColor {
			if !inBlock {
				// Line outside any block - normal color, no marker
				fmt.Fprintf(&output, "  %4d | %s\n", lineNum, line)
			} else if entered {
				// Line inside entered block - green with * marker
				fmt.Fprintf(&output, "%s* %4d | %s%s\n", ColorGreen, lineNum, line, ColorReset)
			} else {
				// Line inside non-entered block - gray/dimmed, no marker
				fmt.Fprintf(&output, "%s  %4d | %s%s\n", ColorGray, lineNum, line, ColorReset)
			}
		} else {
			// Plain text - use * marker for entered blocks
			if inBlock && entered {
				fmt.Fprintf(&output, "* %4d | %s\n", lineNum, line)
			} else {
				fmt.Fprintf(&output, "  %4d | %s\n", lineNum, line)
			}
		}
	}

	return output.String()
}

// VCLFileInfoWithBlocks contains source and block-level coverage for a single VCL file
type VCLFileInfoWithBlocks struct {
	ConfigID int
	Filename string
	Source   string
	Blocks   *coverage.FileBlocks
}

// FormatTestFailureWithBlocks formats a test failure message with block-level coverage.
func FormatTestFailureWithBlocks(testName string, errors []string, files []VCLFileInfoWithBlocks, backendCalls int, useColor bool) string {
	var output strings.Builder

	// Test name
	if useColor {
		fmt.Fprintf(&output, "\n%s%sFAILED:%s %s\n", ColorBold, ColorRed, ColorReset, testName)
	} else {
		fmt.Fprintf(&output, "\nFAILED: %s\n", testName)
	}

	// Error messages
	for _, err := range errors {
		if useColor {
			fmt.Fprintf(&output, "  %s✗%s %s\n", ColorRed, ColorReset, err)
		} else {
			fmt.Fprintf(&output, "  ✗ %s\n", err)
		}
	}

	// VCL execution trace
	if len(files) > 0 {
		if useColor {
			fmt.Fprintf(&output, "\n%s%sVCL Block Coverage:%s\n", ColorBold, ColorYellow, ColorReset)
		} else {
			fmt.Fprintf(&output, "\nVCL Block Coverage:\n")
		}

		// Check if ANY blocks were entered
		anyEntered := false
		for _, file := range files {
			if file.Blocks != nil {
				for _, block := range file.Blocks.Blocks {
					if blockHasAnyEntered(block) {
						anyEntered = true
						break
					}
				}
			}
			if anyEntered {
				break
			}
		}

		if !anyEntered {
			// All execution was in built-in VCL
			if useColor {
				fmt.Fprintf(&output, "%s  (No user VCL blocks entered - request handled entirely by built-in VCL)%s\n",
					ColorGray, ColorReset)
			} else {
				fmt.Fprintf(&output, "  (No user VCL blocks entered - request handled entirely by built-in VCL)\n")
			}

			// Still show main VCL for context
			if len(files) > 0 {
				file := files[0]
				if useColor {
					fmt.Fprintf(&output, "\n%s%s (config %d):%s\n", ColorBold, file.Filename, file.ConfigID, ColorReset)
				} else {
					fmt.Fprintf(&output, "\n%s (config %d):\n", file.Filename, file.ConfigID)
				}
				output.WriteString(FormatVCLWithBlocks(file.Source, nil, useColor))
			}
		} else {
			// Display each file with block coverage
			for i, file := range files {
				if i > 0 {
					output.WriteString("\n")
				}

				// File header
				if useColor {
					fmt.Fprintf(&output, "%s%s (config %d):%s\n",
						ColorBold, file.Filename, file.ConfigID, ColorReset)
				} else {
					fmt.Fprintf(&output, "%s (config %d):\n", file.Filename, file.ConfigID)
				}

				// VCL with block coverage
				output.WriteString(FormatVCLWithBlocks(file.Source, file.Blocks, useColor))

				// Summary of entered/not-entered blocks
				if file.Blocks != nil {
					entered := coverage.GetEnteredBlocks(file.Blocks)
					notEntered := coverage.GetNotEnteredBlocks(file.Blocks)

					if len(entered) > 0 || len(notEntered) > 0 {
						if useColor {
							fmt.Fprintf(&output, "\n%sBlocks entered:%s ", ColorBold, ColorReset)
						} else {
							fmt.Fprintf(&output, "\nBlocks entered: ")
						}
						if len(entered) > 0 {
							names := make([]string, 0, len(entered))
							for _, b := range entered {
								names = append(names, blockDisplayName(b))
							}
							output.WriteString(strings.Join(names, ", "))
						} else {
							output.WriteString("(none)")
						}
						output.WriteString("\n")

						if len(notEntered) > 0 {
							if useColor {
								fmt.Fprintf(&output, "%sBlocks not entered:%s ", ColorBold, ColorReset)
							} else {
								fmt.Fprintf(&output, "Blocks not entered: ")
							}
							names := make([]string, 0, len(notEntered))
							for _, b := range notEntered {
								names = append(names, blockDisplayName(b))
							}
							output.WriteString(strings.Join(names, ", "))
							output.WriteString("\n")
						}
					}
				}
			}
		}

		// Additional trace info
		if useColor {
			fmt.Fprintf(&output, "\n%sBackend Calls:%s %d\n", ColorBold, ColorReset, backendCalls)
		} else {
			fmt.Fprintf(&output, "\nBackend Calls: %d\n", backendCalls)
		}
	}

	return output.String()
}

// blockHasAnyEntered checks if a block or any of its children were entered
func blockHasAnyEntered(block *coverage.Block) bool {
	if block.Entered {
		return true
	}
	for _, child := range block.Children {
		if blockHasAnyEntered(child) {
			return true
		}
	}
	return false
}

// blockDisplayName returns a readable name for a block
func blockDisplayName(block *coverage.Block) string {
	switch block.Type {
	case coverage.BlockTypeSub:
		return block.Name
	case coverage.BlockTypeIf:
		return fmt.Sprintf("if@%d", block.HeaderLine)
	case coverage.BlockTypeElseIf:
		return fmt.Sprintf("elseif@%d", block.HeaderLine)
	case coverage.BlockTypeElse:
		return fmt.Sprintf("else@%d", block.HeaderLine)
	default:
		return fmt.Sprintf("block@%d", block.HeaderLine)
	}
}
