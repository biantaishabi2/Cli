---
name: taskctl
description: Use this skill to operate the `taskctl` CLI for task orchestration with dependency DAGs. Trigger this skill when users ask to create/update/list/delete tasks, set `blocked_by` or `blocks` dependencies, compute ready tasks, validate graph integrity, generate DAG JSON, or produce ASCII DAG views for planning and execution reviews.
---

# Taskctl

## Overview

Use `taskctl` from PATH.

Core constraints:

- status flow: `pending -> in_progress -> completed`
- `deleted` removes task permanently
- no self-dependency
- no missing dependency target
- no dependency cycle
- blocked tasks cannot move to `in_progress` or `completed`

## Workflow

1. Choose a store file path.
2. Create tasks with complete fields.
3. Update owner, metadata, and dependencies.
4. Run `ready` to identify executable tasks.
5. Run `dag` or `dag-ascii` for graph review.
6. Run `validate` before accepting task plan changes.

## Command Patterns

```bash
# help
taskctl --help

# create task
taskctl --store <STORE_JSON> create \
  --subject "Run tests" \
  --description "Execute backend tests and collect logs" \
  --active-form "Running tests" \
  --metadata '{"priority":"P1","module":"quality"}'

# update task content, owner, dependencies, status
taskctl --store <STORE_JSON> update \
  --task-id <TASK_ID> \
  --owner qa@team \
  --metadata '{"priority":"P0","obsolete":null}' \
  --add-blocked-by <DEP_1>,<DEP_2> \
  --status in-progress

# inspect graph
taskctl --store <STORE_JSON> ready
taskctl --store <STORE_JSON> dag
taskctl --store <STORE_JSON> dag-ascii

# validate graph invariants
taskctl --store <STORE_JSON> validate
```

## Field Semantics

- `subject`: imperative task title.
- `description`: context + acceptance criteria.
- `active_form`: progressive action text.
- `owner`: responsible human or agent.
- `metadata`: object payload; null value deletes key on update.
- `add_blocked_by`: dependency task IDs required before start.
- `add_blocks`: task IDs blocked by current task.

## Acceptance Checklist

1. Ensure `validate` returns `{ "ok": true }`.
2. Ensure `dag` reflects intended module sequencing.
3. Ensure `ready` includes only tasks with resolved dependencies.
4. Include both JSON DAG and ASCII DAG when a human review is requested.
