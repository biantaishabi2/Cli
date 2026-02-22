# taskctl

Local Rust CLI for task orchestration with dependency DAG support.

## Capabilities

- Task lifecycle: `create`, `update`, `delete`, `get`, `list`
- Dependency graph: `add-blocked-by`, `add-blocks`
- Scheduling view: `ready`
- Graph checks: `validate`
- Graph export: `dag` / `generate`
- Human-readable graph: `dag-ascii` / `ascii`
- Two-graph compute core:
  - `research` (假设驱动研究推理 + 贝叶斯更新)
  - `execute compile`

## Quick Start

```bash
# from repository root
cargo run --manifest-path taskctl/Cargo.toml -- --help

# create
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json create \
  --subject "Run tests" \
  --description "Execute backend tests" \
  --active-form "Running tests" \
  --metadata '{"priority":"P1","module":"quality"}'

# update
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json update \
  --task-id <TASK_ID> \
  --owner qa@team \
  --add-blocked-by <DEP_ID_1>,<DEP_ID_2> \
  --status in-progress

# validate + DAG
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json validate
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag-ascii

# two-graph compute
cargo run --manifest-path taskctl/Cargo.toml -- research reduce --input ./research.json
cargo run --manifest-path taskctl/Cargo.toml -- execute compile --input ./execute.json
```

## Default Store

If `--store` is not passed, the default file is:

- `$HOME/cli/taskctl/tasks.json`

## Testing

```bash
cargo test --manifest-path taskctl/Cargo.toml
```

Fixtures and integration tests:

- `taskctl/tests/fixtures/realistic_tasks.json`
- `taskctl/tests/fixtures/realistic_dag.json`
- `taskctl/tests/cli_realistic.rs`

### BDD + 可观测行为契约

`taskctl` 的测试以行为场景为主，关注“可观测输出”而非内部实现细节。核心场景如下：

- `bdd_ready_task_becomes_progressable_only_when_unblocked`
  - Given：任务 B 被任务 A 阻塞
  - When：A 未完成时尝试启动 B
  - Then：返回 Blocked；A 完成后 B 出现在 `ready` 列表并可启动

- `bdd_multiple_ready_tasks_can_run_in_parallel`
  - Given：多个无依赖任务同时在 `pending`
  - When：调用 `ready`
  - Then：所有任务都可见于可就绪集合（可并行执行）

- `bdd_state_is_observable_from_store_file`
  - Given：任务流与依赖定义写入 store 文件
  - Then：文件内容可直接验证版本、状态、依赖与元数据；重载后行为一致

这三类场景分别对应：
- 任务执行前置条件（blocked / not blocked）
- 并发调度能力（可并行就绪）
- 持久化契约（可观测证据）

## DAG Output

- `dag` prints JSON to stdout (`topo_order`, `layers`, `nodes`, `edges`).
- `dag-ascii` prints ASCII graph text to stdout.
- To persist DAG output:

```bash
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag > dag.json
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag-ascii > dag.txt
```

## Two-Graph Output Contract

`research` 和 `execute` 命令共享稳定的顶层输出格式：

- `schema_version`: fixed `1.0`
- `result`: `ok` or `error`
- Core artifact (only one on success):
  - `graph` (research reduce)
  - `dag` (execute compile)
- `diagnostics`:
  - required: `rules_hit`, `conflicts`
  - optional: `warnings`

Error response adds `error`:

- `error.code`
- `error.message`
- optional `error.cycle` (for DAG cycle diagnostics)

## Error Codes

- `E0001 INVALID_INPUT`
  - phase: `research reduce` / `execute compile`
  - exit code: `2`
  - used for schema/semantic validation failures

- `E1001 DAG_CYCLE_DETECTED`
  - phase: `execute compile`
  - exit code: `2`
  - payload includes cycle path, e.g. `["A","B","A"]`

---

## Research 工具使用指南

### 核心思想

做研究就像做科学：**靠直觉（gut feeling）→ 大胆假设 → 小心求证 → 更新判断 → 收敛到结论**。

机器人只需要做两件事：
1. **声明假设**：对问题先下一个判断（prior）
2. **添加证据**：每次发现新信息，声明它支持还是反对假设

工具会自动帮你：
- 用贝叶斯推理更新后验概率（posterior）
- 给出行动建议（verdict）：act / investigate / contested

**你不需要关心公式，只需要诚实地声明你发现了什么。**

### 化学键比喻（证据强度）

来自 Seed 论文的分子结构理论，不同来源的证据有不同的"键能"：

| bond_type | 比喻 | 权重 | 含义 | 什么时候用 |
|-----------|------|------|------|-----------|
| `deduction` | 共价键 | ×1.0 | 逻辑推导、代码确认、测试验证 | 读了代码/跑了测试，确定性很高 |
| `verification` | 氢键 | ×0.7 | 回检校验、交叉引用、文档印证 | 从文档/日志/第二来源确认了信息 |
| `exploration` | 范德华力 | ×0.3 | 试探猜测、直觉推断、模式匹配 | 有一个感觉，但还没确认 |

**原则**：强推导 > 交叉验证 > 弱猜测。exploration 的证据不管多少条都不会压过一条 deduction。

### Verdict 决策阈值

