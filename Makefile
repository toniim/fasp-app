.PHONY: help dev build build-darwin build-darwin-signed sign build-windows test lint mocks clean install run release

# Single source of truth for the version: internal/version/version.go
VERSION    := $(shell grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
MODULE     := github.com/heytonyne/fasp/internal/version
LDFLAGS    := -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildTime=$(BUILD_TIME)' -X '$(MODULE).GitCommit=$(GIT_COMMIT)'

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

build: ## Build production binary (stamps version from version.go)
	@echo "Building $(VERSION) ($(GIT_COMMIT))"
	@wails build -ldflags "$(LDFLAGS)"

build-darwin: ## Build for macOS
	@wails build -platform darwin/arm64 -ldflags "$(LDFLAGS)"

build-darwin-signed: ## Build for macOS with code signing (preserves screen recording permission)
	@wails build -platform darwin/arm64 -ldflags "$(LDFLAGS)"
	@./scripts/sign-macos.sh

sign: ## Sign macOS app (run after build to preserve permissions)
	@./scripts/sign-macos.sh

build-windows: ## Build for Windows
	@wails build -platform windows/amd64 -ldflags "$(LDFLAGS)"

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
	@./build/bin/fasp

release: ## Tag the current commit with the version from version.go and push it
	@echo "Releasing $(VERSION)"
	@if [ -n "$$(git status --porcelain)" ]; then echo "Working tree not clean — commit first"; exit 1; fi
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then echo "Tag $(VERSION) already exists — bump version.go first"; exit 1; fi
	@git tag "$(VERSION)"
	@git push origin "$(VERSION)"
	@echo "Tagged and pushed $(VERSION)"

