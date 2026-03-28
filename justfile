# Manifold — TUI for monitoring CI/CD pipelines

# Default recipe: show available commands
default:
    @just --list

# Build the binary
build:
    @mkdir -p bin
    go build -o bin/manifold .

# Run the application
run *args:
    go run . {{args}}

# Run all tests
test:
    go test -race ./... -v

# Run tests with coverage report
test-cover:
    go test -race -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Run static analysis
vet:
    @go vet ./...

# Apply Go modernization fixes
fix:
    @go fix ./...

# Format code
fmt:
    @gofmt -w .

# Run linters (cyclomatic complexity + dead assignments)
lint:
    #!/usr/bin/env bash
    set -o pipefail
    rc=0
    gocyclo -over 15 . || rc=1
    ineffassign ./... || rc=1
    exit $rc

# Check formatting (CI-friendly, fails if unformatted)
fmt-check:
    @test -z "$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Run all checks (format + vet + lint + tests)
check: fmt-check vet lint test

# Clean build artifacts
clean:
    @rm -rf bin coverage.out dist

# Build a snapshot release (no publish)
release-snapshot:
    goreleaser release --snapshot --clean
