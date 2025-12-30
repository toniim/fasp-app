#!/bin/bash

# Build script for Grabix with version info
# Usage: ./scripts/build.sh [platform]
#   platform: darwin/arm64, darwin/amd64, darwin/universal (default)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get platform from argument or default to universal
PLATFORM="${1:-darwin/universal}"

echo -e "${GREEN}Building Grabix for ${PLATFORM}...${NC}"

# Get version from version.go
VERSION=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
echo -e "${YELLOW}Version: ${VERSION}${NC}"

# Get git commit
if git rev-parse --git-dir > /dev/null 2>&1; then
    GIT_COMMIT=$(git rev-parse HEAD)
    echo -e "${YELLOW}Git Commit: ${GIT_COMMIT:0:7}${NC}"
else
    GIT_COMMIT="unknown"
    echo -e "${YELLOW}Git Commit: ${GIT_COMMIT}${NC}"
fi

# Get build time
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')
echo -e "${YELLOW}Build Time: ${BUILD_TIME}${NC}"

# Build with wails
echo -e "${GREEN}Running wails build...${NC}"
wails build -platform "${PLATFORM}" -ldflags "\
  -X 'github.com/heytonyne/grabix/internal/version.Version=${VERSION}' \
  -X 'github.com/heytonyne/grabix/internal/version.BuildTime=${BUILD_TIME}' \
  -X 'github.com/heytonyne/grabix/internal/version.GitCommit=${GIT_COMMIT}' \
"

echo -e "${GREEN}Build complete!${NC}"
echo -e "${YELLOW}Binary location: build/bin/grabix.app${NC}"

# Show architecture info
if [ -f "build/bin/grabix.app/Contents/MacOS/grabix" ]; then
    echo -e "${YELLOW}Architecture:${NC}"
    file build/bin/grabix.app/Contents/MacOS/grabix
fi

