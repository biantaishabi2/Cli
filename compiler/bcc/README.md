# BCC — Architecture Compiler

**架构编译器：把架构约束当作语法规则来检查**

就像传统编译器检查语法错误，BCC 检查架构错误。
从代码中提取结构，构建代码知识图谱，验证架构合规性，生成治理报告。

## 架构编译器 vs 传统编译器

| 维度 | 传统编译器 (GCC) | 架构编译器 (BCC) |
|------|-----------------|-----------------|
| 输入 | 源代码 | 代码结构 + 架构约束 |
| 输出 | 机器码 | 架构合规性报告 + 测试 |
| 核心 | 语法 → 语义 → 生成 | 结构 → 关系 → 验证 → 治理 |
| 错误检查 | `if (x = 1)` 语法错误 | `api → dao` 架构违规 |

## 编译流程

```
源代码 (Elixir/TS/PHP)
    ↓ extract --batch          [Frontend: 解析代码结构]
AST / 代码结构
    ↓ graph-index build        [IR: 构建中间表示]
代码知识图谱 (SQLite)          [中间表示: 持久化的代码关系]
    ├── 函数调用图 (caller → callee)
    ├── 类继承图 (parent → child)
    └── 模块依赖图 (import → export)
    ↓ arch validate            [Optimizer: 验证架构约束]
架构约束检查
    ├── 分层合规 (api→service→dao)
    ├── 依赖方向 (core 不依赖 support)
    └── 循环依赖检测
    ↓ bugfix / bdd seed        [Backend: 生成测试]
架构报告 / 违规列表 / BDD 场景
    ↓ bddc                     [Runtime: 执行验证]
可执行测试
```

## 核心价值

1. **代码知识图谱**：持久化代码结构，支持跨版本查询
2. **架构门禁**：CI 中自动检测架构违规，防止技术债务
3. **影响分析**：改动前预知影响范围，降低重构风险
4. **测试生成**：从 bugfix 历史自动生成回归测试

## 与 BDDC 的关系

```
BCC (架构编译器)          BDDC (测试运行时)
    ↓ 生成 BDD 场景    →      ↓ 执行测试
   docs/bdd/**/*.dsl  →    ExUnit 测试报告
```

BCC 生成测试场景，BDDC 执行验证，形成"架构约束 → 测试验证"的闭环。

## 安装

```bash
# 首次安装（编译 + symlink 到 ~/.local/bin）
./compiler/bcc/install.sh --link --rebuild

# 后续更新（代码改动后重新编译）
./compiler/bcc/install.sh --rebuild
```

需要 Rust 工具链（`cargo`）。安装后确保 `~/.local/bin` 在 `PATH` 中。

## 子命令

```
bcc compile   YAML 契约 → Elixir 模块骨架
bcc extract   源码 → FileRecord JSON（Elixir/TypeScript/PHP）
bcc trace     文档覆盖审计（status/report/seed）
bcc bugfix    git bugfix 历史 → bddc DSL 场景
bcc arch      架构矩阵与门禁（matrix/validate/report/export-module-map）
bcc bdd       新项目场景种子（seed）
```

### bcc compile

```bash
bcc compile contract.yaml -o output/
bcc compile contract.yaml --dry-run        # 只校验不生成
bcc compile contract.yaml --emit-ast       # 输出 AST JSON
```

### bcc extract

```bash
bcc extract lib/shop/order/cart.ex --mode ast    # JSON 结构
bcc extract lib/shop/order/cart.ex --mode doc    # 分析文档
bcc extract lib/shop/order/cart.ex --mode yaml   # YAML draft
bcc extract app/controllers/Foo.php --mode ast   # PHP 支持
```

### bcc trace

```bash
bcc trace status lib/ docs/           # 覆盖率概览
bcc trace report lib/ docs/ --output report/  # 生成报告
bcc trace seed lib/ docs/ --write     # 补充缺失文档模板
```

### bcc bugfix

从 git bugfix 历史中提取 BDD 场景，四步流水线：

```
collect(c)  → git log 扫描、分级(A/B/C)、自动打标签
context(x)  → diff + 函数体 before/after 提取
generate(g) → codex exec 生成 bddc DSL 场景
organize(o) → 按模块归类、重复检测、覆盖率报告
```

```bash
bcc bugfix /path/to/repo -o output/                    # 全量执行
bcc bugfix /path/to/repo -o output/ -s c               # 只扫描
bcc bugfix /path/to/repo -o output/ -s x               # 扫描 + 上下文
bcc bugfix /path/to/repo -o output/ -s g --limit 20    # 前 20 个到生成
bcc bugfix /path/to/repo -o output/ --lang elixir      # Elixir 项目
bcc bugfix /path/to/repo -o output/ --lang typescript   # TypeScript 项目
```

### bcc arch

