package coverage

import (
	"fmt"
	"path/filepath"

	"github.com/perbu/vclparser/pkg/ast"
	"github.com/perbu/vclparser/pkg/parser"
)

// AnalyzeVCL parses VCL source and extracts block structure for coverage analysis.
// The vclPath is used for error reporting and include resolution.
func AnalyzeVCL(source string, vclPath string) (*FileBlocks, error) {
	vclDir := filepath.Dir(vclPath)

	root, err := parser.Parse(source, vclPath,
		parser.WithResolveIncludes(vclDir),
		parser.WithSkipSubroutineValidation(true),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing VCL: %w", err)
	}

	fb := &FileBlocks{
		Filename: vclPath,
		Blocks:   make([]*Block, 0),
	}

	// Walk declarations to find subroutines
	for _, decl := range root.Declarations {
		if subDecl, ok := decl.(*ast.SubDecl); ok {
			block := extractSubBlock(subDecl)
			fb.Blocks = append(fb.Blocks, block)
		}
	}

	return fb, nil
}

// extractSubBlock creates a Block from a subroutine declaration
func extractSubBlock(sub *ast.SubDecl) *Block {
	block := &Block{
		Type:       BlockTypeSub,
		Name:       sub.Name,
		HeaderLine: sub.Start().Line,
		Children:   make([]*Block, 0),
	}

	if sub.Body != nil {
		block.OpenBrace = sub.Body.Start().Line
		block.CloseBrace = sub.Body.End().Line

		// Extract nested blocks from statements
		block.Children = extractNestedBlocks(sub.Body.Statements)
	}

	return block
}

// extractNestedBlocks extracts if/elseif/else blocks from a list of statements
func extractNestedBlocks(statements []ast.Statement) []*Block {
	var blocks []*Block

	for _, stmt := range statements {
		if ifStmt, ok := stmt.(*ast.IfStatement); ok {
			ifBlocks := extractIfChain(ifStmt)
			blocks = append(blocks, ifBlocks...)
		}
	}

	return blocks
}

// extractIfChain extracts blocks from an if/elseif/else chain
func extractIfChain(ifStmt *ast.IfStatement) []*Block {
	var blocks []*Block

	// Extract the "if" block
	ifBlock := extractIfBlock(ifStmt, BlockTypeIf)
	blocks = append(blocks, ifBlock)

	// Handle else branch
	if ifStmt.Else != nil {
		elseBlocks := extractElseBranch(ifStmt.Else)
		blocks = append(blocks, elseBlocks...)
	}

	return blocks
}

// extractIfBlock creates a Block from an if or elseif statement
func extractIfBlock(ifStmt *ast.IfStatement, blockType BlockType) *Block {
	block := &Block{
		Type:       blockType,
		Name:       conditionString(ifStmt.Condition),
		HeaderLine: ifStmt.Start().Line,
		Children:   make([]*Block, 0),
	}

	// The "then" part could be a BlockStatement or a single statement
	if blockStmt, ok := ifStmt.Then.(*ast.BlockStatement); ok {
		block.OpenBrace = blockStmt.Start().Line
		block.CloseBrace = blockStmt.End().Line
		block.Children = extractNestedBlocks(blockStmt.Statements)
	} else {
		// Single statement without braces - use the statement's position
		block.OpenBrace = ifStmt.Then.Start().Line
		block.CloseBrace = ifStmt.Then.End().Line
	}

	return block
}

// extractElseBranch handles the else part of an if statement
func extractElseBranch(elseStmt ast.Statement) []*Block {
	var blocks []*Block

	switch stmt := elseStmt.(type) {
	case *ast.IfStatement:
		// This is an "else if" (elseif)
		elseifBlock := extractIfBlock(stmt, BlockTypeElseIf)
		blocks = append(blocks, elseifBlock)

		// Continue the chain if there's another else
		if stmt.Else != nil {
			blocks = append(blocks, extractElseBranch(stmt.Else)...)
		}

	case *ast.BlockStatement:
		// This is a plain "else { ... }"
		elseBlock := &Block{
			Type:       BlockTypeElse,
			Name:       "",
			HeaderLine: stmt.Start().Line, // "else" keyword is before the brace
			OpenBrace:  stmt.Start().Line,
			CloseBrace: stmt.End().Line,
			Children:   extractNestedBlocks(stmt.Statements),
		}
		blocks = append(blocks, elseBlock)

	default:
		// Single statement else (rare in VCL)
		elseBlock := &Block{
			Type:       BlockTypeElse,
			Name:       "",
			HeaderLine: elseStmt.Start().Line,
			OpenBrace:  elseStmt.Start().Line,
			CloseBrace: elseStmt.End().Line,
			Children:   make([]*Block, 0),
		}
		blocks = append(blocks, elseBlock)
	}

	return blocks
}

// conditionString extracts a readable string from a condition expression
func conditionString(expr ast.Expression) string {
	if expr == nil {
		return ""
	}
	// Use the AST node's String() method if available
	return expr.String()
}
