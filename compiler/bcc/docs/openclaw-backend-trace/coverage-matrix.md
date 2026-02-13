# PRD 能力覆盖矩阵（文件级）

## 说明
- 本矩阵基于 `docs/backend-trace/manual-tracking-full.md` 的“已完成 832 文件”与“缺口 853 文件”双层清单。
- 当前目录下不存在独立的“PRD 条目清单”文档，因此本矩阵以每个人工分析文档中的 `与 PRD 需求映射` 标签作为 PRD 条目代理。
- 结论分级规则：
  - `完整`：条目有 1 个以上证据文件，且不属于纯入口/纯类型/纯常量弱证据模式。
  - `部分`：条目仅有 1 个证据文件，且该文件为 `index/types/constants/schema/manifest/registry` 等契约或入口类文件（启发式）。
  - `缺失`：无证据文件。

## 总览结论
- 文件级反推覆盖：`832 / 1685`（49.2%）
- PRD 映射条目（去重后）：`1321`
- 证据总行数（允许一条目多证据）：`1367`
- 覆盖结论：`完整 1279`，`部分 42`，`缺失 0`

## 补证后判定（人工复核）
- 42 条“部分覆盖候选”已补充运行时证据（见 `coverage-supplement.md`）。
- 建议判定更新为：`完整 1321`，`部分 0`，`缺失 0`。

## 模块覆盖
| 模块 | 文件数 | 已反推 | 结论 |
|---|---:|---:|---|
| `src/acp` | 10 | 0 | 待补齐 |
| `src/agents` | 235 | 235 | 完整 |
| `src/auto-reply` | 121 | 0 | 待补齐 |
| `src/browser` | 52 | 0 | 待补齐 |
| `src/canvas-host` | 2 | 0 | 待补齐 |
| `src/channel-web.ts` | 1 | 0 | 待补齐 |
| `src/channels` | 77 | 77 | 完整 |
| `src/cli` | 139 | 0 | 待补齐 |
| `src/compat` | 1 | 0 | 待补齐 |
| `src/commands` | 174 | 1 | 待补齐 |
| `src/config` | 93 | 93 | 完整 |
| `src/cron` | 23 | 23 | 完整 |
| `src/daemon` | 19 | 0 | 待补齐 |
| `src/discord` | 45 | 45 | 完整 |
| `src/entry.ts` | 1 | 0 | 待补齐 |
| `src/extensionAPI.ts` | 1 | 0 | 待补齐 |
| `src/gateway` | 133 | 133 | 完整 |
| `src/globals.ts` | 1 | 0 | 待补齐 |
| `src/hooks` | 22 | 0 | 待补齐 |
| `src/imessage` | 12 | 0 | 待补齐 |
| `src/infra` | 123 | 123 | 完整 |
| `src/line` | 21 | 0 | 待补齐 |
| `src/link-understanding` | 6 | 0 | 待补齐 |
| `src/logger.ts` | 1 | 0 | 待补齐 |
| `src/logging` | 10 | 0 | 待补齐 |
| `src/logging.ts` | 1 | 0 | 待补齐 |
| `src/macos` | 3 | 0 | 待补齐 |
| `src/markdown` | 7 | 0 | 待补齐 |
| `src/media` | 12 | 0 | 待补齐 |
| `src/media-understanding` | 27 | 0 | 待补齐 |
| `src/memory` | 32 | 32 | 完整 |
| `src/node-host` | 3 | 0 | 待补齐 |
| `src/pairing` | 3 | 0 | 待补齐 |
| `src/plugin-sdk` | 1 | 0 | 待补齐 |
| `src/plugins` | 30 | 30 | 完整 |
| `src/polls.ts` | 1 | 0 | 待补齐 |
| `src/process` | 5 | 0 | 待补齐 |
| `src/providers` | 4 | 0 | 待补齐 |
| `src/routing` | 3 | 0 | 待补齐 |
| `src/runtime.ts` | 1 | 0 | 待补齐 |
| `src/security` | 10 | 0 | 待补齐 |
| `src/sessions` | 6 | 0 | 待补齐 |
| `src/shared` | 1 | 0 | 待补齐 |
| `src/signal` | 14 | 0 | 待补齐 |
| `src/slack` | 43 | 0 | 待补齐 |
| `src/telegram` | 40 | 40 | 完整 |
| `src/terminal` | 10 | 0 | 待补齐 |
| `src/test-helpers` | 2 | 0 | 待补齐 |
| `src/test-utils` | 2 | 0 | 待补齐 |
| `src/tts` | 1 | 0 | 待补齐 |
| `src/tui` | 24 | 0 | 待补齐 |
| `src/types` | 9 | 0 | 待补齐 |
| `src/utils` | 12 | 0 | 待补齐 |
| `src/utils.ts` | 1 | 0 | 待补齐 |
| `src/version.ts` | 1 | 0 | 待补齐 |
| `src/web` | 43 | 0 | 待补齐 |
| `src/whatsapp` | 1 | 0 | 待补齐 |
| `src/wizard` | 8 | 0 | 待补齐 |

