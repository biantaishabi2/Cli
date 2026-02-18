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
  mkdir -p "$sandbox/bin" "$sandbox/work"

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
      printf '%b' "${MOCK_MERGE_OUTPUT:-CONFLICT (content): Merge conflict in conflict.txt\nAutomatic merge failed; fix conflicts and then commit the result.\n}" >&2
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

  cat > "$sandbox/bin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  cat > "$sandbox/bin/cargo" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
exit 0
SH

  chmod +x "$sandbox/bin/gh" "$sandbox/bin/git" "$sandbox/bin/go" "$sandbox/bin/cargo"
  echo "$sandbox"
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

test_prefers_github_merge_ref() {
  local sandbox
  sandbox=$(new_sandbox)

  local git_log="$sandbox/git.log"
  set +e
  PATH="$sandbox/bin:$PATH" \
  GITHUB_TOKEN=token \
  MOCK_GIT_LOG="$git_log" \
  MOCK_FETCH_MERGE_REF=ok \
  MOCK_BASE_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  MOCK_HEAD_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
  MOCK_FETCH_HEAD_SHA=cccccccccccccccccccccccccccccccccccccccc \
  "$SCRIPT_UNDER_TEST" 123 >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "0" "$status" "merge ref 可用时应成功" || return 1
  assert_contains "$sandbox/stdout" "baseline=merge-result" "应输出统一口径日志" || return 1
  assert_contains "$sandbox/stdout" "merge_ref_source=github-merge-ref" "应优先使用 github merge ref" || return 1
  assert_contains "$sandbox/stdout" "base_sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" "应输出 base sha" || return 1
  assert_contains "$sandbox/stdout" "head_sha=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" "应输出 head sha" || return 1
  assert_contains "$sandbox/stdout" "merge_sha=cccccccccccccccccccccccccccccccccccccccc" "应输出 merge sha" || return 1
  assert_not_contains "$git_log" " merge --no-ff --no-edit " "不应走本地 merge" || return 1
  return 0
}

test_conflict_fail_and_files() {
  local sandbox
  sandbox=$(new_sandbox)

  set +e
  PATH="$sandbox/bin:$PATH" \
  GITHUB_TOKEN=token \
  MOCK_FETCH_MERGE_REF=fail \
  MOCK_MERGE_STATUS=1 \
  MOCK_CONFLICT_FILES=$'conflict/a.txt\nconflict/b.txt\n' \
  "$SCRIPT_UNDER_TEST" 456 >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  assert_eq "1" "$status" "本地 merge 冲突应失败" || return 1
  assert_contains "$sandbox/stderr" "CONFLICT:" "冲突日志需带 CONFLICT 前缀" || return 1
  assert_contains "$sandbox/stderr" "conflict/a.txt" "应输出冲突文件 a" || return 1
  assert_contains "$sandbox/stderr" "conflict/b.txt" "应输出冲突文件 b" || return 1
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
run_case "merge ref 优先" test_prefers_github_merge_ref
run_case "冲突失败语义" test_conflict_fail_and_files

echo "[unit] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
