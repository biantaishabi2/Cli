# Cli

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml)
[![Release](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml)

> **LLM 时代的软件工程操作系统**：taskctl 编排复杂工作流，BCC 编译代码结构为可验证的知识图谱，niuma 实现从需求到合并的全自动开发，BDDC 执行行为驱动测试。人定义规则和目标，机器处理执行和验证。
>
> 📖 详细哲学阐述见 [`PHILOSOPHY.md`](PHILOSOPHY.md)

## 最终结果

本仓库交付一条可执行的软件工程闭环，输出以下结果：

- 可验证的架构契约：模块层级、分层规则、flow、boundary、event
- 可执行的行为验收：从契约生成场景并运行测试，输出通过/失败结果
- 可编排的任务执行：基于 DAG 的任务依赖、调度与推进
- 可自动化的研发流程：Issue 到代码、PR、迭代与合并

## 实现方式

标准执行链路：

```bash
# 1) 架构与行为校验
bcc arch validate --target ... --transition ... --gates ... --actual ... --out-dir ...

# 2) 场景生成
bcc bdd seed --source <bdd-source-dir> --output <seed-out> -s organize

# 3) 行为验收执行
bddc check --in <seed-out>/features --out test/bdd_generated --instructions <instructions.exs>
```

## 文档导航

- BCC（架构契约、校验、场景生成）：[`compiler/bcc/README.md`](compiler/bcc/README.md)
- BDDC（场景编译与执行）：[`compiler/bddc/README.md`](compiler/bddc/README.md)
- Niuma（自动化研发流程）：[`automation/niuma/README.md`](automation/niuma/README.md)
- Taskctl（任务编排与 DAG）：[`orchestration/taskctl/README.md`](orchestration/taskctl/README.md)

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
bcc arch report \
  --scenario-validation versions/v4-draft/scenario-validation.tsv \
  --gate-evaluation versions/v4-draft/gate-evaluation.tsv \
  --summary versions/v4-draft/summary.json \
  --out versions/v4-draft/architecture-debt.md
bcc arch export-module-map \
  --module-map artifacts/module_map.json \
  --module-registry seed/module-registry.v3.yaml \
  --out artifacts/module_map.bugfix.json
bcc arch export-mermaid \
  --module-map artifacts/module_map.bugfix.json \
  --output docs/architecture.md
bcc bdd seed \
  --source docs/backend-trace/bdd-seed-input \
  --output docs/backend-trace/scenarios/v3-seed \
  -s check

# bddc（Elixir escript）
cd compiler/bddc
mix deps.get
mix escript.build
./bdd_compiler --help
```

## 场景化用法

- 新项目（Greenfield）：`compile -> arch matrix -> arch validate -> bdd seed -> bddc check`
- 存量项目（Brownfield）：`extract -> graph-index build -> arch matrix -> arch validate -> arch report -> bugfix -> bddc`
- 代码质量门禁：`extract -> analyze smell [--smell-gate N]`
- 架构评审（辅助）：`extract -> arch matrix/validate -> arch export-mermaid`

命令细节入口：
- Greenfield / Brownfield / arch 子命令：[`compiler/bcc/README.md`](compiler/bcc/README.md)
- BDD 执行与指令集：[`compiler/bddc/README.md`](compiler/bddc/README.md)
- 多 Issue 自动化执行：[`automation/niuma/README.md`](automation/niuma/README.md)

## 工具与目录映射

承载多个命令行工具的 monorepo，按职责分类管理。

### `orchestration/`
- [**`taskctl/`**](orchestration/taskctl/README.md)（Rust）
  任务编排 CLI：任务依赖、DAG、调度与执行。

### `compiler/`
- [**`bcc/`**](compiler/bcc/README.md)（Rust + Elixir emit）
  架构编译器：seed 契约建模、结构/行为校验、门禁与测试素材生成。
- [**`bddc/`**](compiler/bddc/README.md)（Elixir escript）
  BDD 测试运行时：将场景编译并执行，输出验收结果。

### `automation/`
- [**`niuma/`**](automation/niuma/README.md)（Go）
  AI 自动化开发流程：Issue → Plan → Code → PR → Iterate → Control。

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
