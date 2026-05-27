BINARY_NAME := bifrost
CMD_PATH    := ./cmd/bifrost
BUILD_DIR   := ./bin

GO      := go
GOLINT  := golangci-lint

.PHONY: all build run clean lint test test-verbose tidy help

## all: build the bifrost binary (default)
all: build

## build: compile bifrost into ./bin/
build:
	@echo "→ Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY_NAME)"

## run: build and run bifrost with arguments (usage: make run ARGS="tenant list")
run: build
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

## clean: remove build artifacts
clean:
	@echo "→ Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "✓ Clean"

## lint: run golangci-lint across the project
lint:
	@echo "→ Running linter..."
	$(GOLINT) run ./...
	@echo "✓ Lint passed"

## test: run all tests
test:
	@echo "→ Running tests..."
	$(GO) test ./... -count=1
	@echo "✓ Tests passed"

## test-verbose: run all tests with verbose output
test-verbose:
	$(GO) test ./... -v -count=1

## test-coverage: run tests and output coverage report
test-coverage:
	@echo "→ Running tests with coverage..."
	$(GO) test ./... -coverprofile=coverage.out -count=1
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report written to coverage.html"

## tidy: tidy and verify go modules
tidy:
	@echo "→ Tidying modules..."
	$(GO) mod tidy
	$(GO) mod verify
	@echo "✓ Modules tidy"

## install: install bifrost into GOPATH/bin
install:
	@echo "→ Installing $(BINARY_NAME)..."
	$(GO) install $(CMD_PATH)
	@echo "✓ Installed"

## help: print this help message
help:
	@echo ""
	@echo "KaaS Bifrost — available make targets:"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
	@echo ""