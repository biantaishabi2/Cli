#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[gate] $*"
}

warn() {
  echo "[gate][warning] $*" >&2
}

usage() {
  echo "usage: $0 <pr-number>" >&2
}

to_lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

to_upper() {
  printf '%s' "$1" | tr '[:lower:]' '[:upper:]'
}

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

join_csv() {
  local safe_items=()
  local item=""
  for item in "$@"; do
    safe_items+=("${item//,/;}")
  done
  local IFS=,
  printf '%s' "${safe_items[*]}"
}

as_bool() {
  case "$(to_lower "$(trim "${1:-}")")" in
    1|true|yes|on) echo "true" ;;
    *) echo "false" ;;
  esac
}

is_docs_like_file() {
  local file="$1"
  case "$file" in
    docs/*|*/docs/*|*.md|*.rst|*.adoc|*.txt|*.markdown)
      return 0
      ;;
  esac
  return 1
}

contains_glob_meta() {
  local pattern="$1"
  [[ "$pattern" == *"*"* || "$pattern" == *"?"* || "$pattern" == *"["* ]]
}

normalize_glob_pattern() {
  local pattern
  pattern="$(trim "${1:-}")"
  if [[ -z "$pattern" ]]; then
    return 0
  fi
  if ! contains_glob_meta "$pattern"; then
    if [[ "$pattern" == */ ]]; then
      pattern="${pattern}*"
    else
      pattern="${pattern}*"
    fi
  fi
  printf '%s' "$pattern"
}

state_category() {
  local raw_state="$1"
  local state
  state="$(to_upper "$(trim "$raw_state")")"
  case "$state" in
    SUCCESS|PASSED|PASS|COMPLETED)
      echo "success"
      ;;
    TIMED_OUT|TIMEOUT)
      echo "timeout"
      ;;
    SKIPPED|NOT_RUN)
      echo "not_run"
      ;;
    CANCELLED|CANCELED)
      echo "cancelled"
      ;;
    PENDING|IN_PROGRESS|QUEUED|REQUESTED|WAITING)
      echo "pending"
      ;;
    FAILURE|FAILED|ERROR|STARTUP_FAILURE|ACTION_REQUIRED|STALE|NEUTRAL)
      echo "failed"
      ;;
    *)
      echo "failed"
      ;;
  esac
}

emit_gate_outputs() {
  if [[ -z "${GITHUB_OUTPUT:-}" ]]; then
    return 0
  fi
  if ! touch "$GITHUB_OUTPUT" 2>/dev/null; then
    return 0
  fi
  if ! {
  {
    echo "run_mode=$RUN_MODE"
    echo "risk_level=$RISK_LEVEL"
    echo "required_jobs=$(join_csv "${REQUIRED_JOBS[@]}")"
    echo "actual_jobs=$(join_csv "${ACTUAL_JOBS[@]}")"
    echo "missing_jobs=$(join_csv "${MISSING_JOBS[@]}")"
    echo "reason_code=$REASON_CODE"
    echo "retry_count=$RETRY_COUNT"
  } >> "$GITHUB_OUTPUT"
  }; then
    return 0
  fi
}

emit_decision_log() {
  log "run_mode=$RUN_MODE risk_level=$RISK_LEVEL required_jobs=$(join_csv "${REQUIRED_JOBS[@]}") actual_jobs=$(join_csv "${ACTUAL_JOBS[@]}") missing_jobs=$(join_csv "${MISSING_JOBS[@]}") reason_code=$REASON_CODE retry_count=$RETRY_COUNT"
}

