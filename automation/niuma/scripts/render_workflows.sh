#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TEMPLATE="$ROOT_DIR/automation/niuma/workflows/templates/niuma-entry.yml.tmpl"
TEMPLATE_ORCHESTRATE="$ROOT_DIR/automation/niuma/workflows/templates/niuma-orchestrate-entry.yml.tmpl"
TEMPLATE_ITERATE="$ROOT_DIR/automation/niuma/workflows/templates/niuma-iterate-entry.yml.tmpl"
TEMPLATE_DISCUSS="$ROOT_DIR/automation/niuma/workflows/templates/niuma-discuss-entry.yml.tmpl"
TEMPLATE_DISPATCH="$ROOT_DIR/automation/niuma/workflows/templates/niuma-dispatch-completed.yml.tmpl"

if [ ! -f "$TEMPLATE" ]; then
  echo "template not found: $TEMPLATE" >&2
  exit 1
fi

render_entry_workflow() {
  local output="$1"
  local name="$2"
  local display_name="$3"
  local workflow="$4"
  local action="$5"
  local reusable="$6"
  local job_name="$7"
  local permissions="$8"
  local with_extra="${9:-}"

  python3 - "$TEMPLATE" "$output" "$name" "$display_name" "$workflow" "$action" "$reusable" "$job_name" "$permissions" "$with_extra" <<'PY'
import pathlib
import sys

template_path = pathlib.Path(sys.argv[1])
output_path = pathlib.Path(sys.argv[2])

content = template_path.read_text(encoding="utf-8")
replacements = {
    "__NAME__": sys.argv[3],
    "__DISPLAY_NAME__": sys.argv[4],
    "__WORKFLOW__": sys.argv[5],
    "__ACTION__": sys.argv[6],
    "__REUSABLE__": sys.argv[7],
    "__JOB_NAME__": sys.argv[8],
    "__PERMISSIONS__": sys.argv[9],
    "__WITH_EXTRA__": sys.argv[10],
}
for key, value in replacements.items():
    content = content.replace(key, value)
output_path.write_text(content, encoding="utf-8")
PY
}

render_entry_workflow \
  "$ROOT_DIR/.github/workflows/niuma-plan.yml" \
  "plan" \
  "niuma - Plan Draft" \
  "plan" \
  "plan" \
  "niuma-plan-reusable.yml" \
  "plan-draft" \
  $'  issues: write\n  contents: read'

implement_with_extra="$(cat <<'EOT'
      gate_max_retries: ${{ vars.NIUMA_INTEGRATION_GATE_MAX_RETRIES || '2' }}
      default_pr_run_mode: ${{ vars.DEFAULT_PR_RUN_MODE || 'full' }}
      critical_regression_required: ${{ vars.CRITICAL_REGRESSION_REQUIRED || 'true' }}
      infra_retry_max: ${{ vars.INFRA_RETRY_MAX || '2' }}
      high_risk_paths: ${{ vars.HIGH_RISK_PATHS || '' }}
      trigger_label: ${{ github.event.label.name }}
EOT
)"

render_entry_workflow \
  "$ROOT_DIR/.github/workflows/niuma-implement.yml" \
  "implement" \
  "niuma - Implement" \
  "implement" \
  "implement" \
  "niuma-implement-reusable.yml" \
  "implement" \
  $'  issues: write\n  pull-requests: write\n  contents: write' \
  "$implement_with_extra"

review_with_extra="$(cat <<'EOT'
      default_pr_run_mode: ${{ vars.DEFAULT_PR_RUN_MODE || 'full' }}
      critical_regression_required: ${{ vars.CRITICAL_REGRESSION_REQUIRED || 'true' }}
      infra_retry_max: ${{ vars.INFRA_RETRY_MAX || '2' }}
      high_risk_paths: ${{ vars.HIGH_RISK_PATHS || '' }}
EOT
)"

render_entry_workflow \
  "$ROOT_DIR/.github/workflows/niuma-review.yml" \
  "review" \
  "niuma - Review" \
  "review" \
  "review" \
  "niuma-review-reusable.yml" \
  "review" \
  $'  issues: write\n  pull-requests: write' \
  "$review_with_extra"

cp "$TEMPLATE_ORCHESTRATE" "$ROOT_DIR/.github/workflows/niuma-orchestrate.yml"
cp "$TEMPLATE_ITERATE" "$ROOT_DIR/.github/workflows/niuma-iterate.yml"
cp "$TEMPLATE_DISCUSS" "$ROOT_DIR/.github/workflows/niuma-discuss.yml"
cp "$TEMPLATE_DISPATCH" "$ROOT_DIR/.github/workflows/niuma-dispatch-completed.yml"

echo "rendered: .github/workflows/niuma-plan.yml"
echo "rendered: .github/workflows/niuma-implement.yml"
echo "rendered: .github/workflows/niuma-review.yml"
echo "rendered: .github/workflows/niuma-orchestrate.yml"
echo "rendered: .github/workflows/niuma-iterate.yml"
echo "rendered: .github/workflows/niuma-discuss.yml"
echo "rendered: .github/workflows/niuma-dispatch-completed.yml"
