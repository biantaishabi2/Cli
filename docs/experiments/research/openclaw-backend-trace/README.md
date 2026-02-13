# OpenClaw 后端文件反推文档（复用入口）

## 目的
用于统一 `openclaw` 后端源码文件级文档写法与审计口径，支持后续复制到其他仓库使用。

## 文档结构
- 模板：`backend-file-analysis-template.md`
- 单文件分析文档路径：`docs/backend-trace/files/<源文件路径>.md`

## 文件命名规则
- 源文件 `src/agents/auth-profiles/store.ts` 对应文档：`docs/backend-trace/files/src/agents/auth-profiles/store.ts.md`
- 文件名与源文件一一对应（追加 `.md`）。

## 复用说明
- 先创建 `docs/backend-trace/files/` 目录树，再按源文件路径逐文件补齐。
- 每份文档至少包含：
  - 职责
  - 行为意图
  - 行为
  - 约束
  - 输入输出
  - 调用链位置
  - 状态与副作用
  - 异常处理
  - 与 PRD 需求映射
  - 溯源证据

## 说明
- 若接入审计脚本（`audit-backend-trace.ts / audit-backend-ast.ts`），可按 `docs/backend-trace/files/*` 的齐套性、章节完整性来校验。
