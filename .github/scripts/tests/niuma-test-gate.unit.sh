#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
SCRIPT_UNDER_TEST="$ROOT_DIR/.github/scripts/niuma-test-gate.sh"

PASS_COUNT=0
FAIL_COUNT=0

fail() {
  echo "[unit][FAIL] $1" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass() {
  echo "[unit][PASS] $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

assert_eq() {
  local expected="$1"
  local actual="$2"
  local msg="$3"
  if [[ "$expected" != "$actual" ]]; then
    fail "$msg (expected=$expected, actual=$actual)"
    return 1
  fi
  return 0
}

assert_contains() {
  local file="$1"
  local pattern="$2"
  local msg="$3"
  if ! grep -Fq "$pattern" "$file"; then
    fail "$msg (missing: $pattern)"
    return 1
  fi
  return 0
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  local msg="$3"
  if grep -Fq "$pattern" "$file"; then
    fail "$msg (unexpected: $pattern)"
    return 1
  fi
  return 0
}

new_sandbox() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin" "$sandbox/work/.github/niuma"

  cat > "$sandbox/bin/gh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

case "$*" in
  *"--json headRefName"*)
    echo "${MOCK_HEAD_REF:-feature}"
    ;;
  *"--json baseRefName"*)
    echo "${MOCK_BASE_REF:-master}"
    ;;
  *"--json headRefOid"*)
    echo "${MOCK_HEAD_SHA:-1111111111111111111111111111111111111111}"
    ;;
  *"--json baseRefOid"*)
    echo "${MOCK_BASE_SHA:-2222222222222222222222222222222222222222}"
    ;;
  *"--json files"*)
    printf '%b' "${MOCK_FILES:-README.md\n}"
    ;;
  *"pr checks "*)
    if [[ -n "${MOCK_CHECKS_SEQ:-}" ]]; then
      counter_file="${MOCK_CHECK_COUNTER_FILE:-/tmp/niuma_gate_mock_counter}"
      idx=1
      if [[ -f "$counter_file" ]]; then
        idx=$(cat "$counter_file")
        idx=$((idx + 1))
      fi
      echo "$idx" > "$counter_file"
      awk -v idx="$idx" '
        BEGIN {part=1}
        /^__NEXT__$/ {part++; next}
        part==idx {print}
      ' <<< "${MOCK_CHECKS_SEQ}"
      exit 0
    fi
    printf '%b' "${MOCK_CHECKS:-smoke\tSUCCESS\n}"
    ;;
  *)
    echo "unexpected gh args: $*" >&2
    exit 1
    ;;
esac
SH

  cat > "$sandbox/bin/git" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="${MOCK_GIT_LOG:-/dev/null}"
echo "git $*" >> "$LOG_FILE"

args=("$@")
idx=0
while [[ "${args[$idx]:-}" == "-c" ]]; do
  idx=$((idx + 2))
done
cmd="${args[$idx]:-}"

