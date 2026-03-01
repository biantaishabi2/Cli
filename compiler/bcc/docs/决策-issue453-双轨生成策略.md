# 决策文档：Issue #453 双轨生成策略（UniBO + Seed 渐进）

## 背景

Issue #453 原始讨论聚焦 `seed -> 代码生成`，并提出 CRUD 与复杂业务分流。后续讨论明确了一个关键缺口：如果仅依赖 `phx.gen` 的默认 Web 入口，系统边界会和 Context 形态耦合，导致后续从 CRUD 演进到 DDD 时外部接口震荡。

同时，UniBO 在 PoC 中已经验证了“声明式模型 + GraphQL 契约层”的解耦路径，可作为已建模业务的主路径。

## 决策结论

1. 采用双轨路线，而不是单一路线。
2. 路线选择依据是“建模成熟度”，不是业务行业标签。
3. 对外稳定边界统一为 GraphQL 契约层（manifest/schema/resolver）。
4. `phx.gen` 仅作为内部脚手架，不作为系统边界生成器。

## 双轨定义

1. UniBO 轨道（model-first）
- 适用：实体关系、行为、流程可清晰建模的业务。
- 产物：模型、代码、GraphQL 契约、BDD source 联动生成。

2. 453 修正版轨道（seed 渐进）
- 适用：业务仍在探索、模型暂不稳定的场景。
- 要求：先保证 GraphQL 契约层稳定，再逐步升级内部实现。

## 路线选择标准（立项检查）

满足以下 4 条，优先使用 UniBO：

1. 可稳定定义实体关系和关键约束（主键、引用、状态边界）。
2. 可将核心行为抽象为 `action/validation/workflow/event`。
3. 需要长期稳定的对外 API 契约。
4. 团队接受 model-first 迭代方式（先改模型，再生成代码/测试）。

若不满足，先走 453 修正版，待模型稳定后再迁移到 UniBO。

## 453 修正版的生成边界

1. 允许：`phx.gen.context` / `phx.gen.schema`（内部脚手架）。
2. 不建议作为边界：`phx.gen.json` / `phx.gen.html` / `phx.gen.live`。
3. 默认产物应包含或对接 GraphQL 契约层，避免 Controller 与 Context 强耦合。

## CRUD -> DDD 渐进分级（L0-L3）

1. L0（CRUD 启动）
- 产物：schema/migration/repo CRUD/context adapter/GraphQL 契约。
- 约束：Context 保持薄封装。

2. L1（规则外提）
- 触发：出现显式业务约束（errors/validations）。
- 产物：policy/rules 模块。

3. L2（用例分层）
- 触发：command/query 分化、跨实体事务增多。
- 产物：application use_case（commands/queries）。

4. L3（DDD 聚合）
- 触发：flow/event/workflow 复杂度持续上升。
- 产物：aggregate/domain service/event handler。

## 门禁与验收

1. GraphQL 契约门禁：manifest/schema 变更需可审计。
2. 行为门禁：关键 query/mutation 契约测试通过。
3. 架构门禁：限制业务逻辑回流到 Context。
4. 流程门禁：L2/L3 场景纳入 BDD 回归。

## 执行拆分（Sub-Issues）

parent: #453

1. #506 定义 GraphQL 契约层产物与 CLI 开关
2. #507 实现 seed -> GraphQL 契约产物生成（depends-on: #506）
3. #508 增加契约门禁与 L0-L3 回归测试（depends-on: #507）

## 非目标

1. 不在本轮直接替换所有历史项目到 UniBO。
2. 不在本轮一次性完成全量 DDD 自动生成。
3. 不把“改 YAML”误解为零成本演进。
