#!/bin/bash
# Print the app version from internal/version/version.go (e.g. v26.11.3) and
# write it, without the "v", into wails.json as info.productVersion. Wails puts
# that value in the Windows file/installer metadata and in the macOS
# Info.plist, which otherwise say 1.0.0.
#
# Usage: VERSION=$(./scripts/stamp-version.sh)
set -euo pipefail

VERSION=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Could not read Version from internal/version/version.go" >&2
    exit 1
fi

node -e '
const fs = require("fs");
const cfg = JSON.parse(fs.readFileSync("wails.json", "utf8"));
cfg.info = { ...(cfg.info || {}), productVersion: process.argv[1] };
fs.writeFileSync("wails.json", JSON.stringify(cfg, null, 2) + "\n");
' "${VERSION#v}"

echo "$VERSION"
