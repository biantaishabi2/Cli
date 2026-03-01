# 决策：Issue #453 BCC 对接 UniBO GraphQL 复用边界（双轨生成策略）

## 背景

Issue #453 已确认 UniBO 侧 GraphQL 解耦层（manifest + runtime）已可用。  
BCC 不应再次实现同构 GraphQL runtime/controller/resolver 能力。

本决策用于固化 BCC 与 UniBO 的职责边界、可复用产物与输入输出契约。

## 职责边界（MUST / MUST NOT）

### BCC（MUST）

- MUST 从 seed 中提取接口契约，输出 UniBO 可消费的 API Contract 文件（`api-contract.json`）。
- MUST 保持既有 `bcc arch generate --emit code` 行为兼容。
- MUST 在 `--emit api-contract` 时执行最小契约校验（必须存在 `boundaries[].contracts`）。

### UniBO（MUST）

- MUST 负责 GraphQL manifest 解析与装配。
- MUST 负责 runtime/controller/resolver/schema 的生成与执行。
- MUST 在同 major 版本下向后兼容地消费 BCC 契约，并忽略未知字段。

### 禁止重复实现（MUST NOT）

- BCC MUST NOT 新增或生成 GraphQL runtime 模板。
- BCC MUST NOT 新增或生成 controller/resolver 模板。
- BCC MUST NOT 在 BCC 内复制 UniBO 的 manifest/runtime 解释执行逻辑。

## 复用产物边界

- 可复用：UniBO 既有 `manifest + runtime`。
- BCC 新增输出：`--emit api-contract` 仅生成 `api-contract.json`。
- 不允许输出：任何 GraphQL 运行时代码或控制器模板。

## CLI 语义冻结

- `--emit code`：保留原有代码生成路径（`generate-commands.sh` + `*.ex`）。
- `--emit api-contract`：仅输出对接契约文件。
- `--emit all`：当前版本冻结为与 `--emit code` 等价，保证历史脚本兼容。

## BCC -> UniBO 契约格式（v1）

输出 JSON 根字段：

- `contract_schema_version`（示例：`1.0.0`）
- `producer`（`name` + `version`）
- `seed_version`
- `generated_at`（RFC3339）
- `contracts[]`

`contracts[]` 最小必填字段：

- `module_id`
- `name`
- `kind`
- `input`
- `output`

可选字段：

- `errors`
- `fields`
- `metadata`

## 版本演进规则

- major：不兼容变更（字段删除/重命名、语义破坏）。
- minor：向后兼容扩展（仅新增可选字段）。
- patch：非语义修订（文档、描述、格式性修复）。

UniBO 消费策略：

- 同 major 版本可消费。
- 对未知字段执行忽略策略，避免因扩展字段导致失败。

## 错误与回滚策略

- `--emit api-contract` 若 seed 缺少 contracts，返回可读错误并非 0 退出。
- 回滚策略：保持 `--emit code` / `--emit all` 不变，并继续禁止在 BCC 生成 runtime/controller。
