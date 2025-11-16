# VCLTest VTC Test Suite

This directory contains VTC (Varnish Test Case) files for testing the VCLTest framework itself. These tests verify that VCLTest maintains compatibility with Varnish and VTest2.

## Directory Structure

```
tests/
├── README.md          # This file
├── b*.vtc             # Basic tests (can run automatically)
└── terminal/          # Tests requiring terminal interaction
    ├── README.md      # Terminal tests documentation
    └── t*.vtc         # Terminal-specific tests
```

## Test Categories

### Basic Tests (b*.vtc)

Basic tests can be run automatically in CI/CD environments. They test core VCL functionality:

- `b00001.vtc` - Basic VCL instrumentation
- `b00002.vtc` - Backend replacement
- `b00003.vtc` - Synthetic responses
- `b00004.vtc` - Multiple subroutines

### Terminal Tests (terminal/t*.vtc)

Terminal tests require interactive terminal features and should be run manually. See `terminal/README.md` for details.

## Prerequisites

To run these tests, you need:
- Varnish 7.x or later installed
- `varnishtest` command available in PATH
- `varnishd` and `varnishlog` commands available

## Running Tests

### Run all basic tests:
```bash
varnishtest tests/b*.vtc
```

### Run a specific test:
```bash
varnishtest tests/b00001.vtc
```

### Run with verbose output:
```bash
varnishtest -v tests/b00001.vtc
```

### Run tests and keep temp files on failure:
```bash
varnishtest -L tests/b00001.vtc
```

## Test Naming Convention

- **b#####.vtc** - Basic tests (automated)
- **t#####.vtc** - Terminal tests (manual)

Numbers are zero-padded 5-digit sequences for easy sorting.

## VTest2 Compatibility

These tests ensure VCLTest stays compatible with VTest2 by:

1. Testing standard VCL constructs that VCLTest must support
2. Verifying VCL instrumentation doesn't break Varnish
3. Checking that backend replacement works correctly
4. Validating trace logging functionality

## Adding New Tests

When adding new tests:

1. Use the appropriate prefix (`b` for basic, `t` for terminal)
2. Use the next available number in sequence
3. Include a descriptive test name in the `varnishtest` directive
4. Add comments explaining what the test verifies
5. Update this README if adding a new test category

## Continuous Integration

Basic tests (b*.vtc) can be integrated into CI/CD:

```bash
#!/bin/bash
# Run all basic tests
for test in tests/b*.vtc; do
    echo "Running $test..."
    varnishtest "$test" || exit 1
done
echo "All tests passed!"
```

## Troubleshooting

### varnishtest not found
Install Varnish: `apt-get install varnish` (Debian/Ubuntu) or equivalent for your system.

### Tests fail with "Cannot find varnishd"
Ensure varnishd is in your PATH: `which varnishd`

### Permission errors
varnishtest may need to create temporary directories. Run with appropriate permissions.
