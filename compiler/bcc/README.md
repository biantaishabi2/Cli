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

1. **seed 契约建模**：以 seed 作为架构真相源，统一描述模块关系与治理约束
2. **层级与分层治理**：支持子模块层级（`parent`）与分层约束（`layer` / `domain_kind`）校验
3. **多视图架构检查**：覆盖 `flow` / `boundary` / `event` 三类视图，而不只是依赖边
4. **行为契约测试闭环**：从 seed 合同导出 BDD source，进入 `bcc bdd seed -> bddc check`
5. **代码知识图谱**：持久化代码结构，支持跨版本查询
6. **架构门禁**：CI 中自动检测架构违规，防止技术债务
7. **影响分析**：改动前预知影响范围，降低重构风险
8. **测试生成**：从 bugfix 历史自动生成回归测试

## Seed 能力（近期增强）

- `modules[].parent`：表达子模块嵌套关系，支撑模块层级治理。
- `modules[].layer` / `modules[].domain_kind`：表达层归属与域归类，进入 validate 主流程。
- `flows`：关键调用链/时序路径约束（防跳层、防捷径）。
- `boundaries`：公共 API 与内部实现边界约束（防绕过边界直接调用内部实现）。
- `events`：事件生产/消费约束（防孤儿事件、幽灵消费者）。
- `boundaries.contracts` + flow/event 行为字段：行为契约输入，支持导出 BDD source。

可视化（`arch export-mermaid`）定位为评审和对账辅助，不是核心门禁本身。

## 与 BDDC 的关系

```
BCC (架构编译器)          BDDC (测试运行时)
    ↓ 生成 BDD 场景    →      ↓ 执行测试
   docs/bdd/**/*.dsl  →    ExUnit 测试报告
```

BCC 负责“从架构契约和代码结构生成可验证场景”，BDDC 负责“把场景编译并执行”。

标准链路：

```bash
# 1) seed + 实际关系 -> 结构/行为校验（并可导出 bdd source）
bcc arch validate --target ... --transition ... --gates ... --actual ... --out-dir ...

# 2) bdd source -> 可执行 BDD 场景种子
bcc bdd seed --source <bdd-source-dir> --output <seed-out> -s organize

# 3) BDDC 编译执行
bddc check --in <seed-out>/features --out test/bdd_generated --instructions <instructions.exs>
```

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
bcc compile        YAML 契约 → Elixir 模块骨架
bcc extract        源码 → FileRecord JSON（Elixir/TypeScript/PHP/Rust）
bcc trace          文档覆盖审计（status/report/seed）
bcc bugfix         git bugfix 历史 → bddc DSL 场景
bcc arch           架构矩阵与门禁（matrix/validate/report/export-module-map）
bcc bdd            新项目场景种子（seed）
bcc analyze smell  代码异味检测（重复/噪音/防御/错误处理/安全）
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

# 启用注入识别（MVP 内置规则）
bcc arch matrix \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --ast-file docs/backend-trace/artifacts/trace2contract/module-relations.json \
  --out-dir docs/backend-trace/trace2contract/seed \
  --version v3 --emit all \
  --detect-injection

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

# 5) 从 seed 生成对接产物
# 兼容模式：沿用原有代码生成（generate-commands.sh + *.ex）
bcc arch generate \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --emit code \
  --output docs/backend-trace/artifacts/arch-generate

# 对接 UniBO：只输出 API Contract，不生成 runtime/controller/resolver
bcc arch generate \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --emit api-contract \
  --output docs/backend-trace/artifacts/arch-generate
# 输出文件:
# - docs/backend-trace/artifacts/arch-generate/unibo-api-contract.json
# - docs/backend-trace/artifacts/arch-generate/api-contract.json

# 对接 UniBO runtime bridge（复用 runtime，不生成自研 controller/resolver）
bcc arch generate \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --emit api-contract \
  --emit-runtime-bridge true \
  --output docs/backend-trace/artifacts/arch-generate
# 追加输出:
# - docs/backend-trace/artifacts/arch-generate/unibo-runtime-bridge.yaml

