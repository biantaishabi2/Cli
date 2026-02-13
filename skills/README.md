# Skills Catalog (For Agents)

This directory is for agent-facing skill definitions, not end-user feature docs.

## Layout Convention

Use two-level layout for scale:

- `skills/<category>/<tool>/SKILL.md`
- `skills/<category>/<tool>/agents/openai.yaml`

Each CLI tool should map to exactly one skill folder under a category.

## Current Skills

- Category `orchestration`
  - Tool `taskctl`
    - Path: `skills/orchestration/taskctl/`
    - Purpose: task orchestration + DAG validation + graph views
  - Tool `github-issue-taskctl`
    - Path: `skills/orchestration/github-issue-taskctl/`
    - Purpose: GitHub Issue/PR operation and synchronization with taskctl task states

## Category Guidelines

- `orchestration`: task planning, DAG, workflow state transitions
- `scaffolding`: code skeleton generation and compiler workflows
- `ops`: deployment, release, runtime operations
- `quality`: testing, linting, verification utilities

## Agent Usage Model

1. Select the tool-specific skill by path and name.
2. Execute commands exactly from the skill command patterns.
3. Keep state in tool data files (not in transient memory only).
4. Run validation command before returning success.
