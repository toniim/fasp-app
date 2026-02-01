#!/bin/bash
# Create self-signed certificate for development code signing
# This certificate allows screen recording permissions to persist across rebuilds
#
# Usage: ./scripts/create-dev-certificate.sh
#
# After running, use: ./scripts/sign-macos.sh "Grabix Dev"

set -e

CERT_NAME="Grabix Dev"

echo "Creating self-signed certificate: '$CERT_NAME'"
echo ""

# Check if certificate already exists
if security find-certificate -c "$CERT_NAME" ~/Library/Keychains/login.keychain-db >/dev/null 2>&1; then
    echo "Certificate '$CERT_NAME' already exists."
    echo "To recreate, delete it first from Keychain Access."
    exit 0
fi

# Create certificate using Keychain Access
# Note: This opens a dialog for user confirmation
cat << 'EOF'
Opening Keychain Access to create certificate...

In the Certificate Assistant:
  1. Name: Grabix Dev
  2. Identity Type: Self Signed Root
  3. Certificate Type: Code Signing
  4. Click "Create"
  5. When prompted, always trust this certificate

EOF

open -a "Keychain Access"

echo ""
echo "After creating the certificate, sign your app with:"
echo "  ./scripts/sign-macos.sh \"$CERT_NAME\""
echo ""
echo "Or add to Makefile:"
echo "  SIGN_CERT=\"$CERT_NAME\" make sign"
