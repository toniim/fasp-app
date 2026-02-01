#!/bin/bash
# Sign macOS app to preserve screen recording permissions
# Usage: ./scripts/sign-macos.sh [certificate-name]
#
# Without certificate: uses ad-hoc signing (works locally)
# With certificate: uses specified certificate (for distribution)

set -e

APP_PATH="build/bin/grabix.app"
ENTITLEMENTS="build/darwin/entitlements.plist"
CERT_NAME="${1:-${SIGN_CERT:--}}"  # Arg > env var > ad-hoc

if [ ! -d "$APP_PATH" ]; then
    echo "Error: App not found at $APP_PATH"
    echo "Run 'wails build' first"
    exit 1
fi

echo "Signing grabix.app..."
echo "  Certificate: ${CERT_NAME}"
echo "  Entitlements: ${ENTITLEMENTS}"

# Remove existing signature
codesign --remove-signature "$APP_PATH" 2>/dev/null || true

# Sign the app bundle (deep signs all nested code)
codesign --force --deep --sign "$CERT_NAME" \
    --entitlements "$ENTITLEMENTS" \
    --options runtime \
    "$APP_PATH"

# Verify signature
echo ""
echo "Verifying signature..."
codesign --verify --verbose=2 "$APP_PATH"

echo ""
echo "Done! App signed successfully."
echo ""
echo "NOTE: After first run, grant Screen Recording permission in:"
echo "  System Settings → Privacy & Security → Screen Recording"
echo ""
echo "The permission will persist across rebuilds as long as you:"
echo "  1. Use the same signing method"
echo "  2. Don't change the bundle identifier"
