.PHONY: all build test vet staticcheck clean install-tools

# Build variables
BINARY_SERVER=bin/server
BINARY_AGENT=bin/agent
BINARY_KEYGEN=bin/keygen

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
GOLIST=$(GOCMD) list
GOGET=$(GOCMD) get

all: test build

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GOCMD) install golang.org/x/tools/cmd/goimports@latest
	$(GOCMD) install honnef.co/go/tools/cmd/staticcheck@latest

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify

# Tidy go.mod
tidy:
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

# Build all packages (ensures they're in the build cache)
build-all:
	@echo "Building all packages..."
	$(GOBUILD) ./...

# Build binaries
build:
	@echo "Building binaries..."
	@./build.sh

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -short -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	$(GOTEST) -v -short ./...

# Run go vet
vet: build-all
	@echo "Running go vet..."
	$(GOVET) ./...

# Run staticcheck (alternative to statictest)
staticcheck: build-all
	@echo "Running staticcheck..."
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed. Run 'make install-tools' first."; \
		exit 1; \
	fi

# Run go vet with statictest tool
statictest: build-all
	@echo "Running statictest..."
	@if command -v statictest >/dev/null 2>&1; then \
		$(GOVET) -vettool=$$(which statictest) ./...; \
	else \
		echo "statictest not installed"; \
		echo "This is typically provided by the Yandex Practicum CI environment"; \
		echo "Running regular go vet instead..."; \
		$(GOVET) ./...; \
	fi

# Prepare for CI (download deps and build all packages)
ci-prepare: deps build-all
	@echo "CI preparation complete"

# Full CI check
ci-check: ci-prepare test vet statictest
	@echo "CI checks passed"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.txt
	@$(GOCMD) clean -cache

# List all packages
list:
	@$(GOLIST) ./...

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@test -z $$(gofmt -l . | tee /dev/stderr)

help:
	@echo "Available targets:"
	@echo "  all          - Run tests and build binaries"
	@echo "  build        - Build binaries"
	@echo "  build-all    - Build all packages"
	@echo "  test         - Run tests"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  vet          - Run go vet"
	@echo "  staticcheck  - Run staticcheck"
	@echo "  statictest   - Run go vet with statictest"
	@echo "  ci-prepare   - Prepare environment for CI"
	@echo "  ci-check     - Run full CI checks"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy go.mod"
	@echo "  fmt          - Format code"
	@echo "  fmt-check    - Check code formatting"
	@echo "  clean        - Clean build artifacts"
	@echo "  install-tools- Install development tools"
	@echo "  list         - List all packages"
	@echo "  help         - Show this help message"

