# Cli

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml)
[![Release](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml)

> **LLM 时代的软件工程操作系统**：taskctl 编排复杂工作流，BCC 编译代码结构为可验证的知识图谱，niuma 实现从需求到合并的全自动开发，BDDC 执行行为驱动测试。人定义规则和目标，机器处理执行和验证。
>
> 📖 详细哲学阐述见 [`PHILOSOPHY.md`](PHILOSOPHY.md)

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
  **架构编译器**：把架构约束当作语法规则来检查。
  从代码提取结构，构建知识图谱，验证架构合规性，生成治理报告。
  
  核心价值：
  - **代码知识图谱**：extract → SQLite 持久化 → 图搜索（caller/callee/继承/模块依赖）
  - **架构门禁**：arch validate 检测分层违规（如 api→dao 跳过 service），CI 自动拦截
  - **影响分析**：改动前预知影响范围，降低重构风险
  - **测试生成**：bugfix 历史 → BDD 场景 → BDDC 执行验证
  
  典型链路：
  - Greenfield：`compile -> arch matrix -> arch validate -> bdd seed`
  - Brownfield：`extract -> graph-index build -> arch validate -> bugfix`
  - **案例参考**: [`compiler/bcc/examples/openclaw-arch/`](compiler/bcc/examples/openclaw-arch/)

### `automation/`
- [**`niuma/`**](automation/niuma/README.md)（Go，Phase 2.5/2.6 已就绪，Phase 3 开发中）
  **AI 驱动的全自动开发机器人**：Issue → Plan → Code → PR → Iterate → **Control（多 Issue 协调）**
  
  **Phase 2.5/2.6（已就绪）**：
  - 多 AI provider "左右互搏"讨论
  - worktree 隔离开发
  - review-iterate 自动交流
  - 给 Issue 加 `bot:fix` 标签即触发全流程
  
  **Phase 3（开发中）**：
  - **多 Issue 协调** (`niuma control`)：扫描所有 bot:fix issue → AI 分析依赖 → 调 taskctl 建 DAG → 按序执行
  - **Integration 分支**：批量 PR 合并验证，CI 联合检查冲突
  - **批量合并**：按拓扑序自动合并，人只需最终批准
  
  人只做三件事：建 issue + 加 `bot:fix` 标签 + 最终批准合并。其他全部 AI 自动化。

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
