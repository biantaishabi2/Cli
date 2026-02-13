# OpenClaw 未覆盖后端文件清单（基于当前 backend-trace 范围）

- 总 non-test .ts: 1685
- 已覆盖: 832
- 缺口: 853

## acp (10)
- `src/acp/client.ts`
- `src/acp/commands.ts`
- `src/acp/event-mapper.ts`
- `src/acp/index.ts`
- `src/acp/meta.ts`
- `src/acp/server.ts`
- `src/acp/session-mapper.ts`
- `src/acp/session.ts`
- `src/acp/translator.ts`
- `src/acp/types.ts`

## auto-reply (121)
- `src/auto-reply/chunk.ts`
- `src/auto-reply/command-auth.ts`
- `src/auto-reply/command-detection.ts`
- `src/auto-reply/commands-args.ts`
- `src/auto-reply/commands-registry.data.ts`
- `src/auto-reply/commands-registry.ts`
- `src/auto-reply/commands-registry.types.ts`
- `src/auto-reply/dispatch.ts`
- `src/auto-reply/envelope.ts`
- `src/auto-reply/group-activation.ts`
- `src/auto-reply/heartbeat.ts`
- `src/auto-reply/inbound-debounce.ts`
- `src/auto-reply/media-note.ts`
- `src/auto-reply/model.ts`
- `src/auto-reply/reply.ts`
- `src/auto-reply/reply/abort.ts`
- `src/auto-reply/reply/agent-runner-execution.ts`
- `src/auto-reply/reply/agent-runner-helpers.ts`
- `src/auto-reply/reply/agent-runner-memory.ts`
- `src/auto-reply/reply/agent-runner-payloads.ts`
- `src/auto-reply/reply/agent-runner-utils.ts`
- `src/auto-reply/reply/agent-runner.ts`
- `src/auto-reply/reply/audio-tags.ts`
- `src/auto-reply/reply/bash-command.ts`
- `src/auto-reply/reply/block-reply-coalescer.ts`
- `src/auto-reply/reply/block-reply-pipeline.ts`
- `src/auto-reply/reply/block-streaming.ts`
- `src/auto-reply/reply/body.ts`
- `src/auto-reply/reply/commands-allowlist.ts`
- `src/auto-reply/reply/commands-approve.ts`
- `src/auto-reply/reply/commands-bash.ts`
- `src/auto-reply/reply/commands-compact.ts`
- `src/auto-reply/reply/commands-config.ts`
- `src/auto-reply/reply/commands-context-report.ts`
- `src/auto-reply/reply/commands-context.ts`
- `src/auto-reply/reply/commands-core.ts`
- `src/auto-reply/reply/commands-info.ts`
- `src/auto-reply/reply/commands-models.ts`
- `src/auto-reply/reply/commands-plugin.ts`
- `src/auto-reply/reply/commands-ptt.ts`
- `src/auto-reply/reply/commands-session.ts`
- `src/auto-reply/reply/commands-status.ts`
- `src/auto-reply/reply/commands-subagents.ts`
- `src/auto-reply/reply/commands-tts.ts`
- `src/auto-reply/reply/commands-types.ts`
- `src/auto-reply/reply/commands.ts`
- `src/auto-reply/reply/config-commands.ts`
- `src/auto-reply/reply/config-value.ts`
- `src/auto-reply/reply/debug-commands.ts`
- `src/auto-reply/reply/directive-handling.auth.ts`
- `src/auto-reply/reply/directive-handling.fast-lane.ts`
- `src/auto-reply/reply/directive-handling.impl.ts`
- `src/auto-reply/reply/directive-handling.model-picker.ts`
- `src/auto-reply/reply/directive-handling.model.ts`
- `src/auto-reply/reply/directive-handling.parse.ts`
- `src/auto-reply/reply/directive-handling.persist.ts`
- `src/auto-reply/reply/directive-handling.queue-validation.ts`
- `src/auto-reply/reply/directive-handling.shared.ts`
- `src/auto-reply/reply/directive-handling.ts`
- `src/auto-reply/reply/directives.ts`
- `src/auto-reply/reply/dispatch-from-config.ts`
- `src/auto-reply/reply/exec.ts`
- `src/auto-reply/reply/exec/directive.ts`
- `src/auto-reply/reply/followup-runner.ts`
- `src/auto-reply/reply/get-reply-directives-apply.ts`
- `src/auto-reply/reply/get-reply-directives-utils.ts`
- `src/auto-reply/reply/get-reply-directives.ts`
- `src/auto-reply/reply/get-reply-inline-actions.ts`
- `src/auto-reply/reply/get-reply-run.ts`
- `src/auto-reply/reply/get-reply.ts`
- `src/auto-reply/reply/groups.ts`
- `src/auto-reply/reply/history.ts`
- `src/auto-reply/reply/inbound-context.ts`
- `src/auto-reply/reply/inbound-dedupe.ts`
- `src/auto-reply/reply/inbound-meta.ts`
- `src/auto-reply/reply/inbound-text.ts`
- `src/auto-reply/reply/line-directives.ts`
- `src/auto-reply/reply/memory-flush.ts`
- `src/auto-reply/reply/mentions.ts`
- `src/auto-reply/reply/model-selection.ts`
- `src/auto-reply/reply/normalize-reply.ts`
- `src/auto-reply/reply/provider-dispatcher.ts`
- `src/auto-reply/reply/queue.ts`
- `src/auto-reply/reply/queue/cleanup.ts`
- `src/auto-reply/reply/queue/directive.ts`
- `src/auto-reply/reply/queue/drain.ts`
- `src/auto-reply/reply/queue/enqueue.ts`
- `src/auto-reply/reply/queue/normalize.ts`
- `src/auto-reply/reply/queue/settings.ts`
- `src/auto-reply/reply/queue/state.ts`
- `src/auto-reply/reply/queue/types.ts`
- `src/auto-reply/reply/reply-directives.ts`
- `src/auto-reply/reply/reply-dispatcher.ts`
- `src/auto-reply/reply/reply-elevated.ts`
- `src/auto-reply/reply/reply-inline.ts`
- `src/auto-reply/reply/reply-payloads.ts`
- `src/auto-reply/reply/reply-reference.ts`
- `src/auto-reply/reply/reply-tags.ts`
- `src/auto-reply/reply/reply-threading.ts`
- `src/auto-reply/reply/response-prefix-template.ts`
- `src/auto-reply/reply/route-reply.ts`
- `src/auto-reply/reply/session-reset-model.ts`
- `src/auto-reply/reply/session-updates.ts`
- `src/auto-reply/reply/session-usage.ts`
- `src/auto-reply/reply/session.ts`
- `src/auto-reply/reply/stage-sandbox-media.ts`
- `src/auto-reply/reply/streaming-directives.ts`
- `src/auto-reply/reply/subagents-utils.ts`
- `src/auto-reply/reply/test-ctx.ts`
- `src/auto-reply/reply/test-helpers.ts`
- `src/auto-reply/reply/typing-mode.ts`
- `src/auto-reply/reply/typing.ts`
- `src/auto-reply/reply/untrusted-context.ts`
- `src/auto-reply/send-policy.ts`
- `src/auto-reply/skill-commands.ts`
- `src/auto-reply/status.ts`
- `src/auto-reply/templating.ts`
- `src/auto-reply/thinking.ts`
- `src/auto-reply/tokens.ts`
- `src/auto-reply/tool-meta.ts`
- `src/auto-reply/types.ts`

