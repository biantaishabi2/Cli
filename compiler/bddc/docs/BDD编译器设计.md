# BDD 编译器设计（文档即测试，DSL -> ExUnit）

## 0. 背景
本项目需要大量 BDD（Given/When/Then）验收用例，覆盖多个业务模块（售后、支付、结算等）。

现状问题：
- 纯手写 ExUnit 集成测试：可跑，但维护成本高、容易遗漏“验收文档”与“代码实现”的同步。
- 纯文档验收：可读，但无法自动回归。

目标是做一个“编译器一样的东西”：以 **DSL** 写验收场景，按既定理论/约束编译成可运行的 ExUnit 测试，类似平级项目 Stitch 的 UI 编译器（DSL -> 产物）。

## 0.1 当前进度（截至 2026-02-09）
已落地（可执行/可验收）：
- 编译链路：`docs/bdd/*.dsl` -> `bdd_compiler compile` 生成 `test/bdd_generated/*_test.exs` -> `mix test test/bdd_generated` 全绿
- 失败要早：未知指令/缺参/多参/类型不匹配/未定义变量/跨文件 SCENARIO ID 重复，均在编译期硬失败并携带 file/line/raw 定位
- 场景结构门禁：每个 SCENARIO 至少 1 个 WHEN + 1 个 THEN，否则硬失败
- 时间门禁：使用 `now()` 前必须先 `WHEN clock_freeze ...`（保证可复现）
- 指令集版本化：默认 v1；当场景 `TAGS` 含 `bdd_v2` 时启用 v2（允许 HTTP driver 指令）
- 治理：`bdd_compiler lint`（弱断言 / sleep 反模式 / 顺序依赖）+ `bdd_compiler check` 一键门禁
- 金标准（Golden Master）验收清单：`docs/bdd/BDD编译器金标准验收.md`（以真实手写测试为基线，用于衡量 DSL/编译器是否“等价表达”可观测行为）
- 结算路线B相关 BDD 场景已覆盖（见 `docs/bdd/`）：
  - 售后退款在出单前完成：当期吸收、无 Adjustment
  - Step7 对账 diff 阻断执行
  - 上期锁定后发生退款：下期生成 Adjustment 并推进扣除状态
  - 负向：invalid_params / invalid_status / duplicate_settlement

## 0.2 增量进度（截至 2026-02-11）
- 财务文档中的 B2B 客户对账新增指令已落地（`given_*`、`cs_*`、`assert_cs_*`）。
- 运行时实现已模块化为 `test/support/bdd/customer_statement_runtime.ex`，由 `test/support/bdd/instructions_v1.ex` 分发。
- `assert_received_event` 已兼容 `type=` 写法并支持 `settlement_no` 校验。
- 全量验收命令通过：
  - `MIX_ENV=test mix bdd.check --instructions test/support/bdd/instructions_v1.ex --in docs/bdd --out test/bdd_generated --docs-root docs`
  - 结果：`227 tests, 0 failures, 1 skipped`

## 1. 目标与非目标

### 1.1 目标（必须满足）
- 单一来源：验收文档中的场景块可被机器解析并编译为测试，避免“文档一套、测试一套”漂移。
- BDD 只断言“可观测行为”，不测内部实现细节。
- 数据准备仅允许“内置指令（data primitives）+ 工厂模式”，禁止 DSL 直接写 SQL/Ecto，减少不确定性与误用。
- 可扩展：支持多模块，不是专为某一个模块定制。
- 失败要早：编译期（而不是运行期）尽量发现错误（未知指令、缺参数、类型不匹配）。

### 1.2 非目标（明确不做）
- 不做“任意自然语言 -> 测试代码”的魔法生成。
- 不允许 DSL 执行任意 Elixir 表达式、任意 Repo 操作、任意 SQL。
- 不追求 100% 自动生成可跑测试（所有数据与断言都自动推断）；第一版的目标是“可编译 + 可跑 + 稳定”。

## 2. 核心决策（本轮讨论结论）

### 2.1 DSL 是否要先有文档？
需要。并且推荐“单一来源”：**人能看懂的设计文档中嵌入可编译 DSL 场景块**。
- 人阅读：背景、解释段落（自然语言）。
- 机器编译：只解析规范块（DSL）。

这样避免“双维护”：先写文档再写 DSL 的重复劳动。

### 2.2 数据创建策略
数据创建只能允许内置 data 指令（含工厂/夹具），原因：
- 降低 DSL 作者瞎写导致的不确定性；
- 不暴露表结构细节给 DSL；
- 便于在不同项目/模块迁移（只需要迁移指令实现包/指令集）。

### 2.3 “可观测边界”是否必须按 DDD？
不要求。核心不是 DDD 名字，而是“接口边界”：
- 如果系统没有 DDD 分层，只有一个 `Service`，那它的公开方法就视为 **应用用例入口（Application Service 等价物）**。
- BDD 的 When 必须走稳定边界：HTTP / Service 公共方法 / 事件入口（按优先级）。

## 3. 接口驱动（When 走哪个接口？）
BDD 的 When 必须从“系统边界”驱动，按优先级使用：

1. HTTP/API（最推荐）
- 最贴近真实用户行为，稳定、可迁移。

2. 应用服务（Application Service / 用例入口）
- 当模块没有 HTTP 时，以应用服务作为用例边界。

3. 单体 Service（非 DDD 也可）
- 视为应用服务等价物，但要求只调用其“公开方法”。

4. 事件入口（PubSub/消息）
- 当业务本身由事件驱动时，事件就是接口边界（例如 `payment:events` 的 `refund_success`）。

约束：
- DSL 不允许直接调用私有函数。
- 每个 When 指令必须来自注册过的 BDD 指令集（指令注册表）。

## 4. 可观测范围（Then 允许断言什么？）
Then 只允许断言可观测结果，分四类：

A. 外部响应（HTTP status / JSON body）
- 最强可观测；优先使用。

B. 事件（PubSub / Domain Event）
- 可观测：收到事件、payload 满足约束。

C. 持久化后的“业务事实”
- 允许读 DB/Read Model 来断言业务事实（例如状态、金额、是否产生结算行、是否产生 Adjustment）。
- 禁止断言实现细节（临时字段、内部中间结构、非业务事实的 metadata）。
- 断言也必须通过内置 assert steps，避免 DSL 直接写查询。

D. 外部副作用（支付/三方调用）
- 通过 mock/spy “被调用一次、参数正确”来断言（同样走内置 steps）。

## 5. DSL 设计（v1）
### 5.1 目录约定
- DSL 文档：`docs/bdd/*.dsl`
- 编译输出：`test/bdd_generated/*_test.exs`

### 5.2 语法（行式，易解析）
- 场景头：
  - `[SCENARIO: <ID>] TITLE: <标题> TAGS: <tag1> <tag2> ...`
- 变量：
  - `LET <name>=uuid()`
  - `LET <name>=now()`
  - `LET <name>=dec("10.00")`
  - `LET <name>=dt("2026-02-09T00:00:00Z")`
  - `LET <name>=date("2026-01-31")`
  - `LET <name>="文本"`
  - 引用变量：`$name`
- 步骤：
  - `GIVEN <step_name> key=value ...`
  - `WHEN <step_name> key=value ...`
  - `THEN <step_name> key=value ...`

值类型（v1）：
- `uuid()` / `now()` / `dec("10.00")` / `dt("...Z")` / `date("YYYY-MM-DD")` / `"string"` / `int`（支持负数，例如 `-1000`）/ `$var`

### 5.3 设计原则
- DSL 不直接引用表名/字段名。
- DSL 不包含任意代码执行能力。
- 所有指令都必须在指令注册表中注册，未注册则编译失败。

## 6. BDD 指令集（API 适配集合）设计
> 说明：这里不再用“steps/步骤”作为工程术语，避免误解。
> BDD 文本里仍然是 Given/When/Then 的“步骤”写法，但编译器层面我们把它定义为 **指令（primitive）**：
> - **驱动指令（act）**：调用系统边界（HTTP / 应用服务 / 单体 Service / 事件入口）
> - **断言指令（assert）**：断言可观测结果（响应/事件/业务事实/外部副作用）
> - **数据指令（data）**：数据准备（仅允许通过内置工厂/夹具）

### 6.1 指令（primitive）的契约
每条指令必须声明：
- `name`：指令名（DSL 中 Given/When/Then 的动作名）
- `kind`：`data` | `act` | `assert`
- 参数签名：必填/可选 + 类型（uuid/decimal/datetime/string/int/var）
- 输出：会写入哪些变量（例如创建 payment 输出 `$payment_id`）
- 可观测类别：A/B/C/D（act/assert 必填；data 视为准备阶段）

### 6.2 “插件”在这里是什么意思（语义澄清）
“插件”不是业务概念，只是工程组织方式：把指令实现按模块/上下文拆包。
- aftersales 指令实现：提交售后、查询售后事实、断言状态等
- payment 指令实现：创建支付、广播退款事件等
- settlement 指令实现：出单、查询结算行、断言 Adjustment 等

编译器核心只依赖“指令契约”，不依赖具体业务模块；业务模块以“指令实现包”的形式接入（即 API 适配集合）。

### 6.3 指令集分层（按可观测行为语义）
为降低不确定性，指令按“可观测行为语义”分层，并在编译期做强约束：
- `data`：只做数据准备与工厂落库，不做断言、不触发对外副作用。
- `act`：只驱动系统边界（HTTP/Service/Event），不做断言。
- `assert`：只断言可观测结果（A/B/C/D），不做驱动动作/写库（除非是诊断输出）。

### 6.4 v1 指令清单（fixture 验收样例最小集合）
> v1 的目标不是“覆盖所有业务”，而是用最小指令清单验证编译器能力（Parse/Validate/Emit + 变量传递 + eventually + 数据工厂约束）。
> 以下清单是 v1 fixture（refund_success 3 场景）所需的最小集合，后续按模块逐步扩展。

#### 6.4.1 Meta 指令（跨场景通用）
- `clock_freeze at=datetime_utc`（kind=act）
  - 作用：固定场景时钟；后续 `now()` 从该时钟读取
- `clock_travel seconds=int`（kind=act，可选）
  - 作用：推进场景时钟（用于超时/跨期）

> 注：时钟类指令更像“测试运行时控制接口”，语义上属于 act（驱动测试环境），不是业务数据准备。

#### 6.4.2 AfterSales/Payment fixture 指令
data（数据工厂）：
- `start_refund_handler`（kind=data）
  - 作用：确保 `PaymentRefundEventHandler` 启动并订阅 topic（幂等）
