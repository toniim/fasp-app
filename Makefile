.PHONY: help dev build build-darwin build-darwin-signed sign build-windows test lint mocks clean install run release check-wails

# Single source of truth for the version: internal/version/version.go
VERSION    := $(shell grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S')
MODULE     := github.com/heytonyne/fasp/internal/version
# The wails3 CLI must match the library in go.mod: the binding generator and
# the runtime package are versioned together.
WAILS_VERSION := $(shell sed -n 's|.*wailsapp/wails/v3 \(v[^ ]*\).*|\1|p' go.mod)
# Everything runs on the toolchain pinned in go.mod (Wails v3 needs Go 1.26; the
# Go 1.27 release is not used), as in CI.
GO_TOOLCHAIN  := $(shell sed -n 's/^toolchain //p' go.mod)
export GOTOOLCHAIN = $(GO_TOOLCHAIN)
LDFLAGS    := -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildTime=$(BUILD_TIME)' -X '$(MODULE).GitCommit=$(GIT_COMMIT)'
# The Taskfile build tasks add these to `go build -ldflags`.
export FASP_LDFLAGS := $(LDFLAGS)
# Vite dev server port used by `make dev`.
VITE_PORT := 9245

# ScreenCaptureKit screenshots need macOS 14; the Taskfile sets the same target
# for the app build, and this covers `make test` and `go build`.
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
	@go install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)

check-wails: ## Fail if the wails3 CLI version differs from go.mod
	@cli=$$(wails3 version 2>&1 | head -n 1 | tr -d '[:space:]'); \
	if [ "$$cli" != "$(WAILS_VERSION)" ]; then \
		echo "wails3 CLI is '$$cli' but go.mod pins $(WAILS_VERSION)."; \
		echo "Run: make install (or go install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION))"; \
		exit 1; \
	fi

dev: check-wails ## Run in development mode with hot reload
	@wails3 dev -config ./build/config.yml -port $(VITE_PORT)

# macOS builds the .app bundle (wails3 package); Windows builds fasp.exe.
ifeq ($(shell uname -s),Darwin)
HOST_BUILD := wails3 package
else
HOST_BUILD := wails3 build
endif

build: check-wails ## Build production binary (stamps version from version.go)
	@echo "Building $(VERSION) ($(GIT_COMMIT))"
	@$(HOST_BUILD)

build-darwin: check-wails ## Build for macOS
	@wails3 package GOOS=darwin GOARCH=arm64

build-darwin-signed: check-wails ## Build for macOS with code signing (preserves screen recording permission)
	@wails3 package GOOS=darwin GOARCH=arm64
	@./scripts/sign-macos.sh

sign: ## Sign macOS app (run after build to preserve permissions)
	@./scripts/sign-macos.sh

build-windows: check-wails ## Build for Windows (fasp.exe and the NSIS installer; the installer installs WebView2 if missing; needs makensis)
	@wails3 package GOOS=windows GOARCH=amd64

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

