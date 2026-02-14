# Cli

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml)
[![Release](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml)

承载多个命令行工具的 monorepo，按职责分类管理。

## 目录：工具总览

### `orchestration/`
- **`taskctl/`**（Rust，已就绪）
  任务编排 CLI：支持任务增删改查、依赖关系（blockedBy/blocks）、校验、DAG 生成与导出，适配 Agent 工作流。

### `compiler/`
- **`bddc/`**（Elixir escript，已就绪）
  BDD 编译器：DSL 解析 → 指令集生成 → 运行时覆盖校验 → 测试代码生成。从 shop 项目迁入。

- **`bcc/`**（Rust + Elixir emit，PoC 已完成）
  后端编译器：三命令（compile/extract/trace）已跑通端到端闭环。compile: YAML → AST → Pass Pipeline → `.ex`；extract: 源码 → FileRecord JSON；trace: 文档覆盖审计。

## 快速上手

```bash
# 构建所有 Rust 工具（workspace 根目录）
cargo build --release
./target/release/taskctl --help
./target/release/bcc --help

# bcc 快速验证
./target/release/bcc compile compiler/bcc/fixtures/session_service.yaml --dry-run
./target/release/bcc extract compiler/bcc/fixtures/sample_service.ex --mode ast
./target/release/bcc trace status compiler/bcc/fixtures/trace_project/src compiler/bcc/fixtures/trace_project/docs/backend-trace/files/src

# bddc（Elixir escript）
cd compiler/bddc
mix deps.get
mix escript.build
./bdd_compiler --help
```

## Build Release Artifact

```bash
./scripts/build-release.sh v0.1.0
```

Generated files:

- `dist/taskctl-v0.1.0-<target>.tar.gz`
- `dist/taskctl-v0.1.0-<target>.tar.gz.sha256`

Supported `<target>` values:

- `linux-x86_64`
- `darwin-arm64`

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
- Skill files:
  - `skills/orchestration/taskctl/`
  - `skills/orchestration/github-issue-taskctl/`

## Agent Workflow

- `docs/workflows/github-issue-taskctl-loop.md`