- `create_payment order_id=uuid user_id=uuid total_amount=decimal now=datetime_utc -> $payment_id`（kind=data）
  - 作用：创建支付单（用于 `payment_id` 分支）；必须保证业务编号唯一性
- `create_after_sales after_sales_id=uuid order_id=uuid supplier_id=uuid user_id=uuid type=enum order_item_id=uuid product_id=string product_name=string ordered_quantity=int quantity=int unit_price=decimal`（kind=data）
  - 作用：通过“受控工厂/应用服务”创建售后与明细（落 after_sales_orders + after_sales_items）

act（驱动系统边界）：
- `broadcast_refund_success payment_id=uuid refund_amount=decimal completed_at=datetime_utc`（kind=act）
- `broadcast_refund_success order_id=uuid refund_amount=decimal completed_at=datetime_utc`（kind=act）
  - 作用：广播 `payment:events` 的 `refund_success`

assert（可观测断言）：
- `assert_refund_written after_sales_id=uuid expected_item_status=string`（kind=assert，类别=C）
  - 断言：refund_info.status==completed；after_sales_items.status==expected
- `assert_not_written after_sales_id=uuid expected_item_status=string`（kind=assert，类别=C）
  - 断言：refund_info 仍为空；after_sales_items.status==expected

### 6.5 指令签名的稳定性与扩展方式
- v1 指令签名一旦发布，应通过“版本化”（见 11.5、15.3）演进，避免静默破坏历史 DSL。
- 新增业务模块时，优先新增 `act/assert` 指令（系统边界 + 可观测断言），其次才是 data 工厂指令。

### 6.6 示例：售后退款回写（已落地的真实业务链路）
现有可观测链路：
- When：广播 `payment:events` 的 `refund_success`
- Then：回写 `after_sales_orders.refund_info(status=completed, processed_at=...)`
- Then：标记 `after_sales_items.status=completed`

现有风险点与决策（已在代码实现并补 BDD 集成测试）：
- 同一订单存在多张“可退款售后单”时不回写，返回歧义（避免写错单）。

## 7. 编译器实现（Shop v1）
### 7.1 为什么用 Elixir 实现
第一版必须用 Elixir：
- 产物是 ExUnit 测试；
- 数据准备依赖 Ecto、事件依赖 Phoenix.PubSub；
- 用 Go/Rust 会引入大量胶水成本，反而更慢更不稳。

“跨项目复用”靠两层：
- DSL 规范（语言无关）
- 各项目自己的“BDD 指令实现包（API 适配集合）”+ 后端编译器（可用 Elixir 或其它语言重写）

### 7.2 编译流水线（像编译器一样）
1. Parse：DSL -> AST
2. Validate：step 存在、参数齐全、类型匹配
3. Lower：AST -> ExUnit IR（测试模块/用例/steps 调用序列）
4. Emit：写入 `test/bdd_generated/*_test.exs`

### 7.3 运行方式
```bash
bdd_compiler compile
mix test test/bdd_generated
```

## 8. 质量门禁（像 Stitch 的 compiler gate）
- `bdd_compiler compile` 必须是可重复、确定性的（同 DSL 生成同样的测试文件）。
- 编译失败必须提示：文件、行号、原因（未知指令/缺参/类型不匹配）。
- 可选：CI 阶段强制运行编译后的测试。

### 8.1 工程风险点（必须在设计中显式覆盖）
本编译器的复杂度低于 UI 编译器，但有 4 个“实现起来最容易踩坑”的工程点，必须作为门禁考虑：

1. 编译期错误足够可读
- 目标：未知指令/缺参/类型不匹配必须在 `bdd_compiler compile` 阶段失败，并携带文件名/行号/指令名。

2. 输出确定性（deterministic）
- 目标：同一 DSL 输入多次编译，输出文件内容完全一致；否则无法作为门禁与可审计产物。

3. 指令签名稳定（需要版本化）
- 目标：指令参数一旦调整，不应静默破坏历史 DSL；必须通过版本化与兼容层控制升级。

4. 数据准备不污染开发库
- 背景：本项目 `Shop.DataCase` 明确“测试环境与开发环境共用同一个数据库，且不清理任何数据”。
- 目标：data 指令必须采用“唯一性/幂等/最小副作用”的工厂策略，避免重复运行造成脏数据或与开发数据冲突。

## 9. 已有落地与对齐（本项目现状）
本轮在 shop 项目里已经落地/修复的相关能力（供指令实现包复用）：
- 售后 items 入参校验（`type in [:return, :refund_only]` 必须携带 items）
- 退款成功事件回写售后 refund_info + after_sales_items 完成状态
- 结算路线B：as_of 之前同一期吸收、锁定后跨期 Adjustment
- Step7 风控：对账 diff != 0 阻断 execute_settlement

对应的可观测 BDD 集成测试已经存在（手写版）：
- `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs`
- `test/financial_settlement/integration/settlement_route_b_after_sales_e2e_test.exs`
- `test/financial_settlement/integration/settlement_route_b_recon_block_test.exs`

下一步：把这些手写用例抽象成“BDD 指令实现包（API 适配集合）”，并用 DSL 编译生成等价测试，作为编译器 v1 的自举验收。

## 9.1 当前实现进度（截至 2026-02-09）
> 这一节是“验收口径”：让读者一眼知道 **编译器（通用）做到了哪**、**指令集（业务适配）做到了哪**、**还有什么没做**。

已实现（可直接跑通）：
- ✅ 编译链路：`docs/bdd/*.dsl` + `docs/**/*.md` 内嵌 ```bdd block``` -> `bdd_compiler compile` -> 生成 `test/bdd_generated/*_generated_test.exs` -> `mix test test/bdd_generated`
- ✅ 编译期 hard fail：未知指令/缺参/类型不匹配/变量未定义/场景结构（至少 1 WHEN + 1 THEN）/THEN 不能调用 act/GIVEN 不能调用 assert
- ✅ 时间稳定性门禁：使用 `now()` 的场景必须先 `WHEN clock_freeze ...`（否则编译失败）
- ✅ 测试层级（scope）门禁：`TAGS: unit/integration/e2e` 限制可用指令（unit 仅允许 test_runtime 指令）
- ✅ 治理与门禁：`bdd_compiler lint` + `bdd_compiler check`（compile + lint --fail-on-warn + 生成测试）
- ✅ 结算路线B的 DSL 自举验收样例（`docs/bdd/settlement_route_b_*.dsl` + 负向用例）
- ✅ Flaky 治理（门禁参数化）：`mix bdd.check --exclude-flaky`、`--rerun-failures N（或 bdd_compiler check 的兼容链路）`
- ✅ Contract/CDC 门禁：`bdd_compiler contract.check`（consumer/provider JSON contract diff）
- ✅ Fuzz（deterministic）：`bdd_compiler fuzz`（固定 seed + shrinking）
- ✅ 受控 Snapshot（HTTP）：`assert_http_snapshot` + `BDD_SNAPSHOT_ACCEPT=1` 接受基线（snapshot 文件本地化，不提交）
- ✅ Mutation testing 接入点：`bdd_compiler mutation.report`（可接外部 mutation cmd，或输出基础报告）
- ✅ Mutation testing（最小可运行）：`bdd_compiler mutation.run`（受控 AST 变异 + 运行 `mix test test/bdd` 子集 + 输出 survivors 报告）

增强项（不阻塞当前落地）：
- 已全部落地（见本文档末尾“增强项清单”）；当前设计文档不再保留 Backlog 文件，避免反复出现“还有什么没做”。

## 9.2 用“真实结算/售后用例集”验收编译器（推荐的验收基线）
你的诉求是“测试编译器要用真实业务来验收”。这里把仓库里现存的“结算/售后 BDD 风格用例”全部清点出来，并按**接口边界层次**归类。

### 9.2.0 总量先澄清（为什么你会觉得“只有这么点”）
目前仓库里与“售后/结算”相关的测试**远不止 10 条**：
- `test/financial_settlement/**` + `test/aftersales/**`：共 **57 个测试文件 / 604 个 test case**

但其中只有一部分是“BDD 风格（Given/When/Then 命名与写法）”或“DSL BDD（编译器直接消费）”：
- DSL BDD（`docs/bdd/*.dsl`）：共 **10 个 SCENARIO**（编译器可直接编译）
- 手写 BDD 风格（`test/**` 中 `test "Given ..."`）：共 **33 个 test case**（真实口径对照组，需迁移成 DSL 才能用编译器生成）

> 下面 9.2.1/9.2.2 的清单，分别对应“可直接编译的 DSL”与“手写对照组”。

### 9.2.1 DSL 用例集（编译器直接消费）
目录：`docs/bdd/*.dsl`

现状统计（共 10 个场景）：
- 3 个：事件边界（event）+ test_runtime：`docs/bdd/aftersales_refund_success.dsl`
- 1 个：HTTP 边界（http）+ test_runtime：`docs/bdd/http_smoke.dsl`
- 6 个：应用服务边界（app_service）+ test_runtime：
  - `docs/bdd/settlement_negative_cases.dsl`（3）
  - `docs/bdd/settlement_route_b_after_sales.dsl`（1）
  - `docs/bdd/settlement_route_b_cross_period_adjustment.dsl`（1）
  - `docs/bdd/settlement_route_b_recon_block.dsl`（1）

验收命令（编译器层面的“真实用例”）：
```bash
bdd_compiler compile
mix test test/bdd_generated
```

### 9.2.2 手写 ExUnit 用例集（真实口径的“对照组”）
这些是仓库里已经存在的“Given/When/Then 风格”手写测试，它们不是 DSL，不会被编译器直接编译；但它们是我们写 DSL 时最可靠的业务口径对照组。

结算路线 B（应用服务边界）：
- `test/financial_settlement/integration/settlement_route_b_flow_test.exs`：15 个用例（均为 `test "Given ..."`）
  - 驱动边界：应用服务 `SettlementApplicationService.*`（app_service）
- `test/financial_settlement/integration/settlement_route_b_recon_block_test.exs`：1 个用例
  - 驱动边界：应用服务（app_service）
- `test/financial_settlement/integration/settlement_route_b_after_sales_e2e_test.exs`：1 个用例
  - 驱动边界：应用服务（app_service）+ 事件/适配组合（属于 end-to-end 链路）

售后退款回写（事件边界）：
- `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs`：3 个用例
  - 驱动边界：`Phoenix.PubSub.broadcast(..., {:refund_success, ...})`（event）

