# VCLTest Limitations and Known Issues

This document tracks current limitations, known issues, and areas that need future improvement.

## Current Limitations

### 1. VCL Parser Dependency

**Issue**: The original design called for using the `github.com/perbu/vclparser` library for proper AST-based VCL parsing and instrumentation.

**Status**: The vclparser library doesn't exist or isn't published yet.

**Current Workaround**: Implemented a simplified regex-based VCL instrumenter in `pkg/instrument/instrument.go`. This works for basic VCL files but has limitations:

- May not handle complex nested conditionals correctly
- May have issues with multiline statements
- Doesn't have full VCL syntax awareness
- Could break on edge cases like:
  - Comments within statements
  - String literals containing VCL-like syntax
  - Inline C blocks (`C{ ... }C`)
  - Complex VMOD usage

**Future Fix**: When the vclparser library becomes available, replace the regex-based implementation with proper AST-based instrumentation.

**Files Affected**:
- `pkg/instrument/instrument.go` - Contains the simplified implementation
- `go.mod` - vclparser dependency removed temporarily

---

### 2. VCL Instrumentation Edge Cases

**Issue**: The current regex-based instrumenter may not correctly handle all VCL constructs.

**Known Edge Cases**:

1. **Nested If Statements**: Deep nesting may not be traced correctly
   ```vcl
   if (condition1) {
       if (condition2) {
           if (condition3) {
               # May miss some traces here
           }
       }
   }
   ```

2. **Inline C Blocks**: Not preserved correctly
   ```vcl
   C{
       #include <stdio.h>
   }C
   ```

3. **Complex String Literals**: Strings containing braces or VCL keywords
   ```vcl
   set req.http.X-Test = "sub vcl_recv { }";
   ```

4. **VMOD Calls**: May not instrument complex VMOD usage correctly
   ```vcl
   import directors;
   new vdir = directors.round_robin();
   ```

**Workaround**: Test your VCL files with simple, well-structured code. Avoid deeply nested logic until proper parsing is implemented.

---

### 3. Line Number Mapping

**Issue**: The current implementation uses a simplified 1:1 line mapping between instrumented and original VCL.

**Impact**: When instrumentation adds lines (like `import std;`), the line numbers in traces may not exactly match the original VCL file.

**Future Fix**: Implement proper line mapping in the `LineMap` field of `instrument.Result`.

---

### 4. Multiple Backend Support

**Issue**: Only single backend replacement is currently supported.

**Status**: Not yet implemented (Phase 3 feature).

**Impact**: Tests with multiple backend definitions may not work correctly.

**Current Behavior**: All backends get replaced with the same mock backend address.

---

### 5. Test Discovery and Batch Execution

**Issue**: Can only run tests from a single file at a time.

**Status**: Planned for Phase 2.

**Workaround**: Use shell scripts to run multiple test files:
```bash
for test in tests/*.yaml; do
    ./vcltest "$test"
done
```

---

### 6. Varnish Installation Required

**Issue**: VCLTest requires Varnish to be installed on the system.

**Impact**: Cannot run tests in environments without Varnish (like many CI/CD systems).

**Prerequisites**:
- varnishd (Varnish daemon)
- varnishlog (for trace capture)
- Varnish 7.x or later recommended

**Workaround**: Use Docker containers with Varnish pre-installed for CI/CD.

---

### 7. VTC Test Execution

**Issue**: While VTC test files have been created in `tests/`, they cannot be executed in the current environment.

**Reason**: `varnishtest` tool is not available in this environment.

**Status**: VTC files are syntactically correct and ready to run when Varnish is available.

**To Run Tests**:
```bash
# When Varnish is installed:
varnishtest tests/b*.vtc           # Run basic tests
varnishtest tests/terminal/t*.vtc  # Run terminal tests

# Or use the provided script:
./run-vtc-tests.sh
```

**Note on Terminology**: VTC tests are run using the `varnishtest` tool (part of Varnish Test Case framework, also known as VTest or VTest2). The tool that runs these tests is `varnishtest`, not "GTest" - if you see references to "GTest" in the context of VTC tests, it likely refers to `varnishtest` or is a typo.

---

### 8. Error Messages

**Issue**: Some error messages could be more helpful.

**Areas for Improvement**:
- VCL parsing errors should show line numbers
- Test failures should suggest common fixes
- Backend connection errors should explain common causes

---

### 9. Performance

**Issue**: Each test starts its own varnishd instance.

**Impact**: Tests can be slow, especially when running many tests.

**Future Optimization**: Reuse varnishd instances across tests (started in Phase 1, needs refinement).

---

### 10. Platform Support

**Status**: Currently developed and tested on Linux only.

**Untested Platforms**:
- macOS (should work with Homebrew Varnish)
- Windows (likely requires WSL)

---

## Weird Things / Technical Debt

### 1. Import std Injection

The instrumenter always injects `import std;` after the VCL version line. If the VCL already imports std, the regex tries to skip it, but this could be more robust.

**File**: `pkg/instrument/instrument.go:67-85`

---

### 2. Hardcoded Indentation Detection

The `getIndent()` function uses simple character-by-character iteration. This works but could be more elegant.

**File**: `pkg/instrument/instrument.go:156-163`

---

### 3. No VCL Validation

The instrumenter doesn't validate that the VCL is syntactically correct before instrumentation. Invalid VCL will only fail when loaded into varnishd.

**Future Fix**: Add pre-validation step or better error reporting from varnishd.

---

### 4. Backend Address Parsing

Backend addresses are split on `:` which could fail for IPv6 addresses.

**File**: `pkg/instrument/instrument.go:93-98`

**Example Problem**:
```go
// This will break for "[::1]:8080"
parts := strings.Split(backendAddr, ":")
```

---

### 5. Test Result Line Mapping

The VCL execution trace uses the `ExecutedLines` map, but the mapping between original and instrumented VCL could be better tracked.

**Files**:
- `pkg/instrument/instrument.go` - Should build better LineMap
- `pkg/runner/output.go:116-140` - Uses the mapping for output

---

## Future Enhancements

### Phase 2 Features
- Test discovery (auto-find *.yaml files)
- Batch test execution
- Summary reporting
- Test filtering

### Phase 3 Features
- Advanced assertions (regex, cache behavior, TTL)
- Multiple backends per test
- Multi-request tests
- Parallel execution
- HTML reports
- Watch mode

---

## How to Contribute

If you encounter limitations or issues not listed here:

1. Document the issue in this file under "Current Limitations"
2. Include:
   - Clear description of the issue
   - Example that demonstrates the problem
   - Current workaround (if any)
   - Suggested fix (if known)
3. Link to relevant code files and line numbers
4. Commit the updated LIMITATIONS.md

---

## Version History

- **2024-11-16**: Initial version
  - Documented vclparser dependency issue
  - Listed regex-based instrumenter limitations
  - Added VTC test execution notes
  - Cataloged technical debt items
