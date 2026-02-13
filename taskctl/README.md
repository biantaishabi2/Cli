# taskctl

Local Rust CLI for task orchestration with dependency DAG support.

## Capabilities

- Task lifecycle: `create`, `update`, `delete`, `get`, `list`
- Dependency graph: `add-blocked-by`, `add-blocks`
- Scheduling view: `ready`
- Graph checks: `validate`
- Graph export: `dag` / `generate`
- Human-readable graph: `dag-ascii` / `ascii`

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

## DAG Output

- `dag` prints JSON to stdout (`topo_order`, `layers`, `nodes`, `edges`).
- `dag-ascii` prints ASCII graph text to stdout.
- To persist DAG output:

```bash
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag > dag.json
cargo run --manifest-path taskctl/Cargo.toml -- --store ./tasks.json dag-ascii > dag.txt
```
