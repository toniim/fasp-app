#!/bin/bash
# Sign the macOS app so the Screen Recording grant survives rebuilds.
# Usage: ./scripts/sign-macos.sh [certificate-name]
#
# Identity, in order: argument > $SIGN_CERT > "Fasp Dev" (if it exists in the
# keychain, see create-dev-certificate.sh) > ad-hoc.
#
# Ad-hoc signing ties the grant to the binary's cdhash, so every rebuild loses
# it. A fixed certificate ties it to "identifier + certificate" instead.

set -e

APP_PATH="build/bin/fasp.app"
ENTITLEMENTS="build/darwin/entitlements.plist"
DEV_CERT="Fasp Dev"

CERT_NAME="${1:-${SIGN_CERT:-}}"
if [ -z "$CERT_NAME" ]; then
    if security find-identity -p codesigning | grep -q "\"$DEV_CERT\""; then
        CERT_NAME="$DEV_CERT"
    else
        CERT_NAME="-"
    fi
fi

if [ ! -d "$APP_PATH" ]; then
    echo "Error: App not found at $APP_PATH"
    echo "Run 'wails build' first"
    exit 1
fi

echo "Signing fasp.app..."
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
echo "  then quit and reopen the app (macOS only applies it on launch)."
echo ""
if [ "$CERT_NAME" = "-" ]; then
    echo "WARNING: ad-hoc signature. The grant is tied to this exact build and is"
    echo "lost on the next rebuild. Run ./scripts/create-dev-certificate.sh once"
    echo "to sign with a fixed identity instead."
else
    echo "The grant persists across rebuilds while you keep signing with"
    echo "'$CERT_NAME' and the bundle identifier stays the same."
fi
