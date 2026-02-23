#!/usr/bin/env bash
set -euo pipefail

# 渲染入口工作流：将模板拷贝到 .github/workflows
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

for name in "${FILES[@]}"; do
  src="$TEMPLATE_DIR/${name}-entry.yml.tmpl"
  dst="$WORKFLOW_DIR/${name}.yml"
  if [[ ! -f "$src" ]]; then
    echo "missing template: $src" >&2
    exit 1
  fi
  cp "$src" "$dst"
  echo "rendered: $dst"
done

