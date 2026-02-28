#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
SCRIPT_UNDER_TEST="$ROOT_DIR/.github/scripts/niuma-test-gate.sh"

PASS_COUNT=0
FAIL_COUNT=0

fail() {
  echo "[bdd][FAIL] $1" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass() {
  echo "[bdd][PASS] $1"
  PASS_COUNT=$((PASS_COUNT + 1))
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

make_mock_bin() {
  local bin_dir="$1"

  cat > "$bin_dir/gh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

case "$*" in
  *"--json headRefName"*)
    echo "${MOCK_HEAD_REF}"
    ;;
  *"--json baseRefName"*)
    echo "${MOCK_BASE_REF}"
    ;;
  *"--json headRefOid"*)
    echo "${MOCK_HEAD_SHA}"
    ;;
  *"--json baseRefOid"*)
    echo "${MOCK_BASE_SHA}"
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

  cat > "$bin_dir/sleep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  chmod +x "$bin_dir/gh" "$bin_dir/sleep"
}

init_repo() {
  local sandbox="$1"
  local pr_number="$2"
  local with_merge_ref="$3"

  local origin="$sandbox/origin"
  local work="$sandbox/work"

  git init -b master "$origin" >/dev/null
  pushd "$origin" >/dev/null
  git config user.name "test"
  git config user.email "test@example.com"

  mkdir -p automation/niuma docs
  echo "base" > automation/niuma/base.txt
  echo "doc" > docs/readme.md
  git add automation/niuma/base.txt docs/readme.md
  git commit -m "base" >/dev/null
  local base_sha
  base_sha=$(git rev-parse HEAD)

  git checkout -b feature >/dev/null
  echo "feature" >> automation/niuma/base.txt
  git add automation/niuma/base.txt
  git commit -m "feature" >/dev/null
  local head_sha
  head_sha=$(git rev-parse HEAD)

  local merge_sha=""
  if [[ "$with_merge_ref" == "yes" ]]; then
    git checkout master >/dev/null
    git checkout -b tmp-merge >/dev/null
    git merge --no-ff --no-edit feature >/dev/null
    merge_sha=$(git rev-parse HEAD)
    git update-ref "refs/pull/${pr_number}/merge" "$merge_sha"
    git checkout master >/dev/null
    git branch -D tmp-merge >/dev/null
  fi

  popd >/dev/null
  git clone "$origin" "$work" >/dev/null
  printf '%s|%s|%s|%s\n' "$base_sha" "$head_sha" "$merge_sha" "$work"
}

write_critical_config() {
  local work_dir="$1"
  mkdir -p "$work_dir/.github/niuma"
  cat > "$work_dir/.github/niuma/critical-regressions.yml" <<'EOF'
schema_version: 1
critical_jobs:
  - critical-agent-loop
  - critical-dispatch
EOF
}

run_gate_case() {
  local sandbox="$1"
  local pr_number="$2"
  local stdout_file="$3"
  local stderr_file="$4"
  shift 4
  (
    cd "$sandbox/work"
    env \
      PATH="$sandbox/bin:$PATH" \
      GITHUB_TOKEN=token \
      MOCK_BASE_REF=master \
      MOCK_HEAD_REF=feature \
      MOCK_BASE_SHA="$BASE_SHA" \
      MOCK_HEAD_SHA="$HEAD_SHA" \
      "$@" \
      "$SCRIPT_UNDER_TEST" "$pr_number"
  ) >"$stdout_file" 2>"$stderr_file"
}

scenario_high_risk_missing_critical() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 301 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  write_critical_config "$sandbox/work"

  set +e
  run_gate_case "$sandbox" 301 "$sandbox/stdout" "$sandbox/stderr" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\n'
  local status=$?
  set -e

  [[ "$status" -ne 0 ]] || { fail "场景1应失败：关键回归未运行必须阻塞"; return 1; }
  assert_contains "$sandbox/stdout" "reason_code=CRITICAL_REGRESSION_MISSING" "场景1应输出关键回归缺失原因码" || return 1
  assert_contains "$sandbox/stdout" "missing_jobs=critical-agent-loop,critical-dispatch" "场景1应输出 missing_jobs" || return 1
  return 0
}

