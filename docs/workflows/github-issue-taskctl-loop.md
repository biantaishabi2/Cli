# GitHub Issue + Taskctl 协作循环（agent 运行）

## 目标
- 外部入口尽量用 Issue（人类友好）。
- 任务执行核心放在本地：`taskctl` + `bddc`。
- 关键事件回写到 Issue：状态、证据、结论。

## 目录
- `taskctl`: 任务建模与依赖编排（源头状态）
- `GitHub Issue`: 人类入口与协作记录（沟通）
- `bddc`: 行为验收（可观测结果）

## 标准字段映射

`taskctl` 元数据建议包含：
- `source: github_issue`
- `issue_id: <issue number>`
- `issue_url: <issue link>`
- `priority: P0|P1|P2`
- `module: <biz module>`
- `owner: <agent>`

## 任务生命周期
1. 人员创建 Issue（title/description/acceptance）。
2. 本地/脚本拉取待处理 Issue。
3. 创建/更新 `taskctl` 任务，写入元数据。
4. 维护 `blocked_by` 依赖，形成 DAG。
5. 调用 `taskctl ready` 获取可执行任务。
6. agent 领取任务：`update --status in-progress --owner ...`。
7. 执行任务并产出证据（测试输出、日志、变更文件）。
8. 验收通过后 `update --status completed`。
9. 回写 Issue：开始/进行中/完成 + 证据链接。
10. 所有子任务完成后，主 issue 关闭或进入下一个迭代。

## 关键命令（示例）

### 1) 从 issue 建立任务（手工）
```bash
cd taskctl
./target/debug/taskctl \
  --store ../.taskctl/tasks.json create \
  --subject "[ISS-123] Design API" \
  --description "Issue requirement and acceptance criteria" \
  --metadata '{"source":"github_issue","issue_id":"123","issue_url":"https://github.com/org/repo/issues/123","module":"api","priority":"P1"}'
```

### 2) 查询可就绪任务
```bash
./target/debug/taskctl --store ../.taskctl/tasks.json ready
```

### 3) 更新任务状态
```bash
./target/debug/taskctl --store ../.taskctl/tasks.json update \
  --task-id <task-id> --status in-progress --owner robot-1

./target/debug/taskctl --store ../.taskctl/tasks.json update \
  --task-id <task-id> --status completed
```

### 4) 验证 DAG 有效性
```bash
./target/debug/taskctl --store ../.taskctl/tasks.json validate
./target/debug/taskctl --store ../.taskctl/tasks.json dag > /tmp/task-dag.json
```

## 回写 Issue（建议）
- 状态变更：`in-progress`, `blocked`, `completed`
- 附件：
  - bddc 运行摘要
  - 提交 hash
  - 证据文件/日志路径

示例（用 gh）：
```bash
gh issue comment <number> --body "任务开始：<task-id>，当前状态 in-progress。"
gh issue comment <number> --body "任务完成：<task-id>，BDD 通过（附件见 ./artifacts）。"
```

## 备注
- 所有外部协作与可追溯证据建议以 `taskctl` 的 `tasks.json` 与 Issue 时间线为准。
- 该文档是可迭代草案，目标是先闭环：`issue -> taskctl -> bddc -> issue`。
