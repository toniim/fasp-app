#!/bin/bash
# Package build/bin/fasp.app (from `wails build -platform darwin/universal`)
# into a zip for distribution without an Apple Developer ID.
#
# Usage: ./scripts/package-macos.sh [output.zip]
#
# The app is signed ad-hoc: a universal binary has to carry a signature to run
# on Apple Silicon at all, and without a Developer ID ad-hoc is all we have.
# Users therefore have to clear Gatekeeper once (see README, "Install > macOS").
set -euo pipefail

APP="build/bin/fasp.app"
OUT="${1:-build/bin/fasp-macos-universal.zip}"
BIN="$APP/Contents/MacOS/fasp"

if [ ! -d "$APP" ]; then
    echo "Error: $APP not found. Run 'wails build -platform darwin/universal' first." >&2
    exit 1
fi

# Both architectures, or Intel / Apple Silicon users get a broken download.
archs=$(lipo -archs "$BIN")
echo "Architectures: $archs"
for arch in x86_64 arm64; do
    case " $archs " in
        *" $arch "*) ;;
        *) echo "Error: $BIN lacks $arch" >&2; exit 1 ;;
    esac
done

"$(dirname "$0")/sign-macos.sh" -
codesign --verify --deep --strict --verbose=2 "$APP"

# ditto keeps the bundle's symlinks and metadata intact, unlike plain zip.
rm -f "$OUT"
ditto -c -k --sequesterRsrc --keepParent "$APP" "$OUT"
echo "Packaged $OUT ($(du -h "$OUT" | cut -f1))"