parse_critical_jobs_config() {
  local config_file="$1"
  local schema_version=""
  local in_jobs=0
  local line=""
  local pure_line=""
  local -a jobs=()
  local -A seen=()

  if [[ ! -f "$config_file" ]]; then
    return 1
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    pure_line="${line%%#*}"
    pure_line="$(trim "$pure_line")"
    [[ -z "$pure_line" ]] && continue

    if [[ "$pure_line" =~ ^schema_version:[[:space:]]*(.+)$ ]]; then
      schema_version="$(trim "${BASH_REMATCH[1]}")"
      continue
    fi

    if [[ "$pure_line" == "critical_jobs:" ]]; then
      in_jobs=1
      continue
    fi

    if [[ $in_jobs -eq 1 ]]; then
      if [[ "$pure_line" =~ ^-[[:space:]]*(.+)$ ]]; then
        local job_name
        job_name="$(trim "${BASH_REMATCH[1]}")"
        if [[ -z "$job_name" ]]; then
          warn "critical config 非法：存在空 job 名称（$config_file）"
          return 2
        fi
        if [[ -n "${seen[$job_name]:-}" ]]; then
          warn "critical config 非法：存在重复 job 名称 '$job_name'（$config_file）"
          return 2
        fi
        seen[$job_name]=1
        jobs+=("$job_name")
        continue
      fi
      if [[ "$pure_line" =~ ^[a-zA-Z0-9_]+:[[:space:]]* ]]; then
        in_jobs=0
      fi
    fi
  done < "$config_file"

  if [[ "$schema_version" != "1" ]]; then
    warn "critical config 非法：schema_version 必须为 1（$config_file）"
    return 2
  fi

  if [[ ${#jobs[@]} -eq 0 ]]; then
    warn "critical config 非法：critical_jobs 不能为空（$config_file）"
    return 2
  fi

  CRITICAL_JOBS=("${jobs[@]}")
  return 0
}

match_required_job() {
  local required="$1"
  local actual_name="$2"
  local required_lower actual_lower
  required_lower="$(to_lower "$required")"
  actual_lower="$(to_lower "$actual_name")"

  case "$required_lower" in
    smoke|full)
      [[ "$actual_lower" == *"$required_lower"* ]]
      ;;
    *)
      [[ "$actual_lower" == "$required_lower" ]]
      ;;
  esac
}

fetch_checks() {
  local checks_raw=""
  if ! checks_raw="$(gh pr checks "$PR_NUMBER" --json name,state --jq '.[] | "\(.name)\t\(.state)"' 2>/tmp/niuma-gate-checks.err)"; then
    local err_msg
    err_msg="$(tail -n 1 /tmp/niuma-gate-checks.err 2>/dev/null || true)"
    REASON_CODE="CHECKS_QUERY_FAILED"
    ACTUAL_JOBS=()
    MISSING_JOBS=("${REQUIRED_JOBS[@]}")
    warn "查询 PR checks 失败: ${err_msg:-unknown error}"
    rm -f /tmp/niuma-gate-checks.err
    return 1
  fi
  rm -f /tmp/niuma-gate-checks.err

  CHECK_NAMES=()
  CHECK_STATES=()
  ACTUAL_JOBS=()
  local row=""
  while IFS= read -r row || [[ -n "$row" ]]; do
    [[ -z "$row" ]] && continue
    local name="${row%%$'\t'*}"
    local state="${row#*$'\t'}"
    if [[ "$row" != *$'\t'* ]]; then
      name="$row"
      state="UNKNOWN"
    fi
    CHECK_NAMES+=("$name")
    CHECK_STATES+=("$state")
    ACTUAL_JOBS+=("$name:$state")
  done <<< "$checks_raw"
  return 0
}

evaluate_required_jobs() {
  MISSING_JOBS=()
  local coverage_block=0
  local timeout_block=0
  local failed_block=0
  local pending_block=0
  local required="" idx=""

  for required in "${REQUIRED_JOBS[@]}"; do
    local -a matched_states=()
    for idx in "${!CHECK_NAMES[@]}"; do
      if match_required_job "$required" "${CHECK_NAMES[$idx]}"; then
        matched_states+=("${CHECK_STATES[$idx]}")
      fi
    done

    if [[ ${#matched_states[@]} -eq 0 ]]; then
      MISSING_JOBS+=("$required")
      continue
    fi

    local has_success=0
    local has_timeout=0
    local has_not_run=0
    local has_cancelled=0
    local has_pending=0
    local has_failed=0
    local raw_state=""
    for raw_state in "${matched_states[@]}"; do
      case "$(state_category "$raw_state")" in
        success) has_success=1 ;;
        timeout) has_timeout=1 ;;
        not_run) has_not_run=1 ;;
        cancelled) has_cancelled=1 ;;
        pending) has_pending=1 ;;
        failed) has_failed=1 ;;
      esac
    done

    if [[ "$has_success" -eq 1 ]]; then
      continue
    fi
    if [[ "$has_timeout" -eq 1 ]]; then
      timeout_block=1
      continue
    fi
    if [[ "$has_not_run" -eq 1 || "$has_cancelled" -eq 1 ]]; then
      coverage_block=1
      continue
    fi
    if [[ "$has_pending" -eq 1 ]]; then
      pending_block=1
      continue
    fi
    if [[ "$has_failed" -eq 1 ]]; then
      failed_block=1
      continue
    fi
  done

  if [[ ${#MISSING_JOBS[@]} -gt 0 ]]; then
    if [[ "$RUN_MODE" == "critical" ]]; then
      REASON_CODE="CRITICAL_REGRESSION_MISSING"
    elif [[ "$RISK_LEVEL" == "high" ]]; then
      REASON_CODE="INSUFFICIENT_COVERAGE_FOR_HIGH_RISK"
    else
      REASON_CODE="REQUIRED_JOBS_MISSING"
    fi
    return 1
  fi

  if [[ "$coverage_block" -eq 1 ]]; then
    if [[ "$RISK_LEVEL" == "high" ]]; then
      REASON_CODE="INSUFFICIENT_COVERAGE_FOR_HIGH_RISK"
    else
      REASON_CODE="REQUIRED_JOBS_NOT_EXECUTED"
    fi
    return 1
  fi

  if [[ "$failed_block" -eq 1 ]]; then
    REASON_CODE="REQUIRED_JOBS_FAILED"
    return 1
  fi

  if [[ "$pending_block" -eq 1 ]]; then
    REASON_CODE="REQUIRED_JOBS_PENDING"
    return 1
  fi

  if [[ "$timeout_block" -eq 1 ]]; then
    REASON_CODE="REQUIRED_JOBS_TIMEOUT"
    return 2
  fi

  REASON_CODE="PASS"
  return 0
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

PR_NUMBER="$1"
if ! [[ "$PR_NUMBER" =~ ^[0-9]+$ ]]; then
  echo "invalid pr-number: $PR_NUMBER" >&2
  usage
  exit 2
fi

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "GITHUB_TOKEN is required" >&2
  exit 2
fi

DEFAULT_PR_RUN_MODE_RAW="${DEFAULT_PR_RUN_MODE:-full}"
DEFAULT_PR_RUN_MODE="$(to_lower "$(trim "$DEFAULT_PR_RUN_MODE_RAW")")"
if [[ "$DEFAULT_PR_RUN_MODE" != "smoke" && "$DEFAULT_PR_RUN_MODE" != "critical" && "$DEFAULT_PR_RUN_MODE" != "full" ]]; then
  warn "DEFAULT_PR_RUN_MODE 非法: '$DEFAULT_PR_RUN_MODE_RAW'，回退为 full"
  DEFAULT_PR_RUN_MODE="full"
fi

CRITICAL_REGRESSION_REQUIRED="$(as_bool "${CRITICAL_REGRESSION_REQUIRED:-true}")"

INFRA_RETRY_MAX_RAW="${INFRA_RETRY_MAX:-2}"
if [[ "$INFRA_RETRY_MAX_RAW" =~ ^[0-9]+$ ]]; then
  INFRA_RETRY_MAX="$INFRA_RETRY_MAX_RAW"
else
  warn "INFRA_RETRY_MAX 非法: '$INFRA_RETRY_MAX_RAW'，回退为 2"
  INFRA_RETRY_MAX="2"
fi

PENDING_RETRY_MAX_RAW="${PENDING_RETRY_MAX:-30}"
if [[ "$PENDING_RETRY_MAX_RAW" =~ ^[0-9]+$ ]]; then
  PENDING_RETRY_MAX="$PENDING_RETRY_MAX_RAW"
else
  warn "PENDING_RETRY_MAX 非法: '$PENDING_RETRY_MAX_RAW'，回退为 30"
  PENDING_RETRY_MAX="30"
fi

PENDING_RETRY_INTERVAL_RAW="${PENDING_RETRY_INTERVAL:-10}"
if [[ "$PENDING_RETRY_INTERVAL_RAW" =~ ^[0-9]+$ ]]; then
  PENDING_RETRY_INTERVAL="$PENDING_RETRY_INTERVAL_RAW"
else
  warn "PENDING_RETRY_INTERVAL 非法: '$PENDING_RETRY_INTERVAL_RAW'，回退为 10"
  PENDING_RETRY_INTERVAL="10"
fi

HIGH_RISK_PATHS_DEFAULT="automation/niuma/**,.github/workflows/**,.github/scripts/**,compiler/bcc/**,orchestration/**"
HIGH_RISK_PATHS_RAW="${HIGH_RISK_PATHS:-$HIGH_RISK_PATHS_DEFAULT}"
CRITICAL_CONFIG_FILE="${CRITICAL_REGRESSION_CONFIG:-.github/niuma/critical-regressions.yml}"

HEAD_REF=$(gh pr view "$PR_NUMBER" --json headRefName --jq '.headRefName')
BASE_REF=$(gh pr view "$PR_NUMBER" --json baseRefName --jq '.baseRefName')
HEAD_SHA=$(gh pr view "$PR_NUMBER" --json headRefOid --jq '.headRefOid')
BASE_SHA=$(gh pr view "$PR_NUMBER" --json baseRefOid --jq '.baseRefOid')

if [[ -z "$HEAD_REF" || "$HEAD_REF" == "null" ]]; then
  echo "unable to resolve PR head ref for #$PR_NUMBER" >&2
  exit 1
fi
if [[ -z "$BASE_REF" || "$BASE_REF" == "null" ]]; then
  echo "unable to resolve PR base ref for #$PR_NUMBER" >&2
  exit 1
fi

if [[ -z "$HEAD_SHA" || "$HEAD_SHA" == "null" ]]; then
  HEAD_SHA="unknown"
fi
if [[ -z "$BASE_SHA" || "$BASE_SHA" == "null" ]]; then
  BASE_SHA="unknown"
fi

WORK_BRANCH="niuma-gate-$PR_NUMBER"
MERGE_REF_SOURCE=""
MERGE_SHA=""

log "baseline=merge-result"

if git fetch origin "pull/${PR_NUMBER}/merge"; then
  MERGE_SHA=$(git rev-parse FETCH_HEAD)
  git checkout -B "$WORK_BRANCH" "$MERGE_SHA"
  MERGE_REF_SOURCE="github-merge-ref"
else
  log "merge ref unavailable, fallback to local merge"

  git fetch origin "$BASE_REF"
  git fetch origin "$HEAD_REF"
  git checkout -B "$WORK_BRANCH" "origin/$BASE_REF"

  set +e
  MERGE_OUTPUT=$(git -c user.name='niuma-gate' -c user.email='niuma-gate@local' merge --no-ff --no-edit "origin/$HEAD_REF" 2>&1)
  MERGE_STATUS=$?
  set -e

  if [[ $MERGE_STATUS -ne 0 ]]; then
    CONFLICT_FILES=$(git diff --name-only --diff-filter=U || true)
    echo "CONFLICT: failed to merge origin/$HEAD_REF into origin/$BASE_REF" >&2
    if [[ -n "$CONFLICT_FILES" ]]; then
      echo "CONFLICT: files:" >&2
      while IFS= read -r file; do
        [[ -z "$file" ]] && continue
        echo "CONFLICT: $file" >&2
      done <<< "$CONFLICT_FILES"
    fi

    MERGE_SUMMARY=$(printf '%s\n' "$MERGE_OUTPUT" | grep -E 'CONFLICT|Automatic merge failed|^error:' | head -n 20 || true)
    if [[ -z "$MERGE_SUMMARY" ]]; then
      MERGE_SUMMARY=$(printf '%s\n' "$MERGE_OUTPUT" | tail -n 20)
    fi
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      echo "CONFLICT: $line" >&2
    done <<< "$MERGE_SUMMARY"

    git merge --abort >/dev/null 2>&1 || true
    exit 1
  fi

  MERGE_SHA=$(git rev-parse HEAD)
  MERGE_REF_SOURCE="local-merge"
fi

log "merge_ref_source=$MERGE_REF_SOURCE"
log "base_sha=$BASE_SHA"
log "head_sha=$HEAD_SHA"
if [[ -n "$MERGE_SHA" ]]; then
  log "merge_sha=$MERGE_SHA"
fi

mapfile -t CHANGED_FILES < <(gh pr view "$PR_NUMBER" --json files --jq '.files[].path')

mapfile -t HIGH_RISK_PATTERNS < <(printf '%s\n' "$HIGH_RISK_PATHS_RAW" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sed '/^$/d')

RISK_LEVEL="low"
RISK_REASON="default-low"
for changed_file in "${CHANGED_FILES[@]}"; do
  for pattern_raw in "${HIGH_RISK_PATTERNS[@]}"; do
    pattern="$(normalize_glob_pattern "$pattern_raw")"
    [[ -z "$pattern" ]] && continue
    if [[ "$changed_file" == $pattern ]]; then
      RISK_LEVEL="high"
      RISK_REASON="high_risk_paths:$pattern_raw"
      break 2
    fi
  done
done

if [[ "$RISK_LEVEL" != "high" ]]; then
  DOCS_ONLY=1
  for changed_file in "${CHANGED_FILES[@]}"; do
    if ! is_docs_like_file "$changed_file"; then
      DOCS_ONLY=0
      break
    fi
  done
  if [[ "$DOCS_ONLY" -eq 1 ]]; then
    RISK_LEVEL="low"
    RISK_REASON="change_type:docs_only"
  else
    RISK_LEVEL="high"
    RISK_REASON="change_type:non_docs"
  fi
fi

RUN_MODE="$DEFAULT_PR_RUN_MODE"
REQUIRED_JOBS=()
CRITICAL_JOBS=()

if [[ "$RISK_LEVEL" == "low" ]]; then
  RUN_MODE="smoke"
  REQUIRED_JOBS=("smoke")
else
  if [[ "$CRITICAL_REGRESSION_REQUIRED" == "true" ]]; then
    if parse_critical_jobs_config "$CRITICAL_CONFIG_FILE"; then
      RUN_MODE="critical"
      REQUIRED_JOBS=("${CRITICAL_JOBS[@]}")
    else
      RUN_MODE="full"
      REQUIRED_JOBS=("full")
      warn "high-risk PR 未找到有效 critical 清单，安全回退 full"
      RISK_REASON="${RISK_REASON};critical_fallback_full"
    fi
  else
    case "$DEFAULT_PR_RUN_MODE" in
      smoke)
        RUN_MODE="smoke"
        REQUIRED_JOBS=("smoke")
        ;;
      critical)
        if parse_critical_jobs_config "$CRITICAL_CONFIG_FILE"; then
          RUN_MODE="critical"
          REQUIRED_JOBS=("${CRITICAL_JOBS[@]}")
        else
          RUN_MODE="full"
          REQUIRED_JOBS=("full")
          warn "DEFAULT_PR_RUN_MODE=critical 但 critical 清单无效，安全回退 full"
          RISK_REASON="${RISK_REASON};critical_fallback_full"
        fi
        ;;
      *)
        RUN_MODE="full"
        REQUIRED_JOBS=("full")
        ;;
    esac
  fi
