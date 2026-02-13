# Skills

This repository stores agent-facing skills for CLI tools.

## Important

These files are for robots/agents to execute consistent workflows. They are not product docs.

## Skill Structure

Skills are grouped by category and tool name:

- `skills/<category>/<tool>/SKILL.md`
- `skills/<category>/<tool>/agents/openai.yaml`

Current catalog:

- `skills/orchestration/taskctl/`

See full catalog and conventions in `skills/README.md`.

## Install A Skill To Local Codex

```bash
mkdir -p ~/.codex/skills
cp -R skills/orchestration/taskctl ~/.codex/skills/taskctl
```

For active development, use a symlink:

```bash
mkdir -p ~/.codex/skills
ln -sfn "$(pwd)/skills/orchestration/taskctl" ~/.codex/skills/taskctl
```

## Use

Reference the skill in prompts, for example: `use taskctl skill`.
