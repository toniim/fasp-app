#!/bin/bash
# Smoke-test the packaged macOS app the way a user gets it: unzip
# fasp-macos-universal.zip, check the signature survived, launch the app and
# require it to still be running after a few seconds.
#
# Usage: ./scripts/smoke-macos.sh [fasp-macos-universal.zip]
set -euo pipefail

ZIP="${1:-build/bin/fasp-macos-universal.zip}"
WAIT=10

dir=$(mktemp -d)
log="$dir/fasp.log"
trap 'rm -rf "$dir"' EXIT

ditto -x -k "$ZIP" "$dir"
app="$dir/fasp.app"
codesign --verify --deep --strict "$app"
echo "Signature OK after unzip"

"$app/Contents/MacOS/fasp" >"$log" 2>&1 &
pid=$!
sleep "$WAIT"

if kill -0 "$pid" 2>/dev/null; then
    echo "fasp still running after ${WAIT}s"
    kill "$pid"
    wait "$pid" 2>/dev/null || true
    echo "--- app output ---"; cat "$log"
    exit 0
fi

status=0
wait "$pid" || status=$?
echo "fasp exited within ${WAIT}s (status $status)"
echo "--- app output ---"; cat "$log"
exit 1