fi

REASON_CODE="INIT"
RETRY_COUNT=0
PENDING_RETRY_COUNT=0
ACTUAL_JOBS=()
MISSING_JOBS=()
CHECK_NAMES=()
CHECK_STATES=()

log "risk_level=$RISK_LEVEL risk_reason=$RISK_REASON"

while true; do
  if ! fetch_checks; then
    emit_gate_outputs
    emit_decision_log
    exit 1
  fi

  if evaluate_required_jobs; then
    emit_gate_outputs
    emit_decision_log
    log "all required checks passed"
    exit 0
  fi

  if [[ "$REASON_CODE" == "REQUIRED_JOBS_TIMEOUT" && "$RETRY_COUNT" -lt "$INFRA_RETRY_MAX" ]]; then
    RETRY_COUNT=$((RETRY_COUNT + 1))
    REASON_CODE="TIMEOUT_RETRYING"
    emit_gate_outputs
    emit_decision_log
    sleep $((RETRY_COUNT * 5))
    continue
  fi

  if [[ "$REASON_CODE" == "REQUIRED_JOBS_PENDING" && "$PENDING_RETRY_COUNT" -lt "$PENDING_RETRY_MAX" ]]; then
    PENDING_RETRY_COUNT=$((PENDING_RETRY_COUNT + 1))
    REASON_CODE="PENDING_RETRYING"
    emit_gate_outputs
    emit_decision_log
    sleep "$PENDING_RETRY_INTERVAL"
    continue
  fi

  if [[ "$REASON_CODE" == "REQUIRED_JOBS_TIMEOUT" ]]; then
    REASON_CODE="TIMEOUT_BLOCKED"
  fi
  if [[ "$REASON_CODE" == "REQUIRED_JOBS_PENDING" ]]; then
    REASON_CODE="PENDING_BLOCKED"
  fi
  emit_gate_outputs
  emit_decision_log
  exit 1
done
