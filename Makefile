.PHONY: build test clean schema

# Default target
build: vcltest schema

# Build the vcltest binary
vcltest: cmd/vcltest/*.go pkg/**/*.go
	go build -o vcltest ./cmd/vcltest

# Generate JSON schema
schema: vcltest
	./vcltest -generate-schema > docs/schema.json

# Run all tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f vcltest
