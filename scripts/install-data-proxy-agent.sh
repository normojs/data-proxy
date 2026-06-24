#!/usr/bin/env sh
set -eu

REPO="${DATA_PROXY_AGENT_REPO:-normojs/data-proxy}"
VERSION="${DATA_PROXY_AGENT_VERSION:-latest}"
INSTALL_DIR="${DATA_PROXY_AGENT_INSTALL_DIR:-}"
GITHUB_API="${DATA_PROXY_AGENT_GITHUB_API:-https://api.github.com}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
    return
  fi
  echo "missing required command: curl or wget" >&2
  exit 1
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) echo "unsupported os: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
}

resolve_latest_version() {
  tmp_json="$(mktemp)"
  download "${GITHUB_API%/}/repos/${REPO}/releases/latest" "$tmp_json"
  tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp_json" | head -n 1)"
  rm -f "$tmp_json"
  if [ -z "$tag" ]; then
    echo "failed to resolve latest data-proxy-agent release tag" >&2
    exit 1
  fi
  echo "$tag"
}

verify_sha256() {
  file="$1"
  checksum_file="$2"
  expected="$(awk '{print $1}' "$checksum_file")"
  if [ -z "$expected" ]; then
    echo "empty checksum file: $checksum_file" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  else
    echo "missing required command: sha256sum or shasum" >&2
    exit 1
  fi
  if [ "$actual" != "$expected" ]; then
    echo "sha256 mismatch for $file" >&2
    echo "expected: $expected" >&2
    echo "actual:   $actual" >&2
    exit 1
  fi
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_version)"
fi

if [ "$OS" = "windows" ]; then
  EXT="zip"
  BINARY="dpa.exe"
  LEGACY_BINARY="data-proxy-agent.exe"
else
  EXT="tar.gz"
  BINARY="dpa"
  LEGACY_BINARY="data-proxy-agent"
fi

ASSET="data-proxy-agent-${VERSION}-${OS}-${ARCH}.${EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

ARCHIVE="$TMP_DIR/$ASSET"
CHECKSUM="$TMP_DIR/$ASSET.sha256"
download "$BASE_URL/$ASSET" "$ARCHIVE"
download "$BASE_URL/$ASSET.sha256" "$CHECKSUM"
verify_sha256 "$ARCHIVE" "$CHECKSUM"

EXTRACT_DIR="$TMP_DIR/extract"
mkdir -p "$EXTRACT_DIR"
case "$EXT" in
  zip)
    need_cmd unzip
    unzip -q "$ARCHIVE" -d "$EXTRACT_DIR"
    ;;
  tar.gz)
    tar -xzf "$ARCHIVE" -C "$EXTRACT_DIR"
    ;;
esac

FOUND="$(find "$EXTRACT_DIR" -type f -name "$BINARY" | head -n 1)"
if [ -z "$FOUND" ]; then
  FOUND="$(find "$EXTRACT_DIR" -type f -name "$LEGACY_BINARY" | head -n 1)"
  if [ -z "$FOUND" ]; then
    echo "$BINARY or $LEGACY_BINARY not found in $ASSET" >&2
    exit 1
  fi
fi

if [ -z "$INSTALL_DIR" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"
TARGET="$INSTALL_DIR/$BINARY"
cp "$FOUND" "$TARGET"
chmod 755 "$TARGET"
LEGACY_TARGET="$INSTALL_DIR/$LEGACY_BINARY"
if [ "$OS" = "windows" ]; then
  cp "$TARGET" "$LEGACY_TARGET"
  chmod 755 "$LEGACY_TARGET"
else
  rm -f "$LEGACY_TARGET"
  ln -s "$BINARY" "$LEGACY_TARGET"
fi

"$TARGET" self-test
echo "dpa installed: $TARGET"
echo "compatibility command installed: $LEGACY_TARGET"
echo "version: $VERSION"
