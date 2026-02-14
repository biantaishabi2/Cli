#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
安装 bcc 到本机（默认 ~/.local/bin）

用法:
  ./compiler/bcc/install.sh
  ./compiler/bcc/install.sh --rebuild
  ./compiler/bcc/install.sh --prefix ~/.local/bin
  ./compiler/bcc/install.sh --link

参数:
  --prefix DIR   安装目录（默认: ~/.local/bin）
  --link         使用 symlink 安装（默认: copy）
  --rebuild      安装前强制执行 cargo build --release
  -h, --help     显示帮助

说明:
  - bcc 是 Rust 编译的原生二进制，无运行时依赖
  - 如果你的 PATH 没包含 ~/.local/bin，可在 shell 配置里加:
      export PATH="$HOME/.local/bin:$PATH"
EOF
}

PREFIX="${HOME}/.local/bin"
MODE="copy"
REBUILD="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      PREFIX="${2:-}"
      shift 2
      ;;
    --link)
      MODE="link"
      shift
      ;;
    --rebuild)
      REBUILD="1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN_SRC="$WORKSPACE_DIR/target/release/bcc"

mkdir -p "$PREFIX"

if [[ "$REBUILD" == "1" ]] || [[ ! -f "$BIN_SRC" ]]; then
  echo "编译 bcc (release)..."
  (cd "$WORKSPACE_DIR" && cargo build --release -p bcc)
fi

if [[ ! -f "$BIN_SRC" ]]; then
  echo "未找到可执行文件: $BIN_SRC (build failed?)" >&2
  exit 1
fi

BIN_DST="$PREFIX/bcc"

case "$MODE" in
  copy)
    cp -f "$BIN_SRC" "$BIN_DST"
    chmod +x "$BIN_DST"
    ;;
  link)
    ln -sfn "$BIN_SRC" "$BIN_DST"
    ;;
esac

echo "已安装: $BIN_DST"
"$BIN_DST" --version
echo "验证: bcc --help"
