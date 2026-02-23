#!/usr/bin/env bash
set -euo pipefail

# 统一入口：workflow 模板渲染/校验/发布
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPTS_DIR="$ROOT_DIR/automation/niuma/scripts"

usage() {
  cat <<'EOF'
usage:
  bash automation/niuma/scripts/workflows.sh render
  bash automation/niuma/scripts/workflows.sh check
  bash automation/niuma/scripts/workflows.sh publish --repo <owner/repo> [--branch <name>] [--message <msg>] [--source-dir <dir>]
EOF
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

cmd="$1"
shift

case "$cmd" in
  render)
    bash "$SCRIPTS_DIR/render_workflows.sh" "$@"
    ;;
  check)
    bash "$SCRIPTS_DIR/check_workflows_sync.sh" "$@"
    ;;
  publish)
    bash "$SCRIPTS_DIR/publish_workflows.sh" "$@"
    ;;
  *)
    echo "unknown subcommand: $cmd" >&2
    usage
    exit 1
    ;;
esac