其它与“结算/售后”相关且同样采用 Given 风格命名的用例（用于扩充口径时参考）：
- `test/financial_settlement/services/refund_deduction_service_test.exs`：2 个 Given 风格用例（服务/仓储层次）
- `test/financial_settlement/services/settlement_recon_bridge_test.exs`：2 个 Given 风格用例（服务/仓储层次）
- `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs`：3 个 Given 风格用例（DB/Repository 层次）
- `test/aftersales/domain/services/after_sales_service_test.exs`：4 个 Given 风格用例（domain/service 层次）

### 9.2.3 “编译器差不多了”的验收标准（你要的那句话落地）
当满足下面两点，就可以认为编译器已达到“用真实业务验收”的可用水平：
1) **现有 DSL 用例集**（`docs/bdd/*.dsl` 共 10 场景）可以稳定编译并运行全绿：
   - `bdd_compiler compile && mix test test/bdd_generated`
2) **结算路线 B + 售后退款回写**的手写 Given/When/Then 用例（上面列出的核心 20 条：15+1+1+3）被逐步迁移为 DSL，并能生成等价测试全绿。

> 说明：第 2 点不是“编译器能力缺失”，而是“真实口径迁移”的工作量。迁移完成后，编译器的真实验收集就从“10 条”升级为“覆盖核心链路的 20+ 条”。如果把仓库里现有 **33 条手写 Given 风格**都迁移完，则验收集会进一步扩大。

## 10. 实施计划（可一步一步落地，v1 -> v2）
本节的目标是把“设计”拆成可以按顺序交付、每一步都有验收点的任务清单。

> 说明：本仓库目前已经完成到 M4（含 lint/分层/时间门禁/自举 fixture）。下面的步骤保留为“从 0 复现/迁移”的指南。

### 10.0 分部实施（里程碑/可验收交付物）
为了避免“做一半无法验证”的风险，实施按里程碑拆分；每个里程碑都必须有明确的 BDD（Given/When/Then）验收，且可通过命令一键运行。

里程碑总览：
1. M0：约束冻结（DSL + 指令契约 + 目录/输出约定）
2. M1：编译器核心（Parse + Validate），失败要早且报错可读
3. M2：编译器后端（Emit + Mix task），输出确定性 + 可运行
4. M3：最小指令实现包（fixture 验收样例），证明“指令机制可用”
5. M4：治理增强（lint/分层/tag 约束/时间与异步稳定性）逐步上线

每个里程碑的“验收测试”分两类：
- 编译器自身的 VDD/BDD（测试编译器的可观测行为：错误信息、输出文件、确定性）
- 自举验收（编译生成的测试可运行且全绿）

### 10.1 v1 范围（先做可闭环的最小集合）
v1 的核心交付是“模块无关的编译器核心 + 指令契约机制”，并用 1 条真实链路作为验收样例（fixture，可替换）：
- 编译器核心（模块无关）：Parse / Validate / Emit / Mix task / 错误定位 / 确定性输出
- 指令契约机制（模块无关）：指令名、参数签名、类型、输出变量
- 验收样例（fixture，可替换）：售后退款成功事件回写（`payment:events refund_success`）
  - 覆盖 `payment_id` 分支与 `order_id` 分支
  - 覆盖“同一订单多张可退款售后单 -> 歧义跳过回写”

v1 不做：
- HTTP 驱动（先以应用服务/事件入口为主）
- 任意模块的任意数据工厂（只做 v1 场景用到的最小工厂）

### 10.2 v1 交付拆解（每步都有验收）
> 你可以把这一节当成“从 0 到 1 的可执行 TODO”。每一步都给出：
> - 要做什么（产物在哪里）
> - 怎么验证（命令 + 预期现象）
>
> 约束提醒：本项目 `Shop.DataCase` **不清库**，因此 data 指令必须遵循唯一性/幂等/最小副作用（见 10.4）。

#### 10.2.0 可执行步骤与验证清单（按顺序执行）
> 推荐执行顺序：先让“编译器自身 VDD”全绿，再跑“生成测试（自举验收）”。

0. 环境与目录检查（只做一次）
   - 目标：确认目录约定与输出约定存在，不会误写到别处。
   - 产物约定：
     - DSL 输入：`docs/bdd/*.dsl`
     - 生成输出：`test/bdd_generated/*_generated_test.exs`（建议忽略提交）
   - 验证：
     - `ls docs/bdd`
     - `bdd_compiler compile`（应生成/覆盖 `test/bdd_generated/` 下文件）

1. DSL 文件约定与样例（先有文档再有代码）
   - 要做什么：
     - 新增至少 1 个 `.dsl` 场景文件到 `docs/bdd/`。
     - 每个文件包含 1..N 个场景，每个场景必须先写 `[SCENARIO: ...]` 头。
   - 验证（编译器 VDD 层）：
     - 写一个“缺少 SCENARIO 头”的 DSL，执行 `bdd_compiler compile` 应失败，错误包含 `file/line/raw`。
   - 验证（自举层）：
     - `bdd_compiler compile` 后，`test/bdd_generated` 目录下应出现对应 `*_generated_test.exs`。

2. Parser（DSL -> AST）与错误定位
   - 要做什么：
     - 实现 `Shop.BDD.DslParser` 解析 `docs/bdd/*.dsl`，并为每一条步骤附带 meta：`file/line/raw`。
   - 怎么验证：
     - 运行 `mix test test/bdd/compiler_vdd_test.exs`
     - 预期：包含“非法 DSL 报错带行号/原始行”的用例全绿。

3. 指令注册表（Registry）与类型校验（Validate）
   - 要做什么：
     - 在注册表声明指令：`name/kind(args types)/outputs/rules`。
     - 在 Validate 阶段做硬校验：未知指令、缺参、多参、类型不匹配、未定义变量、规则（例如 exactly_one_of）。
   - 怎么验证：
     - `mix test test/bdd/compiler_vdd_test.exs`
     - 预期：未知指令/未定义变量/违反约束的用例必须“编译期硬失败”，并指到具体 DSL 行。

4. 编译器后端（AST -> ExUnit 代码，Emit）
   - 要做什么：
     - 把场景编译成可读的 ExUnit 测试。
     - 生成的测试必须保留：SCENARIO ID、TITLE、TAGS、原始步骤文本（用于调试）。
     - 输出必须 deterministic（同输入同输出）。
   - 怎么验证：
     - `mix test test/bdd/compiler_vdd_test.exs`（deterministic 用例）
     - `bdd_compiler compile && ls test/bdd_generated`
     - 预期：生成文件存在；同一 DSL 连续编译两次，生成文件内容逐字节一致。

5. Mix 任务（编译入口）
   - 要做什么：
     - 提供 `bdd_compiler compile`：
       - 默认扫描 `docs/bdd/*.dsl`
       - 输出覆盖写入 `test/bdd_generated/`
       - 失败要早：Validate 不通过直接报错退出
   - 怎么验证：
     - `bdd_compiler compile`
     - 预期：0 退出码，并提示“生成了 N 个测试文件”。

6. v1 指令实现包（最小闭环：data/act/assert）
   - 要做什么：
     - 提供运行期指令实现（供生成测试调用），并遵循分层语义：
       - data：只做数据准备（工厂落库），禁止断言/外部副作用
       - act：只驱动边界（事件/应用服务/HTTP），禁止断言
       - assert：只断言可观测事实（A/B/C/D），禁止驱动动作
   - 怎么验证（自举验收）：
     - `bdd_compiler compile`
     - `mix test test/bdd_generated`
     - 预期：fixture 场景全绿。

7. 自举门禁（Compiler Gate）
   - 要做什么：
     - 将 v1 DSL fixture 放进仓库（`docs/bdd/*.dsl`）
     - 在 CI/本地流程中强制：
       - `bdd_compiler compile`
       - `mix test test/bdd_generated`
   - 怎么验证：
     - 人工在 DSL 里引入一个非法点（如未知指令）再跑 `bdd_compiler compile`，应在编译阶段直接失败（不会生成“半对的测试”）。

8. v1.1（可复现时间）：CLOCK/now() 最佳实践固化
   - 背景：时间是集成测试最常见的 flakiness 来源（微秒精度/边界抖动）。
   - 要做什么：
     - 提供 `WHEN clock_freeze at=dt("...Z")`（act）
     - 提供 `dt("...Z")` 表达式与受控 `now()`（从场景时钟读取）
     - 规则：场景使用 `now()` 前必须先 `clock_freeze`（编译期硬失败）
   - 怎么验证：
     - `mix test test/bdd/compiler_vdd_test.exs`（包含 now_without_freeze 必须失败的用例）
     - `bdd_compiler compile && mix test test/bdd_generated`（生成测试全绿且可复现）

9. 下一步（v2 之前的推荐落地顺序：先扩展指令包，再上治理门禁）
   - 扩展指令实现包（自举扩展验收）：
     - 目标：把“结算路线B/对账阻断”等现有手写集成测试抽象成 DSL，然后生成等价测试跑通。
   - 治理增强（M4）：
     - 跨文件 `SCENARIO ID` 唯一性校验
     - 场景结构门禁（至少 1 WHEN + 1 THEN；G/W/T 只能调用对应 kind 指令）
      - 逐步引入 `bdd_compiler lint`（弱断言/反模式提示）
   - 建议提供“一键门禁命令”：
     - `bdd_compiler check`（顺序执行：`bdd_compiler compile` -> `bdd_compiler lint --fail-on-warn` -> `MIX_ENV=test mix test test/bdd_generated`）

### 10.3 v2 扩展方向（后续再做）
> v2 的核心不是“堆功能”，而是把 v1 的成功模式规模化：
> - 指令实现包按模块扩展（settlement/payment/...）
> - 治理能力逐步上线（lint + 门禁）
> - 驱动边界更贴近真实用户（HTTP driver）

#### 10.3.0 v2 可执行步骤与验证清单（推荐顺序）
1. 扩展指令实现包到结算模块（先自举扩展验收）
   - 目标：把“已存在的手写集成测试”改写成 DSL，并生成等价测试，确保链路可复用。
   - 要做什么：
     - 新增 DSL：
       - `docs/bdd/settlement_route_b_after_sales.dsl`（对应 `settlement_route_b_after_sales_e2e_test`）
       - `docs/bdd/settlement_route_b_recon_block.dsl`（对应 `settlement_route_b_recon_block_test`）
     - 新增/扩展指令（只做最小集合，先跑通）：
       - data：创建订单/订单项/售后、创建结算周期/快照输入、创建必要的结算相关基础数据
       - act：触发“生成结算/锁定/执行/对账”等系统边界动作（优先应用服务或事件入口）
       - assert：断言可观测事实（结算单状态、行数、金额、Adjustment、对账 diff 阻断等）
   - 怎么验证：
     - `bdd_compiler compile`
     - `mix test test/bdd_generated`
     - 预期：新增的结算 DSL 生成测试全绿，并能替代/覆盖原手写用例的关键断言点。

