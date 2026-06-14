#!/bin/bash

# Release script for Fasp.
# Tags the current commit with the version from internal/version/version.go
# (the single source of truth) and pushes the tag.
#
# Usage: ./scripts/release.sh
# Bump internal/version/version.go and commit BEFORE running this.

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Read the version from version.go
VERSION=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo -e "${RED}Could not read Version from internal/version/version.go${NC}"
  exit 1
fi

echo -e "${GREEN}Releasing ${VERSION}${NC}"

# Refuse if the working tree is dirty
if [ -n "$(git status --porcelain)" ]; then
  echo -e "${RED}Working tree not clean — commit your changes first.${NC}"
  exit 1
fi

# Refuse if the tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo -e "${RED}Tag ${VERSION} already exists — bump internal/version/version.go first.${NC}"
  exit 1
fi

git tag "$VERSION"
git push origin "$VERSION"

echo -e "${GREEN}Tagged and pushed ${VERSION}${NC}"
echo -e "${YELLOW}Next: create the GitHub release for ${VERSION} and attach the built binaries.${NC}"
