#!/usr/bin/env bash
set -euo pipefail

# 一键发布 workflow 到目标仓库（通过 GitHub Contents API）
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SOURCE_DIR="$ROOT_DIR/.github/workflows"
GITHUB_SCRIPTS_DIR="$ROOT_DIR/.github/scripts"

REPO=""
BRANCH=""
MODE="entry"
MESSAGE="chore: sync niuma workflows from Cli"
SKIP_VERIFY=0
MEN_TARGET_REPO="biantaishabi2/men"
PUBLISHED_PAIRS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --branch)
      BRANCH="${2:-}"
      shift 2
      ;;
    --message)
      MESSAGE="${2:-}"
      shift 2
      ;;
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --source-dir)
      SOURCE_DIR="${2:-}"
      shift 2
      ;;
    --skip-verify)
      SKIP_VERIFY=1
      shift
      ;;
    *)
      echo "unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$REPO" ]]; then
  echo "usage: $0 --repo <owner/repo> [--mode entry|full] [--branch <name>] [--message <msg>] [--source-dir <dir>] [--skip-verify]" >&2
  exit 1
fi

if [[ "$MODE" != "entry" && "$MODE" != "full" ]]; then
  echo "invalid --mode: $MODE (expected: entry|full)" >&2
  exit 1
fi

if [[ "$REPO" == "$MEN_TARGET_REPO" && "$MODE" != "full" ]]; then
  echo "men 回归仓库必须使用 --mode full，确保 reusable workflow 修复已同步" >&2
  exit 1
fi

ENTRY_FILES=(
  "niuma-plan.yml"
  "niuma-implement.yml"
  "niuma-review.yml"
  "niuma-orchestrate.yml"
  "niuma-iterate.yml"
  "niuma-discuss.yml"
  "niuma-dispatch-completed.yml"
)

REUSABLE_FILES=(
  "niuma-plan-reusable.yml"
  "niuma-implement-reusable.yml"
  "niuma-review-reusable.yml"
  "niuma-orchestrate-reusable.yml"
  "niuma-iterate-reusable.yml"
  "niuma-discuss-reusable.yml"
)

confirm_target_repo() {
  if ! gh repo view "$REPO" >/dev/null 2>&1; then
    echo "target repo not accessible: $REPO" >&2
    return 1
  fi
  if [[ "$REPO" == "$MEN_TARGET_REPO" ]]; then
    echo "target repo confirmed: $REPO (men 回归目标 #69/#78)"
  else
    echo "warning: 当前目标仓库不是 $MEN_TARGET_REPO，发布完成后请至少同步一次到 $MEN_TARGET_REPO 并回归 #69/#78" >&2
  fi
}

put_file() {
  local local_file="$1"
  local remote_path="$2"
  local sha=""
  local content=""

  if [[ ! -f "$local_file" ]]; then
    echo "missing source file: $local_file" >&2
    return 1
  fi

  if sha="$(gh api "repos/$REPO/contents/$remote_path" --jq .sha 2>/dev/null)"; then
    :
  else
    sha=""
  fi

  content="$(base64 -w 0 < "$local_file")"

  args=(
    -X PUT
    "repos/$REPO/contents/$remote_path"
    -f "message=$MESSAGE"
    -f "content=$content"
  )
  if [[ -n "$sha" ]]; then
    args+=(-f "sha=$sha")
  fi
  if [[ -n "$BRANCH" ]]; then
    args+=(-f "branch=$BRANCH")
  fi

  gh api "${args[@]}" >/dev/null
  echo "published: $REPO/$remote_path"
  PUBLISHED_PAIRS+=("$local_file::$remote_path")
}

verify_remote_file() {
  local local_file="$1"
  local remote_path="$2"
  local local_content=""
  local remote_content=""

  local_content="$(base64 -w 0 < "$local_file")"
  remote_content="$(gh api "repos/$REPO/contents/$remote_path" --jq '.content' | tr -d '\n')"

  if [[ "$local_content" != "$remote_content" ]]; then
    echo "verify failed: $REPO/$remote_path content mismatch" >&2
    return 1
  fi
  echo "verified: $REPO/$remote_path"
}

fetch_remote_text() {
  local remote_path="$1"
  gh api "repos/$REPO/contents/$remote_path" --jq '.content' | tr -d '\n' | base64 -d
}

verify_implement_self_check_contract() {
  local content=""
  content="$(fetch_remote_text ".github/workflows/niuma-implement-reusable.yml")"

  if ! grep -Fq 'working-directory: ${{ github.workspace }}/merge-result' <<< "$content"; then
    echo "verify failed: implement self-check 缺少 merge-result working-directory" >&2
    return 1
  fi
  if ! grep -Fq '--repo-dir "$MERGE_RESULT_DIR"' <<< "$content"; then
    echo "verify failed: implement self-check 缺少 --repo-dir \"\$MERGE_RESULT_DIR\"" >&2
    return 1
  fi
  if grep -Fq '--repo-dir "$WORKSPACE"' <<< "$content"; then
    echo "verify failed: implement self-check 不应回落到 workspace 基线" >&2
    return 1
  fi

  echo "verified: implement self-check merge-result baseline contract"
}

confirm_target_repo

for file in "${ENTRY_FILES[@]}"; do
  put_file "$SOURCE_DIR/$file" ".github/workflows/$file"
done

if [[ "$MODE" == "full" ]]; then
  for file in "${REUSABLE_FILES[@]}"; do
    put_file "$SOURCE_DIR/$file" ".github/workflows/$file"
  done

  if [[ -f "$GITHUB_SCRIPTS_DIR/niuma-test-gate.sh" ]]; then
    put_file "$GITHUB_SCRIPTS_DIR/niuma-test-gate.sh" ".github/scripts/niuma-test-gate.sh"
  else
    echo "warning: skip .github/scripts/niuma-test-gate.sh (not found in source repo)" >&2
  fi
fi

if [[ "$SKIP_VERIFY" -eq 0 ]]; then
  echo "running post-publish verification..."
  for pair in "${PUBLISHED_PAIRS[@]}"; do
    local_file="${pair%%::*}"
    remote_path="${pair##*::}"
    verify_remote_file "$local_file" "$remote_path"
  done
  if [[ "$MODE" == "full" ]]; then
    verify_implement_self_check_contract
  fi
  echo "post-publish verification passed: ${#PUBLISHED_PAIRS[@]} files"
fi

if [[ "$REPO" == "$MEN_TARGET_REPO" ]]; then
  echo "regression hint: 请在 men 仓库回归 issue #69 和 #78 场景。"
else
  echo "regression hint: 发布到 $MEN_TARGET_REPO 后回归 issue #69 和 #78 场景。"
fi
