#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

cat > "$ROOT_DIR/.github/workflows/niuma-plan.yml" <<'YAML'
name: niuma - Plan Draft

on:
  issues:
    types: [labeled]

permissions:
  issues: write
  contents: read

jobs:
  route-event:
    runs-on: self-hosted
    outputs:
      decision: ${{ steps.route.outputs.decision }}
      reason: ${{ steps.route.outputs.reason }}
      action: ${{ steps.route.outputs.action }}
    steps:
      - name: Setup niuma binary
        run: |
          niuma_path="$(command -v niuma || true)"
          if [ -z "$niuma_path" ] && [ -x "/usr/local/bin/niuma" ]; then
            niuma_path="/usr/local/bin/niuma"
          fi
          if [ -z "$niuma_path" ]; then
            echo "::error::找不到 niuma 二进制，请确保 runner 上已安装 niuma（PATH=$PATH）"
            exit 1
          fi
          echo "NIUMA_BIN=$niuma_path" >> "$GITHUB_ENV"

      - name: Route Event
        id: route
        env:
          REPO: ${{ github.repository }}
        run: |
          output="$($NIUMA_BIN control route-event \
            --repo "$REPO" \
            --workflow "plan" \
            --event-name "$GITHUB_EVENT_NAME" \
            --event-path "$GITHUB_EVENT_PATH")"
          echo "$output"
          echo "decision=$(echo "$output" | awk -F= '/^decision=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "reason=$(echo "$output" | awk -F= '/^reason=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "action=$(echo "$output" | awk -F= '/^action=/{print $2; exit}')" >> "$GITHUB_OUTPUT"

  plan-draft:
    needs: route-event
    if: needs.route-event.outputs.decision == 'run' && needs.route-event.outputs.action == 'plan'
    uses: ./.github/workflows/niuma-plan-reusable.yml
    with:
      repo: ${{ github.repository }}
      issue_number: ${{ format('{0}', github.event.issue.number) }}
    secrets: inherit
YAML

cat > "$ROOT_DIR/.github/workflows/niuma-implement.yml" <<'YAML'
name: niuma - Implement

on:
  issues:
    types: [labeled]

permissions:
  issues: write
  pull-requests: write
  contents: write

jobs:
  route-event:
    runs-on: self-hosted
    outputs:
      decision: ${{ steps.route.outputs.decision }}
      reason: ${{ steps.route.outputs.reason }}
      action: ${{ steps.route.outputs.action }}
    steps:
      - name: Setup niuma binary
        run: |
          niuma_path="$(command -v niuma || true)"
          if [ -z "$niuma_path" ] && [ -x "/usr/local/bin/niuma" ]; then
            niuma_path="/usr/local/bin/niuma"
          fi
          if [ -z "$niuma_path" ]; then
            echo "::error::找不到 niuma 二进制，请确保 runner 上已安装 niuma（PATH=$PATH）"
            exit 1
          fi
          echo "NIUMA_BIN=$niuma_path" >> "$GITHUB_ENV"

      - name: Route Event
        id: route
        env:
          REPO: ${{ github.repository }}
        run: |
          output="$($NIUMA_BIN control route-event \
            --repo "$REPO" \
            --workflow "implement" \
            --event-name "$GITHUB_EVENT_NAME" \
            --event-path "$GITHUB_EVENT_PATH")"
          echo "$output"
          echo "decision=$(echo "$output" | awk -F= '/^decision=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "reason=$(echo "$output" | awk -F= '/^reason=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "action=$(echo "$output" | awk -F= '/^action=/{print $2; exit}')" >> "$GITHUB_OUTPUT"

  implement:
    needs: route-event
    if: needs.route-event.outputs.decision == 'run' && needs.route-event.outputs.action == 'implement'
    uses: ./.github/workflows/niuma-implement-reusable.yml
    with:
      repo: ${{ github.repository }}
      issue_number: ${{ format('{0}', github.event.issue.number) }}
      gate_max_retries: ${{ vars.NIUMA_INTEGRATION_GATE_MAX_RETRIES || '2' }}
      trigger_label: ${{ github.event.label.name }}
    secrets: inherit
YAML

cat > "$ROOT_DIR/.github/workflows/niuma-review.yml" <<'YAML'
name: niuma - Review

on:
  issues:
    types: [labeled]

permissions:
  issues: write
  pull-requests: write

jobs:
  route-event:
    runs-on: self-hosted
    outputs:
      decision: ${{ steps.route.outputs.decision }}
      reason: ${{ steps.route.outputs.reason }}
      action: ${{ steps.route.outputs.action }}
    steps:
      - name: Setup niuma binary
        run: |
          niuma_path="$(command -v niuma || true)"
          if [ -z "$niuma_path" ] && [ -x "/usr/local/bin/niuma" ]; then
            niuma_path="/usr/local/bin/niuma"
          fi
          if [ -z "$niuma_path" ]; then
            echo "::error::找不到 niuma 二进制，请确保 runner 上已安装 niuma（PATH=$PATH）"
            exit 1
          fi
          echo "NIUMA_BIN=$niuma_path" >> "$GITHUB_ENV"

      - name: Route Event
        id: route
        env:
          REPO: ${{ github.repository }}
        run: |
          output="$($NIUMA_BIN control route-event \
            --repo "$REPO" \
            --workflow "review" \
            --event-name "$GITHUB_EVENT_NAME" \
            --event-path "$GITHUB_EVENT_PATH")"
          echo "$output"
          echo "decision=$(echo "$output" | awk -F= '/^decision=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "reason=$(echo "$output" | awk -F= '/^reason=/{print $2; exit}')" >> "$GITHUB_OUTPUT"
          echo "action=$(echo "$output" | awk -F= '/^action=/{print $2; exit}')" >> "$GITHUB_OUTPUT"

  review:
    needs: route-event
    if: needs.route-event.outputs.decision == 'run' && needs.route-event.outputs.action == 'review'
    uses: ./.github/workflows/niuma-review-reusable.yml
    with:
      repo: ${{ github.repository }}
      issue_number: ${{ format('{0}', github.event.issue.number) }}
    secrets: inherit
YAML

echo "rendered: .github/workflows/niuma-plan.yml"
echo "rendered: .github/workflows/niuma-implement.yml"
echo "rendered: .github/workflows/niuma-review.yml"
