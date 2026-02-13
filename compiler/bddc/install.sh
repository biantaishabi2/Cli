#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
安装 bdd_compiler 到本机（默认 ~/.local/bin）

用法:
  ./tools/bdd_compiler/install.sh
  ./tools/bdd_compiler/install.sh --rebuild
  ./tools/bdd_compiler/install.sh --prefix ~/.local/bin
  ./tools/bdd_compiler/install.sh --link
  ./tools/bdd_compiler/install.sh --no-alias

参数:
  --prefix DIR   安装目录（默认: ~/.local/bin）
  --link         使用 symlink 安装（默认: copy）
  --rebuild      安装前强制执行 mix escript.build
  --no-alias     不创建短别名（默认会创建 bddc -> bdd_compiler）
  -h, --help     显示帮助

说明:
  - 该工具是 escript，可执行时需要 Erlang/OTP 提供的 escript 运行环境。
  - 如果你的 PATH 没包含 ~/.local/bin，可在 shell 配置里加:
      export PATH="$HOME/.local/bin:$PATH"
EOF
}

PREFIX="${HOME}/.local/bin"
MODE="copy"
REBUILD="0"
CREATE_ALIAS="1"
ALIAS_NAME="bddc"

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
    --no-alias)
      CREATE_ALIAS="0"
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
TOOL_DIR="$SCRIPT_DIR"
BIN_SRC="$TOOL_DIR/bdd_compiler"
BIN_DST="$PREFIX/bdd_compiler"

mkdir -p "$PREFIX"

if [[ "$REBUILD" == "1" ]] || [[ ! -f "$BIN_SRC" ]]; then
  (
    cd "$TOOL_DIR"
    mix deps.get >/dev/null
    mix escript.build
  )
fi

if [[ ! -f "$BIN_SRC" ]]; then
  echo "未找到可执行文件: $BIN_SRC (build failed?)" >&2
  exit 1
fi

case "$MODE" in
  copy)
    cp -f "$BIN_SRC" "$BIN_DST"
    chmod +x "$BIN_DST"
    ;;
  link)
    ln -sfn "$BIN_SRC" "$BIN_DST"
    ;;
  *)
    echo "未知安装模式: $MODE" >&2
    exit 2
    ;;
esac

echo "已安装: $BIN_DST"
echo "验证: $BIN_DST help"

if [[ "$CREATE_ALIAS" == "1" ]]; then
  ALIAS_DST="$PREFIX/$ALIAS_NAME"
  ln -sfn "$BIN_DST" "$ALIAS_DST"
  echo "已创建别名: $ALIAS_DST -> $BIN_DST"
  echo "验证: $ALIAS_NAME --help"
fi