# 兼容说明：当前版本 emit=all 与 emit=code 等价（冻结语义）
bcc arch generate \
  --seed-file docs/backend-trace/module-registry.seed.yaml \
  --emit all \
  --output docs/backend-trace/artifacts/arch-generate
```

`--detect-injection` 开启后会额外输出 `<version>.relation-classification.json`，每条边包含：
`caller` / `callee` / `call_type` / `via` / `confidence` / `source` / `detector` / `reason`。

`call_type` 当前支持：
- `direct_call`：默认直接调用（含低置信度自动降级）
- `framework_injection`：框架注入（Elixir `use`、NestJS `@Module` / `@Injectable` / `@Controller`）
- `external_registration`：外部库注册（如 `ExternalLib(.Providers).register(InternalModule)`）

兼容性说明：
- 默认不开启 `--detect-injection`，输出与既有 `target/transition/gates` 一致。
- `--injection-patterns` 参数已预留，MVP 阶段仅占位，不参与规则匹配。
- `bcc arch generate --emit api-contract` 缺少 `boundaries[].contracts` 时会报错并非 0 退出。

### BCC + UniBO 门禁回归（#508）

本仓库新增了 BCC + UniBO 联合门禁测试，覆盖：
- GraphQL manifest drift 契约门禁
- BCC bridge + UniBO runtime smoke 集成回归
- L0-L3 演进复杂度最小链路回归

本地执行：

```bash
# 默认 warn（只告警不拦截）
cargo test -p bcc --test contract_manifest_drift --test integration_bcc_unibo_runtime --test regression_l0_l3_evolution

# strict（发现未豁免 BREAKING/过期豁免时失败）
GATE_MODE=strict cargo test -p bcc --test contract_manifest_drift --test integration_bcc_unibo_runtime --test regression_l0_l3_evolution

