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

NIUMA_BIN="${NIUMA_BIN:-niuma}"
ARGS=(state-label set --repo "$REPO" --issue "$ISSUE_NUMBER" --to "$TARGET_STATE")
if [[ -n "$FROM_STATE" ]]; then
  ARGS+=(--from "$FROM_STATE")
fi

"$NIUMA_BIN" "${ARGS[@]}"
