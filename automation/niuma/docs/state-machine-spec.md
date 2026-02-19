# 状态机规范（bot:* 受控单值）

## 目标

将 `bot:*` 从普通标签升级为受控单值状态字段，保证任一时刻最多一个状态标签。

## API 合同

### 1) `state.Transition(issue, from, to)`

- `to` 必填且必须是合法 `bot:*` 状态。
- `from` 可选：
  - 非空：CAS 语义，当前状态必须等于 `from`。
  - 为空：不做前置状态约束，直接迁移到 `to`。
- 写入策略：一次 `ReplaceLabels` 原子替换，不允许散落 `add/remove` 组合。
- 幂等：当前已是 `to` 且无其他 `bot:*` 时直接成功，不写入。

错误语义：

- `ErrConflict`：CAS 冲突或写后校验发现被并发覆盖。
- `ErrInvalidState`：`from/to` 非法，或存在非法 `bot:*` 标签。
- `ErrInvariantViolation`：同一 issue 检测到多个 `bot:*` 状态。

### 2) `state.TransitionWithRetry`

- 针对 `ErrConflict` 自动退避重试。
- 默认退避：`100ms / 300ms / 900ms`。

### 3) `state.Normalize(issue, priority)`

- 用于多状态自愈。
- 按优先级收敛为单状态并继续流程。
- 默认优先级可由 `NIUMA_STATE_PRIORITY` 覆盖。

## CLI 合同

### `niuma state-label set`

```bash
niuma state-label set --repo owner/repo --issue 325 --from bot:plan-draft --to bot:needs-discussion
```

### `niuma state-label normalize`

```bash
niuma state-label normalize --repo owner/repo --issue 325
```

### `niuma state-label clear`

```bash
niuma state-label clear --repo owner/repo --issue 325
```

## 门禁与防绕过

### 本地包装器

- `automation/niuma/scripts/gh`：拦截 `gh issue edit` 直接改 `bot:*`。
- 本地环境：拒绝并提示改用 `niuma state-label`。
- `CI=true`：硬失败（exit 非 0）。
- 推荐安装：`bash automation/niuma/scripts/install-gh-wrapper.sh`（安装到 `~/.local/bin/gh`）。

### 服务端 Guard

- 工作流：`.github/workflows/niuma-label-guard.yml`
- 监听 `issues:labeled/unlabeled`。
- 非 allowlist actor 修改 `bot:*` 时：
  - dry-run：仅评论提示；
  - enforce：自动回滚 + 评论。
- allowlist：默认 `github-actions[bot],niuma-bot`，可由 `NIUMA_LABEL_ALLOWLIST` 追加。

## discuss/control 自愈

- `orchestrator.currentState` 与 `control` 状态写入口均支持多状态自愈。
- 自愈动作会写入审计评论（去重 marker）。

## 测试映射

- 原子替换与幂等：`pkg/state/state_test.go`
- 多状态自愈与去重：`pkg/control/controller_test.go`
- discuss 入口自愈：`pkg/agent/orchestrator_test.go`
- 端到端：`tests/integration/discuss_flow_test.go`
