# taskctl

Local Rust CLI for task orchestration with dependency DAG support.

## Capabilities

- Task lifecycle: `create`, `update`, `delete`, `get`, `list`
- Dependency graph: `add-blocked-by`, `add-blocks`
- Scheduling view: `ready`
- Graph checks: `validate`
- Graph export: `dag` / `generate`
- Human-readable graph: `dag-ascii` / `ascii`
- Three-graph compute core:
  - `research reduce`
  - `plan solve`
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

# three-graph compute
cargo run --manifest-path taskctl/Cargo.toml -- research reduce --input ./research.json
cargo run --manifest-path taskctl/Cargo.toml -- plan solve --input ./plan.json
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

## Three-Graph Output Contract

All `research/plan/execute` commands share a stable top-level contract:

- `schema_version`: fixed `1.0`
- `result`: `ok` or `error`
- Core artifact (only one on success):
  - `graph` (research)
  - `plan_decision` (plan)
  - `dag` (execute)
- `diagnostics`:
  - required: `rules_hit`, `conflicts`
  - optional: `warnings`

Error response adds `error`:

- `error.code`
- `error.message`
- optional `error.cycle` (for DAG cycle diagnostics)

## Error Codes

- `E0001 INVALID_INPUT`
  - phase: `research reduce` / `plan solve` / `execute compile`
  - exit code: `2`
  - used for schema/semantic validation failures (e.g. duplicate node id, leaf with children, missing refs, plan cycle)

- `E1001 DAG_CYCLE_DETECTED`
  - phase: `execute compile`
  - exit code: `2`
  - payload includes cycle path, e.g. `["A","B","A"]`

## Migration Strategy

- Introduce `research reduce` / `plan solve` / `execute compile` in parallel with existing DAG commands.
- Keep old DAG commands unchanged for backward compatibility.
- Runtime side can migrate via feature flag or alias first, then switch default path after stability verification.
