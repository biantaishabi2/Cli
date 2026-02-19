#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
usage: cleanup_worktrees.sh [--repo-dir <path>] [--base-ref <ref>]
USAGE
}

REPO_DIR="."
BASE_REF="origin/master"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-dir)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --repo-dir" >&2
        usage
        exit 2
      fi
      REPO_DIR="$2"
      shift 2
      ;;
    --base-ref)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --base-ref" >&2
        usage
        exit 2
      fi
      BASE_REF="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

log() {
  echo "[cleanup] $*"
}

add_record() {
  local -n target_ref="$1"
  local branch="$2"
  local worktree_path="$3"
  local reason="$4"
  target_ref+=("${branch}"$'\t'"${worktree_path}"$'\t'"${reason}")
}

records_to_json() {
  local -n source_ref="$1"
  if [[ ${#source_ref[@]} -eq 0 ]]; then
    echo "[]"
    return 0
  fi

  printf '%s\n' "${source_ref[@]}" | jq -Rsc '
    split("\n")
    | map(select(length > 0))
    | map(split("\t"))
    | map({branch: .[0], worktree: .[1], reason: .[2]})
  '
}

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

if ! git -C "$REPO_DIR" rev-parse --git-dir >/dev/null 2>&1; then
  echo "invalid git repo: $REPO_DIR" >&2
  exit 1
fi

MAIN_WORKTREE_PATH="$(git -C "$REPO_DIR" rev-parse --show-toplevel)"

deleted_records=()
skipped_records=()
warned_records=()
error_records=()

log "repo_dir=$REPO_DIR"
log "main_worktree=$MAIN_WORKTREE_PATH"
log "base_ref=$BASE_REF"

should_run_cleanup=true

if ! git -C "$REPO_DIR" fetch -p origin; then
  add_record error_records "" "$REPO_DIR" "fetch_prune_failed"
  should_run_cleanup=false
fi

if ! git -C "$REPO_DIR" rev-parse --verify --quiet "$BASE_REF^{commit}" >/dev/null; then
  add_record error_records "" "$REPO_DIR" "base_ref_not_found"
  should_run_cleanup=false
fi

process_worktree() {
  local worktree_path="$1"
  local worktree_head="$2"
  local worktree_branch_ref="$3"

  if [[ -z "$worktree_path" ]]; then
    return 0
  fi

  if [[ "$worktree_path" == "$MAIN_WORKTREE_PATH" ]]; then
    add_record skipped_records "" "$worktree_path" "main_worktree"
    return 0
  fi

  if [[ -z "$worktree_branch_ref" || "$worktree_branch_ref" != refs/heads/* ]]; then
    add_record skipped_records "" "$worktree_path" "non_local_branch_or_detached"
    return 0
  fi

  local branch="${worktree_branch_ref#refs/heads/}"
  local remote_ref="refs/remotes/origin/$branch"

  if git -C "$REPO_DIR" show-ref --verify --quiet "$remote_ref"; then
    add_record skipped_records "$branch" "$worktree_path" "remote_exists"
    return 0
  fi

  if [[ ! -d "$worktree_path" ]]; then
    add_record warned_records "$branch" "$worktree_path" "worktree_path_missing"
    return 0
  fi

  # dirty 工作区仅告警，不做删除。
  local status_output
  status_output="$(git -C "$worktree_path" status --porcelain --untracked-files=all 2>/dev/null || true)"
  if [[ -n "$status_output" ]]; then
    add_record warned_records "$branch" "$worktree_path" "worktree_dirty"
    return 0
  fi

  local branch_tip="$worktree_head"
  if [[ -z "$branch_tip" ]]; then
    branch_tip="$(git -C "$REPO_DIR" rev-parse --verify "$worktree_branch_ref" 2>/dev/null || true)"
  fi
  if [[ -z "$branch_tip" ]]; then
    add_record warned_records "$branch" "$worktree_path" "branch_tip_not_found"
    return 0
  fi

  # 删除前校验提交已包含于 origin/master，避免误删未合入分支。
  if ! git -C "$REPO_DIR" merge-base --is-ancestor "$branch_tip" "$BASE_REF"; then
    add_record warned_records "$branch" "$worktree_path" "not_merged_to_master"
    return 0
  fi

  if ! git -C "$REPO_DIR" worktree remove "$worktree_path"; then
    add_record error_records "$branch" "$worktree_path" "remove_worktree_failed"
    return 0
  fi

  if git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$branch"; then
    if ! git -C "$REPO_DIR" branch -D "$branch"; then
      add_record error_records "$branch" "$worktree_path" "delete_branch_failed"
      return 0
    fi
  fi

  add_record deleted_records "$branch" "$worktree_path" "deleted"
}

current_worktree=""
current_head=""
current_branch_ref=""

if [[ "$should_run_cleanup" == "true" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ -z "$line" ]]; then
      process_worktree "$current_worktree" "$current_head" "$current_branch_ref"
      current_worktree=""
      current_head=""
      current_branch_ref=""
      continue
    fi

    case "$line" in
      worktree\ *)
        current_worktree="${line#worktree }"
        ;;
      HEAD\ *)
        current_head="${line#HEAD }"
        ;;
      branch\ *)
        current_branch_ref="${line#branch }"
        ;;
    esac
  done < <(git -C "$REPO_DIR" worktree list --porcelain; echo)

  if ! git -C "$REPO_DIR" worktree prune; then
    add_record error_records "" "$REPO_DIR" "worktree_prune_failed"
  fi
fi

deleted_json="$(records_to_json deleted_records)"
skipped_json="$(records_to_json skipped_records)"
warned_json="$(records_to_json warned_records)"
errors_json="$(records_to_json error_records)"

summary_json="$(jq -nc \
  --argjson deleted "$deleted_json" \
  --argjson skipped "$skipped_json" \
  --argjson warned "$warned_json" \
  --argjson errors "$errors_json" \
  '{deleted: $deleted, skipped: $skipped, warned: $warned, errors: $errors}')"

printf '[cleanup.summary] %s\n' "$summary_json"

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  output_payload="$(
    cat <<EOF
deleted_count=$(jq -r '.deleted | length' <<<"$summary_json")
skipped_count=$(jq -r '.skipped | length' <<<"$summary_json")
warned_count=$(jq -r '.warned | length' <<<"$summary_json")
error_count=$(jq -r '.errors | length' <<<"$summary_json")
deleted=$(jq -c '.deleted' <<<"$summary_json")
skipped=$(jq -c '.skipped' <<<"$summary_json")
warned=$(jq -c '.warned' <<<"$summary_json")
errors=$(jq -c '.errors' <<<"$summary_json")
EOF
  )"
  printf '%s\n' "$output_payload" | tee -a "$GITHUB_OUTPUT" >/dev/null 2>&1 || true
fi

if [[ $(jq -r '.errors | length' <<<"$summary_json") -gt 0 ]]; then
  exit 1
fi
