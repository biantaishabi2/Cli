# 后端反推溯源文档

- [源码目录结构（后端范围）](./source-tree.md)
- [源码文件清单与分析链接](./source-files.md)
- [PRD 能力覆盖矩阵（文件级）](./coverage-matrix.md)
- [逐文件人工反推清单（已完成）](./manual-tracking.md)
- [逐文件人工反推清单（全量目标）](./manual-tracking-full.md)
- [部分覆盖补证](./coverage-supplement.md)
- [关键导出/函数语义对齐清单](./symbol-alignment-gap.md)
- [风险清单与修复建议](./risk-and-fixes.md)
- [本地审计报告（脚本输出）](./artifacts/backend-docs-audit.md)
- [复核结论（快照）](./check-result-backend-docs-review.md)

人工反推范围是 `src` 下 `*.ts` 的非测试文件（排除 `*.test.ts`/`*.spec.ts` 与 `__tests__`），
当前总文件数 `1685`，已完成 `832`，剩余 `853`。

逐文件人工分析位于 `./files/`，路径与源文件保持一一对应（在源文件后追加 `.md`）。

`manual-tracking-full.md` 用于“全量回看”，`manual-tracking.md` 仅保留已完成 832 文件。

证据与覆盖判定产物位于 `./artifacts/`：
- `./artifacts/prd-tag-evidence.tsv`
- `./artifacts/prd-coverage-judgement.tsv`
- `./artifacts/backend-docs-audit.md`
- `./artifacts/backend-docs-audit-missing.tsv`

## 本地审计工具（不接 CI）

新增脚本用于本地反推文档自检，不依赖 CI：

- `pnpm docs:trace:status`：快速看总量、缺口、行为意图/行为/约束缺口
- `pnpm docs:trace:report`：输出审计产物到 `docs/backend-trace/artifacts/`，其中 `backend-docs-audit.md` 含“文档齐套性”表（按源文件列缺失章节）
- `pnpm docs:trace:seed`：预览待生成的草稿文档清单
- `pnpm docs:trace:seed:write`：为缺口文件批量创建结构化草稿（含“行为意图/行为/约束”等章节）
- `pnpm docs:trace:ast`：AST 结构抽取审计（不落盘，展示文件数/异步/网络/进程调用等聚合）
- `pnpm docs:trace:ast:write`：写入 AST 结构产物到 `docs/backend-trace/artifacts/`：
  - `backend-docs-ast.json`
  - `backend-docs-ast.md`
- `docs/backend-trace/artifacts/backend-docs-audit-missing.tsv`：含 `missingSections` 字段，可直接和 `source-files/manual-tracking` 对齐筛选
