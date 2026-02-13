# 后端文档反推审计报告

- 生成时间：2026-02-13T16:10:33.245Z
- 非测试`.ts` 源文件：1685
- docs/backend-trace/files 有效覆盖：832
- 缺文档文件：853
- Scope 外冗余文档：1

## 行为意图/约束结构检查
- 缺“行为意图/职责”章节：0
- 缺“行为”章节：832
- 缺“约束”章节：832

## 缺文档清单（按模块）
### (root)
  - `src/channel-web.ts`
  - `src/entry.ts`
  - `src/extensionAPI.ts`
  - `src/globals.ts`
  - `src/index.ts`
  - `src/logger.ts`
  - `src/logging.ts`
  - `src/polls.ts`
  - `src/runtime.ts`
  - `src/utils.ts`
  - `src/version.ts`

### acp
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

### auto-reply
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

### browser
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

### canvas-host
  - `src/canvas-host/a2ui.ts`
  - `src/canvas-host/server.ts`

### cli
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

### commands
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
  - `src/commands/onboard-non-interactive/local/auth-choice-inference.ts`
  - `src/commands/onboard-non-interactive/local/auth-choice.ts`
  - `src/commands/onboard-non-interactive/local/daemon-install.ts`
  - `src/commands/onboard-non-interactive/local/gateway-config.ts`
  - `src/commands/onboard-non-interactive/local/output.ts`
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

### compat
  - `src/compat/legacy-names.ts`

### daemon
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

### hooks
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

### imessage
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

### line
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

### link-understanding
  - `src/link-understanding/apply.ts`
  - `src/link-understanding/defaults.ts`
  - `src/link-understanding/detect.ts`
  - `src/link-understanding/format.ts`
  - `src/link-understanding/index.ts`
  - `src/link-understanding/runner.ts`

### logging
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

### macos
  - `src/macos/gateway-daemon.ts`
  - `src/macos/relay-smoke.ts`
  - `src/macos/relay.ts`

### markdown
  - `src/markdown/code-spans.ts`
  - `src/markdown/fences.ts`
  - `src/markdown/frontmatter.ts`
  - `src/markdown/ir.ts`
  - `src/markdown/render.ts`
  - `src/markdown/tables.ts`
  - `src/markdown/whatsapp.ts`

### media
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

### media-understanding
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

### node-host
  - `src/node-host/config.ts`
  - `src/node-host/runner.ts`
  - `src/node-host/with-timeout.ts`

### pairing
  - `src/pairing/pairing-labels.ts`
  - `src/pairing/pairing-messages.ts`
  - `src/pairing/pairing-store.ts`

### plugin-sdk
  - `src/plugin-sdk/index.ts`

### process
  - `src/process/child-process-bridge.ts`
  - `src/process/command-queue.ts`
  - `src/process/exec.ts`
  - `src/process/lanes.ts`
  - `src/process/spawn-utils.ts`

### providers
  - `src/providers/github-copilot-auth.ts`
  - `src/providers/github-copilot-models.ts`
  - `src/providers/github-copilot-token.ts`
  - `src/providers/qwen-portal-oauth.ts`

### routing
  - `src/routing/bindings.ts`
  - `src/routing/resolve-route.ts`
  - `src/routing/session-key.ts`

### security
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

### sessions
  - `src/sessions/level-overrides.ts`
  - `src/sessions/model-overrides.ts`
  - `src/sessions/send-policy.ts`
  - `src/sessions/session-key-utils.ts`
  - `src/sessions/session-label.ts`
  - `src/sessions/transcript-events.ts`

### shared
  - `src/shared/text/reasoning-tags.ts`

### signal
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

### slack
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

### terminal
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

### test-helpers
  - `src/test-helpers/state-dir-env.ts`
  - `src/test-helpers/workspace.ts`

### test-utils
  - `src/test-utils/channel-plugins.ts`
  - `src/test-utils/ports.ts`

### tts
  - `src/tts/tts.ts`

### tui
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

### types
  - `src/types/cli-highlight.d.ts`
  - `src/types/lydell-node-pty.d.ts`
  - `src/types/napi-rs-canvas.d.ts`
  - `src/types/node-edge-tts.d.ts`
  - `src/types/node-llama-cpp.d.ts`
  - `src/types/osc-progress.d.ts`
  - `src/types/pdfjs-dist-legacy.d.ts`
  - `src/types/proper-lockfile.d.ts`
  - `src/types/qrcode-terminal.d.ts`

### utils
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

### web
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

### whatsapp
  - `src/whatsapp/normalize.ts`

### wizard
  - `src/wizard/clack-prompter.ts`
  - `src/wizard/onboarding.completion.ts`
  - `src/wizard/onboarding.finalize.ts`
  - `src/wizard/onboarding.gateway-config.ts`
  - `src/wizard/onboarding.ts`
  - `src/wizard/onboarding.types.ts`
  - `src/wizard/prompts.ts`
  - `src/wizard/session.ts`

## 行为意图章节缺失（前 200）
- 无

## 行为章节缺失（前 200）
- `src/agents/agent-paths.ts`
- `src/agents/agent-scope.ts`
- `src/agents/anthropic-payload-log.ts`
- `src/agents/apply-patch-update.ts`
- `src/agents/apply-patch.ts`
- `src/agents/auth-health.ts`
- `src/agents/auth-profiles.ts`
- `src/agents/auth-profiles/constants.ts`
- `src/agents/auth-profiles/display.ts`
- `src/agents/auth-profiles/doctor.ts`
- `src/agents/auth-profiles/external-cli-sync.ts`
- `src/agents/auth-profiles/oauth.ts`
- `src/agents/auth-profiles/order.ts`
- `src/agents/auth-profiles/paths.ts`
- `src/agents/auth-profiles/profiles.ts`
- `src/agents/auth-profiles/repair.ts`
- `src/agents/auth-profiles/session-override.ts`
- `src/agents/auth-profiles/store.ts`
- `src/agents/auth-profiles/types.ts`
- `src/agents/auth-profiles/usage.ts`
- `src/agents/bash-process-registry.ts`
- `src/agents/bash-tools.exec.ts`
- `src/agents/bash-tools.process.ts`
- `src/agents/bash-tools.shared.ts`
- `src/agents/bash-tools.ts`
- `src/agents/bedrock-discovery.ts`
- `src/agents/bootstrap-files.ts`
- `src/agents/bootstrap-hooks.ts`
- `src/agents/cache-trace.ts`
- `src/agents/channel-tools.ts`
- `src/agents/chutes-oauth.ts`
- `src/agents/claude-cli-runner.ts`
- `src/agents/cli-backends.ts`
- `src/agents/cli-credentials.ts`
- `src/agents/cli-runner.ts`
- `src/agents/cli-runner/helpers.ts`
- `src/agents/cli-session.ts`
- `src/agents/cloudflare-ai-gateway.ts`
- `src/agents/compaction.ts`
- `src/agents/context-window-guard.ts`
- `src/agents/context.ts`
- `src/agents/current-time.ts`
- `src/agents/date-time.ts`
- `src/agents/defaults.ts`
- `src/agents/docs-path.ts`
- `src/agents/failover-error.ts`
- `src/agents/identity-avatar.ts`
- `src/agents/identity-file.ts`
- `src/agents/identity.ts`
- `src/agents/lanes.ts`
- `src/agents/live-auth-keys.ts`
- `src/agents/live-model-filter.ts`
- `src/agents/memory-search.ts`
- `src/agents/minimax-vlm.ts`
- `src/agents/model-auth.ts`
- `src/agents/model-catalog.ts`
- `src/agents/model-compat.ts`
- `src/agents/model-fallback.ts`
- `src/agents/model-scan.ts`
- `src/agents/model-selection.ts`
- `src/agents/models-config.providers.ts`
- `src/agents/models-config.ts`
- `src/agents/openclaw-tools.ts`
- `src/agents/opencode-zen-models.ts`
- `src/agents/pi-embedded-block-chunker.ts`
- `src/agents/pi-embedded-helpers.ts`
- `src/agents/pi-embedded-helpers/bootstrap.ts`
- `src/agents/pi-embedded-helpers/errors.ts`
- `src/agents/pi-embedded-helpers/google.ts`
- `src/agents/pi-embedded-helpers/images.ts`
- `src/agents/pi-embedded-helpers/messaging-dedupe.ts`
- `src/agents/pi-embedded-helpers/openai.ts`
- `src/agents/pi-embedded-helpers/thinking.ts`
- `src/agents/pi-embedded-helpers/turns.ts`
- `src/agents/pi-embedded-helpers/types.ts`
- `src/agents/pi-embedded-messaging.ts`
- `src/agents/pi-embedded-runner.ts`
- `src/agents/pi-embedded-runner/abort.ts`
- `src/agents/pi-embedded-runner/cache-ttl.ts`
- `src/agents/pi-embedded-runner/compact.ts`
- `src/agents/pi-embedded-runner/extensions.ts`
- `src/agents/pi-embedded-runner/extra-params.ts`
- `src/agents/pi-embedded-runner/google.ts`
- `src/agents/pi-embedded-runner/history.ts`
- `src/agents/pi-embedded-runner/lanes.ts`
- `src/agents/pi-embedded-runner/logger.ts`
- `src/agents/pi-embedded-runner/model.ts`
- `src/agents/pi-embedded-runner/run.ts`
- `src/agents/pi-embedded-runner/run/attempt.ts`
- `src/agents/pi-embedded-runner/run/images.ts`
- `src/agents/pi-embedded-runner/run/params.ts`
- `src/agents/pi-embedded-runner/run/payloads.ts`
- `src/agents/pi-embedded-runner/run/types.ts`
- `src/agents/pi-embedded-runner/runs.ts`
- `src/agents/pi-embedded-runner/sandbox-info.ts`
- `src/agents/pi-embedded-runner/session-manager-cache.ts`
- `src/agents/pi-embedded-runner/session-manager-init.ts`
- `src/agents/pi-embedded-runner/system-prompt.ts`
- `src/agents/pi-embedded-runner/tool-result-truncation.ts`
- `src/agents/pi-embedded-runner/tool-split.ts`
- `src/agents/pi-embedded-runner/types.ts`
- `src/agents/pi-embedded-runner/utils.ts`
- `src/agents/pi-embedded-subscribe.handlers.lifecycle.ts`
- `src/agents/pi-embedded-subscribe.handlers.messages.ts`
- `src/agents/pi-embedded-subscribe.handlers.tools.ts`
- `src/agents/pi-embedded-subscribe.handlers.ts`
- `src/agents/pi-embedded-subscribe.handlers.types.ts`
- `src/agents/pi-embedded-subscribe.raw-stream.ts`
- `src/agents/pi-embedded-subscribe.tools.ts`
- `src/agents/pi-embedded-subscribe.ts`
- `src/agents/pi-embedded-subscribe.types.ts`
- `src/agents/pi-embedded-utils.ts`
- `src/agents/pi-embedded.ts`
- `src/agents/pi-extensions/compaction-safeguard-runtime.ts`
- `src/agents/pi-extensions/compaction-safeguard.ts`
- `src/agents/pi-extensions/context-pruning.ts`
- `src/agents/pi-extensions/context-pruning/extension.ts`
- `src/agents/pi-extensions/context-pruning/pruner.ts`
- `src/agents/pi-extensions/context-pruning/runtime.ts`
- `src/agents/pi-extensions/context-pruning/settings.ts`
- `src/agents/pi-extensions/context-pruning/tools.ts`
- `src/agents/pi-model-discovery.ts`
- `src/agents/pi-settings.ts`
- `src/agents/pi-tool-definition-adapter.ts`
- `src/agents/pi-tools.abort.ts`
- `src/agents/pi-tools.before-tool-call.ts`
- `src/agents/pi-tools.policy.ts`
- `src/agents/pi-tools.read.ts`
- `src/agents/pi-tools.schema.ts`
- `src/agents/pi-tools.ts`
- `src/agents/pi-tools.types.ts`
- `src/agents/pty-dsr.ts`
- `src/agents/pty-keys.ts`
- `src/agents/sandbox-paths.ts`
- `src/agents/sandbox.ts`
- `src/agents/sandbox/browser-bridges.ts`
- `src/agents/sandbox/browser.ts`
- `src/agents/sandbox/config-hash.ts`
- `src/agents/sandbox/config.ts`
- `src/agents/sandbox/constants.ts`
- `src/agents/sandbox/context.ts`
- `src/agents/sandbox/docker.ts`
- `src/agents/sandbox/manage.ts`
- `src/agents/sandbox/prune.ts`
- `src/agents/sandbox/registry.ts`
- `src/agents/sandbox/runtime-status.ts`
- `src/agents/sandbox/shared.ts`
- `src/agents/sandbox/tool-policy.ts`
- `src/agents/sandbox/types.docker.ts`
- `src/agents/sandbox/types.ts`
- `src/agents/sandbox/workspace.ts`
- `src/agents/schema/clean-for-gemini.ts`
- `src/agents/schema/typebox.ts`
- `src/agents/session-file-repair.ts`
- `src/agents/session-slug.ts`
- `src/agents/session-tool-result-guard-wrapper.ts`
- `src/agents/session-tool-result-guard.ts`
- `src/agents/session-transcript-repair.ts`
- `src/agents/session-write-lock.ts`
- `src/agents/shell-utils.ts`
- `src/agents/skills-install.ts`
- `src/agents/skills-status.ts`
- `src/agents/skills.ts`
- `src/agents/skills/bundled-context.ts`
- `src/agents/skills/bundled-dir.ts`
- `src/agents/skills/config.ts`
- `src/agents/skills/env-overrides.ts`
- `src/agents/skills/frontmatter.ts`
- `src/agents/skills/plugin-skills.ts`
- `src/agents/skills/refresh.ts`
- `src/agents/skills/serialize.ts`
- `src/agents/skills/types.ts`
- `src/agents/skills/workspace.ts`
- `src/agents/subagent-announce-queue.ts`
- `src/agents/subagent-announce.ts`
- `src/agents/subagent-registry.store.ts`
- `src/agents/subagent-registry.ts`
- `src/agents/synthetic-models.ts`
- `src/agents/system-prompt-params.ts`
- `src/agents/system-prompt-report.ts`
- `src/agents/system-prompt.ts`
- `src/agents/test-helpers/fast-coding-tools.ts`
- `src/agents/test-helpers/fast-core-tools.ts`
- `src/agents/timeout.ts`
- `src/agents/together-models.ts`
- `src/agents/tool-call-id.ts`
- `src/agents/tool-display.ts`
- `src/agents/tool-images.ts`
- `src/agents/tool-policy.conformance.ts`
- `src/agents/tool-policy.ts`
- `src/agents/tool-summaries.ts`
- `src/agents/tools/agent-step.ts`
- `src/agents/tools/agents-list-tool.ts`
- `src/agents/tools/browser-tool.schema.ts`
- `src/agents/tools/browser-tool.ts`
- `src/agents/tools/canvas-tool.ts`
- `src/agents/tools/common.ts`
- `src/agents/tools/cron-tool.ts`
- `src/agents/tools/discord-actions-guild.ts`
- `src/agents/tools/discord-actions-messaging.ts`