2. 引入 HTTP driver 指令（让 BDD 更贴近真实用户行为）
   - 要做什么：
     - 增加 act 指令：`http_post/http_get`（或按模块封装成更语义化的 act 指令）
     - 增加 assert 指令：`assert_http_response`（A 类可观测）
   - 怎么验证：
     - 新增 1 个最小 e2e DSL（仅 1 个场景即可）：`docs/bdd/http_smoke.dsl`
     - `bdd_compiler compile && mix test test/bdd_generated`
     - 预期：从 HTTP 边界驱动的场景可以生成并运行（不要求覆盖全部业务）。

3. 类型系统增强（减少“瞎写”）
   - 要做什么：
     - 支持受控 enum（例如 `type=enum(return|refund_only)` 或注册表限定 string 可选值）
     - 支持受控 bool/date（按需）
   - 怎么验证：
     - 为每个新增类型加 1 个“非法输入必须失败”的 VDD 用例（`test/bdd/compiler_vdd_test.exs`）
     - `mix test test/bdd/compiler_vdd_test.exs` 全绿。

4. 指令集版本化（避免静默破坏历史 DSL）
   - 要做什么：
     - DSL/指令声明显式版本：例如 `TAGS: bdd_v1` / `bdd_v2`（或单独 VERSION 头）
     - 注册表支持 v1/v2 并存，并对旧 DSL 给出明确迁移错误
   - 怎么验证：
     - 写一份旧 DSL（v1）与一份新 DSL（v2）
     - `bdd_compiler compile` 预期：能正确选择对应签名；不兼容变更时报“需要迁移到 v2”。

### 10.5 M4 治理增强（把“最佳实践”变成门禁）
> M4 的目标：让“坏测试/不确定测试/不可维护测试”在编译期就写不出来，或至少先被 lint 标红。

#### 10.5.1 治理项 1：跨文件 SCENARIO ID 唯一性（Hard Fail）
1. 要做什么：
   - `bdd_compiler compile` 扫描所有 `docs/bdd/*.dsl`，收集所有 `SCENARIO ID`，重复则失败。
2. 怎么验证（VDD）：
   - 新增 2 个 DSL 文件，刻意写同一个 `SCENARIO ID`
   - 运行 `bdd_compiler compile`
   - 预期：编译失败，报出重复 ID + 两处 `file/line`。

#### 10.5.2 治理项 2：场景结构门禁（Hard Fail）
1. 要做什么：
   - 每个场景至少包含：1 个 `WHEN` + 1 个 `THEN`，否则编译失败。
   - `GIVEN/WHEN/THEN` 只能调用注册表里对应 kind 的指令，否则编译失败。
2. 怎么验证（VDD）：
   - 写一个只有 GIVEN 的场景，`bdd_compiler compile` 必须失败。
   - 写一个 `THEN` 调用 act 指令的场景，`bdd_compiler compile` 必须失败。

#### 10.5.3 治理项 3：lint（先警告，成熟后可升级为失败）
1. 要做什么：
   - 增加 `bdd_compiler lint`：对 DSL 做静态检查并输出可操作建议（不改代码、不生成文件）。
   - 第一批建议实现的 lint：
     - 弱断言（只 assert 非空/只 assert ok）
     - 固定 sleep 替代 eventually
     - 断言顺序不确定（list[0]）
2. 怎么验证：
   - 写一个“弱断言”的 DSL（例如只断言字段非空）
   - 运行 `bdd_compiler lint`
   - 预期：输出 warning，包含场景 ID 与建议补充的“业务事实断言”。
   - 门禁模式：
     - 运行 `bdd_compiler lint --fail-on-warn`
     - 预期：存在 warning 时命令退出失败（用于 CI 阻断）。

### 10.4 数据工厂约束（因为 DataCase 不清库）
由于 `Shop.DataCase` 不执行数据清理，v1/v2 的 data 指令必须遵循：
- 唯一性：所有业务编号/外部键（如 order_number/payment_no/wechat_transaction_id）必须带 `System.unique_integer/1` 后缀。
- 幂等：对同一业务实体重复创建，应可安全复跑（能 upsert 或避免冲突）。
- 最小副作用：只创建该场景必须的数据，不做全局 delete/cleanup。

## 11. VDD（验证驱动）测试策略：测试编译器本身
说明：这里的“VDD”指“先定义可观测的验证，再实现编译器能力”，写法仍采用 Given/When/Then（避免测内部实现细节）。

### 11.1 Parser（语法/错误定位）
- Given：DSL 缺少 SCENARIO 头 When：parse Then：返回带行号的错误
- Given：非法 token/缺少 `key=value` When：parse Then：错误包含行号与原始行内容

### 11.2 Validate（指令/参数/类型）
- Given：引用未注册指令 When：compile Then：编译失败并提示未知指令名称
- Given：缺少必填参数 When：compile Then：编译失败并指出缺参
- Given：参数类型不匹配（如 refund_amount 不是 Decimal）When：compile Then：编译失败并指出类型

### 11.3 Emit（确定性与可读性）
- Given：同一 DSL 输入 When：连续执行两次 compile Then：生成文件内容完全一致（deterministic）
- Given：生成的测试文件 When：`mix test` Then：可编译并可运行（不依赖手工改代码）

### 11.4 End-to-End（自举验收）
- Given：v1 DSL 样例文件 When：`bdd_compiler compile` Then：生成 `test/bdd_generated/..._test.exs`
- When：`mix test test/bdd_generated` Then：场景全绿（覆盖 payment_id/order_id/歧义跳过）

### 11.5 指令集版本化（防止 DSL 被参数变更打碎）
- Given：指令签名升级（新增必填参数/改类型）When：编译旧 DSL Then：编译器给出“需要迁移到 v2 指令集”的明确错误
- Given：同一指令存在 v1/v2 两套签名 When：编译带版本声明的 DSL Then：选择对应实现并保持可运行

## 11.6 分阶段验收用例（把里程碑落到可执行 BDD）
下面把 M0~M3 的验收写成“可观测行为”的 Given/When/Then，作为实现时的硬对齐标准。

### M0：约束冻结
- Given：团队准备新增 DSL 场景 When：阅读设计 Then：明确目录（`docs/bdd`）与输出（`test/bdd_generated`）约定，且指令必须来自注册表（无 SQL/Ecto）。

### M1：Parse + Validate（失败要早）
- Given：DSL 缺少 `[SCENARIO: ...]` 头 When：`bdd_compiler compile` Then：编译失败并包含文件名/行号/原始行内容。
- Given：DSL 引用未注册指令 When：compile Then：编译失败并指明未知指令名。
- Given：DSL 缺少必填参数/类型不匹配 When：compile Then：编译失败并指出缺参或类型。
- Given：DSL 引用未定义变量 `$x` When：compile Then：编译失败并指出变量未定义。

### M2：Emit + Mix task（确定性与可运行）
- Given：同一份 DSL 输入 When：连续执行两次 `bdd_compiler compile` Then：生成文件内容完全一致（deterministic）。
- Given：成功编译后的输出 When：`mix test test/bdd_generated` Then：测试文件可编译、可运行（不得出现语法/编译错误）。

#### M2 验收标准（必须满足）
M2 不以“业务用例全绿”为目标，但必须满足以下可判定标准：
- 编译产物可编译：`mix test test/bdd_generated` 不得因语法/缺模块/缺引用失败。
- 编译产物可调试：生成的测试文件中必须保留 SCENARIO ID、TITLE、TAGS，以及原始 Given/When/Then 文本（作为注释或结构化常量）。
- 输出确定性：同一 DSL 输入在同一版本编译器下，生成文件内容完全一致（逐字节一致）。
- 禁止反模式的结构性错误：例如 THEN 中出现 act 指令调用、GIVEN 中出现 assert 指令调用，必须在编译期阻断（不会生成“结构上不合法”的测试）。

### M3：fixture 验收样例（证明指令机制可用）
> 注意：这不是“编译器绑定某模块”，而是用一条可替换的真实链路作为验收样例，证明指令机制与变量传递/数据工厂/异步断言可用。

补充：数据工厂的业务原则与验收标准见 `docs/bdd/数据工厂规范.md`（Given 规范，业务视角）。
- Given：fixture DSL（例如 refund_success 的三个场景）When：compile + run Then：生成测试全绿，且只断言可观测行为（refund_info/after_sales_items 状态）。

#### M3 验收标准（必须满足）
M3 以“生成测试全绿”为目标，并且必须满足以下可判定标准：
- 全绿：`bdd_compiler compile && mix test test/bdd_generated` 必须 0 failures。
- 稳定性阈值（防 flaky）：对 `test/bdd_generated` 连续运行 3 次（同一环境）必须全部通过。
  - 注：若引入 `assert_eventually`，必须使用统一 timeout/interval，避免过长等待掩盖问题。
- 只断言可观测行为：fixture 用例的 THEN 只能使用 A/B/C/D 类断言指令；不得出现对内部实现字段/私有函数/中间结构的断言。
- 数据安全：fixture 的 data 指令必须遵循唯一性/幂等/最小副作用（见 10.4、12.7、8.1），不得依赖数据库已有数据。
- 可诊断：失败时必须打印 run_id 与关键实体 ID（order_id/payment_id/after_sales_id），并 dump 业务事实字段（见 12.9）。

### M4：治理增强（lint/门禁逐步上线）
- Given：场景只写弱断言（仅 assert 非空）When：`bdd_compiler lint` Then：输出弱断言警告并指出建议补充的业务事实断言。
- Given：场景在 THEN 中调用 act 指令 When：compile Then：直接失败（结构语义约束）。

> 建议把上述用例落地为 `test/bdd_compiler/*_test.exs`（测试编译器）+ `docs/bdd/*.dsl`（自举 fixture）。

## 12. 测试最佳实践清单（固化到编译器，减少不确定性）
本节从“测试治理器”的角度列出应固化到编译器/指令集的最佳实践。目标是：
- 让“不规范/不健壮/不可维护”的测试在**编译期**就写不出来；
- 让生成的测试天然具有**抗重构性、确定性、可复现性**；
- 让边界/异常场景成为一等公民，而不是靠人自觉补齐。

