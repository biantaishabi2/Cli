#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
SCRIPT_UNDER_TEST="$ROOT_DIR/.github/scripts/niuma-test-gate.sh"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
  echo "[gate-e2e][PASS] $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  echo "[gate-e2e][FAIL] $1" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

assert_contains() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if ! grep -Fq "$pattern" "$file"; then
    fail "$message (missing: $pattern)"
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
  *"--json headRefName"*) echo "feature" ;;
  *"--json baseRefName"*) echo "master" ;;
  *"--json headRefOid"*) echo "1111111111111111111111111111111111111111" ;;
  *"--json baseRefOid"*) echo "2222222222222222222222222222222222222222" ;;
  *"--json files"*) printf '%b' "${MOCK_FILES:-README.md\n}" ;;
  *"pr checks "*)
    if [[ -n "${MOCK_CHECKS_SEQ:-}" ]]; then
      counter_file="${MOCK_CHECK_COUNTER_FILE:-/tmp/niuma_gate_e2e_counter}"
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
      ' <<< "$MOCK_CHECKS_SEQ"
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
case "$1" in
  fetch|checkout) exit 0 ;;
  rev-parse)
    if [[ "$2" == "FETCH_HEAD" ]]; then
      echo "3333333333333333333333333333333333333333"
    else
      echo "4444444444444444444444444444444444444444"
    fi
    ;;
  merge|diff) exit 0 ;;
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

  cat > "$sandbox/work/.github/niuma/critical-regressions.yml" <<'EOF'
schema_version: 1
critical_jobs:
  - critical-agent-loop
  - critical-dispatch
EOF

  echo "$sandbox"
}

scenario_smoke_green_but_high_risk_skipped_block() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\ncritical-agent-loop\tSKIPPED\ncritical-dispatch\tSKIPPED\n' \
    "$SCRIPT_UNDER_TEST" 501
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "smoke-only 假绿必须被阻塞"
    return 1
  fi
  assert_contains "$sandbox/stdout" "reason_code=INSUFFICIENT_COVERAGE_FOR_HIGH_RISK" "应输出高风险覆盖不足原因码" || return 1
  return 0
}

scenario_critical_full_green_pass() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS=$'critical-agent-loop\tSUCCESS\ncritical-dispatch\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 502
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    fail "critical 回归全绿必须通过"
    return 1
  fi
  assert_contains "$sandbox/stdout" "reason_code=PASS" "应输出 PASS" || return 1
  return 0
}

scenario_timeout_retry_exhausted_blocked() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  (
    cd "$sandbox/work"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    INFRA_RETRY_MAX=2 \
    MOCK_CHECK_COUNTER_FILE="$sandbox/check_counter" \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS_SEQ=$'critical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 503
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "timeout 重试耗尽必须阻塞"
    return 1
  fi
  assert_contains "$sandbox/stdout" "reason_code=TIMEOUT_BLOCKED" "应输出 TIMEOUT_BLOCKED" || return 1
  assert_contains "$sandbox/stdout" "retry_count=2" "应输出重试次数" || return 1
  return 0
}

run_case() {
  local name="$1"
  local fn="$2"
  if "$fn"; then
    pass "$name"
  else
    echo "[gate-e2e][CASE-FAIL] $name" >&2
  fi
}

run_case "smoke-only 假绿拦截" scenario_smoke_green_but_high_risk_skipped_block
run_case "critical 全绿通过" scenario_critical_full_green_pass
run_case "timeout 重试后阻塞" scenario_timeout_retry_exhausted_blocked

echo "[gate-e2e] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
