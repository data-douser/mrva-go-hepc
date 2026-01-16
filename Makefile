# Makefile for mrva-go-hepc

# Binary name
BINARY_NAME=hepc-server
CMD_DIR=./cmd/hepc-server

# Go related variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard *.go)

# Build variables
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

.PHONY: help
help: ## Display this help message
	@echo "$(COLOR_BOLD)Available targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'

.PHONY: all
all: clean fmt vet lint test build ## Run all quality checks and build

.PHONY: build
build: ## Build the binary
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME)...$(COLOR_RESET)"
	@mkdir -p $(GOBIN)
	go build $(LDFLAGS) -o $(GOBIN)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(COLOR_GREEN)✓ Binary built: $(GOBIN)/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: install
install: ## Install the binary to GOPATH/bin
	@echo "$(COLOR_GREEN)Installing $(BINARY_NAME)...$(COLOR_RESET)"
	go install $(LDFLAGS) $(CMD_DIR)
	@echo "$(COLOR_GREEN)✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: run
run: ## Run the server with local storage (requires --db-dir)
	@if [ -z "$(DB_DIR)" ]; then \
		echo "$(COLOR_YELLOW)Usage: make run DB_DIR=./path/to/db$(COLOR_RESET)"; \
		exit 1; \
	fi
	go run $(CMD_DIR) --storage local --db-dir $(DB_DIR)

.PHONY: test
test: ## Run tests
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	go test -v -race -coverprofile=coverage.out ./...
	@echo "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

.PHONY: test-coverage
test-coverage: test ## Run tests and show coverage
	@echo "$(COLOR_GREEN)Generating coverage report...$(COLOR_RESET)"
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report: coverage.html$(COLOR_RESET)"

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	go test -bench=. -benchmem ./...

.PHONY: fmt
fmt: ## Format code
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	go fmt ./...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	go vet ./...
	@echo "$(COLOR_GREEN)✓ Vet checks passed$(COLOR_RESET)"

.PHONY: lint
lint: ## Run golangci-lint
	@echo "$(COLOR_GREEN)Running golangci-lint...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	elif [ -x "$(shell go env GOPATH)/bin/golangci-lint" ]; then \
		$(shell go env GOPATH)/bin/golangci-lint run ./...; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not found. Install with: make install-tools$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)✓ Lint checks passed$(COLOR_RESET)"

.PHONY: tidy
tidy: ## Tidy go modules
	@echo "$(COLOR_GREEN)Tidying go modules...$(COLOR_RESET)"
	go mod tidy
	@echo "$(COLOR_GREEN)✓ Modules tidied$(COLOR_RESET)"

.PHONY: deps
deps: ## Download dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	go mod download
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"

.PHONY: verify
verify: ## Verify dependencies
	@echo "$(COLOR_GREEN)Verifying dependencies...$(COLOR_RESET)"
	go mod verify
	@echo "$(COLOR_GREEN)✓ Dependencies verified$(COLOR_RESET)"

.PHONY: clean
clean: ## Remove build artifacts
	@echo "$(COLOR_GREEN)Cleaning...$(COLOR_RESET)"
	@rm -rf $(GOBIN)
	@rm -f coverage.out coverage.html
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "$(COLOR_GREEN)Installing development tools...$(COLOR_RESET)"
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(COLOR_GREEN)✓ Tools installed$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_YELLOW)Note: Add $(shell go env GOPATH)/bin to your PATH if not already present:$(COLOR_RESET)"
	@echo "  export PATH=\"\$$PATH:$(shell go env GOPATH)/bin\""

.PHONY: check
check: fmt vet lint ## Run all checks (fmt, vet, lint)
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

.PHONY: ci
ci: clean deps verify check test build ## Run all CI checks

.PHONY: local-test
local-test: build ## Build and run a local test server
	@echo "$(COLOR_YELLOW)Starting test server on http://127.0.0.1:8070$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Note: This requires a local db-collection directory$(COLOR_RESET)"
	@mkdir -p test-db-collection
	@$(GOBIN)/$(BINARY_NAME) --storage local --db-dir test-db-collection

.PHONY: docker-build
docker-build: ## Build Docker image (if Dockerfile exists)
	@if [ -f Dockerfile ]; then \
		echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"; \
		docker build -t mrva-go-hepc:$(VERSION) .; \
		echo "$(COLOR_GREEN)✓ Docker image built: mrva-go-hepc:$(VERSION)$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)No Dockerfile found$(COLOR_RESET)"; \
	fi

.PHONY: version
version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

.DEFAULT_GOAL := help