# strict + dangerous 一并拦截
GATE_MODE=strict GATE_FAIL_ON_DANGEROUS=true cargo test -p bcc --test contract_manifest_drift --test integration_bcc_unibo_runtime --test regression_l0_l3_evolution
```

门禁报告输出到 `target/gate-reports/*.json`，可在 CI 归档审计。

manifest 基线与白名单：
- 基线文件：`compiler/bcc/tests/fixtures/contracts/baseline_manifest.json`
- 白名单文件：`compiler/bcc/tests/fixtures/contracts/whitelist.toml`
- 白名单格式：
  - `[[entry]] id/reason/approved_by/expires_at`
  - `expires_at` 必须为 RFC3339 时间（如 `2026-12-31T00:00:00Z`）

更新流程（建议）：
1. 先在 `GATE_MODE=strict` 下确认 drift 分类是否符合预期。
2. 若确属可接受变更，同步更新 baseline 并在 PR 说明原因。
3. 必要时新增 whitelist 临时豁免，并设置明确过期时间。
4. 过期豁免必须清偿；strict 模式会对过期条目直接失败。

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

`--source` 目录只会消费 `*.yaml` / `*.yml` 文件；`*.json`、`*.md` 等非 YAML 文件会被忽略。若目录里没有任何 YAML source，命令会直接报错退出。

`--prompt-template` 当前为 DSL 模板文件（占位符替换），不是模型提示词执行入口。
`-s check` 会产出 `quality-check.json`，有不合格场景时返回非零；`-s fix` 会尝试修复并产出 `quality-fix.json`。

### bcc analyze smell

代码异味检测：接收 `bcc extract` 产出的 AST JSON，扫描 6 大类 19 项代码质量问题。

```bash
# 基本用法：从 extract 产出的 JSON 检测异味
bcc analyze smell ast-output.json

# 搭配 smell-gate 做 CI 门禁（超过阈值则非零退出）
bcc analyze smell ast-output.json --smell-gate 10

# 搭配外部 linter 结果合并
bcc analyze smell ast-output.json --linter "eslint:eslint-report.json"
```

检测器通过 tree-sitter 解析 AST，支持 **Python / Elixir / Rust / TypeScript** 四种语言，覆盖以下检测能力：

**重复代码检测** — 找出复制粘贴后只改了变量名的函数，以及 AST 骨架高度相似的函数。

| 规则 | 说明 |
|------|------|
| `structural_duplication` | 结构重复：多个函数结构相同，仅变量名/字面量不同 |
| `boilerplate_skeleton` | 骨架重复：函数 AST 骨架相似度超阈值 |

**噪音代码检测** — 清理不影响运行但严重影响可读性的代码垃圾。

| 规则 | 说明 |
|------|------|
| `empty_function_body` | 空函数体：函数体为空或仅含 `pass`/`Ok(())`/`todo!()` 等占位 |
| `commented_out_code` | 注释掉的代码：连续 5+ 行注释可被 parser 解析为合法代码 |
| `dead_branch` | 死分支：`if false` / `if False` 条件永远为假 |
| `unreachable_code` | 不可达代码：`return`/`raise`/`throw` 后的同级语句 |
| `trivial_comment` | 废话注释：注释内容与紧邻代码高度重复，无附加信息 |
| `excessive_comments` | 过度注释：函数内注释行占比超过 60% |
| `unused_import` | 未使用导入：import 声明在文件中无引用 |

**过度防御检测** — 检测多余的防御性代码和遗留标记。

| 规则 | 说明 |
|------|------|
| `leftover_boilerplate` | 残留样板：TODO/FIXME/HACK 注释未清理 |
| `redundant_type_check` | 冗余类型检查：已有类型标注还手动做 type check |
| `unnecessary_default` | 不必要默认值：schema 定义 required 的字段还写 `.get(key, default)` 兜底 |

**错误处理检测** — 找出异常被吞没或被过宽捕获的风险点。

| 规则 | 说明 |
|------|------|
| `swallowed_error` | 吞没异常：`rescue _ -> :ok` / `except: pass` 静默吞掉错误 |
| `broad_catch` | 过宽捕获：`rescue _` / `except Exception` 通配捕获所有异常 |

**安全模式检测** — 扫描常见安全隐患。

| 规则 | 说明 |
|------|------|
| `hardcoded_credential` | 硬编码凭证：源码中直接写死 key/secret/token/password |
| `injection_risk` | 注入风险：SQL 拼接、eval() 调用 |
| `unsafe_deserialization` | 不安全反序列化：pickle.load / yaml.load 等 |
| `weak_crypto` | 弱加密算法：MD5/SHA1/DES/RC4 |
| `sensitive_data_log` | 敏感数据泄露：日志中打印 password/token 等字段 |

## 闭环

新项目线（Greenfield）：
- `compile -> arch matrix -> arch validate -> bdd seed -> bddc check`

存量项目线（Brownfield）：
- `extract -> arch validate -> arch export-module-map -> bugfix -> bddc check`

`arch validate` 与 `bdd seed check` 是当前主线的门禁点；CI Smoke 由 `cargo test -p bcc --test cli_arch_bdd` 与 `cargo test -p bcc --test extract_golden` 共同覆盖。
## 支持语言

| 语言 | extract | bugfix | tree-sitter |
|------|---------|--------|-------------|
| Elixir | .ex .exs | .ex .exs | tree-sitter-elixir 0.3 |
| TypeScript | .ts .tsx | .ts .tsx | tree-sitter-typescript 0.23 |
| PHP | .php | .php | tree-sitter-php 0.24 |
| Rust | .rs | .rs | tree-sitter-rust 0.23 |

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
cargo test -p bcc --test extract_golden
UPDATE_GOLDEN=1 cargo test -p bcc --test extract_golden
cargo run -- extract fixtures/sample_controller.php --mode ast  # PHP 端到端
cargo run -- arch --help
cargo run -- bdd --help
```

### Golden 基线更新规范

仅在以下条件同时满足时允许更新 `tests/golden/extract/**`：

1. 提取逻辑存在预期行为变更（非偶发波动）。
2. 评审者已确认变更原因与影响范围。
3. PR 中包含对应 fixture/golden diff 说明。

更新命令：

```bash
cd compiler/bcc
UPDATE_GOLDEN=1 cargo test -p bcc --test extract_golden
```

Golden diff 评审清单：

1. 是否仅影响 `calls/imports/exports/declarations/loc_lines` 的预期字段。
2. 每个变化是否能对应到明确的提取规则改动。
3. 是否引入跨语言同构规则之外的新差异。
4. 是否包含必要的新增/修正测试场景说明。

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
