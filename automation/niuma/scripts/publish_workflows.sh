#!/usr/bin/env bash
set -euo pipefail

# 一键发布 workflow 到目标仓库（通过 GitHub Contents API）
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SOURCE_DIR="$ROOT_DIR/.github/workflows"
GITHUB_SCRIPTS_DIR="$ROOT_DIR/.github/scripts"
NIUMA_CONFIG_DIR="$ROOT_DIR/.github/niuma"

DEFAULT_PR_RUN_MODE_VALUE="${DEFAULT_PR_RUN_MODE:-full}"
CRITICAL_REGRESSION_REQUIRED_VALUE="${CRITICAL_REGRESSION_REQUIRED:-true}"
INFRA_RETRY_MAX_VALUE="${INFRA_RETRY_MAX:-2}"
HIGH_RISK_PATHS_VALUE="${HIGH_RISK_PATHS:-automation/niuma/**,.github/workflows/**,.github/scripts/**,compiler/bcc/**,orchestration/**}"
FORCE_POLICY_VARS_VALUE="${FORCE_POLICY_VARS:-false}"
CRITICAL_CONFIG_FILE="$NIUMA_CONFIG_DIR/critical-regressions.yml"

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

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

to_lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

is_bool() {
  case "$(to_lower "$(trim "$1")")" in
    true|false|1|0|yes|no|on|off) return 0 ;;
  esac
  return 1
}