case "$cmd" in
  fetch)
    target="${args[$((idx + 2))]:-}"
    if [[ "$target" == pull/*/merge ]]; then
      if [[ "${MOCK_FETCH_MERGE_REF:-ok}" == "ok" ]]; then
        exit 0
      fi
      echo "fatal: couldn't find remote ref $target" >&2
      exit 1
    fi
    exit 0
    ;;
  rev-parse)
    ref="${args[$((idx + 1))]:-}"
    case "$ref" in
      FETCH_HEAD)
        echo "${MOCK_FETCH_HEAD_SHA:-3333333333333333333333333333333333333333}"
        ;;
      HEAD)
        echo "${MOCK_LOCAL_MERGE_SHA:-4444444444444444444444444444444444444444}"
        ;;
      *)
        echo "5555555555555555555555555555555555555555"
        ;;
    esac
    ;;
  checkout)
    exit 0
    ;;
  merge)
    status="${MOCK_MERGE_STATUS:-0}"
    if [[ "$status" -ne 0 ]]; then
      printf '%b' "${MOCK_MERGE_OUTPUT:-CONFLICT (content): Merge conflict in conflict.txt\n}" >&2
      exit "$status"
    fi
    exit 0
    ;;
  diff)
    printf '%b' "${MOCK_CONFLICT_FILES:-}"
    ;;
  *)
    exit 0
    ;;
esac
SH

  cat > "$sandbox/bin/sleep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  chmod +x "$sandbox/bin/gh" "$sandbox/bin/git" "$sandbox/bin/sleep"
  echo "$sandbox"
}

write_critical_config() {
  local sandbox="$1"
  local jobs_block="$2"
  cat > "$sandbox/work/.github/niuma/critical-regressions.yml" <<EOF
schema_version: 1
critical_jobs:
$jobs_block
EOF
}

test_usage_validation() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  PATH="$sandbox/bin:$PATH" GITHUB_TOKEN=token "$SCRIPT_UNDER_TEST" >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "2" "$status" "no-arg 应返回 usage 错误码" || return 1
  assert_contains "$sandbox/stderr" "usage:" "应输出 usage" || return 1
  return 0
}

test_merge_ref_preferred() {
  local sandbox
  sandbox=$(new_sandbox)
  write_critical_config "$sandbox" "  - critical-core"

  local git_log="$sandbox/git.log"
  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_GIT_LOG="$git_log" \
    MOCK_FETCH_MERGE_REF=ok \
    MOCK_FILES=$'automation/niuma/main.go\n' \
    MOCK_CHECKS=$'critical-core\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 101
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "0" "$status" "merge ref 可用且 required jobs 通过时应成功" || return 1
  assert_contains "$sandbox/stdout" "merge_ref_source=github-merge-ref" "应优先使用 github merge ref" || return 1
  assert_not_contains "$git_log" " merge --no-ff --no-edit " "不应走本地 merge" || return 1
  return 0
}

test_high_risk_missing_critical_blocks() {
  local sandbox
  sandbox=$(new_sandbox)
  write_critical_config "$sandbox" $'  - critical-agent-loop\n  - critical-dispatch'

  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 102
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "1" "$status" "高风险且关键回归缺失必须阻塞" || return 1
  assert_contains "$sandbox/stdout" "reason_code=CRITICAL_REGRESSION_MISSING" "应输出关键回归缺失原因码" || return 1
  assert_contains "$sandbox/stdout" "missing_jobs=critical-agent-loop,critical-dispatch" "应输出 missing_jobs" || return 1
  return 0
}

test_low_risk_docs_smoke_pass() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_FILES=$'docs/guide.md\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\nfull\tSKIPPED\n' \
    "$SCRIPT_UNDER_TEST" 103
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "0" "$status" "低风险文档改动应允许 smoke 模式通过" || return 1
  assert_contains "$sandbox/stdout" "run_mode=smoke" "低风险应选择 smoke 模式" || return 1
  assert_contains "$sandbox/stdout" "risk_level=low" "低风险应输出 low" || return 1
  return 0
}

test_timeout_retry_then_block() {
  local sandbox
  sandbox=$(new_sandbox)
  write_critical_config "$sandbox" "  - critical-timeout"

  local counter_file="$sandbox/check_counter"
  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    INFRA_RETRY_MAX=2 \
    MOCK_CHECK_COUNTER_FILE="$counter_file" \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS_SEQ=$'critical-timeout\tTIMED_OUT\n__NEXT__\ncritical-timeout\tTIMED_OUT\n__NEXT__\ncritical-timeout\tTIMED_OUT\n' \
    "$SCRIPT_UNDER_TEST" 104
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "1" "$status" "timeout 超过自动重试上限后必须阻塞" || return 1
  assert_contains "$sandbox/stdout" "reason_code=TIMEOUT_BLOCKED" "应输出 timeout 阻塞原因码" || return 1
  assert_contains "$sandbox/stdout" "retry_count=2" "应输出重试计数" || return 1
  return 0
}

test_pending_wait_then_pass() {
  local sandbox
  sandbox=$(new_sandbox)
  write_critical_config "$sandbox" "  - critical-agent-loop"

  local counter_file="$sandbox/check_counter"
  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    PENDING_RETRY_MAX=3 \
    PENDING_RETRY_INTERVAL=1 \
    MOCK_CHECK_COUNTER_FILE="$counter_file" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS_SEQ=$'critical-agent-loop\tIN_PROGRESS\n__NEXT__\ncritical-agent-loop\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 105
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "0" "$status" "required jobs pending 收敛后应通过" || return 1
  assert_contains "$sandbox/stdout" "reason_code=PENDING_RETRYING" "等待期间应输出 pending 重试日志" || return 1
  assert_contains "$sandbox/stdout" "reason_code=PASS" "最终应输出 PASS" || return 1
  return 0
}

run_case() {
  local name="$1"
  local fn="$2"
  if "$fn"; then
    pass "$name"
  else
    echo "[unit][CASE-FAIL] $name" >&2
  fi
}

run_case "参数校验" test_usage_validation
run_case "merge ref 优先路径" test_merge_ref_preferred
run_case "高风险关键回归缺失阻塞" test_high_risk_missing_critical_blocks
run_case "低风险文档 smoke 通过" test_low_risk_docs_smoke_pass
run_case "timeout 重试后阻塞" test_timeout_retry_then_block
run_case "pending 收敛后通过" test_pending_wait_then_pass

echo "[unit] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
