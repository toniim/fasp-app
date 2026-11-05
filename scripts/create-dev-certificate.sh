#!/bin/bash
# Create a self-signed code-signing identity for local development.
#
# Why: an ad-hoc signature ("codesign -s -") makes the designated requirement
# the binary's cdhash, which changes on every build, so macOS drops the Screen
# Recording grant after each rebuild. Signing with a fixed certificate makes the
# requirement "identifier + certificate", which stays the same across rebuilds.
#
# Usage: ./scripts/create-dev-certificate.sh
# Then:  make build-darwin-signed   (sign-macos.sh picks the identity up)

set -euo pipefail

CERT_NAME="${1:-Fasp Dev}"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
# LibreSSL ships with macOS and writes PKCS#12 files that `security import` reads.
OPENSSL=/usr/bin/openssl

if security find-identity -p codesigning "$KEYCHAIN" | grep -q "\"$CERT_NAME\""; then
    echo "Identity '$CERT_NAME' already exists in the login keychain."
    exit 0
fi

WORK="$(mktemp -d)"
trap '/bin/rm -rf "$WORK"' EXIT
PASS="$($OPENSSL rand -hex 16)"

cat >"$WORK/cert.cnf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $CERT_NAME
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
EOF

"$OPENSSL" req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout "$WORK/key.pem" -out "$WORK/cert.pem" -config "$WORK/cert.cnf" 2>/dev/null
"$OPENSSL" pkcs12 -export -inkey "$WORK/key.pem" -in "$WORK/cert.pem" \
    -name "$CERT_NAME" -out "$WORK/id.p12" -passout "pass:$PASS"

# -T lets codesign use the private key without a keychain prompt.
security import "$WORK/id.p12" -k "$KEYCHAIN" -P "$PASS" -T /usr/bin/codesign

echo ""
echo "Created identity '$CERT_NAME' (valid 10 years)."
echo "It shows as CSSMERR_TP_NOT_TRUSTED in 'security find-identity'; codesign"
echo "can still sign with it, which is all local builds need."
