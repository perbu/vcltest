package vclmod

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/perbu/vclparser/pkg/ast"
	"github.com/perbu/vclparser/pkg/parser"
	"github.com/perbu/vclparser/pkg/renderer"
)

// BackendAddress represents a backend's host and port
type BackendAddress struct {
	Host string
	Port string
}

// ValidationResult contains warnings and errors from backend validation
type ValidationResult struct {
	Warnings []string
	Errors   []string
}

// ValidateAndModifyBackends parses VCL once, validates backends, and modifies them in a single pass.
// This is more efficient than calling ValidateBackends and ModifyBackends separately.
// The vclPath parameter is used to resolve include directives relative to the VCL file's directory.
// Returns: (modifiedVCL, validationResult, error)
func ValidateAndModifyBackends(vclContent string, vclPath string, backends map[string]BackendAddress) (string, *ValidationResult, error) {
	// Get the directory of the VCL file for resolving includes
	vclDir := filepath.Dir(vclPath)

	// Parse VCL once to get AST, resolving includes
	// Skip subroutine validation initially since subroutines may be in includes
	root, err := parser.Parse(vclContent, vclPath,
		parser.WithResolveIncludes(vclDir),
		parser.WithSkipSubroutineValidation(true),
	)
	if err != nil {
		return "", nil, fmt.Errorf("parsing VCL: %w", err)
	}

	// Collect all backend names from VCL and validate
	vclBackends := make(map[string]bool)
	for _, decl := range root.Declarations {
		if backendDecl, ok := decl.(*ast.BackendDecl); ok {
			vclBackends[backendDecl.Name] = false // false = not used in YAML
		}
	}

	result := &ValidationResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	// Check that all YAML backends exist in VCL
	var vclBackendNames []string
	for name := range vclBackends {
		vclBackendNames = append(vclBackendNames, name)
	}

	for yamlName := range backends {
		if _, exists := vclBackends[yamlName]; !exists {
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
			vclBackends[yamlName] = true // mark as used
		}
	}

	// Warn about VCL backends not defined in YAML
	for vclName, used := range vclBackends {
		if !used {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Backend %q defined in VCL not used in test - will not be overridden", vclName))
		}
	}

	// Return error if any validation errors occurred
	if len(result.Errors) > 0 {
		return "", result, fmt.Errorf("backend validation failed")
	}

	// Modify backends in the AST
	for _, decl := range root.Declarations {
		backendDecl, ok := decl.(*ast.BackendDecl)
		if !ok {
			continue
		}

		// Check if this backend should be modified
		addr, shouldModify := backends[backendDecl.Name]
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
	modifiedVCL := renderer.Render(root)

	return modifiedVCL, result, nil
}

// ValidateBackends checks that all YAML backends exist in VCL and warns about unused VCL backends
// The vclPath parameter is used to resolve include directives relative to the VCL file's directory.
// Returns validation result with warnings for unused VCL backends and errors for missing backends
func ValidateBackends(vclContent string, vclPath string, yamlBackends map[string]BackendAddress) (*ValidationResult, error) {
	// Get the directory of the VCL file for resolving includes
	vclDir := filepath.Dir(vclPath)

	// Parse VCL to get AST, resolving includes
	// Skip subroutine validation initially since subroutines may be in includes
	root, err := parser.Parse(vclContent, vclPath,
		parser.WithResolveIncludes(vclDir),
		parser.WithSkipSubroutineValidation(true),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing VCL: %w", err)
	}

	// Collect all backend names from VCL
	vclBackends := make(map[string]bool)
	for _, decl := range root.Declarations {
		if backendDecl, ok := decl.(*ast.BackendDecl); ok {
			vclBackends[backendDecl.Name] = false // false = not used in YAML
		}
	}

	result := &ValidationResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	// Check that all YAML backends exist in VCL
	var vclBackendNames []string
	for name := range vclBackends {
		vclBackendNames = append(vclBackendNames, name)
	}

	for yamlName := range yamlBackends {
		if _, exists := vclBackends[yamlName]; !exists {
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
			vclBackends[yamlName] = true // mark as used
		}
	}

	// Warn about VCL backends not defined in YAML
	for vclName, used := range vclBackends {
		if !used {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Backend %q defined in VCL not used in test - will not be overridden", vclName))
		}
	}

	// Return error if any validation errors occurred
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("backend validation failed")
	}

	return result, nil
}

// ModifyBackends parses VCL, replaces backend addresses, and returns modified VCL
// The vclPath parameter is used to resolve include directives relative to the VCL file's directory.
// This function modifies the VCL AST to replace .host and .port for each backend
func ModifyBackends(vclContent string, vclPath string, backends map[string]BackendAddress) (string, error) {
	// Get the directory of the VCL file for resolving includes
	vclDir := filepath.Dir(vclPath)

	// Parse VCL to get AST, resolving includes
	// Skip subroutine validation initially since subroutines may be in includes
	root, err := parser.Parse(vclContent, vclPath,
		parser.WithResolveIncludes(vclDir),
		parser.WithSkipSubroutineValidation(true),
	)
	if err != nil {
		return "", fmt.Errorf("parsing VCL: %w", err)
	}

	// Walk AST and modify backend declarations
	for _, decl := range root.Declarations {
		backendDecl, ok := decl.(*ast.BackendDecl)
		if !ok {
			continue
		}

		// Check if this backend should be modified
		addr, shouldModify := backends[backendDecl.Name]
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
	modifiedVCL := renderer.Render(root)
	return modifiedVCL, nil
}

// findClosestMatch attempts to find the closest matching backend name
// Uses simple string distance heuristic (case-insensitive contains)
func findClosestMatch(target string, candidates []string) string {
	targetLower := strings.ToLower(target)

	// First try: case-insensitive exact match
	for _, candidate := range candidates {
		if strings.ToLower(candidate) == targetLower {
			return candidate
		}
	}

	// Second try: substring match
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), targetLower) ||
			strings.Contains(targetLower, strings.ToLower(candidate)) {
			return candidate
		}
	}

	// Return empty if no good match found
	return ""
}
