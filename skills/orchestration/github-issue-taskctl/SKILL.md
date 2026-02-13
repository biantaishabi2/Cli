---
name: github-issue-taskctl
description: Use this skill for GitHub issue/PR workflow and for syncing task execution state between GitHub (issues/PRs) and taskctl. Trigger when users need to create/list/update/close issues or PRs, map issue/PR progress to taskctl task lifecycle, or ask how to do the same flow from CLI as from GitHub Web.
---

# GitHub Issue/PR + Taskctl Loop

## Required Tooling

- `gh` (required)
- `taskctl` (required): source of truth for executable task DAG and state
- `bddc` (optional): for behavior proof before marking task completed

## Core Rules

- Human-facing entry can be via GitHub Web UI or `gh`; CLI and UI are equivalent entry channels.
- Keep `taskctl` as execution source of truth; GitHub should only record traceable notes/comments and links.
- Task metadata should include at least:
  - `source: "github_issue"` or `"github_pr"`
  - `issue_id` / `pr_id`
  - `issue_url` / `pr_url`
  - `module` and `owner` if useful
- State alignment:
  - `taskctl status=in_progress` -> comment to issue/PR that work started
  - `taskctl status=completed` -> comment with evidence summary (tests, hash, artifacts)
  - do not close PR/Issue until required evidence is attached

## Common Workflow

1. List or select source ticket/PR in GitHub.
2. Create/update corresponding taskctl task with metadata.
3. Set dependencies (`add-blocked-by`) and run `validate` / `ready`.
4. Agent executes task and marks progress in taskctl.
5. Agent posts progress/complete comments back with artifact links.
6. Keep issue/PR open until acceptance evidence is verified.

## Command Patterns

### Issue operations

```bash
# Create
gh issue create --title "..." --body "..." --label "P1,backend" 

# List / view
gh issue list --state open
gh issue view <number> --comments --json number,title,state,labels,assignees

# Comment lifecycle (start/complete)
gh issue comment <number> --body "任务开始：<task-id>，当前状态 in_progress。"
gh issue comment <number> --body "任务完成：<task-id>，BDD 通过，见摘要..."

# Status sync
gh issue close <number>
gh issue reopen <number>
gh issue edit <number> --add-label "blocked"
gh issue edit <number> --remove-label "blocked"
```

### PR operations

```bash
gh pr list --state open --author @me
gh pr view <number> --comments --json number,title,state,checks
gh pr create --fill --base main --head <branch>
gh pr checks <number>
gh pr comment <number> --body "实现完成，等待 review。"
gh pr merge <number> --merge --subject "..."
```

### Taskctl bridge

```bash
taskctl --store <STORE_JSON> create \
  --subject "[ISS-<n>] ..." \
  --description "..." \
  --metadata '{"source":"github_issue","issue_id":"123","issue_url":"https://github.com/owner/repo/issues/123"}'

taskctl --store <STORE_JSON> update --task-id <TASK_ID> --status in-progress
taskctl --store <STORE_JSON> ready
taskctl --store <STORE_JSON> dag-ascii
taskctl --store <STORE_JSON> validate
```

## Acceptance Checklist

1. Issue/PR can be created and queried via `gh` from terminal.
2. Taskctl task has linkable Issue/PR metadata.
3. DAG and readiness are valid after each dependency change.
4. State comments are posted to GitHub with status and evidence references.
5. Completion comment appears in Issue/PR before considering done.