## browser (52)
- `src/browser/bridge-server.ts`
- `src/browser/cdp.helpers.ts`
- `src/browser/cdp.ts`
- `src/browser/chrome.executables.ts`
- `src/browser/chrome.profile-decoration.ts`
- `src/browser/chrome.ts`
- `src/browser/client-actions-core.ts`
- `src/browser/client-actions-observe.ts`
- `src/browser/client-actions-state.ts`
- `src/browser/client-actions-types.ts`
- `src/browser/client-actions.ts`
- `src/browser/client-fetch.ts`
- `src/browser/client.ts`
- `src/browser/config.ts`
- `src/browser/constants.ts`
- `src/browser/control-service.ts`
- `src/browser/extension-relay.ts`
- `src/browser/profiles-service.ts`
- `src/browser/profiles.ts`
- `src/browser/pw-ai-module.ts`
- `src/browser/pw-ai.ts`
- `src/browser/pw-role-snapshot.ts`
- `src/browser/pw-session.ts`
- `src/browser/pw-tools-core.activity.ts`
- `src/browser/pw-tools-core.downloads.ts`
- `src/browser/pw-tools-core.interactions.ts`
- `src/browser/pw-tools-core.responses.ts`
- `src/browser/pw-tools-core.shared.ts`
- `src/browser/pw-tools-core.snapshot.ts`
- `src/browser/pw-tools-core.state.ts`
- `src/browser/pw-tools-core.storage.ts`
- `src/browser/pw-tools-core.trace.ts`
- `src/browser/pw-tools-core.ts`
- `src/browser/routes/agent.act.shared.ts`
- `src/browser/routes/agent.act.ts`
- `src/browser/routes/agent.debug.ts`
- `src/browser/routes/agent.shared.ts`
- `src/browser/routes/agent.snapshot.ts`
- `src/browser/routes/agent.storage.ts`
- `src/browser/routes/agent.ts`
- `src/browser/routes/basic.ts`
- `src/browser/routes/dispatcher.ts`
- `src/browser/routes/index.ts`
- `src/browser/routes/tabs.ts`
- `src/browser/routes/types.ts`
- `src/browser/routes/utils.ts`
- `src/browser/screenshot.ts`
- `src/browser/server-context.ts`
- `src/browser/server-context.types.ts`
- `src/browser/server.ts`
- `src/browser/target-id.ts`
- `src/browser/trash.ts`

## canvas-host (2)
- `src/canvas-host/a2ui.ts`
- `src/canvas-host/server.ts`

## channel-web.ts (1)
- `src/channel-web.ts`

