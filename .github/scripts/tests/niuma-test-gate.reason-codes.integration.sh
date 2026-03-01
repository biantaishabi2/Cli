#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
SCRIPT_UNDER_TEST="$ROOT_DIR/.github/scripts/niuma-test-gate.sh"

PASS_COUNT=0
FAIL_COUNT=0

fail() {
  echo "[gate-reason][FAIL] $1" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass() {
  echo "[gate-reason][PASS] $1"
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

read_output_value() {
  local file="$1"
  local key="$2"
  awk -v key="$key" '
    index($0, key "=") == 1 {
      val = substr($0, length(key) + 2)
    }
    END {
      print val
    }
  ' "$file"
}

make_gh_mock() {
  local bin_dir="$1"
  cat > "$bin_dir/gh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *"pr view"* && "$*" == *"--json headRefName"* ]]; then
  echo "${MOCK_HEAD_REF:-feature}"
  exit 0
fi
if [[ "$*" == *"pr view"* && "$*" == *"--json baseRefName"* ]]; then
  echo "${MOCK_BASE_REF:-master}"
  exit 0
fi
if [[ "$*" == *"pr view"* && "$*" == *"--json headRefOid"* ]]; then
  echo "${MOCK_HEAD_SHA:-1111111111111111111111111111111111111111}"
  exit 0
fi
if [[ "$*" == *"pr view"* && "$*" == *"--json baseRefOid"* ]]; then
  echo "${MOCK_BASE_SHA:-2222222222222222222222222222222222222222}"
  exit 0
fi
if [[ "$*" == *"pr view"* && "$*" == *"--json files"* ]]; then
  printf '%b' "${MOCK_FILES:-docs/readme.md\n}"
  exit 0
fi
if [[ "$*" == *"pr checks"* && "$*" == *"--json name,state"* ]]; then
  state_file="${MOCK_CHECKS_STATE_FILE:?}"
  call=0
  if [[ -f "$state_file" ]]; then
    call=$(cat "$state_file")
  fi
  call=$((call + 1))
  echo "$call" > "$state_file"
  eval "entry=\${MOCK_CHECKS_CALL_${call}:-\${MOCK_CHECKS_DEFAULT:-ok|smoke\tSUCCESS\n}}"
  mode="${entry%%|*}"
  payload="${entry#*|}"
  if [[ "$mode" == "ok" ]]; then
    printf '%b' "$payload"
    exit 0
  fi
  printf '%b\n' "$payload" >&2
  exit 1
fi

echo "unexpected gh args: $*" >&2
exit 1
SH
  chmod +x "$bin_dir/gh"
}

init_repo_with_merge_ref() {
  local sandbox="$1"
  local pr_number="$2"

  local origin="$sandbox/origin"
  local work="$sandbox/work"

  git init -b master "$origin" >/dev/null
  pushd "$origin" >/dev/null
  git config user.name "test"
  git config user.email "test@example.com"

  mkdir -p docs
  echo "base" > docs/readme.md
  git add docs/readme.md
  git commit -m "base" >/dev/null
  local base_sha
  base_sha=$(git rev-parse HEAD)

  git checkout -b feature >/dev/null
  echo "feature" >> docs/readme.md
  git add docs/readme.md
  git commit -m "feature" >/dev/null
  local head_sha
  head_sha=$(git rev-parse HEAD)

  git checkout master >/dev/null
  git checkout -b tmp-merge >/dev/null
  git merge --no-ff --no-edit feature >/dev/null
  local merge_sha
  merge_sha=$(git rev-parse HEAD)
  git update-ref "refs/pull/${pr_number}/merge" "$merge_sha"
  git checkout master >/dev/null
  git branch -D tmp-merge >/dev/null
  popd >/dev/null

  git clone "$origin" "$work" >/dev/null
  printf '%s|%s|%s\n' "$base_sha" "$head_sha" "$work"
}

run_gate_case() {
  local sandbox="$1"
  local pr_number="$2"
  local stdout_file="$sandbox/stdout"
  local stderr_file="$sandbox/stderr"
  local output_file="$sandbox/github_output"
  : > "$output_file"

  local repo_info
  repo_info=$(init_repo_with_merge_ref "$sandbox" "$pr_number")
  IFS='|' read -r base_sha head_sha work_dir <<< "$repo_info"

  set +e
  (
    cd "$work_dir"
    env \
      PATH="$sandbox/bin:$PATH" \
      GITHUB_TOKEN=token \
      GITHUB_OUTPUT="$output_file" \
      MOCK_BASE_REF=master \
      MOCK_HEAD_REF=feature \
      MOCK_BASE_SHA="$base_sha" \
      MOCK_HEAD_SHA="$head_sha" \
      MOCK_FILES=$'docs/readme.md\n' \
      "$SCRIPT_UNDER_TEST" "$pr_number"
  ) >"$stdout_file" 2>"$stderr_file"
  local status=$?
  set -e

  echo "$status" > "$sandbox/status"
}

scenario_code_failure_goes_needs_fix_reason() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1=$'ok|smoke\tFAILURE\n' \
  run_gate_case "$sandbox" 5141

  local status
  status=$(cat "$sandbox/status")
  assert_eq "1" "$status" "代码失败场景应返回非 0" || return 1
  assert_eq "REQUIRED_JOBS_FAILED" "$(read_output_value "$sandbox/github_output" reason_code)" "代码失败应分类为 REQUIRED_JOBS_FAILED" || return 1
  assert_eq "0" "$(read_output_value "$sandbox/github_output" retry_count)" "代码失败不应计入基础设施重试" || return 1
  assert_eq "required_jobs_failed" "$(read_output_value "$sandbox/github_output" last_error)" "代码失败应输出稳定 last_error" || return 1
  return 0
}

scenario_network_transient_recovers_after_retry() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  INFRA_RETRY_MAX=2 \
  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1='err|Post "https://api.github.com/graphql": EOF' \
  MOCK_CHECKS_CALL_2='err|Post "https://api.github.com/graphql": EOF' \
  MOCK_CHECKS_CALL_3=$'ok|smoke\tSUCCESS\n' \
  run_gate_case "$sandbox" 5142

  local status
  status=$(cat "$sandbox/status")
  assert_eq "0" "$status" "网络抖动恢复后应返回 0" || return 1
  assert_eq "PASS" "$(read_output_value "$sandbox/github_output" reason_code)" "恢复后应回到 PASS" || return 1
  assert_eq "2" "$(read_output_value "$sandbox/github_output" retry_count)" "应记录两次查询重试" || return 1
  assert_eq "none" "$(read_output_value "$sandbox/github_output" last_error)" "恢复后应清空 last_error" || return 1
  assert_contains "$sandbox/stderr" "NETWORK_TRANSIENT" "重试日志应包含 NETWORK_TRANSIENT 分类" || return 1
  return 0
}

