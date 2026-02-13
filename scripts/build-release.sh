#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-v0.1.0}"

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

echo "Building taskctl ${VERSION} for ${TARGET}"

cargo build --release --manifest-path "$ROOT_DIR/taskctl/Cargo.toml"

mkdir -p "$ROOT_DIR/dist"
cp "$ROOT_DIR/taskctl/target/release/taskctl" "$ROOT_DIR/dist/taskctl"
chmod +x "$ROOT_DIR/dist/taskctl"

ASSET="taskctl-${VERSION}-${TARGET}.tar.gz"
tar -C "$ROOT_DIR/dist" -czf "$ROOT_DIR/dist/$ASSET" taskctl

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$ROOT_DIR/dist/$ASSET" > "$ROOT_DIR/dist/$ASSET.sha256"
else
  shasum -a 256 "$ROOT_DIR/dist/$ASSET" > "$ROOT_DIR/dist/$ASSET.sha256"
fi

echo "Created: $ROOT_DIR/dist/$ASSET"
echo "Created: $ROOT_DIR/dist/$ASSET.sha256"
