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

  mkdir -p docs
  echo "base" > docs/sample.txt
  git add docs/sample.txt
  git commit -m "base" >/dev/null
  local base_sha
  base_sha=$(git rev-parse HEAD)

  git checkout -b feature >/dev/null
  echo "feature" >> docs/sample.txt
  git add docs/sample.txt
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

test_merge_ref_success() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 201 yes)
  IFS='|' read -r base_sha head_sha merge_sha work_dir <<< "$repo_info"

  set +e
  (
    cd "$work_dir"
    PATH="$sandbox/bin:$PATH" \
    GITHUB_TOKEN=token \
    MOCK_BASE_REF=master \
    MOCK_HEAD_REF=feature \
    MOCK_BASE_SHA="$base_sha" \
    MOCK_HEAD_SHA="$head_sha" \
    "$SCRIPT_UNDER_TEST" 201
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    fail "merge ref 命中场景应成功 (status=$status)"
    return 1
  fi
  assert_contains "$sandbox/stdout" "baseline=merge-result" "应输出 merge-result 口径" || return 1
  assert_contains "$sandbox/stdout" "merge_ref_source=github-merge-ref" "应走 github merge ref" || return 1
  assert_contains "$sandbox/stdout" "merge_sha=$merge_sha" "merge_sha 应匹配 merge ref" || return 1
  return 0
}

test_local_merge_fallback() {
  local sandbox
  sandbox=$(mktemp -d)
  mkdir -p "$sandbox/bin"
  make_gh_mock "$sandbox/bin"

  local repo_info
  repo_info=$(init_repo "$sandbox" 202 no)
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
    "$SCRIPT_UNDER_TEST" 202
  ) >"$sandbox/stdout" 2>"$sandbox/stderr"
  local status=$?
  set -e

  if [[ "$status" -ne 0 ]]; then
    fail "merge ref 不可用时本地 merge 兜底应成功 (status=$status)"
    return 1
  fi
  assert_contains "$sandbox/stdout" "merge_ref_source=local-merge" "应走 local-merge 兜底" || return 1
  if ! grep -Eq 'merge_sha=[0-9a-f]{40}' "$sandbox/stdout"; then
    fail "local-merge 兜底应产出 merge_sha"
    return 1
  fi
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

run_case "github merge ref 成功路径" test_merge_ref_success
run_case "merge ref 不可用本地兜底" test_local_merge_fallback

echo "[integration] pass=$PASS_COUNT fail=$FAIL_COUNT"
if [[ "$FAIL_COUNT" -ne 0 ]]; then
  exit 1
fi