scenario_network_transient_exhausts_retry() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  INFRA_RETRY_MAX=2 \
  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1='err|Post "https://api.github.com/graphql": EOF' \
  MOCK_CHECKS_CALL_2='err|Post "https://api.github.com/graphql": EOF' \
  MOCK_CHECKS_CALL_3='err|Post "https://api.github.com/graphql": EOF' \
  run_gate_case "$sandbox" 5143

  local status
  status=$(cat "$sandbox/status")
  assert_eq "1" "$status" "抖动超过重试上限应失败" || return 1
  assert_eq "NETWORK_TRANSIENT" "$(read_output_value "$sandbox/github_output" reason_code)" "EOF 应分类为 NETWORK_TRANSIENT" || return 1
  assert_eq "2" "$(read_output_value "$sandbox/github_output" retry_count)" "超过上限时应稳定记录重试次数" || return 1
  assert_contains "$sandbox/github_output" "last_error=Post \"https://api.github.com/graphql\": EOF" "应输出可观测 last_error" || return 1
  return 0
}

scenario_auth_failed_no_retry() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  INFRA_RETRY_MAX=2 \
  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1='err|HTTP 401 Unauthorized' \
  run_gate_case "$sandbox" 5144

  local status
  status=$(cat "$sandbox/status")
  assert_eq "1" "$status" "认证失败应返回非 0" || return 1
  assert_eq "AUTH_FAILED" "$(read_output_value "$sandbox/github_output" reason_code)" "401 应分类为 AUTH_FAILED" || return 1
  assert_eq "0" "$(read_output_value "$sandbox/github_output" retry_count)" "AUTH_FAILED 不应自动重试" || return 1
  return 0
}

scenario_forbidden_failed_no_retry() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  INFRA_RETRY_MAX=2 \
  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1='err|HTTP 403 Forbidden' \
  run_gate_case "$sandbox" 5146

  local status
  status=$(cat "$sandbox/status")
  assert_eq "1" "$status" "403 场景应返回非 0" || return 1
  assert_eq "AUTH_FAILED" "$(read_output_value "$sandbox/github_output" reason_code)" "403 应分类为 AUTH_FAILED" || return 1
  assert_eq "0" "$(read_output_value "$sandbox/github_output" retry_count)" "403 不应自动重试" || return 1
  return 0
}

scenario_rate_limited_no_retry() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  INFRA_RETRY_MAX=2 \
  MOCK_CHECKS_STATE_FILE="$sandbox/checks.state" \
  MOCK_CHECKS_CALL_1='err|HTTP 429 Too Many Requests' \
  run_gate_case "$sandbox" 5145

  local status
  status=$(cat "$sandbox/status")
  assert_eq "1" "$status" "限流场景应返回非 0" || return 1
  assert_eq "RATE_LIMITED" "$(read_output_value "$sandbox/github_output" reason_code)" "429 应分类为 RATE_LIMITED" || return 1
  assert_eq "0" "$(read_output_value "$sandbox/github_output" retry_count)" "RATE_LIMITED 不应自动重试" || return 1
  return 0
}

run_case() {
  local name="$1"
  local fn="$2"
  if "$fn"; then
    pass "$name"
  else
    echo "[gate-reason][CASE-FAIL] $name" >&2
  fi
}

run_case "代码失败分类" scenario_code_failure_goes_needs_fix_reason
run_case "网络抖动重试后恢复" scenario_network_transient_recovers_after_retry
run_case "网络抖动重试耗尽" scenario_network_transient_exhausts_retry
run_case "认证失败分类" scenario_auth_failed_no_retry
run_case "鉴权失败分类" scenario_forbidden_failed_no_retry
run_case "限流失败分类" scenario_rate_limited_no_retry

echo "[gate-reason] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
