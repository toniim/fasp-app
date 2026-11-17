#!/bin/bash
# Print the app version from internal/version/version.go (e.g. v26.11.3) and
# write it, without the "v", into the build files that carry a version: the
# macOS Info.plist, the Windows file/installer metadata and manifest, and
# build/config.yml. Without it they say 1.0.0.
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
const version = process.argv[1];

// Rewrites one file; a pattern that matches nothing is an error, so a file
// layout change cannot silently leave the old version behind.
function edit(file, fn) {
  const before = fs.readFileSync(file, "utf8");
  const after = fn(before);
  if (after === before && !before.includes(version)) {
    throw new Error(`no version found in ${file}`);
  }
  fs.writeFileSync(file, after);
}

const plistKey = (key) => new RegExp(`(<key>${key}</key>\\s*<string>)[^<]*(</string>)`);
for (const plist of ["build/darwin/Info.plist", "build/darwin/Info.dev.plist"]) {
  edit(plist, (s) => s
    .replace(plistKey("CFBundleVersion"), `$1${version}$2`)
    .replace(plistKey("CFBundleShortVersionString"), `$1${version}$2`));
}

edit("build/windows/info.json", (s) => {
  const cfg = JSON.parse(s);
  cfg.fixed.file_version = version;
  cfg.fixed.product_version = version;
  for (const lang of Object.values(cfg.info)) {
    lang.FileVersion = version;
    lang.ProductVersion = version;
  }
  return JSON.stringify(cfg, null, "\t");
});

edit("build/windows/wails.exe.manifest", (s) =>
  s.replace(/(<assemblyIdentity type="win32" name="com\.wails\.fasp" version=")[^"]*(")/, `$1${version}$2`));

edit("build/windows/nsis/wails_tools.nsh", (s) =>
  s.replace(/(!define INFO_PRODUCTVERSION ")[^"]*(")/, `$1${version}$2`));

edit("build/config.yml", (s) => s.replace(/(^\s*version: )"[^"]*"/m, `$1"${version}"`));
' "${VERSION#v}"

echo "$VERSION"
