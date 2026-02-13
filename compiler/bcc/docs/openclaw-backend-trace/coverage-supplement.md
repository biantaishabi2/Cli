# OpenClaw 后端 PRD 部分覆盖补证

## 说明
- 本文针对 `coverage-matrix.md` 中的 42 条“部分覆盖候选”补充运行时证据。
- 判定口径：若条目除了契约/入口证据外，能补充到明确运行时实现文件，则可升级为“完整（人工复核）”。

## 补证明细
| PRD 条目 | 原证据（契约/入口） | 补充运行时证据 |
|---|---|---|
| `auth-domain-model` | `docs/backend-trace/files/src/agents/auth-profiles/types.ts.md` | `docs/backend-trace/files/src/agents/auth-profiles/store.ts.md` |
| `auth-lock-policy` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` | `docs/backend-trace/files/src/agents/auth-profiles/store.ts.md` |
| `auth-store-versioning` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` | `docs/backend-trace/files/src/agents/auth-profiles/store.ts.md` |
| `channel-plugin-registry-entry` | `docs/backend-trace/files/src/channels/plugins/index.ts.md` | `docs/backend-trace/files/src/channels/plugins/load.ts.md` |
| `channel-plugin-types-public-surface` | `docs/backend-trace/files/src/channels/plugins/types.ts.md` | `docs/backend-trace/files/src/channels/plugins/message-actions.ts.md` |
| `channel-registry-and-aliases` | `docs/backend-trace/files/src/channels/registry.ts.md` | `docs/backend-trace/files/src/channels/plugins/index.ts.md` |
| `config-schema-and-ui-metadata-service` | `docs/backend-trace/files/src/config/schema.ts.md` | `docs/backend-trace/files/src/config/validation.ts.md` |
| `config-types-public-surface` | `docs/backend-trace/files/src/config/types.ts.md` | `docs/backend-trace/files/src/config/io.ts.md` |
| `container-metadata-tracking` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` | `docs/backend-trace/files/src/agents/sandbox/manage.ts.md` |
| `credential-type-system` | `docs/backend-trace/files/src/agents/auth-profiles/types.ts.md` | `docs/backend-trace/files/src/agents/auth-profiles/profiles.ts.md` |
| `cron-domain-model` | `docs/backend-trace/files/src/cron/types.ts.md` | `docs/backend-trace/files/src/cron/service.ts.md` |
| `discord-module-public-entry` | `docs/backend-trace/files/src/discord/index.ts.md` | `docs/backend-trace/files/src/discord/monitor/provider.ts.md` |
| `embedded-attempt-type-contract` | `docs/backend-trace/files/src/agents/pi-embedded-runner/run/types.ts.md` | `docs/backend-trace/files/src/agents/pi-embedded-runner/run/attempt.ts.md` |
| `embedded-helper-types` | `docs/backend-trace/files/src/agents/pi-embedded-helpers/types.ts.md` | `docs/backend-trace/files/src/agents/pi-embedded-helpers.ts.md` |
| `external-cli-sync-policy` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` | `docs/backend-trace/files/src/agents/auth-profiles/external-cli-sync.ts.md` |
| `fault-tolerant-state-load` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` | `docs/backend-trace/files/src/agents/sandbox/manage.ts.md` |
| `gateway-method-context-types` | `docs/backend-trace/files/src/gateway/server-methods/types.ts.md` | `docs/backend-trace/files/src/gateway/server-methods.ts.md` |
| `gateway-protocol-validation-hub` | `docs/backend-trace/files/src/gateway/protocol/index.ts.md` | `docs/backend-trace/files/src/gateway/server/ws-connection/message-handler.ts.md` |
| `memory-contract` | `docs/backend-trace/files/src/memory/types.ts.md` | `docs/backend-trace/files/src/memory/manager.ts.md` |
| `memory-public-api` | `docs/backend-trace/files/src/memory/index.ts.md` | `docs/backend-trace/files/src/memory/search-manager.ts.md` |
| `plugin-manifest-contract-and-loader` | `docs/backend-trace/files/src/plugins/manifest.ts.md` | `docs/backend-trace/files/src/plugins/discovery.ts.md` |
| `plugin-registry-core` | `docs/backend-trace/files/src/plugins/registry.ts.md` | `docs/backend-trace/files/src/plugins/loader.ts.md` |
| `plugin-runtime-orchestrator` | `docs/backend-trace/files/src/plugins/runtime/index.ts.md` | `docs/backend-trace/files/src/plugins/runtime.ts.md` |
| `plugin-runtime-type-contract` | `docs/backend-trace/files/src/plugins/runtime/types.ts.md` | `docs/backend-trace/files/src/plugins/runtime/index.ts.md` |
| `plugin-type-system-contracts` | `docs/backend-trace/files/src/plugins/types.ts.md` | `docs/backend-trace/files/src/plugins/commands.ts.md` |
| `protocol-schema-facade` | `docs/backend-trace/files/src/gateway/protocol/schema.ts.md` | `docs/backend-trace/files/src/gateway/server/ws-connection/message-handler.ts.md` |
| `protocol-static-types-bridge` | `docs/backend-trace/files/src/gateway/protocol/schema/types.ts.md` | `docs/backend-trace/files/src/gateway/client.ts.md` |
| `result-contract` | `docs/backend-trace/files/src/agents/pi-embedded-runner/types.ts.md` | `docs/backend-trace/files/src/agents/pi-embedded-runner/run.ts.md` |
| `run-meta-schema` | `docs/backend-trace/files/src/agents/pi-embedded-runner/types.ts.md` | `docs/backend-trace/files/src/agents/pi-embedded-runner/run.ts.md` |
| `sandbox-defaults` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` | `docs/backend-trace/files/src/agents/sandbox/config.ts.md` |
| `sandbox-policy-model` | `docs/backend-trace/files/src/agents/sandbox/types.ts.md` | `docs/backend-trace/files/src/agents/sandbox/tool-policy.ts.md` |
| `sandbox-registry-persistence` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` | `docs/backend-trace/files/src/agents/sandbox/manage.ts.md` |
| `sandbox-state-layout` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` | `docs/backend-trace/files/src/agents/sandbox/manage.ts.md` |
| `sandbox-tool-policy-baseline` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` | `docs/backend-trace/files/src/agents/sandbox/tool-policy.ts.md` |
| `sandbox-type-contract` | `docs/backend-trace/files/src/agents/sandbox/types.ts.md` | `docs/backend-trace/files/src/agents/sandbox/manage.ts.md` |
| `session-domain-contracts` | `docs/backend-trace/files/src/config/sessions/types.ts.md` | `docs/backend-trace/files/src/config/sessions/store.ts.md` |
| `skill-install-spec-model` | `docs/backend-trace/files/src/agents/skills/types.ts.md` | `docs/backend-trace/files/src/agents/skills/plugin-skills.ts.md` |
| `skills-type-contract` | `docs/backend-trace/files/src/agents/skills/types.ts.md` | `docs/backend-trace/files/src/agents/skills/refresh.ts.md` |
| `status-schema` | `docs/backend-trace/files/src/memory/types.ts.md` | `docs/backend-trace/files/src/memory/status-format.ts.md` |
| `telegram-bot-type-contracts` | `docs/backend-trace/files/src/telegram/bot/types.ts.md` | `docs/backend-trace/files/src/telegram/bot.ts.md` |
| `telegram-module-public-entry` | `docs/backend-trace/files/src/telegram/index.ts.md` | `docs/backend-trace/files/src/telegram/monitor.ts.md` |
| `web-channel-public-entry` | `docs/backend-trace/files/src/channels/web/index.ts.md` | `docs/backend-trace/files/src/gateway/server-ws-runtime.ts.md` |

## 结论
- 42 条“部分覆盖候选”均已补到运行时证据。
- 建议将矩阵结论从：`完整 1279 / 部分 42 / 缺失 0`
- 调整为（人工复核后）：`完整 1321 / 部分 0 / 缺失 0`
