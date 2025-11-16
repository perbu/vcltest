#!/bin/bash
# Run VTC tests for VCLTest framework
# This script runs varnishtest on all VTC test files

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if varnishtest is available
if ! command -v varnishtest &> /dev/null; then
    echo -e "${RED}Error: varnishtest not found${NC}"
    echo "Please install Varnish to run VTC tests."
    echo "  Ubuntu/Debian: apt-get install varnish"
    echo "  macOS: brew install varnish"
    exit 1
fi

# Check Varnish version
echo -e "${YELLOW}Checking Varnish version...${NC}"
varnishd -V 2>&1 || true
echo ""

# Count tests
BASIC_TESTS=(tests/b*.vtc)
TERMINAL_TESTS=(tests/terminal/t*.vtc)

if [ ! -e "${BASIC_TESTS[0]}" ]; then
    echo -e "${RED}No basic tests found in tests/b*.vtc${NC}"
    exit 1
fi

echo -e "${YELLOW}Running basic VTC tests...${NC}"
echo "=================================="
echo ""

# Run basic tests
PASSED=0
FAILED=0
FAILED_TESTS=()

for test in tests/b*.vtc; do
    if [ -f "$test" ]; then
        echo -e "Running ${YELLOW}$(basename "$test")${NC}..."
        if varnishtest "$test"; then
            echo -e "${GREEN}✓ PASSED${NC}"
            ((PASSED++))
        else
            echo -e "${RED}✗ FAILED${NC}"
            ((FAILED++))
            FAILED_TESTS+=("$test")
        fi
        echo ""
    fi
done

# Summary
echo "=================================="
echo -e "${YELLOW}Test Summary${NC}"
echo "=================================="
echo -e "Total:  $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    echo "Failed tests:"
    for test in "${FAILED_TESTS[@]}"; do
        echo "  - $test"
    done
else
    echo -e "Failed: 0"
fi

# Terminal tests info
if [ -e "${TERMINAL_TESTS[0]}" ]; then
    echo ""
    echo "=================================="
    echo -e "${YELLOW}Terminal Tests${NC}"
    echo "=================================="
    echo "Terminal tests were not run automatically."
    echo "To run them manually:"
    echo "  varnishtest tests/terminal/t00001.vtc"
    echo "  varnishtest tests/terminal/t00002.vtc"
fi

# Exit with appropriate code
if [ $FAILED -gt 0 ]; then
    exit 1
else
    exit 0
fi
