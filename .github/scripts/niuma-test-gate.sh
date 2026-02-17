#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <pr-number>" >&2
  exit 2
fi

PR_NUMBER="$1"

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "GITHUB_TOKEN is required" >&2
  exit 2
fi

HEAD_REF=$(gh pr view "$PR_NUMBER" --json headRefName -q '.headRefName')
if [[ -z "$HEAD_REF" ]]; then
  echo "unable to resolve PR head ref for #$PR_NUMBER" >&2
  exit 1
fi

echo "[gate] checkout PR #$PR_NUMBER head: $HEAD_REF"
git fetch origin "$HEAD_REF"
git checkout -B "niuma-gate-$PR_NUMBER" "origin/$HEAD_REF"

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

echo "[gate] run_go=$RUN_GO, run_rust=$RUN_RUST"

if [[ "$RUN_GO" -eq 0 && "$RUN_RUST" -eq 0 ]]; then
  echo "[gate] no niuma/bcc changes detected, skip project gates"
  exit 0
fi

if [[ "$RUN_GO" -eq 1 ]]; then
  echo "[gate] running Go tests for automation/niuma"
  (
    cd automation/niuma
    go test ./...
  )
fi

if [[ "$RUN_RUST" -eq 1 ]]; then
  echo "[gate] running Rust compile gate for bcc"
  cargo test -p bcc --no-run
fi

echo "[gate] all required checks passed"
