.PHONY: help test test-coverage lint lint-fix build clean install all

GOPATH ?= $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies, dev tools, and git hooks
	@echo "[INFO] Installing development environment..."
	@echo ""
	@echo "[1/3] Installing Go dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies installed"
	@echo ""
	@echo "[2/3] Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "  Installing golangci-lint..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@command -v staticcheck >/dev/null 2>&1 || \
		(echo "  Installing staticcheck..." && \
		go install honnef.co/go/tools/cmd/staticcheck@latest)
	@command -v ineffassign >/dev/null 2>&1 || \
		(echo "  Installing ineffassign..." && \
		go install github.com/gordonklaus/ineffassign@latest)
	@command -v misspell >/dev/null 2>&1 || \
		(echo "  Installing misspell..." && \
		go install github.com/client9/misspell/cmd/misspell@latest)
	@command -v errcheck >/dev/null 2>&1 || \
		(echo "  Installing errcheck..." && \
		go install github.com/kisielk/errcheck@latest)
	@command -v gocyclo >/dev/null 2>&1 || \
		(echo "  Installing gocyclo..." && \
		go install github.com/fzipp/gocyclo/cmd/gocyclo@latest)
	@echo "✓ Development tools installed"
	@echo ""
	@echo "[3/3] Installing git hooks..."
	@bash .githooks/install.sh
	@echo ""
	@echo "✅ Installation complete! Ready to develop."

test: ## Run tests with coverage
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | grep total:
	@rm -f coverage.out

test-coverage: ## Run tests with HTML coverage report
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out

lint: ## Run all quality checks (gofmt, vet, staticcheck, misspell, gocyclo, errcheck)
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

lint-fix: ## Run linter with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix ./...

build: ## Build verification
	@echo "Building plugin..."
	@go build -v ./...
	@echo "✓ Build successful"

clean: ## Clean build artifacts and caches
	@echo "Cleaning..."
	@go clean -cache -testcache -modcache
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

all: lint test build ## Run all checks (lint, test, build)
	@echo ""
	@echo "========================================"
	@echo "✅ All checks passed!"
	@echo "========================================"
