// Package coverage provides block-level VCL coverage analysis.
//
// VCL_trace only fires at compound block entries (the { token), not for
// individual statements. This package analyzes VCL using AST parsing to
// identify block boundaries, then maps VCL_trace line numbers to blocks
// to determine which blocks were entered during execution.
package coverage

// Block represents a compound block in VCL (subroutine body, if branch, etc.)
type Block struct {
	Type       BlockType // Type of block (sub, if, elseif, else)
	Name       string    // For subs: "vcl_recv"; for if/elseif: condition text; for else: empty
	HeaderLine int       // Line containing the block header (sub/if/else keyword)
	OpenBrace  int       // Line of the opening {
	CloseBrace int       // Line of the closing }
	Entered    bool      // Whether VCL_trace fired for this block
	Children   []*Block  // Nested blocks (if statements inside subs, etc.)
}

// BlockType indicates the kind of VCL block
type BlockType string

const (
	BlockTypeSub    BlockType = "sub"
	BlockTypeIf     BlockType = "if"
	BlockTypeElseIf BlockType = "elseif"
	BlockTypeElse   BlockType = "else"
)

// FileBlocks contains all blocks extracted from a single VCL file
type FileBlocks struct {
	ConfigID int      // Varnish config ID for this file
	Filename string   // Path to the VCL file
	Blocks   []*Block // Top-level blocks (subroutines)
}

// LineInBlock checks if a line number falls within this block's range (inclusive)
func (b *Block) LineInBlock(line int) bool {
	return line >= b.OpenBrace && line <= b.CloseBrace
}

// AllBlocks returns a flat slice of this block and all descendant blocks
func (b *Block) AllBlocks() []*Block {
	result := []*Block{b}
	for _, child := range b.Children {
		result = append(result, child.AllBlocks()...)
	}
	return result
}

// FindBlockAtLine finds the deepest (most specific) block containing the given line.
// Returns nil if no block contains the line.
func (fb *FileBlocks) FindBlockAtLine(line int) *Block {
	for _, block := range fb.Blocks {
		if found := findBlockAtLineRecursive(block, line); found != nil {
			return found
		}
	}
	return nil
}

func findBlockAtLineRecursive(block *Block, line int) *Block {
	if !block.LineInBlock(line) {
		return nil
	}
	// Check children first to find the most specific block
	for _, child := range block.Children {
		if found := findBlockAtLineRecursive(child, line); found != nil {
			return found
		}
	}
	// No child contains the line, so this block is the most specific
	return block
}

// GetLineStatus returns the coverage status for each line in the file.
// The returned map has line numbers as keys and entered status as values.
// Lines not inside any block are not included in the map.
func (fb *FileBlocks) GetLineStatus() map[int]bool {
	status := make(map[int]bool)
	for _, block := range fb.Blocks {
		fillLineStatus(block, status)
	}
	return status
}

func fillLineStatus(block *Block, status map[int]bool) {
	// Mark all lines in this block with its entered status
	for line := block.OpenBrace; line <= block.CloseBrace; line++ {
		status[line] = block.Entered
	}
	// Children override parent status for their ranges
	for _, child := range block.Children {
		fillLineStatus(child, status)
	}
}