```bash
# 1) 从 seed + AST 生成 target/transition/gates
bcc arch matrix \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --ast-file docs/backend-trace/artifacts/trace2contract/module-relations.json \
  --out-dir docs/backend-trace/trace2contract/seed \
  --version v3 --emit all

# 2) 用实际关系回放并做门禁验证
bcc arch validate \
  --target docs/backend-trace/trace2contract/seed/v3.target-matrix.yaml \
  --transition docs/backend-trace/trace2contract/seed/v3.transition-matrix.yaml \
  --gates docs/backend-trace/trace2contract/seed/v3.gates.yaml \
  --actual docs/backend-trace/artifacts/trace2contract/module-relations.actual.json \
  --out-dir docs/backend-trace/artifacts/trace2contract/versions/v3-draft \
  --profile both

# 调试模式：只出报告，不因 gate/fobidden 非零退出
bcc arch validate ... --fail-on-gate false --fail-on-forbidden false

# 3) 导出 bugfix 可消费的 module_map
bcc arch export-module-map \
  --module-map docs/backend-trace/artifacts/trace2contract/module_map.json \
  --module-registry docs/backend-trace/module-registry.v3.yaml \
  --out docs/backend-trace/artifacts/module_map.bugfix.json

# 4) 聚合报告
bcc arch report \
  --scenario-validation docs/backend-trace/artifacts/trace2contract/versions/v3-draft/scenario-validation.tsv \
  --gate-evaluation docs/backend-trace/artifacts/trace2contract/versions/v3-draft/gate-evaluation.tsv \
  --summary docs/backend-trace/artifacts/trace2contract/versions/v3-draft/summary.json \
  --out docs/backend-trace/artifacts/trace2contract/versions/v3-draft/architecture-debt.md
```

### bcc bdd seed

```bash
# context + generate + organize 全流程（默认 step=organize）
bcc bdd seed \
  --source docs/backend-trace/bdd-seed-input \
  --output docs/backend-trace/scenarios/v3-seed \
  --edge-class stable \
  --prompt-template compiler/bcc/prompts/bdd_seed_generate.txt

# 只跑到 context / generate
bcc bdd seed --source docs/backend-trace/bdd-seed-input --output output/seed -s context
bcc bdd seed --source docs/backend-trace/bdd-seed-input --output output/seed -s generate

# 质量门禁与修复
bcc bdd seed --source docs/backend-trace/bdd-seed-input --output output/seed -s check
bcc bdd seed --source docs/backend-trace/bdd-seed-input --output output/seed -s fix
```

`--prompt-template` 当前为 DSL 模板文件（占位符替换），不是模型提示词执行入口。
`-s check` 会产出 `quality-check.json`，有不合格场景时返回非零；`-s fix` 会尝试修复并产出 `quality-fix.json`。

## 闭环

新项目线（Greenfield）：
- `compile -> arch matrix -> arch validate -> bdd seed -> bddc check`

存量项目线（Brownfield）：
- `extract -> arch validate -> arch export-module-map -> bugfix -> bddc check`

`arch validate` 与 `bdd seed check` 是当前主线的门禁点；`cargo test -p bcc --test cli_arch_bdd` 负责 CI Smoke 覆盖。
## 支持语言

| 语言 | extract | bugfix | tree-sitter |
|------|---------|--------|-------------|
| Elixir | .ex .exs | .ex .exs | tree-sitter-elixir 0.3 |
| TypeScript | .ts .tsx | .ts .tsx | tree-sitter-typescript 0.23 |
| PHP | .php | .php | tree-sitter-php 0.24 |

## Extract 架构

`extract` 模块已拆分为三层，降低多语言实现重复代码：

- `extract/adapter.rs`：语言适配器注册与分发，统一入口调度。
- `extract/common.rs`：通用工具（Parser 初始化与解析、AST 节点取值、调用去重/排序、副作用默认值）。
- `extract/testing.rs`：跨语言测试辅助函数，减少重复断言代码。

新增语言时可按以下最小步骤接入：

1. 实现 `extract/<lang>.rs` 的提取逻辑。
2. 在 `extract/adapter.rs` 注册 `LanguageAdapter`。
3. 复用 `extract/common.rs` 与 `extract/testing.rs`，避免重复样板。

## 测试

```bash
cd compiler/bcc
cargo test          # 单测
cargo run -- extract fixtures/sample_controller.php --mode ast  # PHP 端到端
cargo run -- arch --help
cargo run -- bdd --help
```

## 里程碑

| 阶段 | 状态 | 说明 |
|------|------|------|
| M4 核心 | ✅ | compile/extract/trace 三命令闭环，51 场景通过 |
| M5 扩展 | ✅ | +bugfix 四步流水线 + PHP extract + 多语言，69 场景通过 |
| M6 推广 | 待启动 | CI 集成 + 升级/回滚手册 |

## 文档

- [技术设计文档](docs/技术设计文档-后端编译器.md) — 完整设计、BDD 场景、里程碑
- [BDD 场景提取方案](docs/BDD场景提取方案.md) — bugfix 子命令详细设计
- [架构闭环迁移计划](docs/架构闭环迁移计划.md) — Rust 闭环与架构对齐路径
