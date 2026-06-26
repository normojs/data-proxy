#!/usr/bin/env sh
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 4 ]; then
  echo "usage: $0 <dist-dir> <version> [repo] [output]" >&2
  exit 2
fi

DIST_DIR="${1%/}"
VERSION="$2"
REPO="${3:-${GITHUB_REPOSITORY:-normojs/data-proxy}}"
OUTPUT="${4:-$DIST_DIR/data-proxy-agent-manifest.json}"
BASE_URL="${DATA_PROXY_AGENT_RELEASE_BASE_URL:-https://github.com/${REPO}/releases/download/${VERSION}}"

if [ ! -d "$DIST_DIR" ]; then
  echo "dist dir does not exist: $DIST_DIR" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "version is required" >&2
  exit 1
fi

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

checksum_for() {
  checksum_file="$DIST_DIR/$1.sha256"
  if [ ! -f "$checksum_file" ]; then
    echo "missing checksum: $checksum_file" >&2
    exit 1
  fi
  checksum="$(awk '{print $1}' "$checksum_file" | head -n 1)"
  if [ "${#checksum}" -ne 64 ]; then
    echo "invalid sha256 length for $1: $checksum" >&2
    exit 1
  fi
  case "$checksum" in
    *[!0123456789abcdefABCDEF]*)
      echo "invalid sha256 for $1: $checksum" >&2
      exit 1
      ;;
  esac
  printf '%s' "$checksum" | tr '[:upper:]' '[:lower:]'
}

TMP_OUTPUT="${OUTPUT}.tmp"
mkdir -p "$(dirname "$OUTPUT")"

{
  printf '{\n'
  printf '  "version": "%s",\n' "$(json_escape "$VERSION")"
  printf '  "assets": [\n'
} > "$TMP_OUTPUT"

COUNT=0

add_asset() {
  os="$1"
  arch="$2"
  ext="$3"
  preferred="$4"
  fallback="$5"

  name=""
  if [ -n "$preferred" ] && [ -f "$DIST_DIR/$preferred" ]; then
    name="$preferred"
  elif [ -f "$DIST_DIR/$fallback" ]; then
    name="$fallback"
  else
    return 0
  fi

  sha256="$(checksum_for "$name")"
  url="${BASE_URL%/}/$name"

  if [ "$COUNT" -gt 0 ]; then
    printf ',\n' >> "$TMP_OUTPUT"
  fi

  {
    printf '    {\n'
    printf '      "name": "%s",\n' "$(json_escape "$name")"
    printf '      "url": "%s",\n' "$(json_escape "$url")"
    printf '      "os": "%s",\n' "$(json_escape "$os")"
    printf '      "arch": "%s",\n' "$(json_escape "$arch")"
    printf '      "sha256": "%s"\n' "$sha256"
    printf '    }'
  } >> "$TMP_OUTPUT"

  COUNT=$((COUNT + 1))
}

add_asset linux amd64 tar.gz "" "data-proxy-agent-${VERSION}-linux-amd64.tar.gz"
add_asset linux arm64 tar.gz "" "data-proxy-agent-${VERSION}-linux-arm64.tar.gz"
add_asset darwin amd64 tar.gz "data-proxy-agent-${VERSION}-darwin-amd64-notarized.tar.gz" "data-proxy-agent-${VERSION}-darwin-amd64.tar.gz"
add_asset darwin arm64 tar.gz "data-proxy-agent-${VERSION}-darwin-arm64-notarized.tar.gz" "data-proxy-agent-${VERSION}-darwin-arm64.tar.gz"
add_asset windows amd64 zip "" "data-proxy-agent-${VERSION}-windows-amd64.zip"
add_asset windows arm64 zip "" "data-proxy-agent-${VERSION}-windows-arm64.zip"

if [ "$COUNT" -eq 0 ]; then
  rm -f "$TMP_OUTPUT"
  echo "no data-proxy-agent archives found in $DIST_DIR for $VERSION" >&2
  exit 1
fi

{
  printf '\n'
  printf '  ]\n'
  printf '}\n'
} >> "$TMP_OUTPUT"

mv "$TMP_OUTPUT" "$OUTPUT"
echo "wrote $OUTPUT with $COUNT assets"
