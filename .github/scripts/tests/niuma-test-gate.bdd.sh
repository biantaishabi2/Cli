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
  *)
    echo "unexpected gh args: $*" >&2
    exit 1
    ;;
esac
SH

  cat > "$bin_dir/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

status="${MOCK_GO_EXIT:-0}"
if [[ "$status" -ne 0 ]]; then
  echo "${MOCK_GO_MESSAGE:-go test failed}" >&2
  exit "$status"
fi
exit 0
SH

  cat > "$bin_dir/cargo" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

status="${MOCK_CARGO_EXIT:-0}"
if [[ "$status" -ne 0 ]]; then
  echo "${MOCK_CARGO_MESSAGE:-cargo test failed}" >&2
  exit "$status"
fi
exit 0
SH

  chmod +x "$bin_dir/gh" "$bin_dir/go" "$bin_dir/cargo"
}

init_repo() {
  local sandbox="$1"
  local pr_number="$2"
  local with_merge_ref="$3"
  local conflict_mode="$4"

  local origin="$sandbox/origin"
  local work="$sandbox/work"

  git init -b master "$origin" >/dev/null
  pushd "$origin" >/dev/null
  git config user.name "test"
  git config user.email "test@example.com"

  mkdir -p automation/niuma
  echo "placeholder" > automation/niuma/.keep

  if [[ "$conflict_mode" == "yes" ]]; then
    echo "line=base" > conflict.txt
    git add automation/niuma/.keep conflict.txt
    git commit -m "base" >/dev/null

    git checkout -b feature >/dev/null
    echo "line=feature" > conflict.txt
    git add conflict.txt
    git commit -m "feature-conflict" >/dev/null
    local head_sha
    head_sha=$(git rev-parse HEAD)

    git checkout master >/dev/null
    echo "line=master" > conflict.txt
    git add conflict.txt
    git commit -m "master-conflict" >/dev/null
    local base_sha
    base_sha=$(git rev-parse HEAD)

    popd >/dev/null

    git clone "$origin" "$work" >/dev/null
    printf '%s|%s|%s|%s\n' "$base_sha" "$head_sha" "" "$work"
    return 0
  fi

  echo "base" > docs.txt
  git add automation/niuma/.keep docs.txt
  git commit -m "base" >/dev/null
  local base_sha
  base_sha=$(git rev-parse HEAD)

  git checkout -b feature >/dev/null
  echo "feature" >> docs.txt
  git add docs.txt
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

scenario_merge_result_fail() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 301 yes no)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'automation/niuma/main.go\n' \
    MOCK_GO_EXIT=1 \
    "$SCRIPT_UNDER_TEST" 301
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "场景1应失败：merge-result 下测试失败必须返回非 0"
    return 1
  fi
  assert_contains "$sandbox/stdout" "baseline=merge-result" "场景1应输出 merge-result 口径" || return 1
  return 0
}

scenario_merge_result_pass() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 302 yes no)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'automation/niuma/main.go\n' \
    MOCK_GO_EXIT=0 \
    "$SCRIPT_UNDER_TEST" 302
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    fail "场景2应成功：merge-result 下测试通过必须返回 0"
    return 1
  fi
  assert_contains "$sandbox/stdout" "base_sha=$base_sha" "场景2应输出 base_sha" || return 1
  assert_contains "$sandbox/stdout" "head_sha=$head_sha" "场景2应输出 head_sha" || return 1
  return 0
}

scenario_merge_conflict_fail() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_mock_bin "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 303 no yes)
  IFS='|' read -r base_sha head_sha _ work_dir <<< "$repo_info"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    MOCK_FILES=$'README.md\n' \
    "$SCRIPT_UNDER_TEST" 303
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    fail "场景3应失败：merge 冲突必须直接失败"
    return 1
  fi
  assert_contains "$sandbox/stderr" "CONFLICT:" "场景3需输出 CONFLICT 前缀" || return 1
  assert_contains "$sandbox/stderr" "conflict.txt" "场景3需输出冲突文件清单" || return 1
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

run_case "Given merge-result 失败 Then gate 失败" scenario_merge_result_fail
run_case "Given merge-result 通过 Then gate 通过" scenario_merge_result_pass
run_case "Given merge 冲突 Then gate 冲突失败" scenario_merge_conflict_fail

echo "[bdd] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
