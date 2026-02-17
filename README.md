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
  **BDD 测试运行时**：执行 BCC 生成的 BDD 场景，实现"测试即文档"。
  与 BCC 配合形成闭环：BCC 生成场景 → BDDC 执行测试 → 报告结果。

- [**`bcc/`**](compiler/bcc/README.md)（Rust + Elixir emit，已就绪）
  **代码知识图谱 + 架构治理工具**：从代码提取结构（函数/类/模块/调用关系），
  持久化为可查询图谱，支持架构验证和测试生成。
  
  核心价值：
  - **代码图谱**：extract → 持久化到 SQLite → 图搜索（caller/callee/继承/模块依赖）
  - **架构治理**：arch validate 检测分层违规（如 api→dao 跳过 service）
  - **测试生成**：bugfix 历史 → BDD 场景 → BDDC 执行
  
  典型链路：
  - Greenfield：`compile -> arch matrix -> arch validate -> bdd seed`
  - Brownfield：`extract -> graph-index build -> arch validate -> bugfix`
  - **案例参考**: [`compiler/bcc/examples/openclaw-arch/`](compiler/bcc/examples/openclaw-arch/)

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
