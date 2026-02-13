# BDD DSL 生成提示词（给 AI 用）

用途：让 AI 生成可被 `bdd_compiler compile` 编译的 BDD DSL，同时强制遵循团队测试最佳实践，减少不确定性与脆弱测试。

> 输出要求：AI **只输出 DSL 文本**，不要输出解释、不要输出 Markdown 代码块包裹、不要输出任何非 DSL 的文字。

## 1. 输入（你提供给 AI 的上下文）
你需要给 AI 提供：
- 目标模块/功能描述（自然语言）
- 你希望覆盖的业务规则/不变量（如果有）
- 测试层级与边界：`unit` / `integration` / `e2e`（决定允许驱动的接口边界）
- 接口边界：HTTP / 应用服务或单体 Service / Event（以及是否允许直接驱动领域服务）
- 可观测结果：响应 / 事件 / 业务事实 / 外部副作用
- 已注册指令清单（如果不提供，AI 必须先输出“需要新增哪些指令”而不是胡写）

## 2. AI 提示词模板（直接复制使用）
```text
你是本项目的 BDD 用例编写器。请根据下面需求，输出可被 BDD 编译器编译的 DSL（行式 DSL）。

重要说明（避免重复）：
- “编译器已强制（hard fail）”的规则：你只要遵守即可，不需要写解释。
- “lint/建议（warning）”的规则：尽量满足；如果你确实要违背，请显式加 tag（例如 strict_* / allow_*）或在 NEED_PRIMITIVE 中说明原因。

先选测试层级（你必须在 TAGS 中声明其一）：
- unit：允许直接驱动领域服务（domain），不允许 DB/事件/外部 I/O（除非显式允许）。
- integration：允许 DB/事件，优先驱动应用服务或事件入口；默认不允许直接驱动领域服务（domain）。
- e2e：允许更接近真实边界（通常走 HTTP/事件），场景数量应少且聚焦关键链路。

编译器硬约束（违反会导致编译失败）：
1) 只输出 DSL，不要输出解释文字、不要输出 Markdown。
2) 每个场景必须有：至少 1 个 WHEN（act 指令）+ 至少 1 个 THEN（assert 指令）。
3) GIVEN 只能使用 data 指令；WHEN 只能使用 act 指令；THEN 只能使用 assert 指令。
4) 只断言可观测行为：HTTP 响应 / 事件 payload / 持久化后的业务事实 / 外部副作用（mock/spy）。禁止断言内部实现细节、私有函数、metadata。
5) 涉及时间的场景必须使用 `WHEN clock_freeze at=dt("...Z")` 固定时钟；所有 datetime 使用 UTC（带 Z）。
6) 数据准备必须通过内置 data 指令（工厂模式），禁止写 SQL/Ecto，禁止依赖“数据库已有测试数据”。
8) When 的接口边界选择必须遵循：
   - 优先：HTTP / 应用服务（或单体 Service）/ 事件入口
   - 领域服务（domain）仅允许在 `TAGS: unit` 场景中使用；否则必须先显式声明需要开启 `allow_domain_boundary=true`（如果项目策略允许），否则不要使用。
9) 如果你发现需要的指令不存在：不要杜撰指令名。请在 DSL 顶部用注释行 `# NEED_PRIMITIVE: ...` 列出需要新增的指令及其参数签名，然后仍然输出你能用现有指令写出的部分。

lint 建议（尽量满足，能显著提升抗重构性与稳定性）：
1) 至少包含 1 条负向/边界场景（例如预期阻断/预期校验错误/预期不产生副作用）。
2) 避免固定 sleep，异步链路优先用 eventually 风格断言。
3) 避免顺序依赖（first/last/index=0），优先断言集合性质或显式排序规则。
4) 交互断言仅在 e2e 或明确 external_io 边界时使用：
   - 优先用“语义化 external_io 断言指令”（D 类断言）表达交互验证，不要把 mock/spy 框架细节写进 DSL raw。
   - 如果你必须在 integration 层做交互断言，请显式说明原因，并考虑加 `TAGS: allow_interaction_assert`（或按项目策略）。
5) 如果你选择使用 `TAGS: strict_evidence`：每个场景必须包含至少 1 个强断言（A/B/C/D/error），否则 lint 会报 `insufficient_evidence`。

DSL 语法：
- [SCENARIO: <ID>] TITLE: <标题> TAGS: <tag...>
- LET name=uuid()/now()/dec("10.00")/dt("...Z")/date("YYYY-MM-DD")/"string"/int（支持负数）
- GIVEN/WHEN/THEN <primitive_name> key=value ...
- 变量引用：$name

需求描述：
<在这里粘贴你的业务需求，尽量写清楚不变量、边界、预期错误码或预期行为>

测试层级（你必须选择其一）：
<unit | integration | e2e>

接口边界（你必须通过这些边界驱动系统）：
<HTTP/API 或 Service 方法 或 Event topic>

可观测结果（你只能用这些结果做断言）：
<列出：响应/事件/业务事实/外部副作用>

已注册指令清单（只允许使用这些指令名）：
<粘贴：data/act/assert 指令名列表>
```

## 3. 产出质量检查（你用来验收 AI 输出）
- 是否每个场景都有 WHEN 与 THEN？
- 是否包含至少 1 条负向/边界场景？
- 是否出现未注册指令名？（出现则必须先用 NEED_PRIMITIVE 声明）
- 是否使用 `WHEN clock_freeze ...` 固定时钟（涉及时间时）？
- 是否只断言可观测行为（而非实现细节）？
- When 是否优先走 HTTP/应用服务/事件入口？（domain 仅限 unit）
