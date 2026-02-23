#!/usr/bin/env bash
set -euo pipefail

# 一键发布入口工作流到目标仓库（通过 GitHub Contents API）
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SOURCE_DIR="$ROOT_DIR/.github/workflows"

REPO=""
BRANCH=""
MESSAGE="chore: sync niuma workflow entries from Cli"

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
    --source-dir)
      SOURCE_DIR="${2:-}"
      shift 2
      ;;
    *)
      echo "unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$REPO" ]]; then
  echo "usage: $0 --repo <owner/repo> [--branch <name>] [--message <msg>] [--source-dir <dir>]" >&2
  exit 1
fi

FILES=(
  "niuma-plan.yml"
  "niuma-implement.yml"
  "niuma-review.yml"
  "niuma-orchestrate.yml"
  "niuma-iterate.yml"
  "niuma-discuss.yml"
)

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
}

for file in "${FILES[@]}"; do
  put_file "$SOURCE_DIR/$file" ".github/workflows/$file"
done

