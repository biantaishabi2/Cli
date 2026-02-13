# 后端文档语义与符号对齐清单

更新日期：2026-02-13
范围：`docs/backend-trace` 当前人工分析范围（`src` 下非测试 TypeScript，共 `832` 份已写分析文档）。

## 结论先说
- 文件级覆盖：`832/1685`。
- 关键导出/函数语义对齐（严格口径）：共发现 `35` 个文件存在疑点。
- 这 35 条已全部标为“`doc-updated`”，当前口径为：文档已修订待复检，待脚本复跑后再做最后判定。
- 证据文件（带 `src + mismatch`）：`docs/backend-trace/artifacts/symbol-alignment-mismatch.tsv`

## 1) 已修订待复检（全部 35 项）
- `src/agents/pi-embedded-runner/run.ts`
- `src/agents/pi-tools.ts`
- `src/agents/tools/browser-tool.ts`
- `src/agents/tools/cron-tool.ts`
- `src/agents/tools/web-fetch.ts`
- `src/agents/tools/web-search.ts`
- `src/gateway/node-registry.ts`
- `src/gateway/openai-http.ts`
- `src/gateway/protocol/schema/agents-models-skills.ts`
- `src/gateway/protocol/schema/config.ts`
- `src/gateway/protocol/schema/cron.ts`
- `src/gateway/protocol/schema/devices.ts`
- `src/gateway/protocol/schema/exec-approvals.ts`
- `src/gateway/protocol/schema/nodes.ts`
- `src/gateway/protocol/schema/sessions.ts`
- `src/gateway/protocol/schema/wizard.ts`
- `src/gateway/server-methods/browser.ts`
- `src/gateway/server-methods/usage.ts`
- `src/gateway/server-reload-handlers.ts`
- `src/agents/pi-embedded-subscribe.types.ts`
- `src/gateway/protocol/schema.ts`
- `src/gateway/server.ts`
- `src/gateway/server/tls.ts`
- `src/gateway/test-helpers.ts`
- `src/memory/index.ts`
- `src/memory/openai-batch.ts`
- `src/config/sessions.ts`
- `src/config/types.ts`
- `src/config/zod-schema.providers.ts`
- `src/channels/plugins/allowlist-match.ts`
- `src/channels/plugins/channel-config.ts`
- `src/channels/plugins/types.ts`
- `src/plugins/runtime/index.ts`
- `src/cron/isolated-agent.ts`
- `src/cron/service.ts`

## 建议行动
1. 对上述 35 项做一次复检（脚本 + 人工抽样）。
2. 重点复核聚合导出与重导出的文件，确认文档里已记录“聚合目标”与“语义符号”映射。
3. 复跑后若剩余清单>0，再做下一轮修订。

## 参考
- `docs/backend-trace/artifacts/symbol-alignment-mismatch.tsv`
