#!/usr/bin/env bats

setup() {
  TEST_ROOT="$(mktemp -d "${BATS_TEST_TMPDIR}/cleanup-worktrees.XXXXXX")"
  REMOTE_REPO="$TEST_ROOT/remote.git"
  REPO_DIR="$TEST_ROOT/repo"
  TMPDIR="$TEST_ROOT/tmp"
  export TMPDIR
  mkdir -p "$TMPDIR"

  git init -b master "$REPO_DIR" >/dev/null
  git init --bare "$REMOTE_REPO" >/dev/null

  git -C "$REPO_DIR" config user.name "cleanup-test"
  git -C "$REPO_DIR" config user.email "cleanup-test@example.com"

  cat > "$REPO_DIR/README.md" <<'TXT'
base
TXT
  git -C "$REPO_DIR" add README.md
  git -C "$REPO_DIR" commit -m "init" >/dev/null
  git -C "$REPO_DIR" remote add origin "$REMOTE_REPO"
  git -C "$REPO_DIR" push -u origin master >/dev/null

  SCRIPT_UNDER_TEST="$BATS_TEST_DIRNAME/../cleanup_worktrees.sh"
}

teardown() {
  rm -rf "$TEST_ROOT"
}

create_feature_worktree() {
  local branch="$1"
  local filename="$2"

  git -C "$REPO_DIR" checkout -b "$branch" master >/dev/null
  echo "$branch" > "$REPO_DIR/$filename"
  git -C "$REPO_DIR" add "$filename"
  git -C "$REPO_DIR" commit -m "feat: $branch" >/dev/null
  git -C "$REPO_DIR" push -u origin "$branch" >/dev/null

  git -C "$REPO_DIR" checkout master >/dev/null

  local safe_branch
  safe_branch="${branch//\//-}"
  local worktree_path="$TEST_ROOT/worktree-$safe_branch"
  git -C "$REPO_DIR" worktree add "$worktree_path" "$branch" >/dev/null

  LAST_WORKTREE_PATH="$worktree_path"
}

merge_branch_into_master() {
  local branch="$1"

  git -C "$REPO_DIR" checkout master >/dev/null
  git -C "$REPO_DIR" merge --no-ff "$branch" -m "merge $branch" >/dev/null
  git -C "$REPO_DIR" push origin master >/dev/null
}

delete_remote_branch() {
  local branch="$1"
  git -C "$REPO_DIR" push origin --delete "$branch" >/dev/null
}

extract_summary_json() {
  local output_text="$1"
  printf '%s\n' "$output_text" | grep '^\[cleanup.summary\] ' | sed 's/^\[cleanup.summary\] //'
}

@test "远端已删 + clean + 已合入: 删除 worktree 和本地分支" {
  local branch="fix/A"
  create_feature_worktree "$branch" "a.txt"
  local worktree_path="$LAST_WORKTREE_PATH"

  merge_branch_into_master "$branch"
  delete_remote_branch "$branch"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 0 ]
  [ ! -d "$worktree_path" ]
  ! git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$branch"

  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.deleted | length' <<<"$summary_json")" -eq 1 ]
  [ "$(jq -r '.errors | length' <<<"$summary_json")" -eq 0 ]
}

@test "远端已删 + dirty: 仅 warning 跳过" {
  local branch="fix/B"
  create_feature_worktree "$branch" "b.txt"
  local worktree_path="$LAST_WORKTREE_PATH"

  merge_branch_into_master "$branch"
  delete_remote_branch "$branch"

  echo "dirty" > "$worktree_path/dirty.txt"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 0 ]
  [ -d "$worktree_path" ]
  git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$branch"

  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.warned | length' <<<"$summary_json")" -eq 1 ]
  [ "$(jq -r '.warned[0].reason' <<<"$summary_json")" = "worktree_dirty" ]
}