## cli (139)
- `src/cli/acp-cli.ts`
- `src/cli/argv.ts`
- `src/cli/banner.ts`
- `src/cli/browser-cli-actions-input.ts`
- `src/cli/browser-cli-actions-input/register.element.ts`
- `src/cli/browser-cli-actions-input/register.files-downloads.ts`
- `src/cli/browser-cli-actions-input/register.form-wait-eval.ts`
- `src/cli/browser-cli-actions-input/register.navigation.ts`
- `src/cli/browser-cli-actions-input/register.ts`
- `src/cli/browser-cli-actions-input/shared.ts`
- `src/cli/browser-cli-actions-observe.ts`
- `src/cli/browser-cli-debug.ts`
- `src/cli/browser-cli-examples.ts`
- `src/cli/browser-cli-extension.ts`
- `src/cli/browser-cli-inspect.ts`
- `src/cli/browser-cli-manage.ts`
- `src/cli/browser-cli-shared.ts`
- `src/cli/browser-cli-state.cookies-storage.ts`
- `src/cli/browser-cli-state.ts`
- `src/cli/browser-cli.ts`
- `src/cli/channel-auth.ts`
- `src/cli/channel-options.ts`
- `src/cli/channels-cli.ts`
- `src/cli/cli-name.ts`
- `src/cli/cli-utils.ts`
- `src/cli/command-format.ts`
- `src/cli/command-options.ts`
- `src/cli/completion-cli.ts`
- `src/cli/config-cli.ts`
- `src/cli/cron-cli.ts`
- `src/cli/cron-cli/register.cron-add.ts`
- `src/cli/cron-cli/register.cron-edit.ts`
- `src/cli/cron-cli/register.cron-simple.ts`
- `src/cli/cron-cli/register.ts`
- `src/cli/cron-cli/shared.ts`
- `src/cli/daemon-cli.ts`
- `src/cli/daemon-cli/install.ts`
- `src/cli/daemon-cli/lifecycle.ts`
- `src/cli/daemon-cli/probe.ts`
- `src/cli/daemon-cli/register.ts`
- `src/cli/daemon-cli/response.ts`
- `src/cli/daemon-cli/runners.ts`
- `src/cli/daemon-cli/shared.ts`
- `src/cli/daemon-cli/status.gather.ts`
- `src/cli/daemon-cli/status.print.ts`
- `src/cli/daemon-cli/status.ts`
- `src/cli/daemon-cli/types.ts`
- `src/cli/deps.ts`
- `src/cli/devices-cli.ts`
- `src/cli/directory-cli.ts`
- `src/cli/dns-cli.ts`
- `src/cli/docs-cli.ts`
- `src/cli/exec-approvals-cli.ts`
- `src/cli/gateway-cli.ts`
- `src/cli/gateway-cli/call.ts`
- `src/cli/gateway-cli/dev.ts`
- `src/cli/gateway-cli/discover.ts`
- `src/cli/gateway-cli/register.ts`
- `src/cli/gateway-cli/run-loop.ts`
- `src/cli/gateway-cli/run.ts`
- `src/cli/gateway-cli/shared.ts`
- `src/cli/gateway-rpc.ts`
- `src/cli/help-format.ts`
- `src/cli/hooks-cli.ts`
- `src/cli/logs-cli.ts`
- `src/cli/memory-cli.ts`
- `src/cli/models-cli.ts`
- `src/cli/node-cli.ts`
- `src/cli/node-cli/daemon.ts`
- `src/cli/node-cli/register.ts`
- `src/cli/nodes-camera.ts`
- `src/cli/nodes-canvas.ts`
- `src/cli/nodes-cli.ts`
- `src/cli/nodes-cli/a2ui-jsonl.ts`
- `src/cli/nodes-cli/cli-utils.ts`
- `src/cli/nodes-cli/format.ts`
- `src/cli/nodes-cli/register.camera.ts`
- `src/cli/nodes-cli/register.canvas.ts`
- `src/cli/nodes-cli/register.invoke.ts`
- `src/cli/nodes-cli/register.location.ts`
- `src/cli/nodes-cli/register.notify.ts`
- `src/cli/nodes-cli/register.pairing.ts`
- `src/cli/nodes-cli/register.screen.ts`
- `src/cli/nodes-cli/register.status.ts`
- `src/cli/nodes-cli/register.ts`
- `src/cli/nodes-cli/rpc.ts`
- `src/cli/nodes-cli/types.ts`
- `src/cli/nodes-run.ts`
- `src/cli/nodes-screen.ts`
- `src/cli/outbound-send-deps.ts`
- `src/cli/pairing-cli.ts`
- `src/cli/parse-bytes.ts`
- `src/cli/parse-duration.ts`
- `src/cli/parse-timeout.ts`
- `src/cli/plugin-registry.ts`
- `src/cli/plugins-cli.ts`
- `src/cli/ports.ts`
- `src/cli/profile-utils.ts`
- `src/cli/profile.ts`
- `src/cli/program.ts`
- `src/cli/program/build-program.ts`
- `src/cli/program/command-registry.ts`
- `src/cli/program/config-guard.ts`
- `src/cli/program/context.ts`
- `src/cli/program/help.ts`
- `src/cli/program/helpers.ts`
- `src/cli/program/message/helpers.ts`
- `src/cli/program/message/register.broadcast.ts`
- `src/cli/program/message/register.discord-admin.ts`
- `src/cli/program/message/register.emoji-sticker.ts`
- `src/cli/program/message/register.permissions-search.ts`
- `src/cli/program/message/register.pins.ts`
- `src/cli/program/message/register.poll.ts`
- `src/cli/program/message/register.reactions.ts`
- `src/cli/program/message/register.read-edit-delete.ts`
- `src/cli/program/message/register.send.ts`
- `src/cli/program/message/register.thread.ts`
- `src/cli/program/preaction.ts`
- `src/cli/program/register.agent.ts`
- `src/cli/program/register.configure.ts`
- `src/cli/program/register.maintenance.ts`
- `src/cli/program/register.message.ts`
- `src/cli/program/register.onboard.ts`
- `src/cli/program/register.setup.ts`
- `src/cli/program/register.status-health-sessions.ts`
- `src/cli/program/register.subclis.ts`
- `src/cli/progress.ts`
- `src/cli/prompt.ts`
- `src/cli/route.ts`
- `src/cli/run-main.ts`
- `src/cli/sandbox-cli.ts`
- `src/cli/security-cli.ts`
- `src/cli/skills-cli.ts`
- `src/cli/system-cli.ts`
- `src/cli/tagline.ts`
- `src/cli/tui-cli.ts`
- `src/cli/update-cli.ts`
- `src/cli/wait.ts`
- `src/cli/webhooks-cli.ts`

