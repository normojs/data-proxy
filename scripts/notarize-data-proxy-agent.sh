#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <archive.tar.gz> <version> <arch> <output-dir>" >&2
  exit 2
fi

ARCHIVE="$1"
VERSION="$2"
ARCH="$3"
OUTPUT_DIR="$4"

required_env=(
  APPLE_ID
  APPLE_TEAM_ID
  APPLE_APP_SPECIFIC_PASSWORD
  APPLE_SIGNING_IDENTITY
  APPLE_CERTIFICATE_P12_BASE64
  APPLE_CERTIFICATE_PASSWORD
)

for name in "${required_env[@]}"; do
  if [ -z "${!name:-}" ]; then
    echo "missing $name; skipping notarization" >&2
    exit 0
  fi
done

WORKDIR="$(mktemp -d)"
KEYCHAIN="$WORKDIR/notary.keychain-db"
trap 'security delete-keychain "$KEYCHAIN" >/dev/null 2>&1 || true; rm -rf "$WORKDIR"' EXIT

mkdir -p "$OUTPUT_DIR" "$WORKDIR/extract"
tar -xzf "$ARCHIVE" -C "$WORKDIR/extract"
DPA_BIN="$(find "$WORKDIR/extract" -type f -name dpa | head -n 1)"
LEGACY_AGENT_BIN="$(find "$WORKDIR/extract" -type f -name data-proxy-agent | head -n 1)"
AGENT_BIN="${DPA_BIN:-$LEGACY_AGENT_BIN}"
if [ -z "$AGENT_BIN" ]; then
  echo "dpa or data-proxy-agent binary not found in $ARCHIVE" >&2
  exit 1
fi

security create-keychain -p "" "$KEYCHAIN"
security set-keychain-settings -lut 21600 "$KEYCHAIN"
security unlock-keychain -p "" "$KEYCHAIN"
echo "$APPLE_CERTIFICATE_P12_BASE64" | base64 --decode > "$WORKDIR/cert.p12"
security import "$WORKDIR/cert.p12" -k "$KEYCHAIN" -P "$APPLE_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
security list-keychains -d user -s "$KEYCHAIN" $(security list-keychains -d user | sed 's/"//g')
security set-key-partition-list -S apple-tool:,apple: -s -k "" "$KEYCHAIN"

for bin in "$DPA_BIN" "$LEGACY_AGENT_BIN"; do
  if [ -n "$bin" ]; then
    codesign --force --timestamp --options runtime --sign "$APPLE_SIGNING_IDENTITY" "$bin"
    codesign --verify --strict --verbose=2 "$bin"
  fi
done

NOTARY_ZIP="$WORKDIR/data-proxy-agent-${VERSION}-darwin-${ARCH}-notary.zip"
ditto -c -k "$WORKDIR/extract" "$NOTARY_ZIP"
xcrun notarytool submit "$NOTARY_ZIP" \
  --apple-id "$APPLE_ID" \
  --team-id "$APPLE_TEAM_ID" \
  --password "$APPLE_APP_SPECIFIC_PASSWORD" \
  --wait

for bin in "$DPA_BIN" "$LEGACY_AGENT_BIN"; do
  if [ -n "$bin" ]; then
    xcrun stapler staple "$bin" || true
    codesign --verify --strict --verbose=2 "$bin"
  fi
done

OUT_ARCHIVE="$OUTPUT_DIR/data-proxy-agent-${VERSION}-darwin-${ARCH}-notarized.tar.gz"
tar -czf "$OUT_ARCHIVE" -C "$WORKDIR/extract" .
shasum -a 256 "$OUT_ARCHIVE" > "$OUT_ARCHIVE.sha256"
echo "notarized archive: $OUT_ARCHIVE"
