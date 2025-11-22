.PHONY: build test clean

# Default target
build: vcltest

# Build the vcltest binary
vcltest: cmd/vcltest/*.go pkg/**/*.go
	go build -o vcltest ./cmd/vcltest

# Run all tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f vcltest