## compat (1)
- `src/compat/legacy-names.ts`

## daemon (19)
- `src/daemon/constants.ts`
- `src/daemon/diagnostics.ts`
- `src/daemon/inspect.ts`
- `src/daemon/launchd-plist.ts`
- `src/daemon/launchd.ts`
- `src/daemon/node-service.ts`
- `src/daemon/paths.ts`
- `src/daemon/program-args.ts`
- `src/daemon/runtime-parse.ts`
- `src/daemon/runtime-paths.ts`
- `src/daemon/schtasks.ts`
- `src/daemon/service-audit.ts`
- `src/daemon/service-env.ts`
- `src/daemon/service-runtime.ts`
- `src/daemon/service.ts`
- `src/daemon/systemd-hints.ts`
- `src/daemon/systemd-linger.ts`
- `src/daemon/systemd-unit.ts`
- `src/daemon/systemd.ts`

## entry.ts (1)
- `src/entry.ts`

## extensionAPI.ts (1)
- `src/extensionAPI.ts`

## globals.ts (1)
- `src/globals.ts`

## hooks (22)
- `src/hooks/bundled-dir.ts`
- `src/hooks/bundled/boot-md/handler.ts`
- `src/hooks/bundled/command-logger/handler.ts`
- `src/hooks/bundled/session-memory/handler.ts`
- `src/hooks/bundled/soul-evil/handler.ts`
- `src/hooks/config.ts`
- `src/hooks/frontmatter.ts`
- `src/hooks/gmail-ops.ts`
- `src/hooks/gmail-setup-utils.ts`
- `src/hooks/gmail-watcher.ts`
- `src/hooks/gmail.ts`
- `src/hooks/hooks-status.ts`
- `src/hooks/hooks.ts`
- `src/hooks/install.ts`
- `src/hooks/installs.ts`
- `src/hooks/internal-hooks.ts`
- `src/hooks/llm-slug-generator.ts`
- `src/hooks/loader.ts`
- `src/hooks/plugin-hooks.ts`
- `src/hooks/soul-evil.ts`
- `src/hooks/types.ts`
- `src/hooks/workspace.ts`

## imessage (12)
- `src/imessage/accounts.ts`
- `src/imessage/client.ts`
- `src/imessage/constants.ts`
- `src/imessage/index.ts`
- `src/imessage/monitor.ts`
- `src/imessage/monitor/deliver.ts`
- `src/imessage/monitor/monitor-provider.ts`
- `src/imessage/monitor/runtime.ts`
- `src/imessage/monitor/types.ts`
- `src/imessage/probe.ts`
- `src/imessage/send.ts`
- `src/imessage/targets.ts`

## index.ts (1)
- `src/index.ts`

## line (21)
- `src/line/accounts.ts`
- `src/line/auto-reply-delivery.ts`
- `src/line/bot-access.ts`
- `src/line/bot-handlers.ts`
- `src/line/bot-message-context.ts`
- `src/line/bot.ts`
- `src/line/config-schema.ts`
- `src/line/download.ts`
- `src/line/flex-templates.ts`
- `src/line/http-registry.ts`
- `src/line/index.ts`
- `src/line/markdown-to-line.ts`
- `src/line/monitor.ts`
- `src/line/probe.ts`
- `src/line/reply-chunks.ts`
- `src/line/rich-menu.ts`
- `src/line/send.ts`
- `src/line/signature.ts`
- `src/line/template-messages.ts`
- `src/line/types.ts`
- `src/line/webhook.ts`

## link-understanding (6)
- `src/link-understanding/apply.ts`
- `src/link-understanding/defaults.ts`
- `src/link-understanding/detect.ts`
- `src/link-understanding/format.ts`
- `src/link-understanding/index.ts`
- `src/link-understanding/runner.ts`

## logger.ts (1)
- `src/logger.ts`

