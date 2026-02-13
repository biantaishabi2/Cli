# Cli

这是一个承载多个命令行工具的仓库。  
当前先放 `taskctl`，后续在同一仓库持续新增其他工具并按分类管理。

## 目录：工具总览

### `orchestration/`
- `taskctl/`（已就绪）  
  任务编排 CLI：支持任务增删改查、依赖关系（blockedBy/blocks）、校验、DAG 生成与导出，适配 Agent 工作流。

## 快速上手

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

仓库内提供给 Codex/Agent 使用的技能定义（Skill）：

- Guide: `docs/SKILLS.md`
- Skill catalog: `skills/README.md`
- Skill files: `skills/orchestration/taskctl/`
