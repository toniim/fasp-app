.PHONY: help dev build build-darwin build-darwin-signed sign build-windows test lint mocks clean install run release check-wails

# Single source of truth for the version: internal/version/version.go
VERSION    := $(shell grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
MODULE     := github.com/heytonyne/fasp/internal/version
# The wails CLI must match the library in go.mod: a newer CLI regenerates
# frontend/wailsjs bindings the project does not track.
WAILS_VERSION := $(shell go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)
# That CLI cannot read packages built by newer Go releases ("package runtime
# without types"), so wails runs on the go.mod toolchain, as in CI.
GO_VERSION    := $(shell go mod edit -print | sed -n 's/^go //p')
dev build build-darwin build-darwin-signed build-windows: export GOTOOLCHAIN = go$(GO_VERSION)
LDFLAGS    := -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildTime=$(BUILD_TIME)' -X '$(MODULE).GitCommit=$(GIT_COMMIT)'

# ScreenCaptureKit screenshots need macOS 14. Wails adds its own 10.13 target
# unless CGO_CFLAGS / CGO_LDFLAGS already carry one.
ifeq ($(shell uname -s),Darwin)
export CGO_CFLAGS  += -mmacosx-version-min=14.0
export CGO_LDFLAGS += -mmacosx-version-min=14.0
endif

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	@echo "Installing Go dependencies..."
	@go mod download
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "Installing development tools..."
	@go install github.com/vektra/mockery/v2@latest
	@go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

check-wails: ## Fail if the wails CLI version differs from go.mod
	@cli=$$(wails version 2>/dev/null | head -n 1 | tr -d '[:space:]'); \
	if [ "$$cli" != "$(WAILS_VERSION)" ]; then \
		echo "wails CLI is '$$cli' but go.mod pins $(WAILS_VERSION)."; \
		echo "Run: make install (or go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION))"; \
		exit 1; \
	fi

dev: check-wails ## Run in development mode with hot reload
	@wails dev

build: check-wails ## Build production binary (stamps version from version.go)
	@echo "Building $(VERSION) ($(GIT_COMMIT))"
	@wails build -ldflags "$(LDFLAGS)"

build-darwin: check-wails ## Build for macOS
	@wails build -platform darwin/arm64 -ldflags "$(LDFLAGS)"

build-darwin-signed: check-wails ## Build for macOS with code signing (preserves screen recording permission)
	@wails build -platform darwin/arm64 -ldflags "$(LDFLAGS)"
	@./scripts/sign-macos.sh

sign: ## Sign macOS app (run after build to preserve permissions)
	@./scripts/sign-macos.sh

build-windows: check-wails ## Build for Windows (NSIS installer; auto-installs WebView2 if missing)
	@wails build -platform windows/amd64 -nsis -webview2 download -ldflags "$(LDFLAGS)"

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