| posterior 范围 | verdict | 含义 |
|---------------|---------|------|
| >= 0.7 | `act` | 足够确信，可以行动 |
| <= 0.3 | `investigate` | 信心不足，需要更多证据 |
| 有 supports 又有 conflicts | `contested` | 证据冲突，需要厘清 |
| 其他 | `investigate` | 中间地带，继续调查 |

### 完整工作流

#### 第一步：声明假设

遇到不确定的问题时，先下一个直觉判断：

```bash
# 我怀疑是内存泄漏（70% 确信）
taskctl research hypothesis add --id memory-leak --prior 0.7

# 也可能是连接池耗尽（40% 确信）
taskctl research hypothesis add --id pool-exhaustion --prior 0.4

# 完全不确定，五五开
taskctl research hypothesis add --id config-error --prior 0.5
```

#### 第二步：逐步添加证据

每次发现了新信息（读了代码、跑了测试、看了日志），立即声明：

```bash
# 读代码发现 buffer 没有释放 → 强力支持内存泄漏假设
taskctl research add \
  --evidence-id e1 \
  --conclusion-id memory-leak \
  --relation supports \
  --confidence 0.9 \
  --bond deduction

# 日志显示内存曲线持续上升 → 交叉验证
taskctl research add \
  --evidence-id e2 \
  --conclusion-id memory-leak \
  --relation supports \
  --confidence 0.8 \
  --bond verification

# 但连接池监控显示正常 → 反对连接池耗尽假设
taskctl research add \
  --evidence-id e3 \
  --conclusion-id pool-exhaustion \
  --relation conflicts \
  --confidence 0.85 \
  --bond deduction
```

每次 add，工具自动输出更新后的 posterior 和 verdict：

```json
{
  "ok": true,
  "evidence_id": "e1",
  "conclusion_id": "memory-leak",
  "posterior": 0.87,
  "verdict": "act"
}
```

#### 第三步：检查当前状态

```bash
# 查看所有假设的后验概率
taskctl research hypothesis list

# 查看所有假设 + 证据详情
taskctl research list

# 聚合分析（带 verdict 排序）
taskctl research reduce
```

#### 第四步：根据 verdict 行动

- **act** → 后验概率足够高，按此假设行动（修代码、提 PR）
- **investigate** → 还不够确定，继续调查，多加证据
- **contested** → 有矛盾证据，优先厘清冲突点

### 关键原则

1. **先假设再求证**：不要等到 100% 确定才声明假设。prior = 0.5 表示"完全不确定"，这也是合法的起点。
2. **诚实声明 confidence**：0.9+ = 非常确定；0.7~0.8 = 比较确定；0.5~0.6 = 有点感觉；0.3~0.4 = 弱线索。
3. **正确选择 bond_type**：deduction（亲自验证）> verification（第二来源确认）> exploration（只是猜测）。
4. **收敛后行动**：posterior >= 0.7 且 verdict = act 时，证据链已足够支撑行动。
5. **矛盾时优先解决**：verdict = contested 时，先搞清楚矛盾原因，而不是继续堆支持证据。

### 命令速查

| 操作 | 命令 |
|------|------|
| 声明假设 | `taskctl research hypothesis add --id <ID> --prior <0.0~1.0>` |
| 移除假设 | `taskctl research hypothesis remove --id <ID>` |
| 列出假设 | `taskctl research hypothesis list` |
| 添加证据 | `taskctl research add --evidence-id <EID> --conclusion-id <HID> --relation <supports/conflicts> --confidence <0.0~1.0> --bond <deduction/verification/exploration>` |
| 移除证据 | `taskctl research remove --evidence-id <EID>` |
| 列出全部 | `taskctl research list` |
| 聚合分析 | `taskctl research reduce` |

### 典型场景

#### Bug 定位

```bash
# 1. 直觉：可能是并发竞态
taskctl research hypothesis add --id race-condition --prior 0.6

# 2. 读代码发现没加锁 → 强支持
taskctl research add --evidence-id read-code --conclusion-id race-condition \
  --relation supports --confidence 0.9 --bond deduction
# → posterior ≈ 0.87, verdict: act

# 3. 直接修复
```

#### 架构决策

```bash
# 1. 假设：应该用 WebSocket 而不是轮询
taskctl research hypothesis add --id use-websocket --prior 0.5

# 2. 性能测试显示轮询延迟太高 → 支持
taskctl research add --evidence-id perf-test --conclusion-id use-websocket \
  --relation supports --confidence 0.8 --bond deduction

# 3. 但运维团队说 WS 难维护 → 反对
taskctl research add --evidence-id ops-concern --conclusion-id use-websocket \
  --relation conflicts --confidence 0.6 --bond verification
# → verdict: contested → 需要进一步讨论

# 4. 找到轻量 WS 库解决运维痛点 → 支持
taskctl research add --evidence-id lightweight-lib --conclusion-id use-websocket \
  --relation supports --confidence 0.7 --bond verification
# → posterior 回升, verdict: act
```

## Migration Strategy

- `research` + `execute compile` 与现有 DAG 命令并行运行。
- 旧 DAG 命令保持不变，向后兼容。
- 运行时可通过 feature flag 或 alias 先迁移，验证稳定后切换默认路径。
