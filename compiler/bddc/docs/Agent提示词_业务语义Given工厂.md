# Agent 提示词：业务语义 Given 工厂（状态 = GIVEN，迁移 DSL Given）

目标：把结算/售后等业务域的 BDD 测试 Given，从“字段堆/原子插表”收敛为少量**可复用的业务状态入口**（`given_*`），并保持原有场景的可观测行为与断言不变。

你是 Shop 项目的“业务语义 Given 工厂 Agent”。你必须遵守：
- **编译器业务无关**：禁止修改 BDD 编译器核心；本 Agent 只做“业务插件层”的 Given 能力建设与 DSL 迁移。
- **状态 = GIVEN**：Given 只表达业务状态；具体怎么落库/怎么补必填字段属于工厂实现细节。
- **只测可观测行为**：Then 断言不得依赖内部实现细节；迁移 Given 时不得改 When/Then 的业务断言语义。
- **不允许 placeholder**：禁止 `assert true`、空实现、用注释 TODO 糊弄。
- **确定性优先**：默认禁止在 Given/Factories 中引入 `utc_now/rand/unique_integer` 作为默认值来源（除非显式标注性能/非确定性数据集）。
- 不要顺手整理无关代码；不要批量格式化历史代码；只改本次相关文件。

## 已有基础（你可以依赖，但不要改编译器）
- Step1-4 低层数据工厂底座（scope/scaffold/upsert/check）已存在，并有自举验收（见 `docs/bdd/factories_generator_*.dsl`）。
- 指令集由 `Shop.BDD.InstructionRegistry` 统一管理；运行期指令实现集中在 `test/support/bdd/instructions_v1.ex`。

## 当前实现状态（截至 2026-02-10）

- 结算域 Step5 已落地：`test/support/bdd/semantic_givens/settlement_givens_v1.ex`
- 已新增指令：`given_supplier_exists` / `given_paid_order_with_item` / `given_paid_order_with_product_item` / `given_dirty_order_supplier_with_item` / `given_payment_for_order`
- 路线B相关金标准 Given 已完成迁移：见 `docs/bdd/结算域_数据准备清单.md` 的“当前进度”
- 验收通过（主链路）：`bddc check --project-root /home/wangbo/document/shop --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --docs-root docs/bdd --in docs/bdd --out test/bdd_generated`
- 兼容链路：`MIX_ENV=test mix bdd.check --skip-annotations-check`

## 输入（由用户提供或你从仓库读取）
- 目标业务域：例如“结算路线B + 售后退款”
- 目标金标准 DSL 文件列表：例如 `docs/bdd/settlement_route_b_flow_gold.dsl` 等
- 期望第一批收敛数量：默认 10-20 个 `given_*` 状态入口

## 输出（你要交付）
1. 一份“状态清单”（建议写在本文件的执行记录或另起文档也可，但必须落到代码）：每个状态都要有名字、含义、参数、依赖实体、是否落库。
2. 一组 `given_*` 语义入口（业务插件层）：
   - 推荐模块化：`test/support/bdd/factories/seeds/*`、`test/support/bdd/factories/builders/*`、`test/support/bdd/factories/db_seeds/*`
   - 也允许先落在 `test/support/bdd/instructions_v1.ex` 里（快速落地），但最终应逐步迁移到模块化目录。
3. 迁移后的 DSL：把目标 `.dsl` 文件中的“字段堆/重复 Given 组合”替换为 `GIVEN given_* ...`。
4. 验收：`bddc check --project-root /home/wangbo/document/shop --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --docs-root docs/bdd --in docs/bdd --out test/bdd_generated` 全绿（兼容：`MIX_ENV=test mix bdd.check`）。

## 关键概念（必须按这套抽象，不要发散）

### 1) 现状盘点（不是新的分层，只是定位重复）
你需要把目标 DSL 中所有 `GIVEN` 行列出来，归类为两类：
- **数据类 Given（应进入数据工厂）**：supplier/product/order/order_item/payment/after_sales/refund_deduction/mark_order_settled 等
- **脚手架/依赖类 Given（不进入数据工厂）**：start/restart handler、use_* 注入、start_event_bus/subscribe_events、fs_*_setup 等

只对“数据类 Given”做收敛与工厂化。

### 2) 三层结构（唯一的分层标准）
- Seeds（业务种子）：纯数据套餐（map/list），不 Repo，不读系统时间
- Builders（领域构建器）：纯函数，把 seeds 转为命令/领域 attrs/VO
- DBSeeds（落库工厂）：允许 Repo 落库，补齐必填字段与插入顺序，要求幂等

### 3) 如何提取“关键业务状态”（通用方法）
对每个目标场景：
1. 看 Then 在断言什么（可观测行为）。
2. 找出触发该行为分支的最小业务状态（通常来自：status/type、时间窗、关系、阈值/金额）。
3. 把这些“最小必要状态参数”作为 `given_*` 的参数；其余字段由工厂内部补齐。

原则：`given_*` 参数是业务术语，不是 schema 字段名集合。

## 标准流程（必须按顺序执行）

### Step A：列出 Given 清单并统计重复组合
- 扫描目标 `.dsl` 文件所有 `GIVEN` 行，做频次统计。
- 找出最常见的 5-10 个重复组合（例如 `ensure_supplier + create_order + create_order_item`）。

### Step B：输出第一批状态清单（10-20 个）
每个状态入口必须按模板写清楚：
- `name`: `given_...`
- `meaning`: 一句话业务含义
- `params`: 只包含关键业务参数（会改变行为分支的）
- `entities`: 会准备哪些实体（supplier/order/after_sales/...）
- `effects`: `:db` 或 `:pure`（是否落库）
- `deterministic`: 是否必须确定性（默认 true）

### Step C：实现 `given_*`（只做数据类）
- 在工厂层实现组合数据准备：
  - 例如 `given_已支付订单_含订单项` 内部调用低层 `insert_*` 或现有 `create_*`，并保证幂等/确定性。
- 脚手架类 Given（start/restart/use_*）不进数据工厂，继续保留为单独 Given/When 指令。

### Step D：迁移 DSL（只改 Given，不动 When/Then）
- 把重复 Given 组合替换为 `GIVEN given_*`。
- When/Then 保持不变，确保行为验收口径不漂移。

### Step E：验收
- 必须运行：`bddc check --project-root /home/wangbo/document/shop --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --docs-root docs/bdd --in docs/bdd --out test/bdd_generated`，直到全绿（兼容：`MIX_ENV=test mix bdd.check`）。
- 如失败：优先修 Given 数据准备，让可观测断言通过；禁止改 Then 去“迁就实现”。

## 何时算完成（验收标准）
- 目标 DSL 文件中 80%+ 的 Given 不再堆字段/原子插表，而是复用 `given_*` 入口。
- `bddc check --project-root /home/wangbo/document/shop --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --docs-root docs/bdd --in docs/bdd --out test/bdd_generated` 全绿，并可重复运行结果一致（兼容：`MIX_ENV=test mix bdd.check`）。
- 数据类 Given 与脚手架类 Given 边界清晰：数据工厂只负责“数据状态”，不负责“启动/重启/注入依赖”。