### 12.1 BDD 结构与语义约束（编译期强制）
- 每个场景必须至少包含：1 个 `WHEN(act)` + 1 个 `THEN(assert)`；否则编译失败。
- `GIVEN` 只能调用 `kind=data` 指令；`WHEN` 只能调用 `kind=act`；`THEN` 只能调用 `kind=assert`；否则编译失败。
- 禁止在 `GIVEN` 中做断言；禁止在 `THEN` 中产生副作用（写库/发事件/调用外部）；否则编译失败。
- 场景必须声明稳定的 `SCENARIO ID`，并强制唯一（同仓库内重复 ID 编译失败）。

### 12.2 可观测行为优先（断言白名单）
断言只允许落在以下“可观测面”：
- A：HTTP 响应（status、JSON body、headers）
- B：事件（PubSub/Domain Event 的消息与 payload）
- C：持久化后的业务事实（只读关键字段，例如状态/金额/数量/是否产生结算行）
- D：外部副作用（被调用次数 + 关键参数；通过 mock/spy）

编译器应禁止（或默认禁止，需显式 allowlist）：
- 断言私有函数/内部模块调用
- 断言内部实现字段（临时 metadata、纯技术字段、无业务意义的中间结构）
- 断言“列表第一个/最后一个”这类顺序不确定的结果（除非显式声明排序规则）

### 12.2.1 领域服务能不能测？（结论：能，但默认不作为 BDD 主边界）
领域服务（Domain Service）的返回值/错误码/状态变化当然也是“可观测行为”，因此：
- `unit` 层测试领域服务非常合理：用于验证业务规则/不变量，快且稳定。

但在 `integration/e2e` 的 BDD 中，不建议默认以领域服务作为 When 的驱动边界，原因是治理目标：
- 领域服务更接近内部实现，重构时更容易改动；以它为边界会降低抗重构性。
- BDD（integration/e2e）更推荐从稳定边界驱动：HTTP / 应用服务 / 事件入口。

建议固化到指令集/门禁：
- `TAGS: unit`：允许使用“领域服务驱动指令”（act），Then 只做可观测断言。
- `TAGS: integration/e2e`：默认禁止领域服务驱动指令（lint 或编译失败，按策略配置）；如需使用必须显式声明允许（例如 `allow_domain_boundary=true`）。

### 12.3 类型系统与参数签名（减少“瞎写”）
建议把 DSL 的值类型与校验做强：
- 基础类型：`uuid`、`decimal`、`int`、`string`、`bool`
- 时间类型：`date`、`datetime_utc`（强制 `Z`）、`naive_datetime`（默认禁止，用于明确场景时才允许）
- 枚举类型：`enum(:return)` 或 `"return"` 但需编译期校验可选值
- 引用类型：`$var` 必须先定义或由前序指令输出

编译期必须失败的情况：
- 未注册指令名
- 缺少必填参数/多余未知参数
- 参数类型不匹配（例如需要 decimal 却传 string）
- 变量未定义/重复定义（除非显式覆盖语法）

### 12.4 变量生命周期与依赖可视化（可维护性）
- 强制“显式输出变量”：指令如果会产出 `$payment_id` 等，必须在签名中声明，并在编译期校验“下游引用是否存在上游产出”。
- 禁止隐式覆盖变量（避免同名变量被后续指令覆盖导致场景语义漂移）。
- 生成代码中将“输入参数/输出变量”以注释或结构化形式保留，便于 review 与调试。

### 12.5 时间最佳实践（重点：让时间相关测试稳定、可复现）
时间是集成测试最常见的不稳定来源。建议编译器/指令集内置并强制使用以下策略：

1. 场景级“固定时钟”（推荐强制）
- DSL 支持：`WHEN clock_freeze at=dt("2026-01-20T12:00:00Z")`
- 规则：场景内 `now()` 必须来自同一时钟源，而不是直接调用 `DateTime.utc_now/0`。
- 价值：避免运行时抖动与边界（秒/微秒）差异。

2. 明确 UTC 与时区边界
- 所有 `datetime` 一律使用 UTC（带 `Z`），禁止省略时区。
- 断言中避免依赖“本地时区格式化字符串”；如果必须展示，统一通过项目约定转换（例如 UTC+8 展示逻辑不属于核心事实断言）。

3. as_of/账期类用例强制使用“绝对时间”
- 结算/快照等用例必须用固定 `as_of`，禁止“当前时间推导 as_of”。
- 断言应围绕“processed_at <= as_of”这类可观测事实，而不是断言内部比较逻辑。

4. eventually 断言内的时间容忍
- 异步事件回写场景中，断言 `processed_at` 建议只校验“非空/符合 ISO8601/在 as_of 之前”，避免精确到微秒。

5. 支持 travel（可选）
- DSL 支持：`WHEN clock_travel seconds=3600` 用于模拟超时/跨期场景。
- 但要限制只影响“测试时钟”，不影响系统全局（避免干扰并行测试）。

### 12.6 异步与最终一致性（抗偶发失败）
- 对事件驱动/异步 Task 的断言，默认使用 `assert_eventually`（可配置 timeout/interval），禁止固定 `sleep` 作为唯一手段。
- 编译器可对 `act` 指令标注“是否异步”，若是异步则强制后续 THEN 用 eventually 风格断言（或给出 lint 警告）。

### 12.7 数据准备最佳实践（DataCase 不清库下的硬约束）
由于本项目测试库不清理数据，data 指令必须遵循：
- 唯一性：所有可能触发唯一约束/业务唯一的字段（如 `order_number/payment_no/wechat_transaction_id`）必须自动加入 `System.unique_integer/1` 后缀或 scenario_run_id 前缀。
- 幂等：重复执行场景不应因为重复插入而失败；优先采用 upsert/确定性主键（如 uuid5）策略。
- 最小副作用：只创建该场景必需数据，禁止任何全局 delete/cleanup。
- 禁止依赖“库里已有测试数据”（除非显式 `GIVEN use_fixture name=...` 并可追踪来源）。

### 12.8 边界与负向场景（一等公民）
建议 DSL 支持明确表达“预期失败/预期阻断”，并编译为可观测断言：
- `THEN assert_error code=:reconciliation_diff_not_zero`
- `THEN assert_validation_errors contains="items 不能为空"`
- `THEN assert_no_side_effect`（例如未产生 settlement_lines/未回写 refund_info）

同时建议门禁（可配置）：
- 对关键用例集（按 tag 或目录）要求至少 N 个负向场景，否则 CI 提示或失败。

### 12.9 可诊断性（失败时自动给出证据）
生成测试应自带“失败证据”输出（不泄露敏感数据）：
- 断言失败时 dump：关键实体的业务事实字段（状态、金额、行数、关键时间点）
- 最近一次事件 payload（若断言与事件相关）
- 场景 run_id 与关键 ID（order_id/payment_id/after_sales_id），便于定位

### 12.10 反模式（编译器应 lint 或禁止）
- ❌ 在 THEN 里写库/发事件/调用外部（把 act 混进 assert）
- ❌ 断言内部实现字段/内部模块返回值
- ❌ 使用不确定顺序断言（list[0]）
- ❌ 依赖当前时间/随机数而不固定时钟
- ❌ 用固定 sleep 替代 eventually
- ❌ 依赖数据库已有数据/跨场景共享状态
- ❌ 在非 e2e 场景滥用交互断言（mock/spy/called 等，锁死内部协作细节）

> 建议：编译器提供 `bdd_compiler lint`，对 DSL 做静态检查与反模式提示（v2 再做）。

### 12.11 Khorikov 原则映射（测试价值三支柱）
参考 Vladimir Khorikov 在《Unit Testing: Principles, Practices, and Patterns》中的核心框架：测试的总体价值由三项决定：
- 回归保护（Protection against regressions）
- 抗重构性（Refactoring resistance）
- 快反馈（Fast feedback）

编译器应把 DSL 作者推向“最大化三支柱”的写法：
- 默认优先提高抗重构性：只允许断言可观测行为（见 12.2），限制交互断言的滥用。
- 优先提高回归保护：强制关键不变量断言、要求负向/边界场景（见 12.8），避免“只断言不报错”的弱断言。
- 保障快反馈：按 tag 分层（unit/integration/e2e），限制慢步骤在错误层级出现；引入 deterministic 与 eventually 策略减少 flakiness。

### 12.12 Khorikov：状态验证优先，交互验证慎用
书中的实践倾向可以固化为编译器规则：
- 优先使用状态/输出验证（state verification）：HTTP 响应、事件 payload、业务事实字段。
- 交互验证（interaction verification）仅用于系统边界（外部支付/三方/消息发送），并且断言应只覆盖“关键参数与次数”，避免锁死内部协作顺序。

编译器/指令集建议：
- `assert` 指令默认只允许 A/B/C 类断言（见 12.2）。
- D 类断言（外部副作用）需要显式的 assert 指令（例如 `assert_outgoing_call`），并且必须标注“外部边界端口”。

### 12.13 Khorikov：避免假阳性（False positives）是底线
假阳性（代码坏了但测试仍绿）会制造虚假安全感。常见来源：
- 过度 mock（mock 直接短路真实逻辑）
- 断言过弱（只断言非空/不报错）
- 只测流程不测规则（缺少不变量/边界）

编译器可以提供的治理能力：
- lint：检测“弱断言”模式（例如仅断言 DTO 非空/仅断言 ok）
- 规则门禁：关键用例集必须包含负向/边界场景比例（见 12.8）
- 指令级约束：对 act 指令声明其关键可观测输出，强制下游 THEN 使用对应断言指令

### 12.14 Khorikov：只在系统边界 mock
为了抗重构性：
- 不要 mock 领域内部（实体/值对象/纯逻辑）。
- 只 mock 外部不稳定依赖（I/O、网络、三方、时钟、随机）。

编译器落地方式：
- 指令签名标注依赖类型：`pure` | `db` | `event` | `external_io`
- 只有 `external_io` 才允许交互断言（D 类）；否则给 lint 警告或编译失败（按策略配置）。

### 12.15 Khorikov：分层测试与约束（避免全是慢集成）
建议在 DSL 层做强约束：
- `TAGS: unit`：禁止 db/event/external_io 指令
- `TAGS: integration`：允许 db/event，但 external_io 必须 mock
- `TAGS: e2e`：允许更多真实边界，但场景数量受限（门禁）

## 13. AI 写 BDD 的提示词（让 AI 输出“符合治理”的 DSL）
说明：BDD 场景将由 AI 生成时，必须把“最佳实践与约束”显式写进提示词，避免 AI 生成脆弱/不规范的测试。

建议把提示词作为可复制模板（详见：`docs/bdd/BDD提示词.md`），并要求 AI 输出：
- 仅输出 DSL（不输出解释性文字）
- 每个场景包含 1 个 WHEN + >=1 个 THEN
- 至少包含 1 条负向/边界场景
- 使用 `WHEN clock_freeze ...` 固定时钟（涉及时间时强制）
- 只使用已注册指令名（未知指令必须先声明需要新增哪条指令）

