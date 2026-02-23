# Workflow / Control Boundary

## 分层约束

- workflow 层只负责事件入口、并发与权限、去重与统一命令调用。
- control 层负责业务判定（标签、事件来源、PR 状态、人工升级）并输出 machine-readable 决策。
- workflow 禁止再实现业务白名单、事件来源判断、PR 状态判断等状态机逻辑。

## route-event 契约

统一入口命令：

```bash
niuma control route-event \
  --repo <owner/repo> \
  --workflow <orchestrate|plan|implement|review> \
  --event-name <github_event_name> \
  --event-path <github_event_payload_json>
```

标准输出字段：

- `decision=run|skip|fail`
- `reason=<snake_case>`
- `action=<orchestrate|plan|implement|review|iterate|discuss|none>`

退出语义：

- `run`: 退出码 0
- `skip`: 退出码 0
- `fail`: 退出码非 0

## 可观测性

control 侧必须输出结构化审计日志，至少包含：

- `workflow`
- `event_name`
- `action`
- `decision`
- `reason`
- `correlation_id`

workflow 只依据 `decision/action` 分支，不再解释 `reason` 语义。