## logging (10)
- `src/logging/config.ts`
- `src/logging/console.ts`
- `src/logging/diagnostic.ts`
- `src/logging/levels.ts`
- `src/logging/logger.ts`
- `src/logging/parse-log-line.ts`
- `src/logging/redact-identifier.ts`
- `src/logging/redact.ts`
- `src/logging/state.ts`
- `src/logging/subsystem.ts`

## logging.ts (1)
- `src/logging.ts`

## macos (3)
- `src/macos/gateway-daemon.ts`
- `src/macos/relay-smoke.ts`
- `src/macos/relay.ts`

## markdown (7)
- `src/markdown/code-spans.ts`
- `src/markdown/fences.ts`
- `src/markdown/frontmatter.ts`
- `src/markdown/ir.ts`
- `src/markdown/render.ts`
- `src/markdown/tables.ts`
- `src/markdown/whatsapp.ts`

## media (12)
- `src/media/audio-tags.ts`
- `src/media/audio.ts`
- `src/media/constants.ts`
- `src/media/fetch.ts`
- `src/media/host.ts`
- `src/media/image-ops.ts`
- `src/media/input-files.ts`
- `src/media/mime.ts`
- `src/media/parse.ts`
- `src/media/png-encode.ts`
- `src/media/server.ts`
- `src/media/store.ts`

## media-understanding (27)
- `src/media-understanding/apply.ts`
- `src/media-understanding/attachments.ts`
- `src/media-understanding/audio-preflight.ts`
- `src/media-understanding/concurrency.ts`
- `src/media-understanding/defaults.ts`
- `src/media-understanding/errors.ts`
- `src/media-understanding/format.ts`
- `src/media-understanding/index.ts`
- `src/media-understanding/providers/anthropic/index.ts`
- `src/media-understanding/providers/deepgram/audio.ts`
- `src/media-understanding/providers/deepgram/index.ts`
- `src/media-understanding/providers/google/audio.ts`
- `src/media-understanding/providers/google/index.ts`
- `src/media-understanding/providers/google/video.ts`
- `src/media-understanding/providers/groq/index.ts`
- `src/media-understanding/providers/image.ts`
- `src/media-understanding/providers/index.ts`
- `src/media-understanding/providers/minimax/index.ts`
- `src/media-understanding/providers/openai/audio.ts`
- `src/media-understanding/providers/openai/index.ts`
- `src/media-understanding/providers/shared.ts`
- `src/media-understanding/providers/zai/index.ts`
- `src/media-understanding/resolve.ts`
- `src/media-understanding/runner.ts`
- `src/media-understanding/scope.ts`
- `src/media-understanding/types.ts`
- `src/media-understanding/video.ts`

## node-host (3)
- `src/node-host/config.ts`
- `src/node-host/runner.ts`
- `src/node-host/with-timeout.ts`

## pairing (3)
- `src/pairing/pairing-labels.ts`
- `src/pairing/pairing-messages.ts`
- `src/pairing/pairing-store.ts`

## plugin-sdk (1)
- `src/plugin-sdk/index.ts`

## polls.ts (1)
- `src/polls.ts`

## process (5)
- `src/process/child-process-bridge.ts`
- `src/process/command-queue.ts`
- `src/process/exec.ts`
- `src/process/lanes.ts`
- `src/process/spawn-utils.ts`

## providers (4)
- `src/providers/github-copilot-auth.ts`
- `src/providers/github-copilot-models.ts`
- `src/providers/github-copilot-token.ts`
- `src/providers/qwen-portal-oauth.ts`

## routing (3)
- `src/routing/bindings.ts`
- `src/routing/resolve-route.ts`
- `src/routing/session-key.ts`

## runtime.ts (1)
- `src/runtime.ts`

## security (10)
- `src/security/audit-extra.async.ts`
- `src/security/audit-extra.sync.ts`
- `src/security/audit-extra.ts`
- `src/security/audit-fs.ts`
- `src/security/audit.ts`
- `src/security/channel-metadata.ts`
- `src/security/external-content.ts`
- `src/security/fix.ts`
- `src/security/skill-scanner.ts`
- `src/security/windows-acl.ts`

## sessions (6)
- `src/sessions/level-overrides.ts`
- `src/sessions/model-overrides.ts`
- `src/sessions/send-policy.ts`
- `src/sessions/session-key-utils.ts`
- `src/sessions/session-label.ts`
- `src/sessions/transcript-events.ts`

## shared (1)
- `src/shared/text/reasoning-tags.ts`

## signal (14)
- `src/signal/accounts.ts`
- `src/signal/client.ts`
- `src/signal/daemon.ts`
- `src/signal/format.ts`
- `src/signal/identity.ts`
- `src/signal/index.ts`
- `src/signal/monitor.ts`
- `src/signal/monitor/event-handler.ts`
- `src/signal/monitor/event-handler.types.ts`
- `src/signal/probe.ts`
- `src/signal/reaction-level.ts`
- `src/signal/send-reactions.ts`
- `src/signal/send.ts`
- `src/signal/sse-reconnect.ts`