## 14. 参考资料
《Unit Testing: Principles, Practices, and Patterns》信息（用于文档引用与团队共识）：
```text
Vladimir Khorikov. Unit Testing: Principles, Practices, and Patterns.
Manning Publications. 2020.
ISBN: 9781617296277
```

官方页面（如需查看目录/样章）：
```text
https://www.manning.com/books/unit-testing
```

Given-When-Then（GWT）结构化风格（用于团队共识与术语对齐）：
```text
https://martinfowler.com/bliki/GivenWhenThen.html
```

测试分层与 ROI（Testing Trophy，用于解释为什么要按 tag 分层治理）：
```text
https://kentcdodds.com/blog/the-testing-trophy-and-testing-classifications
```

避免 flaky 的经验（工程治理参考，特别是避免 sleep/隐式前置条件）：
```text
https://testing.googleblog.com/2008/04/tott-avoiding-flakey-tests.html
https://testing.googleblog.com/2017/04/where-do-our-flaky-tests-come-from.html
```

## 15. 规则落点：编译器强制 vs 提示词指导（避免重复）
目的：把“最佳实践”明确落到两个层面：
- 编译器层面：能强制/能静态检查/能门禁化的，优先写成规则（hard fail 或 lint）。
- 提示词层面：工具暂时无法强制、但能指导 AI 更好选材与覆盖边界的，写进提示词。

### 15.1 编译器规则（Hard Fail：编译失败）
适合放在编译器（`bdd_compiler compile`）阶段强制：
- 结构约束：每个场景必须有 `WHEN(act)` 与 `THEN(assert)`；GIVEN/WHEN/THEN 只能使用对应 kind 指令。
- 指令存在性：未注册指令名直接失败。
- 参数签名：缺参/多参/未知参数直接失败。
- 类型匹配：uuid/decimal/datetime/enum 等类型不匹配直接失败。
- 变量生命周期：引用未定义变量直接失败；隐式覆盖变量（除非显式允许）直接失败。
- 禁止越权：DSL 不能写 SQL/Ecto/任意表达式，只能调用内置 data 指令。
- 时间强制（若场景涉及时间）：必须 `WHEN clock_freeze ...` 且 datetime 必须 UTC（带 Z）。

### 15.2 编译器规则（Lint：给警告/可升级为失败）
适合先做 lint（`bdd_compiler lint`），逐步提高治理强度：
- 弱断言：只断言非空/不报错、缺少关键业务事实断言。
- 断言顺序不确定：断言 list[0]/first/last 而未声明排序。
- 过度交互断言：非 external_io 边界使用 spy/mock 断言。
- 缺少负向/边界：关键用例集（按 tag/目录）负向场景比例不足。
- 不可诊断：失败时没有输出关键证据（run_id、关键实体事实字段）。

### 15.3 编译器能力（需要新增的指令/门禁）
这些属于“工具能力”，应写进设计与 roadmap（通常 v2 起）：
- Contract / CDC：contract 指令与门禁（consumer/provider pipeline）。
- Property-based / fuzz：受控 generator 指令 + 固定 seed + shrinking。
- Snapshot（受控）：强制 “snapshot + 关键显式断言”。
- Mutation testing：周期性门禁/报告，把 surviving mutants 反馈给 DSL 作者/AI。
- Flaky 治理：失败重跑策略、flaky 标记与隔离（与 deterministic/eventually 配合）。

### 15.4 提示词应承担的内容（工具不必重复强调）
提示词应重点指导 AI 做“内容选择”，而不是重复编译器已强制的规则：
- 优先选择业务不变量/规则作为 Then（提升回归保护）。
- 系统性列出边界/负向场景（幂等、歧义、并发、时间边界、权限/状态机边界）。
- 交互断言只用于系统边界（external_io），能用状态验证就不用交互验证。
- 若需要新指令：先用 `# NEED_PRIMITIVE:` 声明所需指令签名，而不是杜撰指令名。

> 提示词里对“编译器已 hard fail 的规则”只需一句“必须满足编译器约束”，避免冗长重复。

## 16. 增强项清单
这一节把本项目里额外强化的治理能力集中列出来，便于验收与复用。

### 16.1 可诊断性门禁（strict_evidence）
- 行为：当场景带 `TAGS: strict_evidence` 时，如果 THEN 没有任何强断言（A/B/C/D/error），`bdd_compiler lint` 给出 `insufficient_evidence` warning。
- 验收：`mix test test/bdd/linter_vdd_test.exs`

### 16.2 目录级 lint 阻断（fail-globs）
- 行为：`bdd_compiler lint --fail-on-warn --fail-globs "docs/bdd/critical/**/*.dsl"` 时，只有匹配 glob 的 warnings 才会阻断（便于渐进收紧）。
- 验收：`mix test test/bdd/lint_task_vdd_test.exs`

### 16.3 Mutation testing（最小可运行）
- 命令：`bdd_compiler mutation.run --in lib/shop/bdd --max-mutants 80 --report tmp/bdd_mutation_report.txt`
- 输出：报告包含 mutation score、killed/survived/invalid 统计与 survivors 列表（文件/变异片段）。
- 验收：先用小样本跑通 `--max-mutants 3`，再逐步调大；该任务默认不建议作为强门禁（threshold 默认 0，可配置）。

### 16.4 指令注册表按业务域拆分（已落地：全域拆分）

**现状问题**：`instruction_registry.ex` 已有 296 条指令、4200+ 行，全部平铺在一个 `specs(:v1)` 的大 map 里。随着新业务域（库存、物流、合同等）加入，文件会持续膨胀，查找困难、多人协作容易冲突。

**目标**：每个业务域的指令放在独立文件中，主注册表只做合并分发。

**目录结构**：

```
lib/shop/bdd/
├── instruction_registry.ex              # 主入口：组装/冲突检测/版本差量合并
├── instruction_registries/
│   ├── common.ex                        # 通用指令（clock_freeze/assert_noop 等）
│   ├── aftersales.ex                    # 售后域指令
│   ├── order.ex                         # 订单/支付域指令
│   ├── settlement.ex                    # 结算域指令
│   ├── reconciliation.ex                # 对账域指令
│   ├── financial_report.ex              # 财务报表域指令
│   ├── inventory.ex                     # 库存域指令
│   ├── generated.ex                     # scaffold/upsert 的临时落盘区（Generated registry）
│   └── ...                              # 以后新域加文件即可
```

**主入口实现（当前代码口径，简化示意）**：

```elixir
# instruction_registry.ex
defp specs(:v1) do
  base = %{} # 主文件不再承载指令，全部下放到各域 registry

  generated = Shop.BDD.InstructionRegistries.Generated.specs(:v1)

  specs =
    base
    |> merge_specs!(Shop.BDD.InstructionRegistries.Common.specs(:v1), :common)
    |> merge_specs!(Shop.BDD.InstructionRegistries.Order.specs(:v1), :order)
    |> merge_specs!(Shop.BDD.InstructionRegistries.Settlement.specs(:v1), :settlement)
    |> merge_specs!(Shop.BDD.InstructionRegistries.AfterSales.specs(:v1), :aftersales)
    |> merge_specs!(Shop.BDD.InstructionRegistries.Reconciliation.specs(:v1), :reconciliation)
    |> merge_specs!(Shop.BDD.InstructionRegistries.FinancialReport.specs(:v1), :financial_report)
    |> merge_specs!(Shop.BDD.InstructionRegistries.Inventory.specs(:v1), :inventory)

  # Generated 是 scaffold 临时落盘区，优先级最低：不允许覆盖已定义的指令签名。
  generated
  |> Map.merge(specs)
  |> normalize_specs()
end
```

**注意**：
- 已落地冲突检测：域与域合并时若出现同名 key，会 fail-fast（防止静默覆盖）。
- v2 合并策略：各域的 `specs(:v2)` 往往是 `Map.merge(specs(:v1), v2_extras)`，主入口应只合并 v2-only delta，避免与 v1 重复 key 冲突（当前代码已按 delta 合并）。

**域模块模板**：

```elixir
# lib/shop/bdd/instruction_registries/inventory.ex
defmodule Shop.BDD.InstructionRegistries.Inventory do
  @moduledoc "库存域 BDD 指令注册表"

  @spec specs(:v1 | :v2) :: map()
  def specs(:v1) do
    %{
      create_inventory_product: %{
        name: :create_inventory_product,
        kind: :given,
        boundary: :db,
        scopes: [:integration],
        args: %{...},
        outputs: %{product_row_id: :uuid},
        rules: [],
        async?: false,
        eventually?: false,
        assert_class: nil
      },
      # ... 该域的其他指令
    }
  end

  def specs(:v2), do: specs(:v1)
end
```

**实施策略（已落地现状）**：
- 指令签名已从 `instruction_registry.ex` 全部迁出，主文件不再承载“296 条指令的大 map”，只负责组装与冲突检测
- 每个业务域在 `instruction_registries/` 下独立维护（`common / order / settlement / aftersales / reconciliation / financial_report / inventory`）
- 主入口合并使用 fail-fast（同名 key 直接报错，避免 `Map.merge/2` 静默覆盖）
- v2 合并按“delta”策略：只合并各域 v2-only 扩展，避免 `specs(:v2)=Map.merge(specs(:v1), v2_extras)` 造成重复 key 冲突
- 编译器核心（Parser/Validator/Emitter）不需要感知业务域拆分，`fetch/2` / `all/1` 等接口保持不变

**验收**：
- `bdd_compiler compile` 正常编译（新域指令可被识别）
- `bdd_compiler annotations.check --project-root /home/wangbo/document/shop` 通过
- `bdd_compiler check` 全部通过
- 历史 DSL / 金标准用例行为保持不变（回归由 `bdd_compiler check` 覆盖）

### 16.5 BDD 编译器独立 CLI 化（已实现：escript 先行）

**现状问题**：BDD 编译器（Parser / Validator / Emitter / Lint）完全嵌入在 Shop 项目中（`lib/shop/bdd/`），作为 mix task 运行。其他 Elixir 项目（如 ZCPG）如果想用同一套 BDD 测试体系，必须把整个编译器代码复制过去或做 git 子模块，维护成本高。

**目标**：将编译器核心抽成独立 CLI 工具，任何 Elixir 项目只需在目录下放指令定义和 DSL 文件，就能编译出 ExUnit 测试。

**落地现状（本仓库）**：
- 编译器已抽到独立 Mix 项目：`tools/bdd_compiler/`
- 产物为 `escript`：`tools/bdd_compiler/bdd_compiler`
- CLI 已支持：`compile` / `lint` / `check`

