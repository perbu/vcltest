package vclmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/perbu/vclparser/pkg/ast"
	"github.com/perbu/vclparser/pkg/parser"
	"github.com/perbu/vclparser/pkg/renderer"
)

// ProcessedVCLFile represents a processed VCL file with modified backend addresses
type ProcessedVCLFile struct {
	AbsolutePath string // Absolute path to the file
	RelativePath string // Path relative to main VCL (for include statements)
	Content      string // Modified VCL content
}

// ProcessVCLWithIncludes processes a VCL file and all its includes
// Returns a list of processed files that should be written to workdir
func ProcessVCLWithIncludes(mainVCLPath string, backends map[string]BackendAddress) ([]ProcessedVCLFile, *ValidationResult, error) {
	walker := &includeWalker{
		backends:     backends,
		visitedFiles: make(map[string]bool),
		processedFiles: make([]ProcessedVCLFile, 0),
		vclBackends:  make(map[string]bool),
		mainVCLDir:   filepath.Dir(mainVCLPath),
	}

	// Walk the include tree
	if err := walker.walkFile(mainVCLPath, mainVCLPath); err != nil {
		return nil, nil, err
	}

	// Validate backends
	result := walker.validateBackends()
	if len(result.Errors) > 0 {
		return nil, result, fmt.Errorf("backend validation failed")
	}

	return walker.processedFiles, result, nil
}

// includeWalker walks the include tree and processes each file
type includeWalker struct {
	backends       map[string]BackendAddress
	visitedFiles   map[string]bool
	processedFiles []ProcessedVCLFile
	vclBackends    map[string]bool // All backends found across all files
	mainVCLDir     string          // Directory of main VCL file
	includeDepth   int
}

const maxIncludeDepth = 10

// walkFile processes a single VCL file and recursively walks its includes
func (w *includeWalker) walkFile(vclPath string, mainVCLPath string) error {
	// Check depth limit
	if w.includeDepth >= maxIncludeDepth {
		return fmt.Errorf("maximum include depth (%d) exceeded at %s", maxIncludeDepth, vclPath)
	}

	// Convert to absolute path for tracking
	absPath, err := filepath.Abs(vclPath)
	if err != nil {
		return fmt.Errorf("resolving path %s: %w", vclPath, err)
	}

	// Check for circular includes
	if w.visitedFiles[absPath] {
		return nil // Already processed, skip (not an error in VCL)
	}
	w.visitedFiles[absPath] = true

	// Read the file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading VCL file %s: %w", vclPath, err)
	}

	// Parse VCL WITHOUT resolving includes (we want to walk them manually)
	program, err := parser.Parse(string(content), absPath,
		parser.WithSkipSubroutineValidation(true),
		parser.WithAllowMissingVersion(true), // Included files may not have version
	)
	if err != nil {
		return fmt.Errorf("parsing VCL %s: %w", vclPath, err)
	}

	// Collect backends from this file
	for _, decl := range program.Declarations {
		if backendDecl, ok := decl.(*ast.BackendDecl); ok {
			w.vclBackends[backendDecl.Name] = false // false = not used in YAML yet
		}
	}

	// Modify backends in this file BEFORE processing includes
	modifiedContent, err := w.modifyBackendsInAST(program)
	if err != nil {
		return fmt.Errorf("modifying backends in %s: %w", vclPath, err)
	}

	// Calculate relative path from main VCL directory
	relativePath, err := filepath.Rel(w.mainVCLDir, absPath)
	if err != nil {
		// If we can't get a relative path, use just the filename
		relativePath = filepath.Base(absPath)
	}

	// Add this file to processed files (main file will be first, then includes in order)
	w.processedFiles = append(w.processedFiles, ProcessedVCLFile{
		AbsolutePath: absPath,
		RelativePath: relativePath,
		Content:      modifiedContent,
	})

	// Process includes after adding this file (so main file is first)
	w.includeDepth++
	for _, decl := range program.Declarations {
		if includeDecl, ok := decl.(*ast.IncludeDecl); ok {
			// Resolve include path relative to current file's directory
			includePath := includeDecl.Path
			if !filepath.IsAbs(includePath) {
				currentDir := filepath.Dir(absPath)
				includePath = filepath.Join(currentDir, includeDecl.Path)
			}

			// Recursively process the included file
			if err := w.walkFile(includePath, mainVCLPath); err != nil {
				return fmt.Errorf("processing include %s: %w", includeDecl.Path, err)
			}
		}
	}
	w.includeDepth--

	return nil
}

// modifyBackendsInAST modifies backend declarations in an AST
func (w *includeWalker) modifyBackendsInAST(program *ast.Program) (string, error) {
	// Walk AST and modify backend declarations
	for _, decl := range program.Declarations {
		backendDecl, ok := decl.(*ast.BackendDecl)
		if !ok {
			continue
		}

		// Check if this backend should be modified
		addr, shouldModify := w.backends[backendDecl.Name]
		if !shouldModify {
			continue
		}

		// Find or create .host and .port properties
		hostFound := false
		portFound := false

		for _, prop := range backendDecl.Properties {
			switch prop.Name {
			case "host":
				// Replace host value
				prop.Value = &ast.StringLiteral{Value: addr.Host}
				hostFound = true
			case "port":
				// Replace port value
				prop.Value = &ast.StringLiteral{Value: addr.Port}
				portFound = true
			}
		}

		// Add missing properties
		if !hostFound {
			backendDecl.Properties = append(backendDecl.Properties, &ast.BackendProperty{
				Name:  "host",
				Value: &ast.StringLiteral{Value: addr.Host},
			})
		}
		if !portFound {
			backendDecl.Properties = append(backendDecl.Properties, &ast.BackendProperty{
				Name:  "port",
				Value: &ast.StringLiteral{Value: addr.Port},
			})
		}
	}

	// Render modified AST back to VCL
	modifiedVCL := renderer.Render(program)
	return modifiedVCL, nil
}

// validateBackends checks that all YAML backends exist in VCL and warns about unused VCL backends
func (w *includeWalker) validateBackends() *ValidationResult {
	result := &ValidationResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	// Check that all YAML backends exist in VCL
	var vclBackendNames []string
	for name := range w.vclBackends {
		vclBackendNames = append(vclBackendNames, name)
	}

	for yamlName := range w.backends {
		if _, exists := w.vclBackends[yamlName]; !exists {
			// Generate helpful error message
			suggestion := findClosestMatch(yamlName, vclBackendNames)
			errMsg := fmt.Sprintf("Backend %q defined in test YAML not found in VCL", yamlName)
			if len(vclBackendNames) > 0 {
				errMsg += fmt.Sprintf("\n  Available backends in VCL: %v", vclBackendNames)
				if suggestion != "" {
					errMsg += fmt.Sprintf("\n  Did you mean %q?", suggestion)
				}
			} else {
				errMsg += "\n  No backends found in VCL"
			}
			result.Errors = append(result.Errors, errMsg)
		} else {
			w.vclBackends[yamlName] = true // mark as used
		}
	}

	// Warn about VCL backends not defined in YAML
	for vclName, used := range w.vclBackends {
		if !used {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Backend %q defined in VCL not used in test - will not be overridden", vclName))
		}
	}

	return result
}