## 约束章节缺失（前 200）
- `src/agents/agent-paths.ts`
- `src/agents/agent-scope.ts`
- `src/agents/anthropic-payload-log.ts`
- `src/agents/apply-patch-update.ts`
- `src/agents/apply-patch.ts`
- `src/agents/auth-health.ts`
- `src/agents/auth-profiles.ts`
- `src/agents/auth-profiles/constants.ts`
- `src/agents/auth-profiles/display.ts`
- `src/agents/auth-profiles/doctor.ts`
- `src/agents/auth-profiles/external-cli-sync.ts`
- `src/agents/auth-profiles/oauth.ts`
- `src/agents/auth-profiles/order.ts`
- `src/agents/auth-profiles/paths.ts`
- `src/agents/auth-profiles/profiles.ts`
- `src/agents/auth-profiles/repair.ts`
- `src/agents/auth-profiles/session-override.ts`
- `src/agents/auth-profiles/store.ts`
- `src/agents/auth-profiles/types.ts`
- `src/agents/auth-profiles/usage.ts`
- `src/agents/bash-process-registry.ts`
- `src/agents/bash-tools.exec.ts`
- `src/agents/bash-tools.process.ts`
- `src/agents/bash-tools.shared.ts`
- `src/agents/bash-tools.ts`
- `src/agents/bedrock-discovery.ts`
- `src/agents/bootstrap-files.ts`
- `src/agents/bootstrap-hooks.ts`
- `src/agents/cache-trace.ts`
- `src/agents/channel-tools.ts`
- `src/agents/chutes-oauth.ts`
- `src/agents/claude-cli-runner.ts`
- `src/agents/cli-backends.ts`
- `src/agents/cli-credentials.ts`
- `src/agents/cli-runner.ts`
- `src/agents/cli-runner/helpers.ts`
- `src/agents/cli-session.ts`
- `src/agents/cloudflare-ai-gateway.ts`
- `src/agents/compaction.ts`
- `src/agents/context-window-guard.ts`
- `src/agents/context.ts`
- `src/agents/current-time.ts`
- `src/agents/date-time.ts`
- `src/agents/defaults.ts`
- `src/agents/docs-path.ts`
- `src/agents/failover-error.ts`
- `src/agents/identity-avatar.ts`
- `src/agents/identity-file.ts`
- `src/agents/identity.ts`
- `src/agents/lanes.ts`
- `src/agents/live-auth-keys.ts`
- `src/agents/live-model-filter.ts`
- `src/agents/memory-search.ts`
- `src/agents/minimax-vlm.ts`
- `src/agents/model-auth.ts`
- `src/agents/model-catalog.ts`
- `src/agents/model-compat.ts`
- `src/agents/model-fallback.ts`
- `src/agents/model-scan.ts`
- `src/agents/model-selection.ts`
- `src/agents/models-config.providers.ts`
- `src/agents/models-config.ts`
- `src/agents/openclaw-tools.ts`
- `src/agents/opencode-zen-models.ts`
- `src/agents/pi-embedded-block-chunker.ts`
- `src/agents/pi-embedded-helpers.ts`
- `src/agents/pi-embedded-helpers/bootstrap.ts`
- `src/agents/pi-embedded-helpers/errors.ts`
- `src/agents/pi-embedded-helpers/google.ts`
- `src/agents/pi-embedded-helpers/images.ts`
- `src/agents/pi-embedded-helpers/messaging-dedupe.ts`
- `src/agents/pi-embedded-helpers/openai.ts`
- `src/agents/pi-embedded-helpers/thinking.ts`
- `src/agents/pi-embedded-helpers/turns.ts`
- `src/agents/pi-embedded-helpers/types.ts`
- `src/agents/pi-embedded-messaging.ts`
- `src/agents/pi-embedded-runner.ts`
- `src/agents/pi-embedded-runner/abort.ts`
- `src/agents/pi-embedded-runner/cache-ttl.ts`
- `src/agents/pi-embedded-runner/compact.ts`
- `src/agents/pi-embedded-runner/extensions.ts`
- `src/agents/pi-embedded-runner/extra-params.ts`
- `src/agents/pi-embedded-runner/google.ts`
- `src/agents/pi-embedded-runner/history.ts`
- `src/agents/pi-embedded-runner/lanes.ts`
- `src/agents/pi-embedded-runner/logger.ts`
- `src/agents/pi-embedded-runner/model.ts`
- `src/agents/pi-embedded-runner/run.ts`
- `src/agents/pi-embedded-runner/run/attempt.ts`
- `src/agents/pi-embedded-runner/run/images.ts`
- `src/agents/pi-embedded-runner/run/params.ts`
- `src/agents/pi-embedded-runner/run/payloads.ts`
- `src/agents/pi-embedded-runner/run/types.ts`
- `src/agents/pi-embedded-runner/runs.ts`
- `src/agents/pi-embedded-runner/sandbox-info.ts`
- `src/agents/pi-embedded-runner/session-manager-cache.ts`
- `src/agents/pi-embedded-runner/session-manager-init.ts`
- `src/agents/pi-embedded-runner/system-prompt.ts`
- `src/agents/pi-embedded-runner/tool-result-truncation.ts`
- `src/agents/pi-embedded-runner/tool-split.ts`
- `src/agents/pi-embedded-runner/types.ts`
- `src/agents/pi-embedded-runner/utils.ts`
- `src/agents/pi-embedded-subscribe.handlers.lifecycle.ts`
- `src/agents/pi-embedded-subscribe.handlers.messages.ts`
- `src/agents/pi-embedded-subscribe.handlers.tools.ts`
- `src/agents/pi-embedded-subscribe.handlers.ts`
- `src/agents/pi-embedded-subscribe.handlers.types.ts`
- `src/agents/pi-embedded-subscribe.raw-stream.ts`
- `src/agents/pi-embedded-subscribe.tools.ts`
- `src/agents/pi-embedded-subscribe.ts`
- `src/agents/pi-embedded-subscribe.types.ts`
- `src/agents/pi-embedded-utils.ts`
- `src/agents/pi-embedded.ts`
- `src/agents/pi-extensions/compaction-safeguard-runtime.ts`
- `src/agents/pi-extensions/compaction-safeguard.ts`
- `src/agents/pi-extensions/context-pruning.ts`
- `src/agents/pi-extensions/context-pruning/extension.ts`
- `src/agents/pi-extensions/context-pruning/pruner.ts`
- `src/agents/pi-extensions/context-pruning/runtime.ts`
- `src/agents/pi-extensions/context-pruning/settings.ts`
- `src/agents/pi-extensions/context-pruning/tools.ts`
- `src/agents/pi-model-discovery.ts`
- `src/agents/pi-settings.ts`
- `src/agents/pi-tool-definition-adapter.ts`
- `src/agents/pi-tools.abort.ts`
- `src/agents/pi-tools.before-tool-call.ts`
- `src/agents/pi-tools.policy.ts`
- `src/agents/pi-tools.read.ts`
- `src/agents/pi-tools.schema.ts`
- `src/agents/pi-tools.ts`
- `src/agents/pi-tools.types.ts`
- `src/agents/pty-dsr.ts`
- `src/agents/pty-keys.ts`
- `src/agents/sandbox-paths.ts`
- `src/agents/sandbox.ts`
- `src/agents/sandbox/browser-bridges.ts`
- `src/agents/sandbox/browser.ts`
- `src/agents/sandbox/config-hash.ts`
- `src/agents/sandbox/config.ts`
- `src/agents/sandbox/constants.ts`
- `src/agents/sandbox/context.ts`
- `src/agents/sandbox/docker.ts`
- `src/agents/sandbox/manage.ts`
- `src/agents/sandbox/prune.ts`
- `src/agents/sandbox/registry.ts`
- `src/agents/sandbox/runtime-status.ts`
- `src/agents/sandbox/shared.ts`
- `src/agents/sandbox/tool-policy.ts`
- `src/agents/sandbox/types.docker.ts`
- `src/agents/sandbox/types.ts`
- `src/agents/sandbox/workspace.ts`
- `src/agents/schema/clean-for-gemini.ts`
- `src/agents/schema/typebox.ts`
- `src/agents/session-file-repair.ts`
- `src/agents/session-slug.ts`
- `src/agents/session-tool-result-guard-wrapper.ts`
- `src/agents/session-tool-result-guard.ts`
- `src/agents/session-transcript-repair.ts`
- `src/agents/session-write-lock.ts`
- `src/agents/shell-utils.ts`
- `src/agents/skills-install.ts`
- `src/agents/skills-status.ts`
- `src/agents/skills.ts`
- `src/agents/skills/bundled-context.ts`
- `src/agents/skills/bundled-dir.ts`
- `src/agents/skills/config.ts`
- `src/agents/skills/env-overrides.ts`
- `src/agents/skills/frontmatter.ts`
- `src/agents/skills/plugin-skills.ts`
- `src/agents/skills/refresh.ts`
- `src/agents/skills/serialize.ts`
- `src/agents/skills/types.ts`
- `src/agents/skills/workspace.ts`
- `src/agents/subagent-announce-queue.ts`
- `src/agents/subagent-announce.ts`
- `src/agents/subagent-registry.store.ts`
- `src/agents/subagent-registry.ts`
- `src/agents/synthetic-models.ts`
- `src/agents/system-prompt-params.ts`
- `src/agents/system-prompt-report.ts`
- `src/agents/system-prompt.ts`
- `src/agents/test-helpers/fast-coding-tools.ts`
- `src/agents/test-helpers/fast-core-tools.ts`
- `src/agents/timeout.ts`
- `src/agents/together-models.ts`
- `src/agents/tool-call-id.ts`
- `src/agents/tool-display.ts`
- `src/agents/tool-images.ts`
- `src/agents/tool-policy.conformance.ts`
- `src/agents/tool-policy.ts`
- `src/agents/tool-summaries.ts`
- `src/agents/tools/agent-step.ts`
- `src/agents/tools/agents-list-tool.ts`
- `src/agents/tools/browser-tool.schema.ts`
- `src/agents/tools/browser-tool.ts`
- `src/agents/tools/canvas-tool.ts`
- `src/agents/tools/common.ts`
- `src/agents/tools/cron-tool.ts`
- `src/agents/tools/discord-actions-guild.ts`
- `src/agents/tools/discord-actions-messaging.ts`

## Scope 外冗余文档（前 200）
- `src/gateway/server/__tests__/test-utils.ts`

