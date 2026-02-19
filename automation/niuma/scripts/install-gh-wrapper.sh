#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_WRAPPER="$SCRIPT_DIR/gh"
TARGET_BIN="${TARGET_BIN:-$HOME/.local/bin/gh}"
DRY_RUN="${DRY_RUN:-false}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--target <path>] [--dry-run]

Install niuma gh wrapper to the target path (default: ~/.local/bin/gh).

Options:
  --target <path>   Install target path (default: ~/.local/bin/gh)
  --dry-run         Print actions only, do not write files
  -h, --help        Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      shift
      if [[ $# -eq 0 ]]; then
        echo "missing value for --target" >&2
        exit 2
      fi
      TARGET_BIN="$1"
      ;;
    --dry-run)
      DRY_RUN=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [[ ! -f "$SOURCE_WRAPPER" ]]; then
  echo "wrapper not found: $SOURCE_WRAPPER" >&2
  exit 1
fi

TARGET_DIR="$(dirname "$TARGET_BIN")"

log() {
  echo "[install-gh-wrapper] $*"
}

run_or_echo() {
  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY_RUN: $*"
  else
    eval "$@"
  fi
}

if [[ "$DRY_RUN" != "true" ]]; then
  mkdir -p "$TARGET_DIR"
else
  log "DRY_RUN: mkdir -p '$TARGET_DIR'"
fi

if [[ -e "$TARGET_BIN" ]]; then
  if cmp -s "$SOURCE_WRAPPER" "$TARGET_BIN"; then
    log "target already up to date: $TARGET_BIN"
  else
    BACKUP_PATH="$TARGET_BIN.bak.$(date +%Y%m%d%H%M%S)"
    if [[ "$DRY_RUN" == "true" ]]; then
      log "DRY_RUN: cp '$TARGET_BIN' '$BACKUP_PATH'"
    else
      cp "$TARGET_BIN" "$BACKUP_PATH"
    fi
    log "backed up existing binary: $BACKUP_PATH"

    if [[ "$DRY_RUN" == "true" ]]; then
      log "DRY_RUN: cp '$SOURCE_WRAPPER' '$TARGET_BIN'"
      log "DRY_RUN: chmod +x '$TARGET_BIN'"
    else
      cp "$SOURCE_WRAPPER" "$TARGET_BIN"
      chmod +x "$TARGET_BIN"
    fi
    log "installed wrapper: $TARGET_BIN"
  fi
else
  if [[ "$DRY_RUN" == "true" ]]; then
    log "DRY_RUN: cp '$SOURCE_WRAPPER' '$TARGET_BIN'"
    log "DRY_RUN: chmod +x '$TARGET_BIN'"
  else
    cp "$SOURCE_WRAPPER" "$TARGET_BIN"
    chmod +x "$TARGET_BIN"
  fi
  log "installed wrapper: $TARGET_BIN"
fi

FIRST_GH="$(command -v gh || true)"
if [[ "$DRY_RUN" == "true" ]]; then
  log "DRY_RUN: current gh resolves to: ${FIRST_GH:-<none>}"
else
  log "current gh resolves to: ${FIRST_GH:-<none>}"
fi

if [[ "$TARGET_DIR" != */.local/bin ]]; then
  log "note: ensure '$TARGET_DIR' is before '/usr/bin' in PATH"
fi