## slack (43)
- `src/slack/accounts.ts`
- `src/slack/actions.ts`
- `src/slack/channel-migration.ts`
- `src/slack/client.ts`
- `src/slack/directory-live.ts`
- `src/slack/format.ts`
- `src/slack/http/index.ts`
- `src/slack/http/registry.ts`
- `src/slack/index.ts`
- `src/slack/monitor.test-helpers.ts`
- `src/slack/monitor.ts`
- `src/slack/monitor/allow-list.ts`
- `src/slack/monitor/auth.ts`
- `src/slack/monitor/channel-config.ts`
- `src/slack/monitor/commands.ts`
- `src/slack/monitor/context.ts`
- `src/slack/monitor/events.ts`
- `src/slack/monitor/events/channels.ts`
- `src/slack/monitor/events/members.ts`
- `src/slack/monitor/events/messages.ts`
- `src/slack/monitor/events/pins.ts`
- `src/slack/monitor/events/reactions.ts`
- `src/slack/monitor/media.ts`
- `src/slack/monitor/message-handler.ts`
- `src/slack/monitor/message-handler/dispatch.ts`
- `src/slack/monitor/message-handler/prepare.ts`
- `src/slack/monitor/message-handler/types.ts`
- `src/slack/monitor/policy.ts`
- `src/slack/monitor/provider.ts`
- `src/slack/monitor/replies.ts`
- `src/slack/monitor/slash.ts`
- `src/slack/monitor/thread-resolution.ts`
- `src/slack/monitor/types.ts`
- `src/slack/probe.ts`
- `src/slack/resolve-channels.ts`
- `src/slack/resolve-users.ts`
- `src/slack/scopes.ts`
- `src/slack/send.ts`
- `src/slack/targets.ts`
- `src/slack/threading-tool-context.ts`
- `src/slack/threading.ts`
- `src/slack/token.ts`
- `src/slack/types.ts`

## terminal (10)
- `src/terminal/ansi.ts`
- `src/terminal/links.ts`
- `src/terminal/note.ts`
- `src/terminal/palette.ts`
- `src/terminal/progress-line.ts`
- `src/terminal/prompt-style.ts`
- `src/terminal/restore.ts`
- `src/terminal/stream-writer.ts`
- `src/terminal/table.ts`
- `src/terminal/theme.ts`

## test-helpers (2)
- `src/test-helpers/state-dir-env.ts`
- `src/test-helpers/workspace.ts`

## test-utils (2)
- `src/test-utils/channel-plugins.ts`
- `src/test-utils/ports.ts`

## tts (1)
- `src/tts/tts.ts`

## tui (24)
- `src/tui/commands.ts`
- `src/tui/components/assistant-message.ts`
- `src/tui/components/chat-log.ts`
- `src/tui/components/custom-editor.ts`
- `src/tui/components/filterable-select-list.ts`
- `src/tui/components/fuzzy-filter.ts`
- `src/tui/components/searchable-select-list.ts`
- `src/tui/components/selectors.ts`
- `src/tui/components/tool-execution.ts`
- `src/tui/components/user-message.ts`
- `src/tui/gateway-chat.ts`
- `src/tui/theme/syntax-theme.ts`
- `src/tui/theme/theme.ts`
- `src/tui/tui-command-handlers.ts`
- `src/tui/tui-event-handlers.ts`
- `src/tui/tui-formatters.ts`
- `src/tui/tui-local-shell.ts`
- `src/tui/tui-overlays.ts`
- `src/tui/tui-session-actions.ts`
- `src/tui/tui-status-summary.ts`
- `src/tui/tui-stream-assembler.ts`
- `src/tui/tui-types.ts`
- `src/tui/tui-waiting.ts`
- `src/tui/tui.ts`

## types (9)
- `src/types/cli-highlight.d.ts`
- `src/types/lydell-node-pty.d.ts`
- `src/types/napi-rs-canvas.d.ts`
- `src/types/node-edge-tts.d.ts`
- `src/types/node-llama-cpp.d.ts`
- `src/types/osc-progress.d.ts`
- `src/types/pdfjs-dist-legacy.d.ts`
- `src/types/proper-lockfile.d.ts`
- `src/types/qrcode-terminal.d.ts`

## utils (12)
- `src/utils/account-id.ts`
- `src/utils/boolean.ts`
- `src/utils/delivery-context.ts`
- `src/utils/directive-tags.ts`
- `src/utils/fetch-timeout.ts`
- `src/utils/message-channel.ts`
- `src/utils/normalize-secret-input.ts`
- `src/utils/provider-utils.ts`
- `src/utils/queue-helpers.ts`
- `src/utils/shell-argv.ts`
- `src/utils/transcript-tools.ts`
- `src/utils/usage-format.ts`

## utils.ts (1)
- `src/utils.ts`

## version.ts (1)
- `src/version.ts`

