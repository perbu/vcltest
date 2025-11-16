# Terminal-Requiring Tests

This directory contains VTC tests that require terminal interaction or specific terminal features.

## Why Separate?

These tests are kept separate from the main test suite because they:

1. **Require interactive terminal**: Tests that need user input or terminal-specific features
2. **Test color output**: Tests that verify ANSI color codes work correctly in terminals
3. **CLI interaction**: Tests that interact with Varnish CLI in ways that might not work in automated environments
4. **Environment-specific**: Tests that depend on terminal environment variables (TERM, NO_COLOR, etc.)

## Running These Tests

These tests should be run manually in a terminal environment:

```bash
# Run individual terminal test
varnishtest tests/terminal/t00001.vtc

# Run all terminal tests
varnishtest tests/terminal/t*.vtc
```

## Test Naming Convention

Terminal tests use the prefix `t` followed by a 5-digit number:
- `t00001.vtc` - First terminal test
- `t00002.vtc` - Second terminal test
- etc.

This distinguishes them from basic tests (prefixed with `b`).