scenario_low_risk_docs_smoke_only() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 302 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  write_critical_config "$sandbox/work"

  set +e
  run_gate_case "$sandbox" 302 "$sandbox/stdout" "$sandbox/stderr" \
    MOCK_FILES=$'docs/readme.md\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\nfull\tSKIPPED\n'
  local status=$?
  set -e

  [[ "$status" -eq 0 ]] || { fail "场景2应成功：低风险文档应允许 smoke"; return 1; }
  assert_contains "$sandbox/stdout" "run_mode=smoke" "场景2应为 smoke 模式" || return 1
  return 0
}

scenario_smoke_pass_but_critical_skipped() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 303 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  write_critical_config "$sandbox/work"

  set +e
  run_gate_case "$sandbox" 303 "$sandbox/stdout" "$sandbox/stderr" \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\ncritical-agent-loop\tSKIPPED\ncritical-dispatch\tSKIPPED\n'
  local status=$?
  set -e

  [[ "$status" -ne 0 ]] || { fail "场景3应失败：高风险 smoke 通过但 critical skipped 不能放行"; return 1; }
  assert_contains "$sandbox/stdout" "reason_code=INSUFFICIENT_COVERAGE_FOR_HIGH_RISK" "场景3应输出覆盖不足原因码" || return 1
  return 0
}

scenario_critical_green_pass() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 304 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  write_critical_config "$sandbox/work"

  set +e
  run_gate_case "$sandbox" 304 "$sandbox/stdout" "$sandbox/stderr" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'critical-agent-loop\tSUCCESS\ncritical-dispatch\tSUCCESS\n'
  local status=$?
  set -e

  [[ "$status" -eq 0 ]] || { fail "场景4应成功：关键回归全绿应放行"; return 1; }
  assert_contains "$sandbox/stdout" "reason_code=PASS" "场景4应输出 PASS" || return 1
  return 0
}

scenario_timeout_retry_blocked() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 305 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  write_critical_config "$sandbox/work"

  set +e
  run_gate_case "$sandbox" 305 "$sandbox/stdout" "$sandbox/stderr" \
    INFRA_RETRY_MAX=2 \
    MOCK_CHECK_COUNTER_FILE="$sandbox/check_counter" \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS_SEQ=$'critical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n'
  local status=$?
  set -e

  [[ "$status" -ne 0 ]] || { fail "场景5应失败：timeout 重试后仍失败必须阻塞"; return 1; }
  assert_contains "$sandbox/stdout" "reason_code=TIMEOUT_BLOCKED" "场景5应输出 TIMEOUT_BLOCKED" || return 1
  assert_contains "$sandbox/stdout" "retry_count=2" "场景5应输出重试次数" || return 1
  return 0
}

scenario_missing_critical_config_fallback_full() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"
  local repo_info
  repo_info=$(init_repo "$sandbox" 306 yes)
  IFS='|' read -r BASE_SHA HEAD_SHA _ _ <<< "$repo_info"
  # 故意不写 critical 配置，验证 full 安全回退

  set +e
  run_gate_case "$sandbox" 306 "$sandbox/stdout" "$sandbox/stderr" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'full\tSUCCESS\n'
  local status=$?
  set -e

  [[ "$status" -eq 0 ]] || { fail "场景6应成功：缺失 critical 配置时 full 成功应放行"; return 1; }
  assert_contains "$sandbox/stdout" "run_mode=full" "场景6应回退 full" || return 1
  assert_contains "$sandbox/stderr" "critical 清单" "场景6应输出回退告警" || return 1
  return 0
}

run_case() {
  local name="$1"
  local fn="$2"
  if "$fn"; then
    pass "$name"
  else
    echo "[bdd][CASE-FAIL] $name" >&2
  fi
}

run_case "场景1 高风险关键回归缺失阻塞" scenario_high_risk_missing_critical
run_case "场景2 低风险文档 smoke 放行" scenario_low_risk_docs_smoke_only
run_case "场景3 smoke 绿但 critical skipped 阻塞" scenario_smoke_pass_but_critical_skipped
run_case "场景4 critical 全绿通过" scenario_critical_green_pass
run_case "场景5 timeout 重试后阻塞" scenario_timeout_retry_blocked
run_case "场景6 缺失 critical 清单回退 full" scenario_missing_critical_config_fallback_full

echo "[bdd] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