#### 职责与落盘约定（关键：与编译器本体分离）

独立 CLI 化后，整体会拆成两层：

- **编译器本体（CLI 工具）**：只做纯文本转换（解析/校验/生成/lint），不依赖任何业务代码，不访问数据库。
- **项目适配层（每个业务项目自己维护）**：包括指令签名（契约）与指令运行时实现（SemanticGivens + dispatch），依赖该项目的 Repo/Service/Schema，负责真正的业务动作与断言。

因此：

- A 项目要用 CLI：A 项目目录里必须有 A 自己的 `instructions*.exs` 与 `test/support/bdd/**`
- B 项目要用 CLI：B 项目目录里必须有 B 自己的 `instructions*.exs` 与 `test/support/bdd/**`
- CLI 只共享“同一套编译器”，不共享各项目的业务指令集与数据工厂实现（除非是“通用指令包”，见下文）。

**推荐落盘位置**：

使用方项目（A/B 项目）：

```
priv/bdd/
  instructions_v1.exs            # 指令签名（纯数据 map，供 CLI 读取）
  instructions_v2.exs            # 可选：v2 扩展（若需要）
docs/bdd/
  *.dsl                          # DSL 场景文件
test/support/bdd/
  instructions_v1.ex             # 运行期 dispatch（run_step!/run!/等）
  semantic_givens/
    <domain>_givens_v1.ex        # Given/When/Then 的语义实现（数据准备/动作/断言）
test/bdd_generated/
  *_generated_test.exs           # CLI 输出（自动生成）
```

CLI 工具（独立项目 / escript）（本仓库实现路径）：

```
tools/bdd_compiler/
  lib/bdd_compiler/dsl_parser.ex
  lib/bdd_compiler/validator.ex
  lib/bdd_compiler/emitter.ex
  lib/bdd_compiler/linter.ex
  lib/bdd_compiler/compiler.ex
  lib/bdd_compiler/cli.ex
  mix.exs   # escript: [main_module: BDDCompiler.CLI]
```

**指令集如何生成**：

- 先用最直接方式：手写 `priv/bdd/instructions_v1.exs`（稳定后再自动化）
- 进阶（推荐演进方向）：在项目代码里用统一注解标记“可观测边界指令”，由生成器（AST 扫描）自动产出：
  - `priv/bdd/instructions_v1.exs`（给 CLI）
  - 可选：`lib/<app>/bdd/instruction_registries/<domain>.ex`（给项目内 mix task/编译期使用）

#### 架构拆分（目标架构与当前实现一致）

```
bdd_compiler（独立项目 / escript）
├── lib/
│   ├── bdd_compiler/dsl_parser.ex
│   ├── bdd_compiler/validator.ex
│   ├── bdd_compiler/emitter.ex
│   ├── bdd_compiler/linter.ex
│   ├── bdd_compiler/compiler.ex
│   ├── bdd_compiler/instruction_set.ex
│   └── bdd_compiler/cli.ex
├── mix.exs                      # escript: [main_module: BDDCompiler.CLI]
└── README.md

任意 Elixir 项目（使用方）
├── priv/bdd/
│   └── instructions.exs         # 本项目的指令签名定义（纯数据，无需依赖编译器模块）
├── docs/bdd/
│   └── *.dsl                    # DSL 场景文件
├── test/support/bdd/
│   └── instructions_v1.ex       # 运行时实现（dispatch + 语义Given）
└── test/bdd_generated/
    └── *.exs                    # 编译器输出（自动生成的 ExUnit 测试）
```

#### CLI 接口设计

```bash
# 构建（本仓库）
cd tools/bdd_compiler
mix deps.get
mix escript.build

# 使用（任意项目，编译器本体：纯文本转换）
./tools/bdd_compiler/bdd_compiler compile \
  --in docs/bdd \
  --out test/bdd_generated \
  --instructions priv/bdd/instructions_v1.exs \
  --instructions-v2 priv/bdd/instructions_v2.exs

./tools/bdd_compiler/bdd_compiler lint \
  --in docs/bdd \
  --instructions priv/bdd/instructions_v1.exs \
  --fail-on-warn

./tools/bdd_compiler/bdd_compiler check \
  --in docs/bdd \
  --out test/bdd_generated \
  --instructions priv/bdd/instructions_v1.exs \
  --instructions-v2 priv/bdd/instructions_v2.exs \
  --fail-on-warn
```

生成代码参数（可选）：
- `--runtime-module Shop.BDD.Instructions.V1`
- `--test-case Shop.DataCase`
- `--module-prefix Shop.BDD.Generated`
- `--docs-root docs`

**保持一致（项目命令，兼容 mix bdd.*）**：

说明：
- `annotations/registry/instructions/contract/fuzz/mutation` 这类项目命令会在目标项目根目录下调用 `mix` 同名任务；
- `factories.*` 为 CLI 内置实现（行为兼容原 `mix bdd.factories.*`）。

```bash
# 默认在当前目录执行；也可显式指定项目根目录
./tools/bdd_compiler/bdd_compiler annotations.check [--project-root .] [--include-test]

./tools/bdd_compiler/bdd_compiler registry.scaffold [--project-root .] --module ... --functions ... [--kind when] [--version v1] [--prefix xxx]

./tools/bdd_compiler/bdd_compiler registry.upsert [--project-root .] --module ... --functions ... [--kind when] [--version v1] [--prefix xxx]

./tools/bdd_compiler/bdd_compiler instructions.docs [--project-root .] [--version v1|v2] [--output docs/bdd/指令集.md]

# factories（CLI 内置实现，兼容原 mix bdd.factories.* 行为）
./tools/bdd_compiler/bdd_compiler factories.scaffold [--project-root .] [--scope priv/bdd/factories_scope.exs] [--out /tmp/out_dir]
./tools/bdd_compiler/bdd_compiler factories.upsert [--project-root .] [--scope priv/bdd/factories_scope.exs]
./tools/bdd_compiler/bdd_compiler factories.check [--project-root .] [--paths test/support/bdd/factories_generated]

# 一键串联（注解检查 -> 指令写回 -> 文档 -> check）
./tools/bdd_compiler/bdd_compiler domain.autowire --project-root . --module ... --functions ... [--prefix xxx] [--kind when] [--version v1] [--strict true|false] [--fail-on-warn true|false] [--dry-run]
```

#### 最终口径（跨项目接入，避免歧义）

这一节用于回答三个高频问题：`v1/v2` 是什么、跨项目怎么配路径、指令集后面还要不要“执行器”。

1. `v1/v2` 的真实含义（不是业务版本）
- `v1`：稳定基础指令集（主干）。
- `v2`：在 `v1` 上做增量扩展（可以只放 delta）。
- 编译器解析规则：先查 `v2`，未命中回退 `v1`，即 `v2 = v1 + delta`。
- 结论：如果项目当前没有双版本演进需求，只提供 `--instructions`（v1）即可。

2. 跨项目必须提供的“两条路径 + 一个模块”
- 路径一（指令签名）：`--instructions`（可选再加 `--instructions-v2`）
- 路径二（DSL 输入）：`--in docs/bdd`（输出用 `--out`）
- 一个模块（运行执行器）：`--runtime-module <App>.BDD.Instructions.V1`

> 编译器本体只做“文档 -> 测试代码”转换；真正执行 Given/When/Then 业务动作的是 `runtime-module`。

3. 指令集生成后，必须还有“执行器”
- 只有指令签名（instructions*.exs）还不够，它只能通过编译期校验。
- 要让生成测试真正可跑，项目里必须实现运行时分发与语义执行（`run!/run_step!` + semantic givens/acts/asserts）。
- 因此“连起来”的完整闭环是：
  1) 维护/生成指令签名（编译期）
  2) 实现运行时执行器（运行期）
  3) `bdd_compiler check` 生成测试并运行 `mix test test/bdd_generated`

4. 一键串联命令（减少手工步骤）
- 纯编译链路：`compile / lint / check`
- 项目维护链路：`annotations.check / registry.scaffold / registry.upsert / instructions.docs / factories.*`
- 一键串联：`domain.autowire`（自动串起注解检查、指令写回、文档刷新与 check 验收）

#### 编译器补强计划（当前优先，非业务指令补丁）

> 本节聚焦“编译器能力补齐”，避免把问题转移成“手工补某个业务域指令”。  
> 说明：当前已新增 `domain.autowire`，本节以其余能力增强为主。

### Step 1：自动指令源（Registry -> 临时 instructions）
要做什么（代码）：
- 文件：`tools/bdd_compiler/lib/bdd_compiler/cli.ex`
- 为 `compile/lint/check` 增加参数：
  - `--project-root`（默认当前目录）
  - `--registry-module`（如 `Shop.BDD.InstructionRegistry`）
- 行为：
  - 若传 `--instructions`：沿用现有逻辑（兼容模式）
  - 若未传 `--instructions` 且传了 `--registry-module`：自动从项目 registry 读 `all(:v1/:v2)`，落临时 `.exs` 后继续编译流程

BDD 验收：
- Given：项目未提供 `--instructions`，但提供了可用 `--registry-module`
- When：执行 `bdd_compiler check --project-root ... --registry-module ...`
- Then：编译器可自动装载指令集，不要求手工导出 `.exs`

### Step 2：指令源优先级与错误语义
要做什么（代码）：
- 文件：`tools/bdd_compiler/lib/bdd_compiler/cli.ex`
- 明确优先级：`--instructions` > `--registry-module` > fail
- 统一错误输出：
  - 缺两者时报“必须提供指令来源”
  - registry 不可加载时报“模块不可用/调用失败”

BDD 验收：
- Given：既不传 `--instructions` 也不传 `--registry-module`
- When：执行 `bdd_compiler compile`
- Then：编译期失败并给出“缺指令来源”的明确报错

### Step 3：Runtime 覆盖校验（编译期 fail-fast）
要做什么（代码）：
- 文件：`tools/bdd_compiler/lib/bdd_compiler/cli.ex`
- 新增编译前检查：
  - 从 DSL AST 收集 `used_instructions`
  - 读取 `--runtime-module` 的可执行能力（约定 `capabilities/0`，返回指令集合）
  - 差集非空则直接失败
- 报错信息必须包含：`scenario_id`、`file:line`、`instruction`、`runtime_module`

BDD 验收：
- Given：DSL 使用了 `foo_bar` 指令，runtime 未实现 `foo_bar`
- When：执行 `bdd_compiler check ... --runtime-module ...`
- Then：在编译阶段失败（不是运行测试时失败），并定位到具体场景与行号

