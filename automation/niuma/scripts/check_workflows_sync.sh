#!/usr/bin/env bash
set -euo pipefail

# 校验入口工作流是否与模板保持一致
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TEMPLATE_DIR="$ROOT_DIR/automation/niuma/workflows/templates"
WORKFLOW_DIR="$ROOT_DIR/.github/workflows"

FILES=(
  "niuma-plan"
  "niuma-implement"
  "niuma-review"
  "niuma-orchestrate"
  "niuma-iterate"
  "niuma-discuss"
)

drift=0
for name in "${FILES[@]}"; do
  src="$TEMPLATE_DIR/${name}-entry.yml.tmpl"
  dst="$WORKFLOW_DIR/${name}.yml"
  if [[ ! -f "$src" ]]; then
    echo "missing template: $src" >&2
    drift=1
    continue
  fi
  if [[ ! -f "$dst" ]]; then
    echo "missing workflow: $dst" >&2
    drift=1
    continue
  fi
  if ! cmp -s "$src" "$dst"; then
    echo "drift: $dst"
    drift=1
  fi
done

if [[ "$drift" -ne 0 ]]; then
  echo "workflow sync check failed" >&2
  exit 1
fi

echo "workflow sync check passed"