## 部分覆盖候选（42）
> 说明：以下条目不是“未实现”，而是“当前证据偏契约层，建议补一条运行时实现证据”。

| PRD 条目 | 当前证据文件 |
|---|---|
| `auth-domain-model` | `docs/backend-trace/files/src/agents/auth-profiles/types.ts.md` |
| `auth-lock-policy` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` |
| `auth-store-versioning` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` |
| `channel-plugin-registry-entry` | `docs/backend-trace/files/src/channels/plugins/index.ts.md` |
| `channel-plugin-types-public-surface` | `docs/backend-trace/files/src/channels/plugins/types.ts.md` |
| `channel-registry-and-aliases` | `docs/backend-trace/files/src/channels/registry.ts.md` |
| `config-schema-and-ui-metadata-service` | `docs/backend-trace/files/src/config/schema.ts.md` |
| `config-types-public-surface` | `docs/backend-trace/files/src/config/types.ts.md` |
| `container-metadata-tracking` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` |
| `credential-type-system` | `docs/backend-trace/files/src/agents/auth-profiles/types.ts.md` |
| `cron-domain-model` | `docs/backend-trace/files/src/cron/types.ts.md` |
| `discord-module-public-entry` | `docs/backend-trace/files/src/discord/index.ts.md` |
| `embedded-attempt-type-contract` | `docs/backend-trace/files/src/agents/pi-embedded-runner/run/types.ts.md` |
| `embedded-helper-types` | `docs/backend-trace/files/src/agents/pi-embedded-helpers/types.ts.md` |
| `external-cli-sync-policy` | `docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` |
| `fault-tolerant-state-load` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` |
| `gateway-method-context-types` | `docs/backend-trace/files/src/gateway/server-methods/types.ts.md` |
| `gateway-protocol-validation-hub` | `docs/backend-trace/files/src/gateway/protocol/index.ts.md` |
| `memory-contract` | `docs/backend-trace/files/src/memory/types.ts.md` |
| `memory-public-api` | `docs/backend-trace/files/src/memory/index.ts.md` |
| `plugin-manifest-contract-and-loader` | `docs/backend-trace/files/src/plugins/manifest.ts.md` |
| `plugin-registry-core` | `docs/backend-trace/files/src/plugins/registry.ts.md` |
| `plugin-runtime-orchestrator` | `docs/backend-trace/files/src/plugins/runtime/index.ts.md` |
| `plugin-runtime-type-contract` | `docs/backend-trace/files/src/plugins/runtime/types.ts.md` |
| `plugin-type-system-contracts` | `docs/backend-trace/files/src/plugins/types.ts.md` |
| `protocol-schema-facade` | `docs/backend-trace/files/src/gateway/protocol/schema.ts.md` |
| `protocol-static-types-bridge` | `docs/backend-trace/files/src/gateway/protocol/schema/types.ts.md` |
| `result-contract` | `docs/backend-trace/files/src/agents/pi-embedded-runner/types.ts.md` |
| `run-meta-schema` | `docs/backend-trace/files/src/agents/pi-embedded-runner/types.ts.md` |
| `sandbox-defaults` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` |
| `sandbox-policy-model` | `docs/backend-trace/files/src/agents/sandbox/types.ts.md` |
| `sandbox-registry-persistence` | `docs/backend-trace/files/src/agents/sandbox/registry.ts.md` |
| `sandbox-state-layout` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` |
| `sandbox-tool-policy-baseline` | `docs/backend-trace/files/src/agents/sandbox/constants.ts.md` |
| `sandbox-type-contract` | `docs/backend-trace/files/src/agents/sandbox/types.ts.md` |
| `session-domain-contracts` | `docs/backend-trace/files/src/config/sessions/types.ts.md` |
| `skill-install-spec-model` | `docs/backend-trace/files/src/agents/skills/types.ts.md` |
| `skills-type-contract` | `docs/backend-trace/files/src/agents/skills/types.ts.md` |
| `status-schema` | `docs/backend-trace/files/src/memory/types.ts.md` |
| `telegram-bot-type-contracts` | `docs/backend-trace/files/src/telegram/bot/types.ts.md` |
| `telegram-module-public-entry` | `docs/backend-trace/files/src/telegram/index.ts.md` |
| `web-channel-public-entry` | `docs/backend-trace/files/src/channels/web/index.ts.md` |

## 全量回链
- 全量条目到证据文件映射：`docs/backend-trace/artifacts/prd-tag-evidence.tsv`
- 逐文件人工反推进度：`docs/backend-trace/manual-tracking.md`
- 部分覆盖补证：`docs/backend-trace/coverage-supplement.md`
- 覆盖判定明细（原始/补证后）：`docs/backend-trace/artifacts/prd-coverage-judgement.tsv`