### Step 4：将 Runtime 覆盖校验接入 `check` 主流程
要做什么（代码）：
- 文件：`tools/bdd_compiler/lib/bdd_compiler/cli.ex`
- `check` 顺序固定为：
  1) 解析 DSL
  2) 装载指令集（显式或自动）
  3) Runtime 覆盖校验
  4) 生成代码
  5) lint

BDD 验收：
- Given：DSL、指令集、runtime 三者都完整
- When：执行 `bdd_compiler check`
- Then：一次命令完成“可编译 + 可校验 + 可生成 + 可 lint”

### Step 5：编译器自身测试（VDD/BDD）
要做什么（代码）：
- 文件：`tools/bdd_compiler/test/cli_help_test.exs`（扩展）与新增 `tools/bdd_compiler/test/cli_check_pipeline_test.exs`
- 覆盖场景：
  - 自动指令源成功路径
  - 缺指令源失败路径
  - runtime 覆盖缺失失败路径
  - `check` 主流程串联成功路径

BDD 验收：
- Given：上述四类最小测试夹具
- When：执行 `cd tools/bdd_compiler && mix test`
- Then：编译器行为稳定、回归可控

### Step 6：用现有 DSL 做真实编译验收（金标准入口）
要做什么（执行与验收，不是补业务指令）：
- 使用现有 DSL 目录：
  - `docs/bdd/*.dsl`
  - `docs/领域设计/财务/1.财务结算上下文设计.md` 内的 ```bdd 代码块（若启用扫描）
- 执行：
  1) `bdd_compiler check --project-root /home/wangbo/document/shop --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --in docs/bdd --out test/bdd_generated`
  2) `MIX_ENV=test mix test test/bdd_generated`

BDD 验收：
- Given：项目现有 DSL（不改业务文档）
- When：运行上述两步
- Then：
  - 若编译器能力不足（如指令源/覆盖校验问题），在 `check` 阶段失败并定位
  - 若编译器能力完整，则成功生成并执行 `test/bdd_generated`

### 最终完成定义（Definition of Done）
- `check` 不再依赖人工导出 `instructions_v1.exs`
- 缺 runtime 实现会在编译期被拦截
- 现有 DSL 可作为真实回归样本执行编译与测试
- 全过程不新增 CLI 子命令，只增强 `compile/lint/check`

#### 指令定义文件格式

项目不再需要依赖编译器的 Elixir 模块，改为纯数据文件：

```elixir
# priv/bdd/instructions.exs
%{
  create_inventory_product: %{
    name: :create_inventory_product,
    kind: :given,
    args: %{
      board_id: %{type: :uuid, required?: true, allowed: nil},
      sku: %{type: :string, required?: true, allowed: nil}
    },
    outputs: %{product_row_id: :uuid},
    boundary: :db,
    scopes: [:integration],
    async?: false,
    eventually?: false,
    assert_class: nil
  },
  # ...
}
```

多个文件可合并加载：

```bash
bdd compile docs/bdd/*.dsl \
    --instructions priv/bdd/instructions_common.exs \
    --instructions priv/bdd/instructions_inventory.exs
```

#### 职责边界

| 组件 | 归属 | 说明 |
|------|------|------|
| Parser / Validator / Emitter / Lint | CLI 工具 | 纯文本转换，不依赖任何业务代码 |
| 指令签名定义 | 使用方项目 | `.exs` 数据文件，描述指令参数和类型 |
| 运行时实现（SemanticGivens） | 使用方项目 | 每个项目自己实现 `run!/5` dispatch |
| DSL 场景文件 | 使用方项目 | 业务测试场景 |
| 生成的 ExUnit 代码 | 使用方项目 | CLI 输出，纳入 `test/` 目录 |

#### 构建方式选型

| 方式 | 产物 | 运行依赖 | 适用场景 |
|------|------|----------|---------|
| **escript**（推荐） | 单个可执行文件 | 需要 Erlang/OTP | 团队内部，机器上已有 Elixir 环境 |
| **Burrito** | 独立二进制 | 无依赖 | 分发给不装 Erlang 的用户 |
| **hex 包** | mix 依赖 | 需要 Elixir | 项目 `mix.exs` 加 `{:bdd_compiler, "~> 1.0"}` |

初期用 escript 最简单。如果以后要分发给非 Elixir 团队，再用 Burrito 打包。

#### 迁移策略

1. **Phase 1（当前）**：编译器留在 Shop，其他项目暂不接入
2. **Phase 2**：将 `lib/shop/bdd/` 下的 Parser / Validator / Emitter / Lint / Determinism 复制到独立项目，加 CLI 入口，`mix escript.build` 验证
3. **Phase 3**：Shop 项目改为依赖 CLI 工具（或 hex 包），删除内嵌的编译器代码
4. **Phase 4**：其他项目接入——只需添加 `priv/bdd/instructions.exs` + `test/support/bdd/` + DSL 文件

#### 统一执行入口（2026-02-11 新增）

为减少“命令太多/流程断链”，Shop 内统一使用一个门禁脚本作为本地与 CI 的执行入口：

```bash
./scripts/bdd_gate.sh
```

脚本内固定串联：
- `bdd_compiler check --project-root ... --registry-module Shop.BDD.InstructionRegistry --runtime-module Shop.BDD.Instructions.V1 --docs-root docs --in docs/bdd --out test/bdd_generated`
- `bdd_compiler factories.check --project-root ... --paths test/support/bdd/factories_generated`
- 如需严格模式（连 `semantic_givens` 一并检查）：`./scripts/bdd_gate.sh --strict-factories`

配套约定：
- 对外 API 变更后，先跑 `bdd_compiler domain.autowire ...` 自动完成注解检查/registry upsert/指令文档刷新，再进门禁。
- CI 侧只需要调用 `./scripts/bdd_gate.sh`（无需关心内部子命令细节）。
- `domain.autowire` 现已内置数据工厂门禁：
  - 默认检查 `test/support/bdd/factories_generated`
  - `--strict-factories true` 时额外检查 `test/support/bdd/semantic_givens`
  - 可用 `--skip-factories-check` 跳过（仅调试，不建议在正式验收使用）

#### Codeup CI 接入现状（2026-02-12）

当前状态（已完成）：
- Jenkins 任务：`shop-ci-codeup`
- 最新验收：`#15` 构建成功
- Jenkins 内执行命令：`./scripts/bdd_gate.sh`（完整门禁，含 runtime + factories）
- 私有依赖仓库连通性已验证（流水线内会执行）：
  - `git ls-remote git@codeup.aliyun.com:60fe1457f71d4ebf89456738/jh_components_lib.git`

当前状态（待权限后完成）：
- Codeup Webhook 尚未配置（需要仓库管理员权限）

Codeup 侧待配置项（拿到权限后直接照填）：
- Webhook URL：
  - `http://<jenkins_host>:8080/generic-webhook-trigger/invoke?token=shop-ci-token`
- 事件：
  - `Push`
- 分支过滤：
  - `refs/heads/dev`
- Jenkins 任务中已存在匹配规则：
  - `regexpFilterText=$ref`
  - `regexpFilterExpression=refs/heads/dev`

验收口径（Webhook 配置完成后）：
1. 向 `dev` 分支推送一个最小提交；
2. Jenkins `shop-ci-codeup` 自动触发（非手动 Build）；
3. 构建日志包含：
   - Codeup webhook 触发记录
   - `./scripts/bdd_gate.sh` 完整门禁通过。

#### Runtime 覆盖校验落地说明（2026-02-12）

为避免 CI 因环境差异触发全量 `mix` 编译链，runtime 覆盖校验改为可读取离线 capabilities 文件：
- 文件：`docs/bdd/_generated/runtime_caps_v1.exs`
- `bdd_gate` 在 `check` 模式下会自动传入：
  - `--runtime-caps-file docs/bdd/_generated/runtime_caps_v1.exs`

更新方式（推荐，使用编译器子命令）：
```bash
bdd_compiler runtime.caps.sync \
  --project-root /home/wangbo/document/shop \
  --runtime-module Shop.BDD.Instructions.V1 \
  --out docs/bdd/_generated/runtime_caps_v1.exs
```

兼容方式（手工脚本）：
```bash
elixir -e '
src = File.read!("test/support/bdd/instructions_v1.ex")
caps =
  Regex.scan(~r/\{\:(?:given|when|then),\s+\:([a-zA-Z0-9_]+)\}\s*->/, src, capture: :all_but_first)
  |> List.flatten()
  |> Enum.map(&String.to_atom/1)
  |> Enum.uniq()
  |> Enum.sort()
File.write!("docs/bdd/_generated/runtime_caps_v1.exs", inspect(caps, pretty: true, limit: :infinity))
'
```

#### 验收标准

- [x] `bdd_compiler compile` 能读取外部 `.exs` 指令定义，生成 ExUnit 代码
- [x] `bdd_compiler lint` 独立运行，不依赖 mix 项目上下文
- [x] Shop 项目迁移后 `bdd_compiler check` 结果不变（2026-02-11 验收：227 tests, 0 failures, 1 skipped）
- [x] 在一个全新 Elixir 项目中，只安装 CLI + 放文件，就能跑通一个 hello-world DSL 场景（2026-02-11 验收通过）

#### 跨项目 smoke 验收记录（2026-02-11）

目标：验证“脱离 Shop 主项目”后，CLI 仍可在最小 Elixir 项目内完成 DSL 编译与执行。

执行摘要：
- 新建最小项目：`mix new smoke_demo --sup`
- 准备文件：
  - `docs/bdd/hello.dsl`
  - `priv/bdd/instructions_v1.exs`
  - `test/support/bdd/instructions_v1.ex`
  - `test/test_helper.exs`（加载 `test/support/**/*.ex`）
- 运行命令：
  - `bdd_compiler compile --project-root <smoke_demo> --instructions priv/bdd/instructions_v1.exs --runtime-module SmokeDemo.BDD.Instructions.V1 --test-case ExUnit.Case --module-prefix SmokeDemo.BDD.Generated --in docs/bdd --out test/bdd_generated`
  - `bdd_compiler check --project-root <smoke_demo> --instructions priv/bdd/instructions_v1.exs --runtime-module SmokeDemo.BDD.Instructions.V1 --test-case ExUnit.Case --module-prefix SmokeDemo.BDD.Generated --in docs/bdd --out test/bdd_generated --no-fail-on-warn`
  - `MIX_ENV=test mix test test/bdd_generated`

结果：
- `1 test, 0 failures`
- 结论：CLI 在新项目中可独立完成“DSL -> 生成测试 -> 执行测试”的最小闭环。
