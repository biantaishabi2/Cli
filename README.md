# Cli

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml)
[![Release](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml)

承载多个命令行工具的 monorepo，按职责分类管理。

## 目录：工具总览

### `orchestration/`
- [**`taskctl/`**](orchestration/taskctl/README.md)（Rust，已就绪）
  任务编排 CLI：支持任务增删改查、依赖关系（blockedBy/blocks）、校验、DAG 生成与导出，适配 Agent 工作流。

### `compiler/`
- [**`bddc/`**](compiler/bddc/README.md)（Elixir escript，已就绪）
  BDD 编译器：DSL 解析 → 指令集生成 → 运行时覆盖校验 → 测试代码生成。从 shop 项目迁入。

- [**`bcc/`**](compiler/bcc/README.md)（Rust + Elixir emit，已就绪）
  后端编译器：六命令（compile/extract/trace/arch/bugfix/bdd seed）已就绪，覆盖新/旧代码闭环。
  典型链路：
  - Greenfield：`compile -> arch matrix -> arch validate -> bdd seed`
  - Brownfield：`extract -> arch validate -> export-module-map -> bugfix`
  - **案例参考**: [`compiler/bcc/examples/openclaw-arch/`](compiler/bcc/examples/openclaw-arch/) - 完整架构分析示例（1685 文件项目，含 v0→v3 版本演进）

### `automation/`
- [**`niuma/`**](automation/niuma/README.md)（Go，已就绪）
  AI 驱动的全自动开发机器人：Issue → Plan → Code → PR → Iterate。
  支持多 AI provider "左右互搏"、worktree 隔离、review-iterate 自动交流。
  给 Issue 加 `bot:fix` 标签即触发全流程，人只在 PR Review 阶段介入。

## 快速上手

```bash
# 构建所有 Rust 工具（workspace 根目录）
cargo build --release
./target/release/taskctl --help
./target/release/bcc --help

# bcc 安装到本地（symlink 模式，后续 cargo build 自动生效）
./compiler/bcc/install.sh --link --rebuild

# bcc 快速验证
bcc compile compiler/bcc/fixtures/session_service.yaml --dry-run
bcc extract compiler/bcc/fixtures/sample_service.ex --mode ast
bcc trace status compiler/bcc/fixtures/trace_project/src compiler/bcc/fixtures/trace_project/docs/backend-trace/files/src
bcc bugfix /path/to/repo -o output/ --lang elixir   # git bugfix → BDD 场景
# 使用 openclaw-arch 案例运行 arch 命令
cd compiler/bcc/examples/openclaw-arch
bcc arch matrix --seed-file seed/v3.target-matrix.yaml --ast-file artifacts/module_registry.json
bcc arch validate \
  --target seed/v3.target-matrix.yaml \
  --transition seed/v3.transition-matrix.yaml \
  --gates seed/v3.gates.yaml \
  --actual artifacts/relation_matrix.actual.json \
  --out-dir versions/v4-draft

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