@test "远端存在: 跳过删除" {
  local branch="fix/C"
  create_feature_worktree "$branch" "c.txt"
  local worktree_path="$LAST_WORKTREE_PATH"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 0 ]
  [ -d "$worktree_path" ]
  git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$branch"

  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.skipped | map(select(.reason == "remote_exists")) | length' <<<"$summary_json")" -eq 1 ]
  [ "$(jq -r '.deleted | length' <<<"$summary_json")" -eq 0 ]
}

@test "未合入保护: 远端已删 + clean + 未合入时不删除" {
  local branch="fix/D"
  create_feature_worktree "$branch" "d.txt"
  local worktree_path="$LAST_WORKTREE_PATH"

  delete_remote_branch "$branch"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 0 ]
  [ -d "$worktree_path" ]
  git -C "$REPO_DIR" show-ref --verify --quiet "refs/heads/$branch"

  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.warned | map(select(.reason == "not_merged_to_master")) | length' <<<"$summary_json")" -eq 1 ]
  [ "$(jq -r '.deleted | length' <<<"$summary_json")" -eq 0 ]
}

@test "重复执行幂等: 第二次执行不失败且不再删除" {
  local branch="fix/E"
  create_feature_worktree "$branch" "e.txt"

  merge_branch_into_master "$branch"
  delete_remote_branch "$branch"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"
  [ "$status" -eq 0 ]

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"
  [ "$status" -eq 0 ]

  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.deleted | length' <<<"$summary_json")" -eq 0 ]
  [ "$(jq -r '.errors | length' <<<"$summary_json")" -eq 0 ]
}

@test "fetch 失败: 记录 fetch_prune_failed 并返回失败" {
  git -C "$REPO_DIR" remote set-url origin "$TEST_ROOT/missing-remote.git"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 1 ]
  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.errors | map(select(.reason == "fetch_prune_failed")) | length' <<<"$summary_json")" -eq 1 ]
}

@test "base ref 不存在: 记录 base_ref_not_found 并返回失败" {
  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR" --base-ref origin/not-exists

  [ "$status" -eq 1 ]
  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.errors | map(select(.reason == "base_ref_not_found")) | length' <<<"$summary_json")" -eq 1 ]
}

@test "worktree 路径缺失: 记录 warning 并跳过" {
  local branch="fix/F"
  create_feature_worktree "$branch" "f.txt"
  local worktree_path="$LAST_WORKTREE_PATH"

  merge_branch_into_master "$branch"
  delete_remote_branch "$branch"
  rm -rf "$worktree_path"

  run "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR"

  [ "$status" -eq 0 ]
  summary_json="$(extract_summary_json "$output")"
  [ "$(jq -r '.warned | map(select(.reason == "worktree_path_missing")) | length' <<<"$summary_json")" -eq 1 ]
}

@test "参数错误: 缺失值与未知参数返回 exit 2" {
  run "$SCRIPT_UNDER_TEST" --repo-dir
  [ "$status" -eq 2 ]
  [[ "$output" == *"missing value for --repo-dir"* ]]

  run "$SCRIPT_UNDER_TEST" --unknown-arg
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown argument: --unknown-arg"* ]]
}

@test "并发执行: 重叠触发时至少一次成功且仓库可用" {
  local branch="fix/G"
  create_feature_worktree "$branch" "g.txt"

  merge_branch_into_master "$branch"
  delete_remote_branch "$branch"

  local out1="$TEST_ROOT/run-1.log"
  local out2="$TEST_ROOT/run-2.log"
  "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR" >"$out1" 2>&1 &
  local pid1="$!"
  "$SCRIPT_UNDER_TEST" --repo-dir "$REPO_DIR" >"$out2" 2>&1 &
  local pid2="$!"

  local status1=0
  local status2=0
  wait "$pid1" || status1="$?"
  wait "$pid2" || status2="$?"

  [[ "$status1" -eq 0 || "$status2" -eq 0 ]]
  [[ "$status1" -le 1 ]]
  [[ "$status2" -le 1 ]]
  git -C "$REPO_DIR" rev-parse --verify --quiet master
}