## 文档齐套性（按源文件）
| 源文件 | 文档路径 | 缺失章节 |
| --- | --- | --- |
| `src/acp/client.ts` | （未建） | 缺失文档 |
| `src/acp/commands.ts` | （未建） | 缺失文档 |
| `src/acp/event-mapper.ts` | （未建） | 缺失文档 |
| `src/acp/index.ts` | （未建） | 缺失文档 |
| `src/acp/meta.ts` | （未建） | 缺失文档 |
| `src/acp/server.ts` | （未建） | 缺失文档 |
| `src/acp/session-mapper.ts` | （未建） | 缺失文档 |
| `src/acp/session.ts` | （未建） | 缺失文档 |
| `src/acp/translator.ts` | （未建） | 缺失文档 |
| `src/acp/types.ts` | （未建） | 缺失文档 |
| `src/agents/agent-paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/agent-paths.ts.md` | 行为、约束 |
| `src/agents/agent-scope.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/agent-scope.ts.md` | 行为、约束 |
| `src/agents/anthropic-payload-log.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/anthropic-payload-log.ts.md` | 行为、约束 |
| `src/agents/apply-patch-update.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/apply-patch-update.ts.md` | 行为、约束 |
| `src/agents/apply-patch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/apply-patch.ts.md` | 行为、约束 |
| `src/agents/auth-health.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-health.ts.md` | 行为、约束 |
| `src/agents/auth-profiles.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/constants.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/constants.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/display.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/display.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/doctor.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/doctor.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/external-cli-sync.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/external-cli-sync.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/oauth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/oauth.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/order.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/order.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/paths.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/profiles.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/profiles.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/repair.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/repair.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/session-override.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/session-override.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/store.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/types.ts.md` | 行为、约束 |
| `src/agents/auth-profiles/usage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/auth-profiles/usage.ts.md` | 行为、约束 |
| `src/agents/bash-process-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bash-process-registry.ts.md` | 行为、约束 |
| `src/agents/bash-tools.exec.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bash-tools.exec.ts.md` | 行为、约束 |
| `src/agents/bash-tools.process.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bash-tools.process.ts.md` | 行为、约束 |
| `src/agents/bash-tools.shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bash-tools.shared.ts.md` | 行为、约束 |
| `src/agents/bash-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bash-tools.ts.md` | 行为、约束 |
| `src/agents/bedrock-discovery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bedrock-discovery.ts.md` | 行为、约束 |
| `src/agents/bootstrap-files.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bootstrap-files.ts.md` | 行为、约束 |
| `src/agents/bootstrap-hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/bootstrap-hooks.ts.md` | 行为、约束 |
| `src/agents/cache-trace.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cache-trace.ts.md` | 行为、约束 |
| `src/agents/channel-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/channel-tools.ts.md` | 行为、约束 |
| `src/agents/chutes-oauth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/chutes-oauth.ts.md` | 行为、约束 |
| `src/agents/claude-cli-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/claude-cli-runner.ts.md` | 行为、约束 |
| `src/agents/cli-backends.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cli-backends.ts.md` | 行为、约束 |
| `src/agents/cli-credentials.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cli-credentials.ts.md` | 行为、约束 |
| `src/agents/cli-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cli-runner.ts.md` | 行为、约束 |
| `src/agents/cli-runner/helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cli-runner/helpers.ts.md` | 行为、约束 |
| `src/agents/cli-session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cli-session.ts.md` | 行为、约束 |
| `src/agents/cloudflare-ai-gateway.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/cloudflare-ai-gateway.ts.md` | 行为、约束 |
| `src/agents/compaction.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/compaction.ts.md` | 行为、约束 |
| `src/agents/context-window-guard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/context-window-guard.ts.md` | 行为、约束 |
| `src/agents/context.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/context.ts.md` | 行为、约束 |
| `src/agents/current-time.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/current-time.ts.md` | 行为、约束 |
| `src/agents/date-time.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/date-time.ts.md` | 行为、约束 |
| `src/agents/defaults.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/defaults.ts.md` | 行为、约束 |
| `src/agents/docs-path.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/docs-path.ts.md` | 行为、约束 |
| `src/agents/failover-error.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/failover-error.ts.md` | 行为、约束 |
| `src/agents/identity-avatar.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/identity-avatar.ts.md` | 行为、约束 |
| `src/agents/identity-file.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/identity-file.ts.md` | 行为、约束 |
| `src/agents/identity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/identity.ts.md` | 行为、约束 |
| `src/agents/lanes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/lanes.ts.md` | 行为、约束 |
| `src/agents/live-auth-keys.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/live-auth-keys.ts.md` | 行为、约束 |
| `src/agents/live-model-filter.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/live-model-filter.ts.md` | 行为、约束 |
| `src/agents/memory-search.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/memory-search.ts.md` | 行为、约束 |
| `src/agents/minimax-vlm.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/minimax-vlm.ts.md` | 行为、约束 |
| `src/agents/model-auth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-auth.ts.md` | 行为、约束 |
| `src/agents/model-catalog.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-catalog.ts.md` | 行为、约束 |
| `src/agents/model-compat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-compat.ts.md` | 行为、约束 |
| `src/agents/model-fallback.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-fallback.ts.md` | 行为、约束 |
| `src/agents/model-scan.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-scan.ts.md` | 行为、约束 |
| `src/agents/model-selection.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/model-selection.ts.md` | 行为、约束 |
| `src/agents/models-config.providers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/models-config.providers.ts.md` | 行为、约束 |
| `src/agents/models-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/models-config.ts.md` | 行为、约束 |
| `src/agents/openclaw-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/openclaw-tools.ts.md` | 行为、约束 |
| `src/agents/opencode-zen-models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/opencode-zen-models.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-block-chunker.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-block-chunker.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/bootstrap.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/bootstrap.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/errors.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/errors.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/google.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/google.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/images.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/images.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/messaging-dedupe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/messaging-dedupe.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/openai.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/openai.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/thinking.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/thinking.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/turns.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/turns.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-helpers/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-helpers/types.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-messaging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-messaging.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/abort.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/abort.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/cache-ttl.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/cache-ttl.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/compact.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/compact.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/extensions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/extensions.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/extra-params.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/extra-params.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/google.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/google.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/history.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/history.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/lanes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/lanes.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/logger.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/logger.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/model.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/model.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run/attempt.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run/attempt.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run/images.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run/images.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run/params.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run/params.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run/payloads.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run/payloads.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/run/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/run/types.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/runs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/runs.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/sandbox-info.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/sandbox-info.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/session-manager-cache.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/session-manager-cache.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/session-manager-init.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/session-manager-init.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/system-prompt.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/system-prompt.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/tool-result-truncation.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/tool-result-truncation.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/tool-split.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/tool-split.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/types.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-runner/utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-runner/utils.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.handlers.lifecycle.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.handlers.lifecycle.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.handlers.messages.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.handlers.messages.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.handlers.tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.handlers.tools.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.handlers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.handlers.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.handlers.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.handlers.types.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.raw-stream.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.raw-stream.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.tools.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-subscribe.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-subscribe.types.ts.md` | 行为、约束 |
| `src/agents/pi-embedded-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded-utils.ts.md` | 行为、约束 |
| `src/agents/pi-embedded.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-embedded.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/compaction-safeguard-runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/compaction-safeguard-runtime.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/compaction-safeguard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/compaction-safeguard.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning/extension.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning/extension.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning/pruner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning/pruner.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning/runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning/runtime.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning/settings.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning/settings.ts.md` | 行为、约束 |
| `src/agents/pi-extensions/context-pruning/tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-extensions/context-pruning/tools.ts.md` | 行为、约束 |
| `src/agents/pi-model-discovery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-model-discovery.ts.md` | 行为、约束 |
| `src/agents/pi-settings.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-settings.ts.md` | 行为、约束 |
| `src/agents/pi-tool-definition-adapter.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tool-definition-adapter.ts.md` | 行为、约束 |
| `src/agents/pi-tools.abort.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.abort.ts.md` | 行为、约束 |
| `src/agents/pi-tools.before-tool-call.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.before-tool-call.ts.md` | 行为、约束 |
| `src/agents/pi-tools.policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.policy.ts.md` | 行为、约束 |
| `src/agents/pi-tools.read.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.read.ts.md` | 行为、约束 |
| `src/agents/pi-tools.schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.schema.ts.md` | 行为、约束 |
| `src/agents/pi-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.ts.md` | 行为、约束 |
| `src/agents/pi-tools.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pi-tools.types.ts.md` | 行为、约束 |
| `src/agents/pty-dsr.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pty-dsr.ts.md` | 行为、约束 |
| `src/agents/pty-keys.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/pty-keys.ts.md` | 行为、约束 |
| `src/agents/sandbox-paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox-paths.ts.md` | 行为、约束 |
| `src/agents/sandbox.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox.ts.md` | 行为、约束 |
| `src/agents/sandbox/browser-bridges.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/browser-bridges.ts.md` | 行为、约束 |
| `src/agents/sandbox/browser.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/browser.ts.md` | 行为、约束 |
| `src/agents/sandbox/config-hash.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/config-hash.ts.md` | 行为、约束 |
| `src/agents/sandbox/config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/config.ts.md` | 行为、约束 |
| `src/agents/sandbox/constants.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/constants.ts.md` | 行为、约束 |
| `src/agents/sandbox/context.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/context.ts.md` | 行为、约束 |
| `src/agents/sandbox/docker.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/docker.ts.md` | 行为、约束 |
| `src/agents/sandbox/manage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/manage.ts.md` | 行为、约束 |
| `src/agents/sandbox/prune.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/prune.ts.md` | 行为、约束 |
| `src/agents/sandbox/registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/registry.ts.md` | 行为、约束 |
| `src/agents/sandbox/runtime-status.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/runtime-status.ts.md` | 行为、约束 |
| `src/agents/sandbox/shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/shared.ts.md` | 行为、约束 |
| `src/agents/sandbox/tool-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/tool-policy.ts.md` | 行为、约束 |
| `src/agents/sandbox/types.docker.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/types.docker.ts.md` | 行为、约束 |
| `src/agents/sandbox/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/types.ts.md` | 行为、约束 |
| `src/agents/sandbox/workspace.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/sandbox/workspace.ts.md` | 行为、约束 |
| `src/agents/schema/clean-for-gemini.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/schema/clean-for-gemini.ts.md` | 行为、约束 |
| `src/agents/schema/typebox.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/schema/typebox.ts.md` | 行为、约束 |
| `src/agents/session-file-repair.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-file-repair.ts.md` | 行为、约束 |
| `src/agents/session-slug.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-slug.ts.md` | 行为、约束 |
| `src/agents/session-tool-result-guard-wrapper.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-tool-result-guard-wrapper.ts.md` | 行为、约束 |
| `src/agents/session-tool-result-guard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-tool-result-guard.ts.md` | 行为、约束 |
| `src/agents/session-transcript-repair.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-transcript-repair.ts.md` | 行为、约束 |
| `src/agents/session-write-lock.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/session-write-lock.ts.md` | 行为、约束 |
| `src/agents/shell-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/shell-utils.ts.md` | 行为、约束 |
| `src/agents/skills-install.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills-install.ts.md` | 行为、约束 |
| `src/agents/skills-status.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills-status.ts.md` | 行为、约束 |
| `src/agents/skills.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills.ts.md` | 行为、约束 |
| `src/agents/skills/bundled-context.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/bundled-context.ts.md` | 行为、约束 |
| `src/agents/skills/bundled-dir.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/bundled-dir.ts.md` | 行为、约束 |
| `src/agents/skills/config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/config.ts.md` | 行为、约束 |
| `src/agents/skills/env-overrides.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/env-overrides.ts.md` | 行为、约束 |
| `src/agents/skills/frontmatter.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/frontmatter.ts.md` | 行为、约束 |
| `src/agents/skills/plugin-skills.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/plugin-skills.ts.md` | 行为、约束 |
| `src/agents/skills/refresh.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/refresh.ts.md` | 行为、约束 |
| `src/agents/skills/serialize.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/serialize.ts.md` | 行为、约束 |
| `src/agents/skills/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/types.ts.md` | 行为、约束 |
| `src/agents/skills/workspace.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/skills/workspace.ts.md` | 行为、约束 |
| `src/agents/subagent-announce-queue.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/subagent-announce-queue.ts.md` | 行为、约束 |
| `src/agents/subagent-announce.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/subagent-announce.ts.md` | 行为、约束 |
| `src/agents/subagent-registry.store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/subagent-registry.store.ts.md` | 行为、约束 |
| `src/agents/subagent-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/subagent-registry.ts.md` | 行为、约束 |
| `src/agents/synthetic-models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/synthetic-models.ts.md` | 行为、约束 |
| `src/agents/system-prompt-params.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/system-prompt-params.ts.md` | 行为、约束 |
| `src/agents/system-prompt-report.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/system-prompt-report.ts.md` | 行为、约束 |
| `src/agents/system-prompt.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/system-prompt.ts.md` | 行为、约束 |
| `src/agents/test-helpers/fast-coding-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/test-helpers/fast-coding-tools.ts.md` | 行为、约束 |
| `src/agents/test-helpers/fast-core-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/test-helpers/fast-core-tools.ts.md` | 行为、约束 |
| `src/agents/timeout.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/timeout.ts.md` | 行为、约束 |
| `src/agents/together-models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/together-models.ts.md` | 行为、约束 |
| `src/agents/tool-call-id.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-call-id.ts.md` | 行为、约束 |
| `src/agents/tool-display.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-display.ts.md` | 行为、约束 |
| `src/agents/tool-images.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-images.ts.md` | 行为、约束 |
| `src/agents/tool-policy.conformance.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-policy.conformance.ts.md` | 行为、约束 |
| `src/agents/tool-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-policy.ts.md` | 行为、约束 |
| `src/agents/tool-summaries.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tool-summaries.ts.md` | 行为、约束 |
| `src/agents/tools/agent-step.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/agent-step.ts.md` | 行为、约束 |
| `src/agents/tools/agents-list-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/agents-list-tool.ts.md` | 行为、约束 |
| `src/agents/tools/browser-tool.schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/browser-tool.schema.ts.md` | 行为、约束 |
| `src/agents/tools/browser-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/browser-tool.ts.md` | 行为、约束 |
| `src/agents/tools/canvas-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/canvas-tool.ts.md` | 行为、约束 |
| `src/agents/tools/common.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/common.ts.md` | 行为、约束 |
| `src/agents/tools/cron-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/cron-tool.ts.md` | 行为、约束 |
| `src/agents/tools/discord-actions-guild.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/discord-actions-guild.ts.md` | 行为、约束 |
| `src/agents/tools/discord-actions-messaging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/discord-actions-messaging.ts.md` | 行为、约束 |
| `src/agents/tools/discord-actions-moderation.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/discord-actions-moderation.ts.md` | 行为、约束 |
| `src/agents/tools/discord-actions-presence.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/discord-actions-presence.ts.md` | 行为、约束 |
| `src/agents/tools/discord-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/discord-actions.ts.md` | 行为、约束 |
| `src/agents/tools/gateway-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/gateway-tool.ts.md` | 行为、约束 |
| `src/agents/tools/gateway.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/gateway.ts.md` | 行为、约束 |
| `src/agents/tools/image-tool.helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/image-tool.helpers.ts.md` | 行为、约束 |
| `src/agents/tools/image-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/image-tool.ts.md` | 行为、约束 |
| `src/agents/tools/memory-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/memory-tool.ts.md` | 行为、约束 |
| `src/agents/tools/message-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/message-tool.ts.md` | 行为、约束 |
| `src/agents/tools/nodes-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/nodes-tool.ts.md` | 行为、约束 |
| `src/agents/tools/nodes-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/nodes-utils.ts.md` | 行为、约束 |
| `src/agents/tools/session-status-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/session-status-tool.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-announce-target.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-announce-target.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-helpers.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-history-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-history-tool.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-list-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-list-tool.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-send-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-send-helpers.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-send-tool.a2a.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-send-tool.a2a.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-send-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-send-tool.ts.md` | 行为、约束 |
| `src/agents/tools/sessions-spawn-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/sessions-spawn-tool.ts.md` | 行为、约束 |
| `src/agents/tools/slack-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/slack-actions.ts.md` | 行为、约束 |
| `src/agents/tools/telegram-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/telegram-actions.ts.md` | 行为、约束 |
| `src/agents/tools/tts-tool.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/tts-tool.ts.md` | 行为、约束 |
| `src/agents/tools/web-fetch-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/web-fetch-utils.ts.md` | 行为、约束 |
| `src/agents/tools/web-fetch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/web-fetch.ts.md` | 行为、约束 |
| `src/agents/tools/web-search.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/web-search.ts.md` | 行为、约束 |
| `src/agents/tools/web-shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/web-shared.ts.md` | 行为、约束 |
| `src/agents/tools/web-tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/web-tools.ts.md` | 行为、约束 |
| `src/agents/tools/whatsapp-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/tools/whatsapp-actions.ts.md` | 行为、约束 |
| `src/agents/transcript-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/transcript-policy.ts.md` | 行为、约束 |
| `src/agents/usage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/usage.ts.md` | 行为、约束 |
| `src/agents/venice-models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/venice-models.ts.md` | 行为、约束 |
| `src/agents/workspace-run.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/workspace-run.ts.md` | 行为、约束 |
| `src/agents/workspace-templates.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/workspace-templates.ts.md` | 行为、约束 |
| `src/agents/workspace.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/agents/workspace.ts.md` | 行为、约束 |
| `src/auto-reply/chunk.ts` | （未建） | 缺失文档 |
| `src/auto-reply/command-auth.ts` | （未建） | 缺失文档 |
| `src/auto-reply/command-detection.ts` | （未建） | 缺失文档 |
| `src/auto-reply/commands-args.ts` | （未建） | 缺失文档 |
| `src/auto-reply/commands-registry.data.ts` | （未建） | 缺失文档 |
| `src/auto-reply/commands-registry.ts` | （未建） | 缺失文档 |
| `src/auto-reply/commands-registry.types.ts` | （未建） | 缺失文档 |
| `src/auto-reply/dispatch.ts` | （未建） | 缺失文档 |
| `src/auto-reply/envelope.ts` | （未建） | 缺失文档 |
| `src/auto-reply/group-activation.ts` | （未建） | 缺失文档 |
| `src/auto-reply/heartbeat.ts` | （未建） | 缺失文档 |
| `src/auto-reply/inbound-debounce.ts` | （未建） | 缺失文档 |
| `src/auto-reply/media-note.ts` | （未建） | 缺失文档 |
| `src/auto-reply/model.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/abort.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner-execution.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner-helpers.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner-memory.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner-payloads.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner-utils.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/agent-runner.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/audio-tags.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/bash-command.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/block-reply-coalescer.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/block-reply-pipeline.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/block-streaming.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/body.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-allowlist.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-approve.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-bash.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-compact.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-config.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-context-report.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-context.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-core.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-info.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-models.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-plugin.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-ptt.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-session.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-status.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-subagents.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-tts.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands-types.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/commands.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/config-commands.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/config-value.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/debug-commands.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.auth.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.fast-lane.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.impl.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.model-picker.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.model.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.parse.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.persist.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.queue-validation.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.shared.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directive-handling.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/directives.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/dispatch-from-config.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/exec.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/exec/directive.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/followup-runner.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply-directives-apply.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply-directives-utils.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply-directives.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply-inline-actions.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply-run.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/get-reply.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/groups.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/history.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/inbound-context.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/inbound-dedupe.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/inbound-meta.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/inbound-text.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/line-directives.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/memory-flush.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/mentions.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/model-selection.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/normalize-reply.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/provider-dispatcher.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/cleanup.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/directive.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/drain.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/enqueue.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/normalize.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/settings.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/state.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/queue/types.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-directives.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-dispatcher.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-elevated.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-inline.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-payloads.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-reference.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-tags.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/reply-threading.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/response-prefix-template.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/route-reply.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/session-reset-model.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/session-updates.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/session-usage.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/session.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/stage-sandbox-media.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/streaming-directives.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/subagents-utils.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/test-ctx.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/test-helpers.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/typing-mode.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/typing.ts` | （未建） | 缺失文档 |
| `src/auto-reply/reply/untrusted-context.ts` | （未建） | 缺失文档 |
| `src/auto-reply/send-policy.ts` | （未建） | 缺失文档 |
| `src/auto-reply/skill-commands.ts` | （未建） | 缺失文档 |
| `src/auto-reply/status.ts` | （未建） | 缺失文档 |
| `src/auto-reply/templating.ts` | （未建） | 缺失文档 |
| `src/auto-reply/thinking.ts` | （未建） | 缺失文档 |
| `src/auto-reply/tokens.ts` | （未建） | 缺失文档 |
| `src/auto-reply/tool-meta.ts` | （未建） | 缺失文档 |
| `src/auto-reply/types.ts` | （未建） | 缺失文档 |
| `src/browser/bridge-server.ts` | （未建） | 缺失文档 |
| `src/browser/cdp.helpers.ts` | （未建） | 缺失文档 |
| `src/browser/cdp.ts` | （未建） | 缺失文档 |
| `src/browser/chrome.executables.ts` | （未建） | 缺失文档 |
| `src/browser/chrome.profile-decoration.ts` | （未建） | 缺失文档 |
| `src/browser/chrome.ts` | （未建） | 缺失文档 |
| `src/browser/client-actions-core.ts` | （未建） | 缺失文档 |
| `src/browser/client-actions-observe.ts` | （未建） | 缺失文档 |
| `src/browser/client-actions-state.ts` | （未建） | 缺失文档 |
| `src/browser/client-actions-types.ts` | （未建） | 缺失文档 |
| `src/browser/client-actions.ts` | （未建） | 缺失文档 |
| `src/browser/client-fetch.ts` | （未建） | 缺失文档 |
| `src/browser/client.ts` | （未建） | 缺失文档 |
| `src/browser/config.ts` | （未建） | 缺失文档 |
| `src/browser/constants.ts` | （未建） | 缺失文档 |
| `src/browser/control-service.ts` | （未建） | 缺失文档 |
| `src/browser/extension-relay.ts` | （未建） | 缺失文档 |
| `src/browser/profiles-service.ts` | （未建） | 缺失文档 |
| `src/browser/profiles.ts` | （未建） | 缺失文档 |
| `src/browser/pw-ai-module.ts` | （未建） | 缺失文档 |
| `src/browser/pw-ai.ts` | （未建） | 缺失文档 |
| `src/browser/pw-role-snapshot.ts` | （未建） | 缺失文档 |
| `src/browser/pw-session.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.activity.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.downloads.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.interactions.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.responses.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.shared.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.snapshot.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.state.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.storage.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.trace.ts` | （未建） | 缺失文档 |
| `src/browser/pw-tools-core.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.act.shared.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.act.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.debug.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.shared.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.snapshot.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.storage.ts` | （未建） | 缺失文档 |
| `src/browser/routes/agent.ts` | （未建） | 缺失文档 |
| `src/browser/routes/basic.ts` | （未建） | 缺失文档 |
| `src/browser/routes/dispatcher.ts` | （未建） | 缺失文档 |
| `src/browser/routes/index.ts` | （未建） | 缺失文档 |
| `src/browser/routes/tabs.ts` | （未建） | 缺失文档 |
| `src/browser/routes/types.ts` | （未建） | 缺失文档 |
| `src/browser/routes/utils.ts` | （未建） | 缺失文档 |
| `src/browser/screenshot.ts` | （未建） | 缺失文档 |
| `src/browser/server-context.ts` | （未建） | 缺失文档 |
| `src/browser/server-context.types.ts` | （未建） | 缺失文档 |
| `src/browser/server.ts` | （未建） | 缺失文档 |
| `src/browser/target-id.ts` | （未建） | 缺失文档 |
| `src/browser/trash.ts` | （未建） | 缺失文档 |
| `src/canvas-host/a2ui.ts` | （未建） | 缺失文档 |
| `src/canvas-host/server.ts` | （未建） | 缺失文档 |
| `src/channel-web.ts` | （未建） | 缺失文档 |
| `src/channels/ack-reactions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/ack-reactions.ts.md` | 行为、约束 |
| `src/channels/allowlist-match.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/allowlist-match.ts.md` | 行为、约束 |
| `src/channels/allowlists/resolve-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/allowlists/resolve-utils.ts.md` | 行为、约束 |
| `src/channels/channel-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/channel-config.ts.md` | 行为、约束 |
| `src/channels/chat-type.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/chat-type.ts.md` | 行为、约束 |
| `src/channels/command-gating.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/command-gating.ts.md` | 行为、约束 |
| `src/channels/conversation-label.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/conversation-label.ts.md` | 行为、约束 |
| `src/channels/dock.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/dock.ts.md` | 行为、约束 |
| `src/channels/location.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/location.ts.md` | 行为、约束 |
| `src/channels/logging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/logging.ts.md` | 行为、约束 |
| `src/channels/mention-gating.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/mention-gating.ts.md` | 行为、约束 |
| `src/channels/plugins/actions/discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/actions/discord.ts.md` | 行为、约束 |
| `src/channels/plugins/actions/discord/handle-action.guild-admin.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/actions/discord/handle-action.guild-admin.ts.md` | 行为、约束 |
| `src/channels/plugins/actions/discord/handle-action.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/actions/discord/handle-action.ts.md` | 行为、约束 |
| `src/channels/plugins/actions/signal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/actions/signal.ts.md` | 行为、约束 |
| `src/channels/plugins/actions/telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/actions/telegram.ts.md` | 行为、约束 |
| `src/channels/plugins/agent-tools/whatsapp-login.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/agent-tools/whatsapp-login.ts.md` | 行为、约束 |
| `src/channels/plugins/allowlist-match.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/allowlist-match.ts.md` | 行为、约束 |
| `src/channels/plugins/bluebubbles-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/bluebubbles-actions.ts.md` | 行为、约束 |
| `src/channels/plugins/catalog.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/catalog.ts.md` | 行为、约束 |
| `src/channels/plugins/channel-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/channel-config.ts.md` | 行为、约束 |
| `src/channels/plugins/config-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/config-helpers.ts.md` | 行为、约束 |
| `src/channels/plugins/config-schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/config-schema.ts.md` | 行为、约束 |
| `src/channels/plugins/config-writes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/config-writes.ts.md` | 行为、约束 |
| `src/channels/plugins/directory-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/directory-config.ts.md` | 行为、约束 |
| `src/channels/plugins/group-mentions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/group-mentions.ts.md` | 行为、约束 |
| `src/channels/plugins/helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/helpers.ts.md` | 行为、约束 |
| `src/channels/plugins/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/index.ts.md` | 行为、约束 |
| `src/channels/plugins/load.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/load.ts.md` | 行为、约束 |
| `src/channels/plugins/media-limits.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/media-limits.ts.md` | 行为、约束 |
| `src/channels/plugins/message-action-names.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/message-action-names.ts.md` | 行为、约束 |
| `src/channels/plugins/message-actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/message-actions.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/discord.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/imessage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/imessage.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/signal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/signal.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/slack.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/slack.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/telegram.ts.md` | 行为、约束 |
| `src/channels/plugins/normalize/whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/normalize/whatsapp.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding-types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding-types.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/channel-access.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/channel-access.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/discord.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/helpers.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/imessage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/imessage.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/signal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/signal.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/slack.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/slack.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/telegram.ts.md` | 行为、约束 |
| `src/channels/plugins/onboarding/whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/onboarding/whatsapp.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/discord.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/imessage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/imessage.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/load.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/load.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/signal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/signal.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/slack.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/slack.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/telegram.ts.md` | 行为、约束 |
| `src/channels/plugins/outbound/whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/outbound/whatsapp.ts.md` | 行为、约束 |
| `src/channels/plugins/pairing-message.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/pairing-message.ts.md` | 行为、约束 |
| `src/channels/plugins/pairing.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/pairing.ts.md` | 行为、约束 |
| `src/channels/plugins/setup-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/setup-helpers.ts.md` | 行为、约束 |
| `src/channels/plugins/slack.actions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/slack.actions.ts.md` | 行为、约束 |
| `src/channels/plugins/status-issues/bluebubbles.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status-issues/bluebubbles.ts.md` | 行为、约束 |
| `src/channels/plugins/status-issues/discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status-issues/discord.ts.md` | 行为、约束 |
| `src/channels/plugins/status-issues/shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status-issues/shared.ts.md` | 行为、约束 |
| `src/channels/plugins/status-issues/telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status-issues/telegram.ts.md` | 行为、约束 |
| `src/channels/plugins/status-issues/whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status-issues/whatsapp.ts.md` | 行为、约束 |
| `src/channels/plugins/status.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/status.ts.md` | 行为、约束 |
| `src/channels/plugins/types.adapters.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/types.adapters.ts.md` | 行为、约束 |
| `src/channels/plugins/types.core.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/types.core.ts.md` | 行为、约束 |
| `src/channels/plugins/types.plugin.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/types.plugin.ts.md` | 行为、约束 |
| `src/channels/plugins/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/types.ts.md` | 行为、约束 |
| `src/channels/plugins/whatsapp-heartbeat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/plugins/whatsapp-heartbeat.ts.md` | 行为、约束 |
| `src/channels/registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/registry.ts.md` | 行为、约束 |
| `src/channels/reply-prefix.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/reply-prefix.ts.md` | 行为、约束 |
| `src/channels/sender-identity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/sender-identity.ts.md` | 行为、约束 |
| `src/channels/sender-label.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/sender-label.ts.md` | 行为、约束 |
| `src/channels/session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/session.ts.md` | 行为、约束 |
| `src/channels/targets.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/targets.ts.md` | 行为、约束 |
| `src/channels/typing.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/typing.ts.md` | 行为、约束 |
| `src/channels/web/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/channels/web/index.ts.md` | 行为、约束 |
| `src/cli/acp-cli.ts` | （未建） | 缺失文档 |
| `src/cli/argv.ts` | （未建） | 缺失文档 |
| `src/cli/banner.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/register.element.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/register.files-downloads.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/register.form-wait-eval.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/register.navigation.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/register.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-input/shared.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-actions-observe.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-debug.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-examples.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-extension.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-inspect.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-manage.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-shared.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-state.cookies-storage.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli-state.ts` | （未建） | 缺失文档 |
| `src/cli/browser-cli.ts` | （未建） | 缺失文档 |
| `src/cli/channel-auth.ts` | （未建） | 缺失文档 |
| `src/cli/channel-options.ts` | （未建） | 缺失文档 |
| `src/cli/channels-cli.ts` | （未建） | 缺失文档 |
| `src/cli/cli-name.ts` | （未建） | 缺失文档 |
| `src/cli/cli-utils.ts` | （未建） | 缺失文档 |
| `src/cli/command-format.ts` | （未建） | 缺失文档 |
| `src/cli/command-options.ts` | （未建） | 缺失文档 |
| `src/cli/completion-cli.ts` | （未建） | 缺失文档 |
| `src/cli/config-cli.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli/register.cron-add.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli/register.cron-edit.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli/register.cron-simple.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli/register.ts` | （未建） | 缺失文档 |
| `src/cli/cron-cli/shared.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/install.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/lifecycle.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/probe.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/register.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/response.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/runners.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/shared.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/status.gather.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/status.print.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/status.ts` | （未建） | 缺失文档 |
| `src/cli/daemon-cli/types.ts` | （未建） | 缺失文档 |
| `src/cli/deps.ts` | （未建） | 缺失文档 |
| `src/cli/devices-cli.ts` | （未建） | 缺失文档 |
| `src/cli/directory-cli.ts` | （未建） | 缺失文档 |
| `src/cli/dns-cli.ts` | （未建） | 缺失文档 |
| `src/cli/docs-cli.ts` | （未建） | 缺失文档 |
| `src/cli/exec-approvals-cli.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/call.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/dev.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/discover.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/register.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/run-loop.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/run.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-cli/shared.ts` | （未建） | 缺失文档 |
| `src/cli/gateway-rpc.ts` | （未建） | 缺失文档 |
| `src/cli/help-format.ts` | （未建） | 缺失文档 |
| `src/cli/hooks-cli.ts` | （未建） | 缺失文档 |
| `src/cli/logs-cli.ts` | （未建） | 缺失文档 |
| `src/cli/memory-cli.ts` | （未建） | 缺失文档 |
| `src/cli/models-cli.ts` | （未建） | 缺失文档 |
| `src/cli/node-cli.ts` | （未建） | 缺失文档 |
| `src/cli/node-cli/daemon.ts` | （未建） | 缺失文档 |
| `src/cli/node-cli/register.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-camera.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-canvas.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/a2ui-jsonl.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/cli-utils.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/format.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.camera.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.canvas.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.invoke.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.location.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.notify.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.pairing.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.screen.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.status.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/register.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/rpc.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-cli/types.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-run.ts` | （未建） | 缺失文档 |
| `src/cli/nodes-screen.ts` | （未建） | 缺失文档 |
| `src/cli/outbound-send-deps.ts` | （未建） | 缺失文档 |
| `src/cli/pairing-cli.ts` | （未建） | 缺失文档 |
| `src/cli/parse-bytes.ts` | （未建） | 缺失文档 |
| `src/cli/parse-duration.ts` | （未建） | 缺失文档 |
| `src/cli/parse-timeout.ts` | （未建） | 缺失文档 |
| `src/cli/plugin-registry.ts` | （未建） | 缺失文档 |
| `src/cli/plugins-cli.ts` | （未建） | 缺失文档 |
| `src/cli/ports.ts` | （未建） | 缺失文档 |
| `src/cli/profile-utils.ts` | （未建） | 缺失文档 |
| `src/cli/profile.ts` | （未建） | 缺失文档 |
| `src/cli/program.ts` | （未建） | 缺失文档 |
| `src/cli/program/build-program.ts` | （未建） | 缺失文档 |
| `src/cli/program/command-registry.ts` | （未建） | 缺失文档 |
| `src/cli/program/config-guard.ts` | （未建） | 缺失文档 |
| `src/cli/program/context.ts` | （未建） | 缺失文档 |
| `src/cli/program/help.ts` | （未建） | 缺失文档 |
| `src/cli/program/helpers.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/helpers.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.broadcast.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.discord-admin.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.emoji-sticker.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.permissions-search.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.pins.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.poll.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.reactions.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.read-edit-delete.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.send.ts` | （未建） | 缺失文档 |
| `src/cli/program/message/register.thread.ts` | （未建） | 缺失文档 |
| `src/cli/program/preaction.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.agent.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.configure.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.maintenance.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.message.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.onboard.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.setup.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.status-health-sessions.ts` | （未建） | 缺失文档 |
| `src/cli/program/register.subclis.ts` | （未建） | 缺失文档 |
| `src/cli/progress.ts` | （未建） | 缺失文档 |
| `src/cli/prompt.ts` | （未建） | 缺失文档 |
| `src/cli/route.ts` | （未建） | 缺失文档 |
| `src/cli/run-main.ts` | （未建） | 缺失文档 |
| `src/cli/sandbox-cli.ts` | （未建） | 缺失文档 |
| `src/cli/security-cli.ts` | （未建） | 缺失文档 |
| `src/cli/skills-cli.ts` | （未建） | 缺失文档 |
| `src/cli/system-cli.ts` | （未建） | 缺失文档 |
| `src/cli/tagline.ts` | （未建） | 缺失文档 |
| `src/cli/tui-cli.ts` | （未建） | 缺失文档 |
| `src/cli/update-cli.ts` | （未建） | 缺失文档 |
| `src/cli/wait.ts` | （未建） | 缺失文档 |
| `src/cli/webhooks-cli.ts` | （未建） | 缺失文档 |
| `src/commands/agent-via-gateway.ts` | （未建） | 缺失文档 |
| `src/commands/agent.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/commands/agent.ts.md` | 行为、约束 |
| `src/commands/agent/delivery.ts` | （未建） | 缺失文档 |
| `src/commands/agent/run-context.ts` | （未建） | 缺失文档 |
| `src/commands/agent/session-store.ts` | （未建） | 缺失文档 |
| `src/commands/agent/session.ts` | （未建） | 缺失文档 |
| `src/commands/agent/types.ts` | （未建） | 缺失文档 |
| `src/commands/agents.bindings.ts` | （未建） | 缺失文档 |
| `src/commands/agents.command-shared.ts` | （未建） | 缺失文档 |
| `src/commands/agents.commands.add.ts` | （未建） | 缺失文档 |
| `src/commands/agents.commands.delete.ts` | （未建） | 缺失文档 |
| `src/commands/agents.commands.identity.ts` | （未建） | 缺失文档 |
| `src/commands/agents.commands.list.ts` | （未建） | 缺失文档 |
| `src/commands/agents.config.ts` | （未建） | 缺失文档 |
| `src/commands/agents.providers.ts` | （未建） | 缺失文档 |
| `src/commands/agents.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice-options.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice-prompt.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.api-key.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.anthropic.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.api-providers.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.copilot-proxy.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.github-copilot.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.google-antigravity.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.google-gemini-cli.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.minimax.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.oauth.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.openai.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.plugin-provider.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.qwen-portal.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.apply.xai.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.default-model.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.model-check.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.preferred-provider.ts` | （未建） | 缺失文档 |
| `src/commands/auth-choice.ts` | （未建） | 缺失文档 |
| `src/commands/auth-token.ts` | （未建） | 缺失文档 |
| `src/commands/channels.ts` | （未建） | 缺失文档 |
| `src/commands/channels/add-mutators.ts` | （未建） | 缺失文档 |
| `src/commands/channels/add.ts` | （未建） | 缺失文档 |
| `src/commands/channels/capabilities.ts` | （未建） | 缺失文档 |
| `src/commands/channels/list.ts` | （未建） | 缺失文档 |
| `src/commands/channels/logs.ts` | （未建） | 缺失文档 |
| `src/commands/channels/remove.ts` | （未建） | 缺失文档 |
| `src/commands/channels/resolve.ts` | （未建） | 缺失文档 |
| `src/commands/channels/shared.ts` | （未建） | 缺失文档 |
| `src/commands/channels/status.ts` | （未建） | 缺失文档 |
| `src/commands/chutes-oauth.ts` | （未建） | 缺失文档 |
| `src/commands/cleanup-utils.ts` | （未建） | 缺失文档 |
| `src/commands/configure.channels.ts` | （未建） | 缺失文档 |
| `src/commands/configure.commands.ts` | （未建） | 缺失文档 |
| `src/commands/configure.daemon.ts` | （未建） | 缺失文档 |
| `src/commands/configure.gateway-auth.ts` | （未建） | 缺失文档 |
| `src/commands/configure.gateway.ts` | （未建） | 缺失文档 |
| `src/commands/configure.shared.ts` | （未建） | 缺失文档 |
| `src/commands/configure.ts` | （未建） | 缺失文档 |
| `src/commands/configure.wizard.ts` | （未建） | 缺失文档 |
| `src/commands/daemon-install-helpers.ts` | （未建） | 缺失文档 |
| `src/commands/daemon-runtime.ts` | （未建） | 缺失文档 |
| `src/commands/dashboard.ts` | （未建） | 缺失文档 |
| `src/commands/docs.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-auth.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-completion.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-config-flow.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-format.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-gateway-daemon-flow.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-gateway-health.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-gateway-services.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-install.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-legacy-config.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-platform-notes.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-prompter.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-sandbox.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-security.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-state-integrity.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-state-migrations.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-ui.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-update.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-workspace-status.ts` | （未建） | 缺失文档 |
| `src/commands/doctor-workspace.ts` | （未建） | 缺失文档 |
| `src/commands/doctor.ts` | （未建） | 缺失文档 |
| `src/commands/gateway-status.ts` | （未建） | 缺失文档 |
| `src/commands/gateway-status/helpers.ts` | （未建） | 缺失文档 |
| `src/commands/google-gemini-model-default.ts` | （未建） | 缺失文档 |
| `src/commands/health-format.ts` | （未建） | 缺失文档 |
| `src/commands/health.ts` | （未建） | 缺失文档 |
| `src/commands/message-format.ts` | （未建） | 缺失文档 |
| `src/commands/message.ts` | （未建） | 缺失文档 |
| `src/commands/model-allowlist.ts` | （未建） | 缺失文档 |
| `src/commands/model-picker.ts` | （未建） | 缺失文档 |
| `src/commands/models.ts` | （未建） | 缺失文档 |
| `src/commands/models/aliases.ts` | （未建） | 缺失文档 |
| `src/commands/models/auth-order.ts` | （未建） | 缺失文档 |
| `src/commands/models/auth.ts` | （未建） | 缺失文档 |
| `src/commands/models/fallbacks.ts` | （未建） | 缺失文档 |
| `src/commands/models/image-fallbacks.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.auth-overview.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.configured.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.format.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.list-command.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.probe.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.registry.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.status-command.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.table.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.ts` | （未建） | 缺失文档 |
| `src/commands/models/list.types.ts` | （未建） | 缺失文档 |
| `src/commands/models/scan.ts` | （未建） | 缺失文档 |
| `src/commands/models/set-image.ts` | （未建） | 缺失文档 |
| `src/commands/models/set.ts` | （未建） | 缺失文档 |
| `src/commands/models/shared.ts` | （未建） | 缺失文档 |
| `src/commands/node-daemon-install-helpers.ts` | （未建） | 缺失文档 |
| `src/commands/node-daemon-runtime.ts` | （未建） | 缺失文档 |
| `src/commands/oauth-env.ts` | （未建） | 缺失文档 |
| `src/commands/oauth-flow.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.config-core.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.config-minimax.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.config-opencode.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.credentials.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.models.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-auth.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-channels.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-custom.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-helpers.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-hooks.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-interactive.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/api-keys.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/auth-choice-inference.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/auth-choice.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/daemon-install.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/gateway-config.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/output.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/skills-config.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/local/workspace.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-non-interactive/remote.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-remote.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-skills.ts` | （未建） | 缺失文档 |
| `src/commands/onboard-types.ts` | （未建） | 缺失文档 |
| `src/commands/onboard.ts` | （未建） | 缺失文档 |
| `src/commands/onboarding/plugin-install.ts` | （未建） | 缺失文档 |
| `src/commands/onboarding/registry.ts` | （未建） | 缺失文档 |
| `src/commands/onboarding/types.ts` | （未建） | 缺失文档 |
| `src/commands/openai-codex-model-default.ts` | （未建） | 缺失文档 |
| `src/commands/openai-model-default.ts` | （未建） | 缺失文档 |
| `src/commands/opencode-zen-model-default.ts` | （未建） | 缺失文档 |
| `src/commands/reset.ts` | （未建） | 缺失文档 |
| `src/commands/sandbox-display.ts` | （未建） | 缺失文档 |
| `src/commands/sandbox-explain.ts` | （未建） | 缺失文档 |
| `src/commands/sandbox-formatters.ts` | （未建） | 缺失文档 |
| `src/commands/sandbox.ts` | （未建） | 缺失文档 |
| `src/commands/sessions.ts` | （未建） | 缺失文档 |
| `src/commands/setup.ts` | （未建） | 缺失文档 |
| `src/commands/signal-install.ts` | （未建） | 缺失文档 |
| `src/commands/status-all.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/agents.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/channels.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/diagnosis.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/format.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/gateway.ts` | （未建） | 缺失文档 |
| `src/commands/status-all/report-lines.ts` | （未建） | 缺失文档 |
| `src/commands/status.agent-local.ts` | （未建） | 缺失文档 |
| `src/commands/status.command.ts` | （未建） | 缺失文档 |
| `src/commands/status.daemon.ts` | （未建） | 缺失文档 |
| `src/commands/status.format.ts` | （未建） | 缺失文档 |
| `src/commands/status.gateway-probe.ts` | （未建） | 缺失文档 |
| `src/commands/status.link-channel.ts` | （未建） | 缺失文档 |
| `src/commands/status.scan.ts` | （未建） | 缺失文档 |
| `src/commands/status.summary.ts` | （未建） | 缺失文档 |
| `src/commands/status.ts` | （未建） | 缺失文档 |
| `src/commands/status.types.ts` | （未建） | 缺失文档 |
| `src/commands/status.update.ts` | （未建） | 缺失文档 |
| `src/commands/systemd-linger.ts` | （未建） | 缺失文档 |
| `src/commands/uninstall.ts` | （未建） | 缺失文档 |
| `src/compat/legacy-names.ts` | （未建） | 缺失文档 |
| `src/config/agent-dirs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/agent-dirs.ts.md` | 行为、约束 |
| `src/config/agent-limits.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/agent-limits.ts.md` | 行为、约束 |
| `src/config/cache-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/cache-utils.ts.md` | 行为、约束 |
| `src/config/channel-capabilities.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/channel-capabilities.ts.md` | 行为、约束 |
| `src/config/commands.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/commands.ts.md` | 行为、约束 |
| `src/config/config-paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/config-paths.ts.md` | 行为、约束 |
| `src/config/config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/config.ts.md` | 行为、约束 |
| `src/config/defaults.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/defaults.ts.md` | 行为、约束 |
| `src/config/env-substitution.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/env-substitution.ts.md` | 行为、约束 |
| `src/config/env-vars.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/env-vars.ts.md` | 行为、约束 |
| `src/config/group-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/group-policy.ts.md` | 行为、约束 |
| `src/config/includes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/includes.ts.md` | 行为、约束 |
| `src/config/io.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/io.ts.md` | 行为、约束 |
| `src/config/legacy-migrate.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy-migrate.ts.md` | 行为、约束 |
| `src/config/legacy.migrations.part-1.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.migrations.part-1.ts.md` | 行为、约束 |
| `src/config/legacy.migrations.part-2.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.migrations.part-2.ts.md` | 行为、约束 |
| `src/config/legacy.migrations.part-3.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.migrations.part-3.ts.md` | 行为、约束 |
| `src/config/legacy.migrations.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.migrations.ts.md` | 行为、约束 |
| `src/config/legacy.rules.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.rules.ts.md` | 行为、约束 |
| `src/config/legacy.shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.shared.ts.md` | 行为、约束 |
| `src/config/legacy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/legacy.ts.md` | 行为、约束 |
| `src/config/logging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/logging.ts.md` | 行为、约束 |
| `src/config/markdown-tables.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/markdown-tables.ts.md` | 行为、约束 |
| `src/config/merge-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/merge-config.ts.md` | 行为、约束 |
| `src/config/merge-patch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/merge-patch.ts.md` | 行为、约束 |
| `src/config/normalize-paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/normalize-paths.ts.md` | 行为、约束 |
| `src/config/paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/paths.ts.md` | 行为、约束 |
| `src/config/plugin-auto-enable.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/plugin-auto-enable.ts.md` | 行为、约束 |
| `src/config/port-defaults.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/port-defaults.ts.md` | 行为、约束 |
| `src/config/redact-snapshot.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/redact-snapshot.ts.md` | 行为、约束 |
| `src/config/runtime-overrides.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/runtime-overrides.ts.md` | 行为、约束 |
| `src/config/schema.field-metadata.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/schema.field-metadata.ts.md` | 行为、约束 |
| `src/config/schema.hints.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/schema.hints.ts.md` | 行为、约束 |
| `src/config/schema.irc.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/schema.irc.ts.md` | 行为、约束 |
| `src/config/schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/schema.ts.md` | 行为、约束 |
| `src/config/sessions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions.ts.md` | 行为、约束 |
| `src/config/sessions/group.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/group.ts.md` | 行为、约束 |
| `src/config/sessions/main-session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/main-session.ts.md` | 行为、约束 |
| `src/config/sessions/metadata.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/metadata.ts.md` | 行为、约束 |
| `src/config/sessions/paths.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/paths.ts.md` | 行为、约束 |
| `src/config/sessions/reset.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/reset.ts.md` | 行为、约束 |
| `src/config/sessions/session-key.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/session-key.ts.md` | 行为、约束 |
| `src/config/sessions/store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/store.ts.md` | 行为、约束 |
| `src/config/sessions/transcript.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/transcript.ts.md` | 行为、约束 |
| `src/config/sessions/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/sessions/types.ts.md` | 行为、约束 |
| `src/config/talk.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/talk.ts.md` | 行为、约束 |
| `src/config/telegram-custom-commands.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/telegram-custom-commands.ts.md` | 行为、约束 |
| `src/config/test-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/test-helpers.ts.md` | 行为、约束 |
| `src/config/types.agent-defaults.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.agent-defaults.ts.md` | 行为、约束 |
| `src/config/types.agents.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.agents.ts.md` | 行为、约束 |
| `src/config/types.approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.approvals.ts.md` | 行为、约束 |
| `src/config/types.auth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.auth.ts.md` | 行为、约束 |
| `src/config/types.base.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.base.ts.md` | 行为、约束 |
| `src/config/types.browser.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.browser.ts.md` | 行为、约束 |
| `src/config/types.channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.channels.ts.md` | 行为、约束 |
| `src/config/types.cron.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.cron.ts.md` | 行为、约束 |
| `src/config/types.discord.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.discord.ts.md` | 行为、约束 |
| `src/config/types.gateway.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.gateway.ts.md` | 行为、约束 |
| `src/config/types.googlechat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.googlechat.ts.md` | 行为、约束 |
| `src/config/types.hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.hooks.ts.md` | 行为、约束 |
| `src/config/types.imessage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.imessage.ts.md` | 行为、约束 |
| `src/config/types.irc.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.irc.ts.md` | 行为、约束 |
| `src/config/types.memory.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.memory.ts.md` | 行为、约束 |
| `src/config/types.messages.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.messages.ts.md` | 行为、约束 |
| `src/config/types.models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.models.ts.md` | 行为、约束 |
| `src/config/types.msteams.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.msteams.ts.md` | 行为、约束 |
| `src/config/types.node-host.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.node-host.ts.md` | 行为、约束 |
| `src/config/types.openclaw.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.openclaw.ts.md` | 行为、约束 |
| `src/config/types.plugins.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.plugins.ts.md` | 行为、约束 |
| `src/config/types.queue.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.queue.ts.md` | 行为、约束 |
| `src/config/types.sandbox.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.sandbox.ts.md` | 行为、约束 |
| `src/config/types.signal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.signal.ts.md` | 行为、约束 |
| `src/config/types.skills.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.skills.ts.md` | 行为、约束 |
| `src/config/types.slack.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.slack.ts.md` | 行为、约束 |
| `src/config/types.telegram.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.telegram.ts.md` | 行为、约束 |
| `src/config/types.tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.tools.ts.md` | 行为、约束 |
| `src/config/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.ts.md` | 行为、约束 |
| `src/config/types.tts.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.tts.ts.md` | 行为、约束 |
| `src/config/types.whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/types.whatsapp.ts.md` | 行为、约束 |
| `src/config/validation.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/validation.ts.md` | 行为、约束 |
| `src/config/version.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/version.ts.md` | 行为、约束 |
| `src/config/zod-schema.agent-defaults.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.agent-defaults.ts.md` | 行为、约束 |
| `src/config/zod-schema.agent-runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.agent-runtime.ts.md` | 行为、约束 |
| `src/config/zod-schema.agents.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.agents.ts.md` | 行为、约束 |
| `src/config/zod-schema.approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.approvals.ts.md` | 行为、约束 |
| `src/config/zod-schema.channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.channels.ts.md` | 行为、约束 |
| `src/config/zod-schema.core.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.core.ts.md` | 行为、约束 |
| `src/config/zod-schema.hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.hooks.ts.md` | 行为、约束 |
| `src/config/zod-schema.providers-core.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.providers-core.ts.md` | 行为、约束 |
| `src/config/zod-schema.providers-whatsapp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.providers-whatsapp.ts.md` | 行为、约束 |
| `src/config/zod-schema.providers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.providers.ts.md` | 行为、约束 |
| `src/config/zod-schema.session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.session.ts.md` | 行为、约束 |
| `src/config/zod-schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/config/zod-schema.ts.md` | 行为、约束 |
| `src/cron/delivery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/delivery.ts.md` | 行为、约束 |
| `src/cron/isolated-agent.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/isolated-agent.ts.md` | 行为、约束 |
| `src/cron/isolated-agent/delivery-target.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/isolated-agent/delivery-target.ts.md` | 行为、约束 |
| `src/cron/isolated-agent/helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/isolated-agent/helpers.ts.md` | 行为、约束 |
| `src/cron/isolated-agent/run.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/isolated-agent/run.ts.md` | 行为、约束 |
| `src/cron/isolated-agent/session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/isolated-agent/session.ts.md` | 行为、约束 |
| `src/cron/normalize.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/normalize.ts.md` | 行为、约束 |
| `src/cron/parse.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/parse.ts.md` | 行为、约束 |
| `src/cron/payload-migration.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/payload-migration.ts.md` | 行为、约束 |
| `src/cron/run-log.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/run-log.ts.md` | 行为、约束 |
| `src/cron/schedule.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/schedule.ts.md` | 行为、约束 |
| `src/cron/service.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service.ts.md` | 行为、约束 |
| `src/cron/service/jobs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/jobs.ts.md` | 行为、约束 |
| `src/cron/service/locked.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/locked.ts.md` | 行为、约束 |
| `src/cron/service/normalize.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/normalize.ts.md` | 行为、约束 |
| `src/cron/service/ops.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/ops.ts.md` | 行为、约束 |
| `src/cron/service/state.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/state.ts.md` | 行为、约束 |
| `src/cron/service/store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/store.ts.md` | 行为、约束 |
| `src/cron/service/timer.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/service/timer.ts.md` | 行为、约束 |
| `src/cron/session-reaper.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/session-reaper.ts.md` | 行为、约束 |
| `src/cron/store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/store.ts.md` | 行为、约束 |
| `src/cron/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/types.ts.md` | 行为、约束 |
| `src/cron/validate-timestamp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/cron/validate-timestamp.ts.md` | 行为、约束 |
| `src/daemon/constants.ts` | （未建） | 缺失文档 |
| `src/daemon/diagnostics.ts` | （未建） | 缺失文档 |
| `src/daemon/inspect.ts` | （未建） | 缺失文档 |
| `src/daemon/launchd-plist.ts` | （未建） | 缺失文档 |
| `src/daemon/launchd.ts` | （未建） | 缺失文档 |
| `src/daemon/node-service.ts` | （未建） | 缺失文档 |
| `src/daemon/paths.ts` | （未建） | 缺失文档 |
| `src/daemon/program-args.ts` | （未建） | 缺失文档 |
| `src/daemon/runtime-parse.ts` | （未建） | 缺失文档 |
| `src/daemon/runtime-paths.ts` | （未建） | 缺失文档 |
| `src/daemon/schtasks.ts` | （未建） | 缺失文档 |
| `src/daemon/service-audit.ts` | （未建） | 缺失文档 |
| `src/daemon/service-env.ts` | （未建） | 缺失文档 |
| `src/daemon/service-runtime.ts` | （未建） | 缺失文档 |
| `src/daemon/service.ts` | （未建） | 缺失文档 |
| `src/daemon/systemd-hints.ts` | （未建） | 缺失文档 |
| `src/daemon/systemd-linger.ts` | （未建） | 缺失文档 |
| `src/daemon/systemd-unit.ts` | （未建） | 缺失文档 |
| `src/daemon/systemd.ts` | （未建） | 缺失文档 |
| `src/discord/accounts.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/accounts.ts.md` | 行为、约束 |
| `src/discord/api.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/api.ts.md` | 行为、约束 |
| `src/discord/audit.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/audit.ts.md` | 行为、约束 |
| `src/discord/chunk.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/chunk.ts.md` | 行为、约束 |
| `src/discord/directory-live.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/directory-live.ts.md` | 行为、约束 |
| `src/discord/gateway-logging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/gateway-logging.ts.md` | 行为、约束 |
| `src/discord/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/index.ts.md` | 行为、约束 |
| `src/discord/monitor.gateway.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor.gateway.ts.md` | 行为、约束 |
| `src/discord/monitor.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor.ts.md` | 行为、约束 |
| `src/discord/monitor/agent-components.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/agent-components.ts.md` | 行为、约束 |
| `src/discord/monitor/allow-list.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/allow-list.ts.md` | 行为、约束 |
| `src/discord/monitor/exec-approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/exec-approvals.ts.md` | 行为、约束 |
| `src/discord/monitor/format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/format.ts.md` | 行为、约束 |
| `src/discord/monitor/gateway-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/gateway-registry.ts.md` | 行为、约束 |
| `src/discord/monitor/listeners.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/listeners.ts.md` | 行为、约束 |
| `src/discord/monitor/message-handler.preflight.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/message-handler.preflight.ts.md` | 行为、约束 |
| `src/discord/monitor/message-handler.preflight.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/message-handler.preflight.types.ts.md` | 行为、约束 |
| `src/discord/monitor/message-handler.process.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/message-handler.process.ts.md` | 行为、约束 |
| `src/discord/monitor/message-handler.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/message-handler.ts.md` | 行为、约束 |
| `src/discord/monitor/message-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/message-utils.ts.md` | 行为、约束 |
| `src/discord/monitor/native-command.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/native-command.ts.md` | 行为、约束 |
| `src/discord/monitor/presence-cache.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/presence-cache.ts.md` | 行为、约束 |
| `src/discord/monitor/provider.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/provider.ts.md` | 行为、约束 |
| `src/discord/monitor/reply-context.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/reply-context.ts.md` | 行为、约束 |
| `src/discord/monitor/reply-delivery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/reply-delivery.ts.md` | 行为、约束 |
| `src/discord/monitor/sender-identity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/sender-identity.ts.md` | 行为、约束 |
| `src/discord/monitor/system-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/system-events.ts.md` | 行为、约束 |
| `src/discord/monitor/threading.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/threading.ts.md` | 行为、约束 |
| `src/discord/monitor/typing.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/monitor/typing.ts.md` | 行为、约束 |
| `src/discord/pluralkit.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/pluralkit.ts.md` | 行为、约束 |
| `src/discord/probe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/probe.ts.md` | 行为、约束 |
| `src/discord/resolve-channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/resolve-channels.ts.md` | 行为、约束 |
| `src/discord/resolve-users.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/resolve-users.ts.md` | 行为、约束 |
| `src/discord/send.channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.channels.ts.md` | 行为、约束 |
| `src/discord/send.emojis-stickers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.emojis-stickers.ts.md` | 行为、约束 |
| `src/discord/send.guild.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.guild.ts.md` | 行为、约束 |
| `src/discord/send.messages.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.messages.ts.md` | 行为、约束 |
| `src/discord/send.outbound.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.outbound.ts.md` | 行为、约束 |
| `src/discord/send.permissions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.permissions.ts.md` | 行为、约束 |
| `src/discord/send.reactions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.reactions.ts.md` | 行为、约束 |
| `src/discord/send.shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.shared.ts.md` | 行为、约束 |
| `src/discord/send.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.ts.md` | 行为、约束 |
| `src/discord/send.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/send.types.ts.md` | 行为、约束 |
| `src/discord/targets.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/targets.ts.md` | 行为、约束 |
| `src/discord/token.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/discord/token.ts.md` | 行为、约束 |
| `src/entry.ts` | （未建） | 缺失文档 |
| `src/extensionAPI.ts` | （未建） | 缺失文档 |
| `src/gateway/assistant-identity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/assistant-identity.ts.md` | 行为、约束 |
| `src/gateway/auth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/auth.ts.md` | 行为、约束 |
| `src/gateway/boot.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/boot.ts.md` | 行为、约束 |
| `src/gateway/call.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/call.ts.md` | 行为、约束 |
| `src/gateway/chat-abort.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/chat-abort.ts.md` | 行为、约束 |
| `src/gateway/chat-attachments.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/chat-attachments.ts.md` | 行为、约束 |
| `src/gateway/chat-sanitize.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/chat-sanitize.ts.md` | 行为、约束 |
| `src/gateway/client.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/client.ts.md` | 行为、约束 |
| `src/gateway/config-reload.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/config-reload.ts.md` | 行为、约束 |
| `src/gateway/control-ui-shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/control-ui-shared.ts.md` | 行为、约束 |
| `src/gateway/control-ui.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/control-ui.ts.md` | 行为、约束 |
| `src/gateway/device-auth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/device-auth.ts.md` | 行为、约束 |
| `src/gateway/exec-approval-manager.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/exec-approval-manager.ts.md` | 行为、约束 |
| `src/gateway/hooks-mapping.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/hooks-mapping.ts.md` | 行为、约束 |
| `src/gateway/hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/hooks.ts.md` | 行为、约束 |
| `src/gateway/http-common.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/http-common.ts.md` | 行为、约束 |
| `src/gateway/http-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/http-utils.ts.md` | 行为、约束 |
| `src/gateway/live-image-probe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/live-image-probe.ts.md` | 行为、约束 |
| `src/gateway/net.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/net.ts.md` | 行为、约束 |
| `src/gateway/node-command-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/node-command-policy.ts.md` | 行为、约束 |
| `src/gateway/node-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/node-registry.ts.md` | 行为、约束 |
| `src/gateway/open-responses.schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/open-responses.schema.ts.md` | 行为、约束 |
| `src/gateway/openai-http.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/openai-http.ts.md` | 行为、约束 |
| `src/gateway/openresponses-http.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/openresponses-http.ts.md` | 行为、约束 |
| `src/gateway/origin-check.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/origin-check.ts.md` | 行为、约束 |
| `src/gateway/probe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/probe.ts.md` | 行为、约束 |
| `src/gateway/protocol/client-info.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/client-info.ts.md` | 行为、约束 |
| `src/gateway/protocol/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/index.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/agent.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/agent.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/agents-models-skills.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/agents-models-skills.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/channels.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/config.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/cron.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/cron.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/devices.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/devices.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/error-codes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/error-codes.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/exec-approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/exec-approvals.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/frames.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/frames.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/logs-chat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/logs-chat.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/nodes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/nodes.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/primitives.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/primitives.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/protocol-schemas.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/protocol-schemas.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/sessions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/sessions.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/snapshot.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/snapshot.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/types.ts.md` | 行为、约束 |
| `src/gateway/protocol/schema/wizard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/protocol/schema/wizard.ts.md` | 行为、约束 |
| `src/gateway/server-broadcast.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-broadcast.ts.md` | 行为、约束 |
| `src/gateway/server-browser.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-browser.ts.md` | 行为、约束 |
| `src/gateway/server-channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-channels.ts.md` | 行为、约束 |
| `src/gateway/server-chat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-chat.ts.md` | 行为、约束 |
| `src/gateway/server-close.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-close.ts.md` | 行为、约束 |
| `src/gateway/server-constants.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-constants.ts.md` | 行为、约束 |
| `src/gateway/server-cron.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-cron.ts.md` | 行为、约束 |
| `src/gateway/server-discovery-runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-discovery-runtime.ts.md` | 行为、约束 |
| `src/gateway/server-discovery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-discovery.ts.md` | 行为、约束 |
| `src/gateway/server-http.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-http.ts.md` | 行为、约束 |
| `src/gateway/server-lanes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-lanes.ts.md` | 行为、约束 |
| `src/gateway/server-maintenance.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-maintenance.ts.md` | 行为、约束 |
| `src/gateway/server-methods-list.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods-list.ts.md` | 行为、约束 |
| `src/gateway/server-methods.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods.ts.md` | 行为、约束 |
| `src/gateway/server-methods/agent-job.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/agent-job.ts.md` | 行为、约束 |
| `src/gateway/server-methods/agent-timestamp.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/agent-timestamp.ts.md` | 行为、约束 |
| `src/gateway/server-methods/agent.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/agent.ts.md` | 行为、约束 |
| `src/gateway/server-methods/agents.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/agents.ts.md` | 行为、约束 |
| `src/gateway/server-methods/browser.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/browser.ts.md` | 行为、约束 |
| `src/gateway/server-methods/channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/channels.ts.md` | 行为、约束 |
| `src/gateway/server-methods/chat.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/chat.ts.md` | 行为、约束 |
| `src/gateway/server-methods/config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/config.ts.md` | 行为、约束 |
| `src/gateway/server-methods/connect.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/connect.ts.md` | 行为、约束 |
| `src/gateway/server-methods/cron.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/cron.ts.md` | 行为、约束 |
| `src/gateway/server-methods/devices.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/devices.ts.md` | 行为、约束 |
| `src/gateway/server-methods/exec-approval.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/exec-approval.ts.md` | 行为、约束 |
| `src/gateway/server-methods/exec-approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/exec-approvals.ts.md` | 行为、约束 |
| `src/gateway/server-methods/health.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/health.ts.md` | 行为、约束 |
| `src/gateway/server-methods/logs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/logs.ts.md` | 行为、约束 |
| `src/gateway/server-methods/models.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/models.ts.md` | 行为、约束 |
| `src/gateway/server-methods/nodes.helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/nodes.helpers.ts.md` | 行为、约束 |
| `src/gateway/server-methods/nodes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/nodes.ts.md` | 行为、约束 |
| `src/gateway/server-methods/send.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/send.ts.md` | 行为、约束 |
| `src/gateway/server-methods/sessions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/sessions.ts.md` | 行为、约束 |
| `src/gateway/server-methods/skills.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/skills.ts.md` | 行为、约束 |
| `src/gateway/server-methods/system.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/system.ts.md` | 行为、约束 |
| `src/gateway/server-methods/talk.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/talk.ts.md` | 行为、约束 |
| `src/gateway/server-methods/tts.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/tts.ts.md` | 行为、约束 |
| `src/gateway/server-methods/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/types.ts.md` | 行为、约束 |
| `src/gateway/server-methods/update.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/update.ts.md` | 行为、约束 |
| `src/gateway/server-methods/usage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/usage.ts.md` | 行为、约束 |
| `src/gateway/server-methods/voicewake.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/voicewake.ts.md` | 行为、约束 |
| `src/gateway/server-methods/web.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/web.ts.md` | 行为、约束 |
| `src/gateway/server-methods/wizard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-methods/wizard.ts.md` | 行为、约束 |
| `src/gateway/server-mobile-nodes.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-mobile-nodes.ts.md` | 行为、约束 |
| `src/gateway/server-model-catalog.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-model-catalog.ts.md` | 行为、约束 |
| `src/gateway/server-node-events-types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-node-events-types.ts.md` | 行为、约束 |
| `src/gateway/server-node-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-node-events.ts.md` | 行为、约束 |
| `src/gateway/server-node-subscriptions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-node-subscriptions.ts.md` | 行为、约束 |
| `src/gateway/server-plugins.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-plugins.ts.md` | 行为、约束 |
| `src/gateway/server-reload-handlers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-reload-handlers.ts.md` | 行为、约束 |
| `src/gateway/server-restart-sentinel.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-restart-sentinel.ts.md` | 行为、约束 |
| `src/gateway/server-runtime-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-runtime-config.ts.md` | 行为、约束 |
| `src/gateway/server-runtime-state.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-runtime-state.ts.md` | 行为、约束 |
| `src/gateway/server-session-key.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-session-key.ts.md` | 行为、约束 |
| `src/gateway/server-shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-shared.ts.md` | 行为、约束 |
| `src/gateway/server-startup-log.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-startup-log.ts.md` | 行为、约束 |
| `src/gateway/server-startup-memory.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-startup-memory.ts.md` | 行为、约束 |
| `src/gateway/server-startup.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-startup.ts.md` | 行为、约束 |
| `src/gateway/server-tailscale.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-tailscale.ts.md` | 行为、约束 |
| `src/gateway/server-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-utils.ts.md` | 行为、约束 |
| `src/gateway/server-wizard-sessions.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-wizard-sessions.ts.md` | 行为、约束 |
| `src/gateway/server-ws-runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server-ws-runtime.ts.md` | 行为、约束 |
| `src/gateway/server.impl.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server.impl.ts.md` | 行为、约束 |
| `src/gateway/server.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server.ts.md` | 行为、约束 |
| `src/gateway/server/close-reason.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/close-reason.ts.md` | 行为、约束 |
| `src/gateway/server/health-state.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/health-state.ts.md` | 行为、约束 |
| `src/gateway/server/hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/hooks.ts.md` | 行为、约束 |
| `src/gateway/server/http-listen.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/http-listen.ts.md` | 行为、约束 |
| `src/gateway/server/plugins-http.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/plugins-http.ts.md` | 行为、约束 |
| `src/gateway/server/tls.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/tls.ts.md` | 行为、约束 |
| `src/gateway/server/ws-connection.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/ws-connection.ts.md` | 行为、约束 |
| `src/gateway/server/ws-connection/message-handler.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/ws-connection/message-handler.ts.md` | 行为、约束 |
| `src/gateway/server/ws-types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/server/ws-types.ts.md` | 行为、约束 |
| `src/gateway/session-utils.fs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/session-utils.fs.ts.md` | 行为、约束 |
| `src/gateway/session-utils.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/session-utils.ts.md` | 行为、约束 |
| `src/gateway/session-utils.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/session-utils.types.ts.md` | 行为、约束 |
| `src/gateway/sessions-patch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/sessions-patch.ts.md` | 行为、约束 |
| `src/gateway/sessions-resolve.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/sessions-resolve.ts.md` | 行为、约束 |
| `src/gateway/test-helpers.e2e.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/test-helpers.e2e.ts.md` | 行为、约束 |
| `src/gateway/test-helpers.mocks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/test-helpers.mocks.ts.md` | 行为、约束 |
| `src/gateway/test-helpers.openai-mock.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/test-helpers.openai-mock.ts.md` | 行为、约束 |
| `src/gateway/test-helpers.server.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/test-helpers.server.ts.md` | 行为、约束 |
| `src/gateway/test-helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/test-helpers.ts.md` | 行为、约束 |
| `src/gateway/tools-invoke-http.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/tools-invoke-http.ts.md` | 行为、约束 |
| `src/gateway/ws-log.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/ws-log.ts.md` | 行为、约束 |
| `src/gateway/ws-logging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/gateway/ws-logging.ts.md` | 行为、约束 |
| `src/globals.ts` | （未建） | 缺失文档 |
| `src/hooks/bundled-dir.ts` | （未建） | 缺失文档 |
| `src/hooks/bundled/boot-md/handler.ts` | （未建） | 缺失文档 |
| `src/hooks/bundled/command-logger/handler.ts` | （未建） | 缺失文档 |
| `src/hooks/bundled/session-memory/handler.ts` | （未建） | 缺失文档 |
| `src/hooks/bundled/soul-evil/handler.ts` | （未建） | 缺失文档 |
| `src/hooks/config.ts` | （未建） | 缺失文档 |
| `src/hooks/frontmatter.ts` | （未建） | 缺失文档 |
| `src/hooks/gmail-ops.ts` | （未建） | 缺失文档 |
| `src/hooks/gmail-setup-utils.ts` | （未建） | 缺失文档 |
| `src/hooks/gmail-watcher.ts` | （未建） | 缺失文档 |
| `src/hooks/gmail.ts` | （未建） | 缺失文档 |
| `src/hooks/hooks-status.ts` | （未建） | 缺失文档 |
| `src/hooks/hooks.ts` | （未建） | 缺失文档 |
| `src/hooks/install.ts` | （未建） | 缺失文档 |
| `src/hooks/installs.ts` | （未建） | 缺失文档 |
| `src/hooks/internal-hooks.ts` | （未建） | 缺失文档 |
| `src/hooks/llm-slug-generator.ts` | （未建） | 缺失文档 |
| `src/hooks/loader.ts` | （未建） | 缺失文档 |
| `src/hooks/plugin-hooks.ts` | （未建） | 缺失文档 |
| `src/hooks/soul-evil.ts` | （未建） | 缺失文档 |
| `src/hooks/types.ts` | （未建） | 缺失文档 |
| `src/hooks/workspace.ts` | （未建） | 缺失文档 |
| `src/imessage/accounts.ts` | （未建） | 缺失文档 |
| `src/imessage/client.ts` | （未建） | 缺失文档 |
| `src/imessage/constants.ts` | （未建） | 缺失文档 |
| `src/imessage/index.ts` | （未建） | 缺失文档 |
| `src/imessage/monitor.ts` | （未建） | 缺失文档 |
| `src/imessage/monitor/deliver.ts` | （未建） | 缺失文档 |
| `src/imessage/monitor/monitor-provider.ts` | （未建） | 缺失文档 |
| `src/imessage/monitor/runtime.ts` | （未建） | 缺失文档 |
| `src/imessage/monitor/types.ts` | （未建） | 缺失文档 |
| `src/imessage/probe.ts` | （未建） | 缺失文档 |
| `src/imessage/send.ts` | （未建） | 缺失文档 |
| `src/imessage/targets.ts` | （未建） | 缺失文档 |
| `src/index.ts` | （未建） | 缺失文档 |
| `src/infra/agent-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/agent-events.ts.md` | 行为、约束 |
| `src/infra/archive.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/archive.ts.md` | 行为、约束 |
| `src/infra/backoff.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/backoff.ts.md` | 行为、约束 |
| `src/infra/binaries.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/binaries.ts.md` | 行为、约束 |
| `src/infra/bonjour-ciao.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/bonjour-ciao.ts.md` | 行为、约束 |
| `src/infra/bonjour-discovery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/bonjour-discovery.ts.md` | 行为、约束 |
| `src/infra/bonjour-errors.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/bonjour-errors.ts.md` | 行为、约束 |
| `src/infra/bonjour.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/bonjour.ts.md` | 行为、约束 |
| `src/infra/brew.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/brew.ts.md` | 行为、约束 |
| `src/infra/canvas-host-url.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/canvas-host-url.ts.md` | 行为、约束 |
| `src/infra/channel-activity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/channel-activity.ts.md` | 行为、约束 |
| `src/infra/channel-summary.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/channel-summary.ts.md` | 行为、约束 |
| `src/infra/channels-status-issues.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/channels-status-issues.ts.md` | 行为、约束 |
| `src/infra/clipboard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/clipboard.ts.md` | 行为、约束 |
| `src/infra/control-ui-assets.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/control-ui-assets.ts.md` | 行为、约束 |
| `src/infra/dedupe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/dedupe.ts.md` | 行为、约束 |
| `src/infra/device-auth-store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/device-auth-store.ts.md` | 行为、约束 |
| `src/infra/device-identity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/device-identity.ts.md` | 行为、约束 |
| `src/infra/device-pairing.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/device-pairing.ts.md` | 行为、约束 |
| `src/infra/diagnostic-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/diagnostic-events.ts.md` | 行为、约束 |
| `src/infra/diagnostic-flags.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/diagnostic-flags.ts.md` | 行为、约束 |
| `src/infra/dotenv.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/dotenv.ts.md` | 行为、约束 |
| `src/infra/env-file.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/env-file.ts.md` | 行为、约束 |
| `src/infra/env.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/env.ts.md` | 行为、约束 |
| `src/infra/errors.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/errors.ts.md` | 行为、约束 |
| `src/infra/exec-approval-forwarder.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/exec-approval-forwarder.ts.md` | 行为、约束 |
| `src/infra/exec-approvals.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/exec-approvals.ts.md` | 行为、约束 |
| `src/infra/exec-host.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/exec-host.ts.md` | 行为、约束 |
| `src/infra/exec-safety.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/exec-safety.ts.md` | 行为、约束 |
| `src/infra/fetch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/fetch.ts.md` | 行为、约束 |
| `src/infra/format-time/format-datetime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/format-time/format-datetime.ts.md` | 行为、约束 |
| `src/infra/format-time/format-duration.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/format-time/format-duration.ts.md` | 行为、约束 |
| `src/infra/format-time/format-relative.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/format-time/format-relative.ts.md` | 行为、约束 |
| `src/infra/fs-safe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/fs-safe.ts.md` | 行为、约束 |
| `src/infra/gateway-lock.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/gateway-lock.ts.md` | 行为、约束 |
| `src/infra/git-commit.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/git-commit.ts.md` | 行为、约束 |
| `src/infra/heartbeat-active-hours.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/heartbeat-active-hours.ts.md` | 行为、约束 |
| `src/infra/heartbeat-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/heartbeat-events.ts.md` | 行为、约束 |
| `src/infra/heartbeat-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/heartbeat-runner.ts.md` | 行为、约束 |
| `src/infra/heartbeat-visibility.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/heartbeat-visibility.ts.md` | 行为、约束 |
| `src/infra/heartbeat-wake.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/heartbeat-wake.ts.md` | 行为、约束 |
| `src/infra/home-dir.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/home-dir.ts.md` | 行为、约束 |
| `src/infra/is-main.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/is-main.ts.md` | 行为、约束 |
| `src/infra/json-file.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/json-file.ts.md` | 行为、约束 |
| `src/infra/machine-name.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/machine-name.ts.md` | 行为、约束 |
| `src/infra/net/fetch-guard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/net/fetch-guard.ts.md` | 行为、约束 |
| `src/infra/net/ssrf.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/net/ssrf.ts.md` | 行为、约束 |
| `src/infra/node-pairing.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/node-pairing.ts.md` | 行为、约束 |
| `src/infra/node-shell.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/node-shell.ts.md` | 行为、约束 |
| `src/infra/openclaw-root.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/openclaw-root.ts.md` | 行为、约束 |
| `src/infra/os-summary.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/os-summary.ts.md` | 行为、约束 |
| `src/infra/outbound/abort.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/abort.ts.md` | 行为、约束 |
| `src/infra/outbound/agent-delivery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/agent-delivery.ts.md` | 行为、约束 |
| `src/infra/outbound/channel-adapters.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/channel-adapters.ts.md` | 行为、约束 |
| `src/infra/outbound/channel-selection.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/channel-selection.ts.md` | 行为、约束 |
| `src/infra/outbound/channel-target.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/channel-target.ts.md` | 行为、约束 |
| `src/infra/outbound/deliver.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/deliver.ts.md` | 行为、约束 |
| `src/infra/outbound/directory-cache.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/directory-cache.ts.md` | 行为、约束 |
| `src/infra/outbound/envelope.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/envelope.ts.md` | 行为、约束 |
| `src/infra/outbound/format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/format.ts.md` | 行为、约束 |
| `src/infra/outbound/message-action-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/message-action-runner.ts.md` | 行为、约束 |
| `src/infra/outbound/message-action-spec.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/message-action-spec.ts.md` | 行为、约束 |
| `src/infra/outbound/message.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/message.ts.md` | 行为、约束 |
| `src/infra/outbound/outbound-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/outbound-policy.ts.md` | 行为、约束 |
| `src/infra/outbound/outbound-send-service.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/outbound-send-service.ts.md` | 行为、约束 |
| `src/infra/outbound/outbound-session.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/outbound-session.ts.md` | 行为、约束 |
| `src/infra/outbound/payloads.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/payloads.ts.md` | 行为、约束 |
| `src/infra/outbound/target-errors.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/target-errors.ts.md` | 行为、约束 |
| `src/infra/outbound/target-normalization.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/target-normalization.ts.md` | 行为、约束 |
| `src/infra/outbound/target-resolver.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/target-resolver.ts.md` | 行为、约束 |
| `src/infra/outbound/targets.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/outbound/targets.ts.md` | 行为、约束 |
| `src/infra/path-env.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/path-env.ts.md` | 行为、约束 |
| `src/infra/ports-format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ports-format.ts.md` | 行为、约束 |
| `src/infra/ports-inspect.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ports-inspect.ts.md` | 行为、约束 |
| `src/infra/ports-lsof.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ports-lsof.ts.md` | 行为、约束 |
| `src/infra/ports-types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ports-types.ts.md` | 行为、约束 |
| `src/infra/ports.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ports.ts.md` | 行为、约束 |
| `src/infra/provider-usage.auth.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.auth.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.antigravity.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.antigravity.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.claude.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.claude.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.codex.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.codex.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.copilot.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.copilot.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.gemini.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.gemini.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.minimax.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.minimax.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.shared.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.ts.md` | 行为、约束 |
| `src/infra/provider-usage.fetch.zai.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.fetch.zai.ts.md` | 行为、约束 |
| `src/infra/provider-usage.format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.format.ts.md` | 行为、约束 |
| `src/infra/provider-usage.load.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.load.ts.md` | 行为、约束 |
| `src/infra/provider-usage.shared.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.shared.ts.md` | 行为、约束 |
| `src/infra/provider-usage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.ts.md` | 行为、约束 |
| `src/infra/provider-usage.types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/provider-usage.types.ts.md` | 行为、约束 |
| `src/infra/restart-sentinel.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/restart-sentinel.ts.md` | 行为、约束 |
| `src/infra/restart.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/restart.ts.md` | 行为、约束 |
| `src/infra/retry-policy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/retry-policy.ts.md` | 行为、约束 |
| `src/infra/retry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/retry.ts.md` | 行为、约束 |
| `src/infra/runtime-guard.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/runtime-guard.ts.md` | 行为、约束 |
| `src/infra/session-cost-usage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/session-cost-usage.ts.md` | 行为、约束 |
| `src/infra/session-maintenance-warning.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/session-maintenance-warning.ts.md` | 行为、约束 |
| `src/infra/shell-env.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/shell-env.ts.md` | 行为、约束 |
| `src/infra/skills-remote.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/skills-remote.ts.md` | 行为、约束 |
| `src/infra/ssh-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ssh-config.ts.md` | 行为、约束 |
| `src/infra/ssh-tunnel.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ssh-tunnel.ts.md` | 行为、约束 |
| `src/infra/state-migrations.fs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/state-migrations.fs.ts.md` | 行为、约束 |
| `src/infra/state-migrations.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/state-migrations.ts.md` | 行为、约束 |
| `src/infra/system-events.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/system-events.ts.md` | 行为、约束 |
| `src/infra/system-presence.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/system-presence.ts.md` | 行为、约束 |
| `src/infra/tailnet.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/tailnet.ts.md` | 行为、约束 |
| `src/infra/tailscale.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/tailscale.ts.md` | 行为、约束 |
| `src/infra/tls/fingerprint.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/tls/fingerprint.ts.md` | 行为、约束 |
| `src/infra/tls/gateway.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/tls/gateway.ts.md` | 行为、约束 |
| `src/infra/transport-ready.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/transport-ready.ts.md` | 行为、约束 |
| `src/infra/unhandled-rejections.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/unhandled-rejections.ts.md` | 行为、约束 |
| `src/infra/update-channels.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/update-channels.ts.md` | 行为、约束 |
| `src/infra/update-check.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/update-check.ts.md` | 行为、约束 |
| `src/infra/update-global.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/update-global.ts.md` | 行为、约束 |
| `src/infra/update-runner.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/update-runner.ts.md` | 行为、约束 |
| `src/infra/update-startup.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/update-startup.ts.md` | 行为、约束 |
| `src/infra/voicewake.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/voicewake.ts.md` | 行为、约束 |
| `src/infra/warning-filter.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/warning-filter.ts.md` | 行为、约束 |
| `src/infra/widearea-dns.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/widearea-dns.ts.md` | 行为、约束 |
| `src/infra/ws.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/ws.ts.md` | 行为、约束 |
| `src/infra/wsl.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/infra/wsl.ts.md` | 行为、约束 |
| `src/line/accounts.ts` | （未建） | 缺失文档 |
| `src/line/auto-reply-delivery.ts` | （未建） | 缺失文档 |
| `src/line/bot-access.ts` | （未建） | 缺失文档 |
| `src/line/bot-handlers.ts` | （未建） | 缺失文档 |
| `src/line/bot-message-context.ts` | （未建） | 缺失文档 |
| `src/line/bot.ts` | （未建） | 缺失文档 |
| `src/line/config-schema.ts` | （未建） | 缺失文档 |
| `src/line/download.ts` | （未建） | 缺失文档 |
| `src/line/flex-templates.ts` | （未建） | 缺失文档 |
| `src/line/http-registry.ts` | （未建） | 缺失文档 |
| `src/line/index.ts` | （未建） | 缺失文档 |
| `src/line/markdown-to-line.ts` | （未建） | 缺失文档 |
| `src/line/monitor.ts` | （未建） | 缺失文档 |
| `src/line/probe.ts` | （未建） | 缺失文档 |
| `src/line/reply-chunks.ts` | （未建） | 缺失文档 |
| `src/line/rich-menu.ts` | （未建） | 缺失文档 |
| `src/line/send.ts` | （未建） | 缺失文档 |
| `src/line/signature.ts` | （未建） | 缺失文档 |
| `src/line/template-messages.ts` | （未建） | 缺失文档 |
| `src/line/types.ts` | （未建） | 缺失文档 |
| `src/line/webhook.ts` | （未建） | 缺失文档 |
| `src/link-understanding/apply.ts` | （未建） | 缺失文档 |
| `src/link-understanding/defaults.ts` | （未建） | 缺失文档 |
| `src/link-understanding/detect.ts` | （未建） | 缺失文档 |
| `src/link-understanding/format.ts` | （未建） | 缺失文档 |
| `src/link-understanding/index.ts` | （未建） | 缺失文档 |
| `src/link-understanding/runner.ts` | （未建） | 缺失文档 |
| `src/logger.ts` | （未建） | 缺失文档 |
| `src/logging.ts` | （未建） | 缺失文档 |
| `src/logging/config.ts` | （未建） | 缺失文档 |
| `src/logging/console.ts` | （未建） | 缺失文档 |
| `src/logging/diagnostic.ts` | （未建） | 缺失文档 |
| `src/logging/levels.ts` | （未建） | 缺失文档 |
| `src/logging/logger.ts` | （未建） | 缺失文档 |
| `src/logging/parse-log-line.ts` | （未建） | 缺失文档 |
| `src/logging/redact-identifier.ts` | （未建） | 缺失文档 |
| `src/logging/redact.ts` | （未建） | 缺失文档 |
| `src/logging/state.ts` | （未建） | 缺失文档 |
| `src/logging/subsystem.ts` | （未建） | 缺失文档 |
| `src/macos/gateway-daemon.ts` | （未建） | 缺失文档 |
| `src/macos/relay-smoke.ts` | （未建） | 缺失文档 |
| `src/macos/relay.ts` | （未建） | 缺失文档 |
| `src/markdown/code-spans.ts` | （未建） | 缺失文档 |
| `src/markdown/fences.ts` | （未建） | 缺失文档 |
| `src/markdown/frontmatter.ts` | （未建） | 缺失文档 |
| `src/markdown/ir.ts` | （未建） | 缺失文档 |
| `src/markdown/render.ts` | （未建） | 缺失文档 |
| `src/markdown/tables.ts` | （未建） | 缺失文档 |
| `src/markdown/whatsapp.ts` | （未建） | 缺失文档 |
| `src/media-understanding/apply.ts` | （未建） | 缺失文档 |
| `src/media-understanding/attachments.ts` | （未建） | 缺失文档 |
| `src/media-understanding/audio-preflight.ts` | （未建） | 缺失文档 |
| `src/media-understanding/concurrency.ts` | （未建） | 缺失文档 |
| `src/media-understanding/defaults.ts` | （未建） | 缺失文档 |
| `src/media-understanding/errors.ts` | （未建） | 缺失文档 |
| `src/media-understanding/format.ts` | （未建） | 缺失文档 |
| `src/media-understanding/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/anthropic/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/deepgram/audio.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/deepgram/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/google/audio.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/google/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/google/video.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/groq/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/image.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/minimax/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/openai/audio.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/openai/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/shared.ts` | （未建） | 缺失文档 |
| `src/media-understanding/providers/zai/index.ts` | （未建） | 缺失文档 |
| `src/media-understanding/resolve.ts` | （未建） | 缺失文档 |
| `src/media-understanding/runner.ts` | （未建） | 缺失文档 |
| `src/media-understanding/scope.ts` | （未建） | 缺失文档 |
| `src/media-understanding/types.ts` | （未建） | 缺失文档 |
| `src/media-understanding/video.ts` | （未建） | 缺失文档 |
| `src/media/audio-tags.ts` | （未建） | 缺失文档 |
| `src/media/audio.ts` | （未建） | 缺失文档 |
| `src/media/constants.ts` | （未建） | 缺失文档 |
| `src/media/fetch.ts` | （未建） | 缺失文档 |
| `src/media/host.ts` | （未建） | 缺失文档 |
| `src/media/image-ops.ts` | （未建） | 缺失文档 |
| `src/media/input-files.ts` | （未建） | 缺失文档 |
| `src/media/mime.ts` | （未建） | 缺失文档 |
| `src/media/parse.ts` | （未建） | 缺失文档 |
| `src/media/png-encode.ts` | （未建） | 缺失文档 |
| `src/media/server.ts` | （未建） | 缺失文档 |
| `src/media/store.ts` | （未建） | 缺失文档 |
| `src/memory/backend-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/backend-config.ts.md` | 行为、约束 |
| `src/memory/batch-gemini.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/batch-gemini.ts.md` | 行为、约束 |
| `src/memory/batch-openai.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/batch-openai.ts.md` | 行为、约束 |
| `src/memory/batch-voyage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/batch-voyage.ts.md` | 行为、约束 |
| `src/memory/embedding-chunk-limits.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embedding-chunk-limits.ts.md` | 行为、约束 |
| `src/memory/embedding-input-limits.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embedding-input-limits.ts.md` | 行为、约束 |
| `src/memory/embedding-model-limits.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embedding-model-limits.ts.md` | 行为、约束 |
| `src/memory/embeddings-gemini.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embeddings-gemini.ts.md` | 行为、约束 |
| `src/memory/embeddings-openai.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embeddings-openai.ts.md` | 行为、约束 |
| `src/memory/embeddings-voyage.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embeddings-voyage.ts.md` | 行为、约束 |
| `src/memory/embeddings.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/embeddings.ts.md` | 行为、约束 |
| `src/memory/headers-fingerprint.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/headers-fingerprint.ts.md` | 行为、约束 |
| `src/memory/hybrid.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/hybrid.ts.md` | 行为、约束 |
| `src/memory/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/index.ts.md` | 行为、约束 |
| `src/memory/internal.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/internal.ts.md` | 行为、约束 |
| `src/memory/manager-cache-key.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/manager-cache-key.ts.md` | 行为、约束 |
| `src/memory/manager-search.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/manager-search.ts.md` | 行为、约束 |
| `src/memory/manager.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/manager.ts.md` | 行为、约束 |
| `src/memory/memory-schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/memory-schema.ts.md` | 行为、约束 |
| `src/memory/node-llama.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/node-llama.ts.md` | 行为、约束 |
| `src/memory/openai-batch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/openai-batch.ts.md` | 行为、约束 |
| `src/memory/provider-key.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/provider-key.ts.md` | 行为、约束 |
| `src/memory/qmd-manager.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/qmd-manager.ts.md` | 行为、约束 |
| `src/memory/qmd-query-parser.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/qmd-query-parser.ts.md` | 行为、约束 |
| `src/memory/search-manager.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/search-manager.ts.md` | 行为、约束 |
| `src/memory/session-files.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/session-files.ts.md` | 行为、约束 |
| `src/memory/sqlite-vec.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/sqlite-vec.ts.md` | 行为、约束 |
| `src/memory/sqlite.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/sqlite.ts.md` | 行为、约束 |
| `src/memory/status-format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/status-format.ts.md` | 行为、约束 |
| `src/memory/sync-memory-files.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/sync-memory-files.ts.md` | 行为、约束 |
| `src/memory/sync-session-files.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/sync-session-files.ts.md` | 行为、约束 |
| `src/memory/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/memory/types.ts.md` | 行为、约束 |
| `src/node-host/config.ts` | （未建） | 缺失文档 |
| `src/node-host/runner.ts` | （未建） | 缺失文档 |
| `src/node-host/with-timeout.ts` | （未建） | 缺失文档 |
| `src/pairing/pairing-labels.ts` | （未建） | 缺失文档 |
| `src/pairing/pairing-messages.ts` | （未建） | 缺失文档 |
| `src/pairing/pairing-store.ts` | （未建） | 缺失文档 |
| `src/plugin-sdk/index.ts` | （未建） | 缺失文档 |
| `src/plugins/bundled-dir.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/bundled-dir.ts.md` | 行为、约束 |
| `src/plugins/cli.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/cli.ts.md` | 行为、约束 |
| `src/plugins/commands.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/commands.ts.md` | 行为、约束 |
| `src/plugins/config-schema.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/config-schema.ts.md` | 行为、约束 |
| `src/plugins/config-state.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/config-state.ts.md` | 行为、约束 |
| `src/plugins/discovery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/discovery.ts.md` | 行为、约束 |
| `src/plugins/enable.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/enable.ts.md` | 行为、约束 |
| `src/plugins/hook-runner-global.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/hook-runner-global.ts.md` | 行为、约束 |
| `src/plugins/hooks.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/hooks.ts.md` | 行为、约束 |
| `src/plugins/http-path.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/http-path.ts.md` | 行为、约束 |
| `src/plugins/http-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/http-registry.ts.md` | 行为、约束 |
| `src/plugins/install.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/install.ts.md` | 行为、约束 |
| `src/plugins/installs.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/installs.ts.md` | 行为、约束 |
| `src/plugins/loader.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/loader.ts.md` | 行为、约束 |
| `src/plugins/manifest-registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/manifest-registry.ts.md` | 行为、约束 |
| `src/plugins/manifest.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/manifest.ts.md` | 行为、约束 |
| `src/plugins/providers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/providers.ts.md` | 行为、约束 |
| `src/plugins/registry.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/registry.ts.md` | 行为、约束 |
| `src/plugins/runtime.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/runtime.ts.md` | 行为、约束 |
| `src/plugins/runtime/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/runtime/index.ts.md` | 行为、约束 |
| `src/plugins/runtime/native-deps.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/runtime/native-deps.ts.md` | 行为、约束 |
| `src/plugins/runtime/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/runtime/types.ts.md` | 行为、约束 |
| `src/plugins/schema-validator.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/schema-validator.ts.md` | 行为、约束 |
| `src/plugins/services.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/services.ts.md` | 行为、约束 |
| `src/plugins/slots.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/slots.ts.md` | 行为、约束 |
| `src/plugins/source-display.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/source-display.ts.md` | 行为、约束 |
| `src/plugins/status.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/status.ts.md` | 行为、约束 |
| `src/plugins/tools.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/tools.ts.md` | 行为、约束 |
| `src/plugins/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/types.ts.md` | 行为、约束 |
| `src/plugins/update.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/plugins/update.ts.md` | 行为、约束 |
| `src/polls.ts` | （未建） | 缺失文档 |
| `src/process/child-process-bridge.ts` | （未建） | 缺失文档 |
| `src/process/command-queue.ts` | （未建） | 缺失文档 |
| `src/process/exec.ts` | （未建） | 缺失文档 |
| `src/process/lanes.ts` | （未建） | 缺失文档 |
| `src/process/spawn-utils.ts` | （未建） | 缺失文档 |
| `src/providers/github-copilot-auth.ts` | （未建） | 缺失文档 |
| `src/providers/github-copilot-models.ts` | （未建） | 缺失文档 |
| `src/providers/github-copilot-token.ts` | （未建） | 缺失文档 |
| `src/providers/qwen-portal-oauth.ts` | （未建） | 缺失文档 |
| `src/routing/bindings.ts` | （未建） | 缺失文档 |
| `src/routing/resolve-route.ts` | （未建） | 缺失文档 |
| `src/routing/session-key.ts` | （未建） | 缺失文档 |
| `src/runtime.ts` | （未建） | 缺失文档 |
| `src/security/audit-extra.async.ts` | （未建） | 缺失文档 |
| `src/security/audit-extra.sync.ts` | （未建） | 缺失文档 |
| `src/security/audit-extra.ts` | （未建） | 缺失文档 |
| `src/security/audit-fs.ts` | （未建） | 缺失文档 |
| `src/security/audit.ts` | （未建） | 缺失文档 |
| `src/security/channel-metadata.ts` | （未建） | 缺失文档 |
| `src/security/external-content.ts` | （未建） | 缺失文档 |
| `src/security/fix.ts` | （未建） | 缺失文档 |
| `src/security/skill-scanner.ts` | （未建） | 缺失文档 |
| `src/security/windows-acl.ts` | （未建） | 缺失文档 |
| `src/sessions/level-overrides.ts` | （未建） | 缺失文档 |
| `src/sessions/model-overrides.ts` | （未建） | 缺失文档 |
| `src/sessions/send-policy.ts` | （未建） | 缺失文档 |
| `src/sessions/session-key-utils.ts` | （未建） | 缺失文档 |
| `src/sessions/session-label.ts` | （未建） | 缺失文档 |
| `src/sessions/transcript-events.ts` | （未建） | 缺失文档 |
| `src/shared/text/reasoning-tags.ts` | （未建） | 缺失文档 |
| `src/signal/accounts.ts` | （未建） | 缺失文档 |
| `src/signal/client.ts` | （未建） | 缺失文档 |
| `src/signal/daemon.ts` | （未建） | 缺失文档 |
| `src/signal/format.ts` | （未建） | 缺失文档 |
| `src/signal/identity.ts` | （未建） | 缺失文档 |
| `src/signal/index.ts` | （未建） | 缺失文档 |
| `src/signal/monitor.ts` | （未建） | 缺失文档 |
| `src/signal/monitor/event-handler.ts` | （未建） | 缺失文档 |
| `src/signal/monitor/event-handler.types.ts` | （未建） | 缺失文档 |
| `src/signal/probe.ts` | （未建） | 缺失文档 |
| `src/signal/reaction-level.ts` | （未建） | 缺失文档 |
| `src/signal/send-reactions.ts` | （未建） | 缺失文档 |
| `src/signal/send.ts` | （未建） | 缺失文档 |
| `src/signal/sse-reconnect.ts` | （未建） | 缺失文档 |
| `src/slack/accounts.ts` | （未建） | 缺失文档 |
| `src/slack/actions.ts` | （未建） | 缺失文档 |
| `src/slack/channel-migration.ts` | （未建） | 缺失文档 |
| `src/slack/client.ts` | （未建） | 缺失文档 |
| `src/slack/directory-live.ts` | （未建） | 缺失文档 |
| `src/slack/format.ts` | （未建） | 缺失文档 |
| `src/slack/http/index.ts` | （未建） | 缺失文档 |
| `src/slack/http/registry.ts` | （未建） | 缺失文档 |
| `src/slack/index.ts` | （未建） | 缺失文档 |
| `src/slack/monitor.test-helpers.ts` | （未建） | 缺失文档 |
| `src/slack/monitor.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/allow-list.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/auth.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/channel-config.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/commands.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/context.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events/channels.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events/members.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events/messages.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events/pins.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/events/reactions.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/media.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/message-handler.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/message-handler/dispatch.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/message-handler/prepare.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/message-handler/types.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/policy.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/provider.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/replies.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/slash.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/thread-resolution.ts` | （未建） | 缺失文档 |
| `src/slack/monitor/types.ts` | （未建） | 缺失文档 |
| `src/slack/probe.ts` | （未建） | 缺失文档 |
| `src/slack/resolve-channels.ts` | （未建） | 缺失文档 |
| `src/slack/resolve-users.ts` | （未建） | 缺失文档 |
| `src/slack/scopes.ts` | （未建） | 缺失文档 |
| `src/slack/send.ts` | （未建） | 缺失文档 |
| `src/slack/targets.ts` | （未建） | 缺失文档 |
| `src/slack/threading-tool-context.ts` | （未建） | 缺失文档 |
| `src/slack/threading.ts` | （未建） | 缺失文档 |
| `src/slack/token.ts` | （未建） | 缺失文档 |
| `src/slack/types.ts` | （未建） | 缺失文档 |
| `src/telegram/accounts.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/accounts.ts.md` | 行为、约束 |
| `src/telegram/allowed-updates.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/allowed-updates.ts.md` | 行为、约束 |
| `src/telegram/api-logging.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/api-logging.ts.md` | 行为、约束 |
| `src/telegram/audit.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/audit.ts.md` | 行为、约束 |
| `src/telegram/bot-access.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-access.ts.md` | 行为、约束 |
| `src/telegram/bot-handlers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-handlers.ts.md` | 行为、约束 |
| `src/telegram/bot-message-context.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-message-context.ts.md` | 行为、约束 |
| `src/telegram/bot-message-dispatch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-message-dispatch.ts.md` | 行为、约束 |
| `src/telegram/bot-message.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-message.ts.md` | 行为、约束 |
| `src/telegram/bot-native-commands.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-native-commands.ts.md` | 行为、约束 |
| `src/telegram/bot-updates.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot-updates.ts.md` | 行为、约束 |
| `src/telegram/bot.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot.ts.md` | 行为、约束 |
| `src/telegram/bot/delivery.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot/delivery.ts.md` | 行为、约束 |
| `src/telegram/bot/helpers.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot/helpers.ts.md` | 行为、约束 |
| `src/telegram/bot/types.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/bot/types.ts.md` | 行为、约束 |
| `src/telegram/caption.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/caption.ts.md` | 行为、约束 |
| `src/telegram/download.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/download.ts.md` | 行为、约束 |
| `src/telegram/draft-chunking.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/draft-chunking.ts.md` | 行为、约束 |
| `src/telegram/draft-stream.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/draft-stream.ts.md` | 行为、约束 |
| `src/telegram/fetch.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/fetch.ts.md` | 行为、约束 |
| `src/telegram/format.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/format.ts.md` | 行为、约束 |
| `src/telegram/group-migration.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/group-migration.ts.md` | 行为、约束 |
| `src/telegram/index.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/index.ts.md` | 行为、约束 |
| `src/telegram/inline-buttons.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/inline-buttons.ts.md` | 行为、约束 |
| `src/telegram/model-buttons.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/model-buttons.ts.md` | 行为、约束 |
| `src/telegram/monitor.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/monitor.ts.md` | 行为、约束 |
| `src/telegram/network-config.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/network-config.ts.md` | 行为、约束 |
| `src/telegram/network-errors.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/network-errors.ts.md` | 行为、约束 |
| `src/telegram/probe.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/probe.ts.md` | 行为、约束 |
| `src/telegram/proxy.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/proxy.ts.md` | 行为、约束 |
| `src/telegram/reaction-level.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/reaction-level.ts.md` | 行为、约束 |
| `src/telegram/send.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/send.ts.md` | 行为、约束 |
| `src/telegram/sent-message-cache.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/sent-message-cache.ts.md` | 行为、约束 |
| `src/telegram/sticker-cache.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/sticker-cache.ts.md` | 行为、约束 |
| `src/telegram/targets.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/targets.ts.md` | 行为、约束 |
| `src/telegram/token.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/token.ts.md` | 行为、约束 |
| `src/telegram/update-offset-store.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/update-offset-store.ts.md` | 行为、约束 |
| `src/telegram/voice.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/voice.ts.md` | 行为、约束 |
| `src/telegram/webhook-set.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/webhook-set.ts.md` | 行为、约束 |
| `src/telegram/webhook.ts` | `/Users/biantaishabi/openclaw/docs/backend-trace/files/src/telegram/webhook.ts.md` | 行为、约束 |
| `src/terminal/ansi.ts` | （未建） | 缺失文档 |
| `src/terminal/links.ts` | （未建） | 缺失文档 |
| `src/terminal/note.ts` | （未建） | 缺失文档 |
| `src/terminal/palette.ts` | （未建） | 缺失文档 |
| `src/terminal/progress-line.ts` | （未建） | 缺失文档 |
| `src/terminal/prompt-style.ts` | （未建） | 缺失文档 |
| `src/terminal/restore.ts` | （未建） | 缺失文档 |
| `src/terminal/stream-writer.ts` | （未建） | 缺失文档 |
| `src/terminal/table.ts` | （未建） | 缺失文档 |
| `src/terminal/theme.ts` | （未建） | 缺失文档 |
| `src/test-helpers/state-dir-env.ts` | （未建） | 缺失文档 |
| `src/test-helpers/workspace.ts` | （未建） | 缺失文档 |
| `src/test-utils/channel-plugins.ts` | （未建） | 缺失文档 |
| `src/test-utils/ports.ts` | （未建） | 缺失文档 |
| `src/tts/tts.ts` | （未建） | 缺失文档 |
| `src/tui/commands.ts` | （未建） | 缺失文档 |
| `src/tui/components/assistant-message.ts` | （未建） | 缺失文档 |
| `src/tui/components/chat-log.ts` | （未建） | 缺失文档 |
| `src/tui/components/custom-editor.ts` | （未建） | 缺失文档 |
| `src/tui/components/filterable-select-list.ts` | （未建） | 缺失文档 |
| `src/tui/components/fuzzy-filter.ts` | （未建） | 缺失文档 |
| `src/tui/components/searchable-select-list.ts` | （未建） | 缺失文档 |
| `src/tui/components/selectors.ts` | （未建） | 缺失文档 |
| `src/tui/components/tool-execution.ts` | （未建） | 缺失文档 |
| `src/tui/components/user-message.ts` | （未建） | 缺失文档 |
| `src/tui/gateway-chat.ts` | （未建） | 缺失文档 |
| `src/tui/theme/syntax-theme.ts` | （未建） | 缺失文档 |
| `src/tui/theme/theme.ts` | （未建） | 缺失文档 |
| `src/tui/tui-command-handlers.ts` | （未建） | 缺失文档 |
| `src/tui/tui-event-handlers.ts` | （未建） | 缺失文档 |
| `src/tui/tui-formatters.ts` | （未建） | 缺失文档 |
| `src/tui/tui-local-shell.ts` | （未建） | 缺失文档 |
| `src/tui/tui-overlays.ts` | （未建） | 缺失文档 |
| `src/tui/tui-session-actions.ts` | （未建） | 缺失文档 |
| `src/tui/tui-status-summary.ts` | （未建） | 缺失文档 |
| `src/tui/tui-stream-assembler.ts` | （未建） | 缺失文档 |
| `src/tui/tui-types.ts` | （未建） | 缺失文档 |
| `src/tui/tui-waiting.ts` | （未建） | 缺失文档 |
| `src/tui/tui.ts` | （未建） | 缺失文档 |
| `src/types/cli-highlight.d.ts` | （未建） | 缺失文档 |
| `src/types/lydell-node-pty.d.ts` | （未建） | 缺失文档 |
| `src/types/napi-rs-canvas.d.ts` | （未建） | 缺失文档 |
| `src/types/node-edge-tts.d.ts` | （未建） | 缺失文档 |
| `src/types/node-llama-cpp.d.ts` | （未建） | 缺失文档 |
| `src/types/osc-progress.d.ts` | （未建） | 缺失文档 |
| `src/types/pdfjs-dist-legacy.d.ts` | （未建） | 缺失文档 |
| `src/types/proper-lockfile.d.ts` | （未建） | 缺失文档 |
| `src/types/qrcode-terminal.d.ts` | （未建） | 缺失文档 |
| `src/utils.ts` | （未建） | 缺失文档 |
| `src/utils/account-id.ts` | （未建） | 缺失文档 |
| `src/utils/boolean.ts` | （未建） | 缺失文档 |
| `src/utils/delivery-context.ts` | （未建） | 缺失文档 |
| `src/utils/directive-tags.ts` | （未建） | 缺失文档 |
| `src/utils/fetch-timeout.ts` | （未建） | 缺失文档 |
| `src/utils/message-channel.ts` | （未建） | 缺失文档 |
| `src/utils/normalize-secret-input.ts` | （未建） | 缺失文档 |
| `src/utils/provider-utils.ts` | （未建） | 缺失文档 |
| `src/utils/queue-helpers.ts` | （未建） | 缺失文档 |
| `src/utils/shell-argv.ts` | （未建） | 缺失文档 |
| `src/utils/transcript-tools.ts` | （未建） | 缺失文档 |
| `src/utils/usage-format.ts` | （未建） | 缺失文档 |
| `src/version.ts` | （未建） | 缺失文档 |
| `src/web/accounts.ts` | （未建） | 缺失文档 |
| `src/web/active-listener.ts` | （未建） | 缺失文档 |
| `src/web/auth-store.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply.impl.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/constants.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/deliver-reply.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/heartbeat-runner.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/loggers.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/mentions.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/ack-reaction.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/broadcast.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/commands.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/echo.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/group-activation.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/group-gating.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/group-members.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/last-route.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/message-line.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/on-message.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/peer.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/monitor/process-message.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/session-snapshot.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/types.ts` | （未建） | 缺失文档 |
| `src/web/auto-reply/util.ts` | （未建） | 缺失文档 |
| `src/web/inbound.ts` | （未建） | 缺失文档 |
| `src/web/inbound/access-control.ts` | （未建） | 缺失文档 |
| `src/web/inbound/dedupe.ts` | （未建） | 缺失文档 |
| `src/web/inbound/extract.ts` | （未建） | 缺失文档 |
| `src/web/inbound/media.ts` | （未建） | 缺失文档 |
| `src/web/inbound/monitor.ts` | （未建） | 缺失文档 |
| `src/web/inbound/send-api.ts` | （未建） | 缺失文档 |
| `src/web/inbound/types.ts` | （未建） | 缺失文档 |
| `src/web/login-qr.ts` | （未建） | 缺失文档 |
| `src/web/login.ts` | （未建） | 缺失文档 |
| `src/web/media.ts` | （未建） | 缺失文档 |
| `src/web/outbound.ts` | （未建） | 缺失文档 |
| `src/web/qr-image.ts` | （未建） | 缺失文档 |
| `src/web/reconnect.ts` | （未建） | 缺失文档 |
| `src/web/session.ts` | （未建） | 缺失文档 |
| `src/web/test-helpers.ts` | （未建） | 缺失文档 |
| `src/web/vcard.ts` | （未建） | 缺失文档 |
| `src/whatsapp/normalize.ts` | （未建） | 缺失文档 |
| `src/wizard/clack-prompter.ts` | （未建） | 缺失文档 |
| `src/wizard/onboarding.completion.ts` | （未建） | 缺失文档 |
| `src/wizard/onboarding.finalize.ts` | （未建） | 缺失文档 |
| `src/wizard/onboarding.gateway-config.ts` | （未建） | 缺失文档 |
| `src/wizard/onboarding.ts` | （未建） | 缺失文档 |
| `src/wizard/onboarding.types.ts` | （未建） | 缺失文档 |
| `src/wizard/prompts.ts` | （未建） | 缺失文档 |
| `src/wizard/session.ts` | （未建） | 缺失文档 |
