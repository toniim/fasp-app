#!/bin/bash
# Store the "Fasp Dev" signing identity in the GitHub secrets the Release
# workflow signs the macOS app with: MACOS_CERT_P12 (base64 PKCS#12) and
# MACOS_CERT_PASSWORD.
#
# Signing releases with the same certificate as local builds keeps the
# Screen Recording grant across updates, and between a release and a local
# build: macOS ties the grant to "identifier + certificate" instead of the
# hash of one build.
#
# Run it once, and again whenever the certificate is replaced. macOS asks for
# the login keychain password to allow the export. Needs `gh` logged in with
# admin rights on the repository.
#
# Usage: ./scripts/upload-signing-cert.sh [identity-name]

set -euo pipefail

CERT_NAME="${1:-Fasp Dev}"
REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"

WORK="$(mktemp -d)"
trap '/bin/rm -rf "$WORK"' EXIT

P12_PASSWORD="$(/usr/bin/openssl rand -hex 16)"
export P12_PASSWORD

swift "$(dirname "$0")/export-signing-cert.swift" "$CERT_NAME" "$WORK/identity.p12"

# Values go through stdin, so they never show up in the process list or output.
base64 -i "$WORK/identity.p12" | gh secret set MACOS_CERT_P12 --repo "$REPO"
printf '%s' "$P12_PASSWORD" | gh secret set MACOS_CERT_PASSWORD --repo "$REPO"

echo "Stored identity '$CERT_NAME' in MACOS_CERT_P12 and MACOS_CERT_PASSWORD on $REPO."
