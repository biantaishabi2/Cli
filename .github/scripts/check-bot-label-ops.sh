#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

TARGET_DIRS=(
  ".github/workflows"
  ".github/scripts"
)

if [ -d "automation/niuma/scripts" ]; then
  TARGET_DIRS+=("automation/niuma/scripts")
fi

# 禁止在自动化脚本/workflow 中直接通过 gh 修改 bot:* 状态标签。
# 允许通过 niuma state-label 进行受控状态迁移。
PATTERN='(--add-label(?:=|\s+)[^#\n]*bot:|--remove-label(?:=|\s+)[^#\n]*bot:)'

MATCHES="$(
  rg -n --pcre2 "$PATTERN" "${TARGET_DIRS[@]}" \
    --glob '*.yml' --glob '*.yaml' --glob '*.sh' \
    --glob '!.github/scripts/check-bot-label-ops.sh' \
    --glob '!automation/niuma/scripts/gh' || true
)"

if [ -n "$MATCHES" ]; then
  echo "ERROR: 检测到直接修改 bot:* 标签的命令（禁止）。"
  echo
  echo "$MATCHES"
  echo
  echo "请改用受控命令："
  echo "  niuma state-label set --repo <owner/repo> --issue <num> --from <bot:current> --to <bot:state>"
  echo
  echo "说明：gh issue edit --add-label/--remove-label bot:* 会破坏状态机一致性。"
  exit 1
fi

echo "OK: 未发现直接修改 bot:* 标签的命令。"
