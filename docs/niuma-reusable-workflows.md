# niuma Reusable Workflows 接入说明

## 目标

- `Cli` 作为 workflow 真源，集中维护编排逻辑。
- 消费者仓库（如 `gong`）仅保留入口触发和仓库差异参数。
- 通过 `workflow_call` 实现“一次修复，多仓生效”。

## 真源与消费者

- 真源：`Cli/.github/workflows/niuma-orchestrate-reusable.yml`
- Cli 入口薄封装：`Cli/.github/workflows/niuma-orchestrate.yml`
- completed 分发：`Cli/.github/workflows/niuma-dispatch-completed.yml`
- 消费者样例：`gong/.github/workflows/niuma-orchestrate.yml`

## v1 输入契约

契约文件：`automation/niuma/contracts/orchestrate_inputs.schema.json`

| 字段 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `repo` | 是 | - | 目标仓库，格式 `owner/repo` |
| `repo_dir` | 否 | `.` | 仓库目录 |
| `build_niuma` | 否 | `true` | 是否在 workflow 内构建 niuma |
| `label_whitelist` | 否 | `bot:queued,bot:pr-reviewable` | `issues:labeled` 允许触发标签 |
| `enable_dispatch_wakeup` | 否 | `true` | 是否允许 `repository_dispatch` 唤醒 |
| `event_id` | 否 | `""` | 外部事件唯一键，空值退化为 run 级唯一键 |
| `dedup_window_hours` | 否 | `24` | `event_id` 去重窗口 |
| `concurrency_key` | 否 | `niuma-orchestrate-${repo}` | 同仓库串行并发键 |

## 消费者接入步骤（以 gong 为例）

1. 保留入口触发：`issues:labeled`、`repository_dispatch(types:[niuma.task.completed])`、`schedule`。
2. 在 job 中改为调用：
   - `uses: biantaishabi2/Cli/.github/workflows/niuma-orchestrate-reusable.yml@<SHA>`
3. 通过 `with` 传递仓库差异参数：
   - `repo: biantaishabi2/gong`
   - `build_niuma: false`（若使用预装二进制）
4. 使用 `secrets: inherit`，不额外扩大 secret 范围。

## 版本策略

灰度期：
- 必须使用 SHA pin（40 位 commit SHA），避免上游未验证变更直接影响消费者。

稳定期：
- 可切换为语义化 tag，例如 `niuma-workflows-vX.Y.Z`。
- 每次升级先在单仓灰度，再推广到多仓。

## 失败降级与回滚

- `niuma.task.completed` dispatch 失败时仅告警（`::warning`），不阻断 close-after 流程。
- `schedule` 作为补偿通道，确保最终一致推进。
- 回滚步骤：
  1. 消费者仓库将 `uses: ...@<SHA>` 切回上一个已验证 SHA。
  2. 如需紧急止损，临时恢复本地 workflow 实现并禁用 reusable 调用。
  3. 在真源修复后重新灰度。

## 常见故障

- `build_niuma=false` 但 runner 无 `niuma`：
  - 处理：预装 `niuma`，或改回 `build_niuma=true`。
- 高频重复 dispatch 导致重复 side effect：
  - 处理：检查 `event_id` 是否稳定生成；确认去重窗口配置是否合理。
- 触发后无推进：
  - 处理：确认入口标签在 `label_whitelist` 中，且 dispatch `event_source=close-after-integration-merge`。
