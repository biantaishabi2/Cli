#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "用法: niuma_state_label.sh <issue> <bot:state> [from_state]" >&2
  exit 1
fi

ISSUE_NUMBER="$1"
TARGET_STATE="$2"
FROM_STATE="${3:-}"
REPO="${REPO:-${NIUMA_REPO:-}}"

if [[ -z "$REPO" ]]; then
  echo "请设置 REPO 或 NIUMA_REPO（owner/repo）" >&2
  exit 1
fi

# 默认走 CAS：未显式传 from 时，自动读取当前 bot:* 作为前置状态。
resolve_current_bot_state() {
  gh issue view "$ISSUE_NUMBER" --repo "$REPO" --json labels \
    | jq -r '[.labels[].name | select(startswith("bot:"))][0] // ""'
}

if [[ -z "$FROM_STATE" ]]; then
  if command -v gh >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
    FROM_STATE="$(resolve_current_bot_state || true)"
    if [[ -n "$FROM_STATE" ]]; then
      echo "[niuma_state_label] auto-detected from=$FROM_STATE (CAS)" >&2
    else
      echo "[niuma_state_label] 未检测到当前 bot 状态，将执行无前置迁移（仅建议用于 bootstrap）" >&2
    fi
  else
    echo "[niuma_state_label] 缺少 gh/jq，无法自动推断 from；将执行无前置迁移" >&2
  fi
fi

NIUMA_BIN="${NIUMA_BIN:-niuma}"
ARGS=(state-label set --repo "$REPO" --issue "$ISSUE_NUMBER" --to "$TARGET_STATE")
if [[ -n "$FROM_STATE" ]]; then
  ARGS+=(--from "$FROM_STATE")
fi

"$NIUMA_BIN" "${ARGS[@]}"
