.PHONY: help dev build test lint mocks clean install

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	@echo "Installing Go dependencies..."
	@go mod download
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "Installing development tools..."
	@go install github.com/vektra/mockery/v2@latest

dev: ## Run in development mode with hot reload
	@wails dev

build: ## Build production binary
	@wails build

build-darwin: ## Build for macOS
	@wails build -platform darwin/arm64

build-windows: ## Build for Windows
	@wails build -platform windows/amd64

test: ## Run all tests
	@go clean -testcache
	@go test -race -cover -v -count=1 -timeout=30s ./...

test-coverage: ## Run tests with coverage report
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@go test -bench=. -benchmem ./...

lint: ## Run linters
	@echo "Running gofmt..."
	@gofmt -l -s -w .
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running go mod tidy..."
	@go mod tidy

mocks: ## Generate mocks for interfaces
	@echo "Generating mocks..."
	@mockery --all --dir=internal/service --output=internal/service/mocks --case=underscore

clean: ## Clean build artifacts
	@rm -rf build/bin
	@rm -rf frontend/dist
	@rm -f coverage.out coverage.html
	@echo "Cleaned build artifacts"

run: ## Run the built binary
	@./build/bin/grabix

