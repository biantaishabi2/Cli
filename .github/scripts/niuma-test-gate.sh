#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[gate] $*"
}

usage() {
  echo "usage: $0 <pr-number>" >&2
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

CHANGED_FILES=$(gh pr view "$PR_NUMBER" --json files --jq '.files[].path')
RUN_GO=0
RUN_RUST=0

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  if [[ "$file" == automation/niuma/* ]]; then
    RUN_GO=1
  fi
  if [[ "$file" == compiler/bcc/* ]]; then
    RUN_RUST=1
  fi
done <<< "$CHANGED_FILES"

log "run_go=$RUN_GO, run_rust=$RUN_RUST"

if [[ "$RUN_GO" -eq 0 && "$RUN_RUST" -eq 0 ]]; then
  log "no niuma/bcc changes detected, skip project gates"
  exit 0
fi

if [[ "$RUN_GO" -eq 1 ]]; then
  log "running Go tests for automation/niuma"
  (
    cd automation/niuma
    go test ./...
  )
fi

if [[ "$RUN_RUST" -eq 1 ]]; then
  log "running Rust compile gate for bcc"
  cargo test -p bcc --no-run
fi

log "all required checks passed"
