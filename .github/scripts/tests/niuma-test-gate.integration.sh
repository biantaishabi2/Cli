#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
SCRIPT_UNDER_TEST="$ROOT_DIR/.github/scripts/niuma-test-gate.sh"

PASS_COUNT=0
FAIL_COUNT=0

fail() {
  echo "[integration][FAIL] $1" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass() {
  echo "[integration][PASS] $1"
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

make_gh_mock() {
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
  chmod +x "$bin_dir/gh"
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
  cat > "$work_dir/.github/niuma/critical-regressions.yml" <<'EOF'
schema_version: 1
critical_jobs:
  - critical-agent-loop
  - critical-dispatch
EOF
}

test_smoke_only_false_green_blocked() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"
  cat > "$sandbox/bin/sleep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH
  chmod +x "$sandbox/bin/sleep"

  local repo_info
  repo_info=$(init_repo "$sandbox" 201 yes)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  mkdir -p "$work_dir/.github/niuma"
  write_critical_config "$work_dir"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'smoke\tSUCCESS\ncritical-agent-loop\tSKIPPED\ncritical-dispatch\tSKIPPED\n' \
    "$SCRIPT_UNDER_TEST" 201
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "高风险 smoke-only 假绿应被阻塞"
    return 1
  fi
  assert_contains "$sandbox/stdout" "reason_code=INSUFFICIENT_COVERAGE_FOR_HIGH_RISK" "应输出高风险覆盖不足原因码" || return 1
  return 0
}

test_critical_all_green_pass() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 202 yes)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  mkdir -p "$work_dir/.github/niuma"
  write_critical_config "$work_dir"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'automation/niuma/pkg/agent/loop_core.go\n' \
    MOCK_CHECKS=$'critical-agent-loop\tSUCCESS\ncritical-dispatch\tSUCCESS\nsmoke\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 202
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    fail "关键回归全绿应通过 (status=$status)"
    return 1
  fi
  assert_contains "$sandbox/stdout" "run_mode=critical" "高风险命中 critical 配置时应选择 critical 模式" || return 1
  assert_contains "$sandbox/stdout" "reason_code=PASS" "通过场景应输出 PASS" || return 1
  return 0
}

test_timeout_retry_then_blocked() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"
  cat > "$sandbox/bin/sleep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH
  chmod +x "$sandbox/bin/sleep"

  local repo_info
  repo_info=$(init_repo "$sandbox" 203 yes)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  mkdir -p "$work_dir/.github/niuma"
  write_critical_config "$work_dir"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    INFRA_RETRY_MAX=2 \
    MOCK_CHECK_COUNTER_FILE="$sandbox/check_counter" \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'automation/niuma/pkg/control/controller.go\n' \
    MOCK_CHECKS_SEQ=$'critical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n__NEXT__\ncritical-agent-loop\tTIMED_OUT\ncritical-dispatch\tSUCCESS\n' \
    "$SCRIPT_UNDER_TEST" 203
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "timeout 重试耗尽后应阻塞"
    return 1
  fi
  assert_contains "$sandbox/stdout" "reason_code=TIMEOUT_BLOCKED" "应输出 TIMEOUT_BLOCKED" || return 1
  assert_contains "$sandbox/stdout" "retry_count=2" "应记录重试次数" || return 1
  return 0
}

run_case() {
  local name="$1"
  local fn="$2"
  if "$fn"; then
    pass "$name"
  else
    echo "[integration][CASE-FAIL] $name" >&2
  fi
}

run_case "smoke-only 假绿拦截" test_smoke_only_false_green_blocked
run_case "critical 全绿通过" test_critical_all_green_pass
run_case "timeout 重试后阻塞" test_timeout_retry_then_blocked

echo "[integration] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
