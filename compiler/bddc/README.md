# BDDC — BDD Test Runtime

**BDD 测试运行时**，与 BCC 配合形成"测试即文档"闭环。

## 定位

| 工具 | 职责 | 输出 |
|------|------|------|
| **BCC** | 分析代码/历史，生成 BDD 场景 | `docs/bdd/**/*.dsl` |
| **BDDC** | 执行 BDD 场景，验证行为 | ExUnit 测试报告 |

## 工作流程

```
代码变更 / Bugfix 历史
    ↓ BCC bugfix
BDD 场景文件 (.dsl)
    ↓ BDDC 编译
ExUnit 测试代码
    ↓ mix test
测试报告（通过/失败）
```

## 独立使用

虽然设计为与 BCC 配合，BDDC 也可独立使用：

## 构建

```bash
cd tools/bdd_compiler
mix deps.get
mix escript.build
```

产物：`tools/bdd_compiler/bdd_compiler`

## 安装（推荐）

安装到 `~/.local/bin`，并创建短别名 `bddc`（软链接到 `bdd_compiler`）：

```bash
./tools/bdd_compiler/install.sh --rebuild
bddc --help
```

如不想创建别名：

```bash
./tools/bdd_compiler/install.sh --rebuild --no-alias
```

## 指令集输入格式

`--instructions` / `--instructions-v2` 接收 `.exs` 文件路径，文件返回值必须是 map：

```elixir
%{
  some_instruction: %{
    name: :some_instruction,
    kind: :given,
    args: %{user_id: %{type: :uuid, required?: true, allowed: nil}},
    outputs: %{user_id: :uuid},
    rules: [],
    scopes: [:integration, :e2e],
    boundary: :service,
    async?: false,
    eventually?: false,
    assert_class: nil
  }
}
```

说明：
- v2 如果只提供 delta（只包含 v2-only 指令），查询时会自动回退到 v1（即 v2 = v1 + delta）。

## 使用

```bash
./tools/bdd_compiler/bdd_compiler compile \
  --in docs/bdd \
  --out test/bdd_generated \
  --instructions /tmp/instructions_v1.exs \
  --runtime-module Shop.BDD.Instructions.V1 \
  --test-case Shop.DataCase \
  --module-prefix Shop.BDD.Generated
```

## 保持一致（项目模式 wrapper）

当你希望“行为与现有 `mix bdd.*` 任务保持一致”时，可以使用这些命令。

说明：
- 注解/注册/文档/扩展能力命令会在目标项目目录下调用 `mix` 运行同名任务；
- `factories.*` 为 CLI 内置实现（行为口径与 `mix bdd.factories.*` 保持一致）；
- 因此目标项目至少需要提供相应模块能力（或同名 mix task，取决于子命令）。

```bash
# 默认 project-root 为当前目录
./tools/bdd_compiler/bdd_compiler annotations.check
./tools/bdd_compiler/bdd_compiler registry.scaffold --module Shop.Foo --functions bar/1
./tools/bdd_compiler/bdd_compiler registry.upsert --module Shop.Foo --functions bar/1
./tools/bdd_compiler/bdd_compiler instructions.docs --version v1 --output docs/bdd/指令集.md
./tools/bdd_compiler/bdd_compiler factories.scaffold --scope priv/bdd/factories_scope.exs
./tools/bdd_compiler/bdd_compiler factories.upsert --scope priv/bdd/factories_scope.exs
./tools/bdd_compiler/bdd_compiler factories.check --paths test/support/bdd/factories_generated
./tools/bdd_compiler/bdd_compiler contract.check --in docs/bdd/contracts
./tools/bdd_compiler/bdd_compiler fuzz --seed 42
./tools/bdd_compiler/bdd_compiler mutation.report --in lib/shop/bdd
./tools/bdd_compiler/bdd_compiler mutation.run --max-mutants 10

# 一键串联（注解→指令→文档→check）
./tools/bdd_compiler/bdd_compiler domain.autowire \
  --project-root /path/to/project \
  --module Shop.Foo.BarService \
  --functions do_x/1,do_y/2 \
  --registry-module Shop.BDD.InstructionRegistry \
  --runtime-module Shop.BDD.Instructions.V1 \
  --in docs/bdd \
  --out test/bdd_generated \
  --strict true \
  --fail-on-warn false

# 只预览将执行的步骤（不真正执行）
./tools/bdd_compiler/bdd_compiler domain.autowire \
  --project-root /path/to/project \
  --module Shop.Foo.BarService \
  --functions do_x/1,do_y/2 \
  --registry-module Shop.BDD.InstructionRegistry \
  --runtime-module Shop.BDD.Instructions.V1 \
  --in docs/bdd \
  --out test/bdd_generated \
  --dry-run

# 指定项目根目录
./tools/bdd_compiler/bdd_compiler annotations.check --project-root /path/to/project
```

```bash
./tools/bdd_compiler/bdd_compiler lint \
  --in docs/bdd \
  --instructions /tmp/instructions_v1.exs \
  --fail-on-warn
```

```bash
./tools/bdd_compiler/bdd_compiler check \
  --in docs/bdd \
  --out test/bdd_generated \
  --instructions /tmp/instructions_v1.exs \
  --fail-on-warn
```
