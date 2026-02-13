# BDDC 协议（最小可执行版）

目标：把 BDD 编译器从“绑定某个项目的实现细节”变成“任何遵守协议的项目都能用”。

本协议定义四件事：
- 指令如何声明（Spec）
- Spec 字段标准
- Runtime 执行契约
- 门禁入口（本地/CI）

> 约定：以下示例均以 v1 指令集为例；v2 作为增量覆盖（可选）。

## 0. 配置与脚手架（推荐）

为减少每条命令重复传参，协议推荐两个配套能力：

1) `.bddc.toml`（配置载体，bddc 默认读取，命令行覆盖）
2) `bddc init`（生成最小四件套 + `.bddc.toml` 的脚手架）

## 1. 最小目录结构

一个接入项目最小需要四类文件：

1) 指令 Spec（必须，提供给 `bddc --instructions`）
- `priv/bdd/instructions_v1.exs`
- （可选）`priv/bdd/instructions_v2.exs`

2) Runtime dispatcher（必须，执行指令）
- `test/support/bdd/instructions_v1.ex`

3) DSL 场景（必须）
- `docs/bdd/*.dsl`

4) 门禁入口（推荐）
- `scripts/bdd_gate.sh`

## 1.1 `.bddc.toml`（配置载体）

`bddc` 默认会读取项目根目录下的 `.bddc.toml`。配置的目标是：把以下参数固化为默认值：
- 指令 spec 路径（`instructions` / `instructions_v2`）
- DSL 输入/输出目录（`in` / `out`）
- runtime 模块（`runtime_module`）
- runtime caps 文件（`runtime_caps_file`）
- 生成测试模块前缀（`module_prefix`）与测试基类（`test_case`）

示例：

```toml
[global]
instructions = ["priv/bdd/instructions_v1.exs"]
instructions_v2 = []
in = "docs/bdd"
out = "test/bdd_generated"
docs_root = "docs"
runtime_module = "MyApp.BDD.Instructions.V1"
runtime_caps_file = "docs/bdd/_generated/runtime_caps_v1.exs"
test_case = "ExUnit.Case"
module_prefix = "MyApp.BDD.Generated"

[runtime.caps.sync]
out = "docs/bdd/_generated/runtime_caps_v1.exs"
out_meta = "docs/bdd/_generated/runtime_caps_v1_meta.exs"
```

说明：
- `global` 会对所有命令生效（仅在命令自身支持该参数时生效）。
- 可以为命令提供单独 section（例如 `runtime.caps.sync`），避免 global 的 `out` 误影响其他命令。

## 1.2 `bddc init`（脚手架）

`bddc init` 用于一次性生成“最小可跑通闭环”的接入骨架：
- `.bddc.toml`
- `priv/bdd/instructions_v1.exs`（含 GENERATED 区域）
- `test/support/bdd/instructions_v1.ex`（runtime dispatcher 模板）
- `test/support/bdd/bddc_runtime.ex`（runtime 公共代码 use 宏模板）
- `test/support/bdd/common_instructions.ex`（common 指令包模板）
- `docs/bdd/hello.dsl`（示例 DSL）
- `scripts/bdd_gate.sh`（门禁入口）

## 2. Spec 文件格式（.exs）

`instructions_v1.exs` 必须返回一个 map：

- key：指令名（atom）
- value：指令 spec（map）

示例（最小字段版）：

```elixir
%{
  given_seed: %{
    name: :given_seed,
    kind: :given,
    args: %{id: %{type: :string, required?: true, allowed: nil}},
    outputs: %{id: :string},
    rules: [],
    scopes: [:integration, :e2e],
    boundary: :service,
    async?: false,
    eventually?: false,
    assert_class: nil
  }
}
```

### 2.1 字段标准

- `name`：atom，必须等于 map key
- `kind`：`:given | :when | :then`
- `args`：参数定义 map（key 为参数名 atom）
  - `type`：`:string | :integer | :decimal | :boolean | :uuid | :atom | :map | :list`（可按项目扩展，但需在编译器校验支持）
  - `required?`：boolean
  - `allowed`：nil 或允许值集合（例如 `[:a, :b]` 或 `["a","b"]`）
- `outputs`：该指令向 ctx 写入的键与类型（用于变量引用 `$id` 等）
- `rules`：参数规则（例如 `exactly_one_of` 等，按编译器支持范围）
- `scopes`：允许的测试层级（例如 `[:integration, :e2e]`）
- `boundary`：可观测边界（例如 `:http | :app | :service | :domain`）
- `async?`：是否允许异步执行
- `eventually?`：是否允许 eventually 语义
- `assert_class`：断言分类（可选，用于 lint/治理）

### 2.2 v2 增量（可选）

- `instructions_v2.exs` 允许只提供“delta 覆盖”；未命中会回退 v1。

## 3. Runtime 执行契约

编译器生成的 ExUnit 测试会调用 runtime module 的以下函数（这是硬契约）：

- `capabilities/0`：返回该 runtime 实现覆盖的指令集合
  - 返回值允许：list / MapSet / map

- `new_run_id/0`：生成一次运行的唯一标识（用于隔离 ctx）

- `run_step!/6`：执行一步指令
  - 形态：`run_step!(ctx, kind, name, args_map, meta_map, dsl_line)`
  - 返回：更新后的 `ctx`

- `get!/3`：从 `ctx` 读取变量（用于 `$var`）
  - 形态：`get!(ctx, key, meta)`

> 备注：如果你的项目不想叫这些函数名，就需要在 bddc 里引入适配层；当前协议选择“固定函数名”，降低接入复杂度。

## 4. DSL（.dsl）最小约定

- 文件：`docs/bdd/*.dsl`
- 每个场景包含：`SCENARIO` 元信息 + 多行 step
- step 形态：
  - `GIVEN <instruction> k="v"`
  - `WHEN  <instruction> k=$var`
  - `THEN  <instruction> ...`

> 变量引用：`$id` 表示引用 ctx 中的 `:id`。

## 5. 门禁入口（本地/CI）

推荐把所有 BDD 门禁统一收敛为一个入口脚本（示例）：

```bash
#!/usr/bin/env bash
set -euo pipefail

# 1) 同步 runtime caps（可选，但推荐，避免 CI 触发 mix 编译链）
bddc runtime.caps.sync \
  --project-root . \
  --runtime-module MyApp.BDD.Instructions.V1 \
  --out docs/bdd/_generated/runtime_caps_v1.exs

# 2) check（compile + lint + runtime 覆盖）
bddc check \
  --project-root . \
  --instructions priv/bdd/instructions_v1.exs \
  --runtime-module MyApp.BDD.Instructions.V1 \
  --runtime-caps-file docs/bdd/_generated/runtime_caps_v1.exs
```

## 6. 约束与边界

- bddc 不内置任何业务指令。
- Spec 与 Runtime 必须由项目按协议提供（可通过 `bddc init` 生成模板）。
- `registry.scaffold/upsert` 的 standalone 反推依赖“可机器读取的声明”，协议推荐使用源码字面量注解：
  - `@bdd_instruction %{name: :xxx, kind: :given, args: %{...}, ...}`
  - 注解必须是字面量（map/list/tuple/atom/string/number/bool/nil），避免执行任意代码。
- `registry.upsert` 的 standalone 写回只会覆盖 target 文件中标记区域：
  - `# BEGIN BDDC GENERATED`
  - `# END BDDC GENERATED`
