#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./scripts/install.sh v0.1.0
# Env override:
#   REPO=biantaishabi2/Cli INSTALL_DIR=$HOME/.local/bin ./scripts/install.sh v0.1.0

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version-tag> (e.g. v0.1.0)" >&2
  exit 1
fi

REPO="${REPO:-biantaishabi2/Cli}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"

case "$ARCH_RAW" in
  x86_64|amd64)
    ARCH="x86_64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported arch: $ARCH_RAW" >&2
    exit 1
    ;;
esac

case "$OS" in
  darwin)
    TARGET="darwin-${ARCH}"
    ;;
  linux)
    TARGET="linux-${ARCH}"
    ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

ASSET="taskctl-${VERSION}-${TARGET}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${ASSET}"
CHECKSUM_URL="${BASE_URL}/${ASSET}.sha256"

echo "Downloading ${ARCHIVE_URL}"
curl -fsSL "$ARCHIVE_URL" -o "$TMP_DIR/$ASSET"
echo "Downloading ${CHECKSUM_URL}"
curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/$ASSET.sha256"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TMP_DIR" && sha256sum -c "$ASSET.sha256")
else
  EXPECTED="$(awk '{print $1}' "$TMP_DIR/$ASSET.sha256")"
  ACTUAL="$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}')"
  if [[ "$EXPECTED" != "$ACTUAL" ]]; then
    echo "Checksum mismatch" >&2
    exit 1
  fi
fi

mkdir -p "$INSTALL_DIR"
tar -C "$TMP_DIR" -xzf "$TMP_DIR/$ASSET"
install -m 755 "$TMP_DIR/taskctl" "$INSTALL_DIR/taskctl"

echo "Installed to $INSTALL_DIR/taskctl"
"$INSTALL_DIR/taskctl" --help >/dev/null
echo "taskctl is ready"
