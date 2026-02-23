#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

cd "$ROOT_DIR"

FILES=(
  .github/workflows/niuma-plan.yml
  .github/workflows/niuma-implement.yml
  .github/workflows/niuma-review.yml
  .github/workflows/niuma-orchestrate.yml
  .github/workflows/niuma-iterate.yml
  .github/workflows/niuma-discuss.yml
  .github/workflows/niuma-dispatch-completed.yml
)

before_hash="$(git ls-files "${FILES[@]}" | xargs sha256sum | sha256sum | awk '{print $1}')"

automation/niuma/scripts/render_workflows.sh >/tmp/niuma-render.log

after_hash="$(git ls-files "${FILES[@]}" | xargs sha256sum | sha256sum | awk '{print $1}')"

if [ "$before_hash" != "$after_hash" ]; then
  echo "workflow files are out of sync with template" >&2
  git --no-pager diff -- "${FILES[@]}" >&2 || true
  exit 1
fi

echo "workflow templates and generated files are in sync"