## web (43)
- `src/web/accounts.ts`
- `src/web/active-listener.ts`
- `src/web/auth-store.ts`
- `src/web/auto-reply.impl.ts`
- `src/web/auto-reply.ts`
- `src/web/auto-reply/constants.ts`
- `src/web/auto-reply/deliver-reply.ts`
- `src/web/auto-reply/heartbeat-runner.ts`
- `src/web/auto-reply/loggers.ts`
- `src/web/auto-reply/mentions.ts`
- `src/web/auto-reply/monitor.ts`
- `src/web/auto-reply/monitor/ack-reaction.ts`
- `src/web/auto-reply/monitor/broadcast.ts`
- `src/web/auto-reply/monitor/commands.ts`
- `src/web/auto-reply/monitor/echo.ts`
- `src/web/auto-reply/monitor/group-activation.ts`
- `src/web/auto-reply/monitor/group-gating.ts`
- `src/web/auto-reply/monitor/group-members.ts`
- `src/web/auto-reply/monitor/last-route.ts`
- `src/web/auto-reply/monitor/message-line.ts`
- `src/web/auto-reply/monitor/on-message.ts`
- `src/web/auto-reply/monitor/peer.ts`
- `src/web/auto-reply/monitor/process-message.ts`
- `src/web/auto-reply/session-snapshot.ts`
- `src/web/auto-reply/types.ts`
- `src/web/auto-reply/util.ts`
- `src/web/inbound.ts`
- `src/web/inbound/access-control.ts`
- `src/web/inbound/dedupe.ts`
- `src/web/inbound/extract.ts`
- `src/web/inbound/media.ts`
- `src/web/inbound/monitor.ts`
- `src/web/inbound/send-api.ts`
- `src/web/inbound/types.ts`
- `src/web/login-qr.ts`
- `src/web/login.ts`
- `src/web/media.ts`
- `src/web/outbound.ts`
- `src/web/qr-image.ts`
- `src/web/reconnect.ts`
- `src/web/session.ts`
- `src/web/test-helpers.ts`
- `src/web/vcard.ts`

## whatsapp (1)
- `src/whatsapp/normalize.ts`

## wizard (8)
- `src/wizard/clack-prompter.ts`
- `src/wizard/onboarding.completion.ts`
- `src/wizard/onboarding.finalize.ts`
- `src/wizard/onboarding.gateway-config.ts`
- `src/wizard/onboarding.ts`
- `src/wizard/onboarding.types.ts`
- `src/wizard/prompts.ts`
- `src/wizard/session.ts`