validate_policy_values() {
  local mode
  mode="$(to_lower "$(trim "$DEFAULT_PR_RUN_MODE_VALUE")")"
  if [[ "$mode" != "smoke" && "$mode" != "critical" && "$mode" != "full" ]]; then
    echo "invalid DEFAULT_PR_RUN_MODE: $DEFAULT_PR_RUN_MODE_VALUE (expected smoke|critical|full)" >&2
    exit 1
  fi
  DEFAULT_PR_RUN_MODE_VALUE="$mode"

  if ! is_bool "$CRITICAL_REGRESSION_REQUIRED_VALUE"; then
    echo "invalid CRITICAL_REGRESSION_REQUIRED: $CRITICAL_REGRESSION_REQUIRED_VALUE (expected true|false)" >&2
    exit 1
  fi
  CRITICAL_REGRESSION_REQUIRED_VALUE="$(to_lower "$(trim "$CRITICAL_REGRESSION_REQUIRED_VALUE")")"
  if [[ "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "1" || "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "yes" || "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "on" ]]; then
    CRITICAL_REGRESSION_REQUIRED_VALUE="true"
  fi
  if [[ "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "0" || "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "no" || "$CRITICAL_REGRESSION_REQUIRED_VALUE" == "off" ]]; then
    CRITICAL_REGRESSION_REQUIRED_VALUE="false"
  fi

  if [[ ! "$INFRA_RETRY_MAX_VALUE" =~ ^[0-9]+$ ]]; then
    echo "invalid INFRA_RETRY_MAX: $INFRA_RETRY_MAX_VALUE (expected non-negative integer)" >&2
    exit 1
  fi

  HIGH_RISK_PATHS_VALUE="$(trim "$HIGH_RISK_PATHS_VALUE")"
  if [[ -z "$HIGH_RISK_PATHS_VALUE" ]]; then
    echo "invalid HIGH_RISK_PATHS: empty value is not allowed" >&2
    exit 1
  fi

  if ! is_bool "$FORCE_POLICY_VARS_VALUE"; then
    echo "invalid FORCE_POLICY_VARS: $FORCE_POLICY_VARS_VALUE (expected true|false)" >&2
    exit 1
  fi
  FORCE_POLICY_VARS_VALUE="$(to_lower "$(trim "$FORCE_POLICY_VARS_VALUE")")"
  if [[ "$FORCE_POLICY_VARS_VALUE" == "1" || "$FORCE_POLICY_VARS_VALUE" == "yes" || "$FORCE_POLICY_VARS_VALUE" == "on" ]]; then
    FORCE_POLICY_VARS_VALUE="true"
  fi
  if [[ "$FORCE_POLICY_VARS_VALUE" == "0" || "$FORCE_POLICY_VARS_VALUE" == "no" || "$FORCE_POLICY_VARS_VALUE" == "off" ]]; then
    FORCE_POLICY_VARS_VALUE="false"
  fi
}

validate_critical_config() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "missing critical regression config: $file" >&2
    exit 1
  fi

  local schema_version
  schema_version="$(awk '
    /^[[:space:]]*#/ {next}
    {
      line=$0
      sub(/[[:space:]]*#.*$/, "", line)
      if (line ~ /^[[:space:]]*schema_version:[[:space:]]*/) {
        sub(/^[[:space:]]*schema_version:[[:space:]]*/, "", line)
        gsub(/[[:space:]]+/, "", line)
        print line
        exit
      }
    }
  ' "$file")"

  if [[ "$schema_version" != "1" ]]; then
    echo "invalid $file: schema_version must be 1" >&2
    exit 1
  fi

  local jobs
  jobs="$(awk '
    BEGIN {in_jobs=0}
    /^[[:space:]]*#/ {next}
    {
      line=$0
      sub(/[[:space:]]*#.*$/, "", line)
    }
    line ~ /^[[:space:]]*critical_jobs:[[:space:]]*$/ {in_jobs=1; next}
    in_jobs && line ~ /^[[:space:]]*-[[:space:]]*/ {
      sub(/^[[:space:]]*-[[:space:]]*/, "", line)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
      if (length(line) > 0) print line
      next
    }
    in_jobs && line ~ /^[[:space:]]*[A-Za-z0-9_]+:[[:space:]]*/ {in_jobs=0}
  ' "$file")"

  if [[ -z "$jobs" ]]; then
    echo "invalid $file: critical_jobs must not be empty" >&2
    exit 1
  fi

  local dup
  dup="$(printf '%s\n' "$jobs" | sort | uniq -d || true)"
  if [[ -n "$dup" ]]; then
    echo "invalid $file: duplicated critical_jobs entries detected: $dup" >&2
    exit 1
  fi
}

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

has_repo_variable() {
  local var_name="$1"
  gh variable list --repo "$REPO" --json name --jq ".[] | select(.name == \"$var_name\") | .name" | grep -q .
}

ensure_repo_variable() {
  local var_name="$1"
  local var_value="$2"

  if [[ "$FORCE_POLICY_VARS_VALUE" == "false" ]] && has_repo_variable "$var_name"; then
    echo "keep repo variable: $REPO/$var_name (already exists)"
    return 0
  fi

  gh variable set "$var_name" --repo "$REPO" --body "$var_value"
  echo "published repo variable: $REPO/$var_name=$var_value"
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
  if ! grep -Fq '--repo-dir "$MERGE_RESULT_REALPATH"' <<< "$content"; then
    echo "verify failed: implement self-check 缺少 --repo-dir \"\$MERGE_RESULT_REALPATH\"" >&2
    return 1
  fi
  if ! grep -Fq 'self-check gate 当前目录必须是 merge-result' <<< "$content"; then
    echo "verify failed: implement self-check 缺少当前目录/merge-result 一致性校验" >&2
    return 1
  fi
  if grep -Fq '--repo-dir "$WORKSPACE"' <<< "$content"; then
    echo "verify failed: implement self-check 不应回落到 workspace 基线" >&2
    return 1
  fi

  echo "verified: implement self-check merge-result baseline contract"
}

validate_policy_values
validate_critical_config "$CRITICAL_CONFIG_FILE"
confirm_target_repo

for file in "${ENTRY_FILES[@]}"; do
  put_file "$SOURCE_DIR/$file" ".github/workflows/$file"
done

ensure_repo_variable "DEFAULT_PR_RUN_MODE" "$DEFAULT_PR_RUN_MODE_VALUE"
ensure_repo_variable "CRITICAL_REGRESSION_REQUIRED" "$CRITICAL_REGRESSION_REQUIRED_VALUE"
ensure_repo_variable "INFRA_RETRY_MAX" "$INFRA_RETRY_MAX_VALUE"
ensure_repo_variable "HIGH_RISK_PATHS" "$HIGH_RISK_PATHS_VALUE"

if [[ "$MODE" == "full" ]]; then
  for file in "${REUSABLE_FILES[@]}"; do
    put_file "$SOURCE_DIR/$file" ".github/workflows/$file"
  done

  put_file "$CRITICAL_CONFIG_FILE" ".github/niuma/critical-regressions.yml"

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
