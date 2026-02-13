# Skills

This repository includes a reusable Codex skill for `taskctl`.

## Skill Location

- `skills/taskctl/SKILL.md`
- `skills/taskctl/agents/openai.yaml`

## What It Does

The skill standardizes how an agent uses `taskctl` to:

- create/update/list/delete tasks
- manage dependencies (`add-blocked-by`, `add-blocks`)
- compute `ready` tasks
- validate graph integrity
- output DAG JSON or ASCII views

## Install Skill To Local Codex

```bash
mkdir -p ~/.codex/skills
cp -R skills/taskctl ~/.codex/skills/taskctl
```

If you want updates to sync automatically during development, use a symlink instead:

```bash
mkdir -p ~/.codex/skills
ln -sfn "$(pwd)/skills/taskctl" ~/.codex/skills/taskctl
```

## Use Skill

After installation, request the skill by name in prompts, e.g. "use taskctl skill".
