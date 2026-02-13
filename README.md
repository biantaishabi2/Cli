# Cli

Monorepo for standalone CLI tools.

## Included Tool

- `taskctl/`: task orchestration CLI with dependency DAG support

## Local Development

```bash
cd taskctl
cargo test
cargo run -- --help
```

## Build Release Artifact

```bash
./scripts/build-release.sh v0.1.0
```

Generated files:

- `dist/taskctl-v0.1.0-<target>.tar.gz`
- `dist/taskctl-v0.1.0-<target>.tar.gz.sha256`

Supported `<target>` values:

- `darwin-arm64`
- `darwin-x86_64`
- `linux-x86_64`

## Install From GitHub Release

```bash
./scripts/install.sh v0.1.0
```

Or install into a custom directory:

```bash
INSTALL_DIR=/usr/local/bin ./scripts/install.sh v0.1.0
```

## Codex Skill

Repository includes a reusable skill definition for `taskctl`.

- Guide: `docs/SKILLS.md`
- Skill files: `skills/taskctl/`
