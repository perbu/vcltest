package coverage

// MatchTracesToBlocks marks blocks as entered based on VCL_trace line numbers.
//
// VCL_trace fires at the first statement inside a compound block, not at the
// opening brace itself. This function marks a block as entered if ANY trace
// falls within its line range [OpenBrace, CloseBrace].
//
// For nested blocks, we process children first so that traces inside child
// blocks mark the child as entered. Parent blocks are marked if they have
// traces that aren't inside any child block.
func MatchTracesToBlocks(fb *FileBlocks, tracedLines []int) {
	if fb == nil {
		return
	}

	// Build a set for O(1) lookup
	tracedSet := make(map[int]bool, len(tracedLines))
	for _, line := range tracedLines {
		tracedSet[line] = true
	}

	// Mark blocks that were entered
	for _, block := range fb.Blocks {
		matchBlockRecursive(block, tracedSet)
	}
}

func matchBlockRecursive(block *Block, tracedSet map[int]bool) {
	// Process children first
	for _, child := range block.Children {
		matchBlockRecursive(child, tracedSet)
	}

	// Check if any traced line falls STRICTLY INSIDE this block's range.
	// We use (OpenBrace, CloseBrace) exclusive of boundaries because:
	// - A trace at OpenBrace could be the "no branch taken" marker from a
	//   previous if-chain, not an actual block entry
	// - A trace at CloseBrace is the closing brace, not a statement
	//
	// VCL_trace fires at statements inside blocks, so valid entry traces
	// should be at lines OpenBrace < line < CloseBrace.
	for line := range tracedSet {
		if line > block.OpenBrace && line < block.CloseBrace {
			// Check if this line is inside any child block
			insideChild := false
			for _, child := range block.Children {
				if line >= child.OpenBrace && line <= child.CloseBrace {
					insideChild = true
					break
				}
			}
			if !insideChild {
				block.Entered = true
				break
			}
		}
	}

	// Also mark as entered if any child was entered (you can't enter a child
	// without entering the parent)
	for _, child := range block.Children {
		if child.Entered {
			block.Entered = true
			break
		}
	}
}

// MatchTracesToBlocksWithTolerance is like MatchTracesToBlocks but allows for
// slight line number differences. This can help with VCL formatting variations
// where the opening brace might be on a different line than expected.
//
// The tolerance parameter specifies how many lines off a trace can be while
// still matching a block. A tolerance of 1 means a trace on line N can match
// a block with OpenBrace at line N-1, N, or N+1.
func MatchTracesToBlocksWithTolerance(fb *FileBlocks, tracedLines []int, tolerance int) {
	if fb == nil {
		return
	}

	// Build a set for O(1) lookup
	tracedSet := make(map[int]bool, len(tracedLines))
	for _, line := range tracedLines {
		tracedSet[line] = true
	}

	// Mark blocks that were entered (with tolerance)
	for _, block := range fb.Blocks {
		matchBlockRecursiveWithTolerance(block, tracedSet, tolerance)
	}
}

func matchBlockRecursiveWithTolerance(block *Block, tracedSet map[int]bool, tolerance int) {
	// Check if this block's opening brace was traced (with tolerance)
	for offset := -tolerance; offset <= tolerance; offset++ {
		if tracedSet[block.OpenBrace+offset] {
			block.Entered = true
			break
		}
	}

	// Process children
	for _, child := range block.Children {
		matchBlockRecursiveWithTolerance(child, tracedSet, tolerance)
	}
}

// GetEnteredBlocks returns all blocks that were marked as entered.
func GetEnteredBlocks(fb *FileBlocks) []*Block {
	var entered []*Block
	for _, block := range fb.Blocks {
		entered = append(entered, getEnteredRecursive(block)...)
	}
	return entered
}

func getEnteredRecursive(block *Block) []*Block {
	var result []*Block
	if block.Entered {
		result = append(result, block)
	}
	for _, child := range block.Children {
		result = append(result, getEnteredRecursive(child)...)
	}
	return result
}

// GetNotEnteredBlocks returns all blocks that were NOT marked as entered.
func GetNotEnteredBlocks(fb *FileBlocks) []*Block {
	var notEntered []*Block
	for _, block := range fb.Blocks {
		notEntered = append(notEntered, getNotEnteredRecursive(block)...)
	}
	return notEntered
}

func getNotEnteredRecursive(block *Block) []*Block {
	var result []*Block
	if !block.Entered {
		result = append(result, block)
	}
	for _, child := range block.Children {
		result = append(result, getNotEnteredRecursive(child)...)
	}
	return result
}