## commands (174)
- `src/commands/agent-via-gateway.ts`
- `src/commands/agent/delivery.ts`
- `src/commands/agent/run-context.ts`
- `src/commands/agent/session-store.ts`
- `src/commands/agent/session.ts`
- `src/commands/agent/types.ts`
- `src/commands/agents.bindings.ts`
- `src/commands/agents.command-shared.ts`
- `src/commands/agents.commands.add.ts`
- `src/commands/agents.commands.delete.ts`
- `src/commands/agents.commands.identity.ts`
- `src/commands/agents.commands.list.ts`
- `src/commands/agents.config.ts`
- `src/commands/agents.providers.ts`
- `src/commands/agents.ts`
- `src/commands/auth-choice-options.ts`
- `src/commands/auth-choice-prompt.ts`
- `src/commands/auth-choice.api-key.ts`
- `src/commands/auth-choice.apply.anthropic.ts`
- `src/commands/auth-choice.apply.api-providers.ts`
- `src/commands/auth-choice.apply.copilot-proxy.ts`
- `src/commands/auth-choice.apply.github-copilot.ts`
- `src/commands/auth-choice.apply.google-antigravity.ts`
- `src/commands/auth-choice.apply.google-gemini-cli.ts`
- `src/commands/auth-choice.apply.minimax.ts`
- `src/commands/auth-choice.apply.oauth.ts`
- `src/commands/auth-choice.apply.openai.ts`
- `src/commands/auth-choice.apply.plugin-provider.ts`
- `src/commands/auth-choice.apply.qwen-portal.ts`
- `src/commands/auth-choice.apply.ts`
- `src/commands/auth-choice.apply.xai.ts`
- `src/commands/auth-choice.default-model.ts`
- `src/commands/auth-choice.model-check.ts`
- `src/commands/auth-choice.preferred-provider.ts`
- `src/commands/auth-choice.ts`
- `src/commands/auth-token.ts`
- `src/commands/channels.ts`
- `src/commands/channels/add-mutators.ts`
- `src/commands/channels/add.ts`
- `src/commands/channels/capabilities.ts`
- `src/commands/channels/list.ts`
- `src/commands/channels/logs.ts`
- `src/commands/channels/remove.ts`
- `src/commands/channels/resolve.ts`
- `src/commands/channels/shared.ts`
- `src/commands/channels/status.ts`
- `src/commands/chutes-oauth.ts`
- `src/commands/cleanup-utils.ts`
- `src/commands/configure.channels.ts`
- `src/commands/configure.commands.ts`
- `src/commands/configure.daemon.ts`
- `src/commands/configure.gateway-auth.ts`
- `src/commands/configure.gateway.ts`
- `src/commands/configure.shared.ts`
- `src/commands/configure.ts`
- `src/commands/configure.wizard.ts`
- `src/commands/daemon-install-helpers.ts`
- `src/commands/daemon-runtime.ts`
- `src/commands/dashboard.ts`
- `src/commands/docs.ts`
- `src/commands/doctor-auth.ts`
- `src/commands/doctor-completion.ts`
- `src/commands/doctor-config-flow.ts`
- `src/commands/doctor-format.ts`
- `src/commands/doctor-gateway-daemon-flow.ts`
- `src/commands/doctor-gateway-health.ts`
- `src/commands/doctor-gateway-services.ts`
- `src/commands/doctor-install.ts`
- `src/commands/doctor-legacy-config.ts`
- `src/commands/doctor-platform-notes.ts`
- `src/commands/doctor-prompter.ts`
- `src/commands/doctor-sandbox.ts`
- `src/commands/doctor-security.ts`
- `src/commands/doctor-state-integrity.ts`
- `src/commands/doctor-state-migrations.ts`
- `src/commands/doctor-ui.ts`
- `src/commands/doctor-update.ts`
- `src/commands/doctor-workspace-status.ts`
- `src/commands/doctor-workspace.ts`
- `src/commands/doctor.ts`
- `src/commands/gateway-status.ts`
- `src/commands/gateway-status/helpers.ts`
- `src/commands/google-gemini-model-default.ts`
- `src/commands/health-format.ts`
- `src/commands/health.ts`
- `src/commands/message-format.ts`
- `src/commands/message.ts`
- `src/commands/model-allowlist.ts`
- `src/commands/model-picker.ts`
- `src/commands/models.ts`
- `src/commands/models/aliases.ts`
- `src/commands/models/auth-order.ts`
- `src/commands/models/auth.ts`
- `src/commands/models/fallbacks.ts`
- `src/commands/models/image-fallbacks.ts`
- `src/commands/models/list.auth-overview.ts`
- `src/commands/models/list.configured.ts`
- `src/commands/models/list.format.ts`
- `src/commands/models/list.list-command.ts`
- `src/commands/models/list.probe.ts`
- `src/commands/models/list.registry.ts`
- `src/commands/models/list.status-command.ts`
- `src/commands/models/list.table.ts`
- `src/commands/models/list.ts`
- `src/commands/models/list.types.ts`
- `src/commands/models/scan.ts`
- `src/commands/models/set-image.ts`
- `src/commands/models/set.ts`
- `src/commands/models/shared.ts`
- `src/commands/node-daemon-install-helpers.ts`
- `src/commands/node-daemon-runtime.ts`
- `src/commands/oauth-env.ts`
- `src/commands/oauth-flow.ts`
- `src/commands/onboard-auth.config-core.ts`
- `src/commands/onboard-auth.config-minimax.ts`
- `src/commands/onboard-auth.config-opencode.ts`
- `src/commands/onboard-auth.credentials.ts`
- `src/commands/onboard-auth.models.ts`
- `src/commands/onboard-auth.ts`
- `src/commands/onboard-channels.ts`
- `src/commands/onboard-custom.ts`
- `src/commands/onboard-helpers.ts`
- `src/commands/onboard-hooks.ts`
- `src/commands/onboard-interactive.ts`
- `src/commands/onboard-non-interactive.ts`
- `src/commands/onboard-non-interactive/api-keys.ts`
- `src/commands/onboard-non-interactive/local.ts`
- `src/commands/onboard-non-interactive/local/auth-choice.ts`
- `src/commands/onboard-non-interactive/local/daemon-install.ts`
- `src/commands/onboard-non-interactive/local/output.ts`
- `src/commands/onboard-non-interactive/local/gateway-config.ts`
- `src/commands/onboard-non-interactive/local/auth-choice-inference.ts`
- `src/commands/onboard-non-interactive/local/skills-config.ts`
- `src/commands/onboard-non-interactive/local/workspace.ts`
- `src/commands/onboard-non-interactive/remote.ts`
- `src/commands/onboard-remote.ts`
- `src/commands/onboard-skills.ts`
- `src/commands/onboard-types.ts`
- `src/commands/onboard.ts`
- `src/commands/onboarding/plugin-install.ts`
- `src/commands/onboarding/registry.ts`
- `src/commands/onboarding/types.ts`
- `src/commands/openai-codex-model-default.ts`
- `src/commands/openai-model-default.ts`
- `src/commands/opencode-zen-model-default.ts`
- `src/commands/reset.ts`
- `src/commands/sandbox-display.ts`
- `src/commands/sandbox-explain.ts`
- `src/commands/sandbox-formatters.ts`
- `src/commands/sandbox.ts`
- `src/commands/sessions.ts`
- `src/commands/setup.ts`
- `src/commands/signal-install.ts`
- `src/commands/status-all.ts`
- `src/commands/status-all/agents.ts`
- `src/commands/status-all/channels.ts`
- `src/commands/status-all/diagnosis.ts`
- `src/commands/status-all/format.ts`
- `src/commands/status-all/gateway.ts`
- `src/commands/status-all/report-lines.ts`
- `src/commands/status.agent-local.ts`
- `src/commands/status.command.ts`
- `src/commands/status.daemon.ts`
- `src/commands/status.format.ts`
- `src/commands/status.gateway-probe.ts`
- `src/commands/status.link-channel.ts`
- `src/commands/status.scan.ts`
- `src/commands/status.summary.ts`
- `src/commands/status.ts`
- `src/commands/status.types.ts`
- `src/commands/status.update.ts`
- `src/commands/systemd-linger.ts`
- `src/commands/uninstall.ts`
