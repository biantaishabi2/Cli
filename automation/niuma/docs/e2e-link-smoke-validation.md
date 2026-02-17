# niuma 端到端链路实测（评论驱动讨论 -> 实现 -> PR）

## 1. 目标与范围

本方案用于验证 niuma 自动链路是否可从 Issue 评论持续推进到 `PLAN_FINAL`，并在定稿后自动进入 implement 阶段创建 PR。

本次仅做最小风险验证：
- 仅新增本文档，不修改任何生产逻辑。
- 验证链路可达与状态流转，不执行合并。

## 2. 前置条件

- 已存在测试 Issue：`#138`
- 仓库已启用相关 GitHub Actions（issues/issue_comment/pull_request_review）
- 具备可操作权限：打标签、评论、查看 workflow run、关闭 PR

## 3. 测试场景

| 场景ID | 输入 | 预期 |
|---|---|---|
| S1_标签触发草案 | Issue 打上 `bot:fix` | 自动生成 `PLAN_DRAFT`，并进入 `bot:needs-discussion` |
| S2_评论驱动讨论 | 在 issue 下新增一条人工评论 | 触发 discussion，新增 `DISCUSSION_SUMMARY`，且 `rev` 递增 |
| S3_讨论收敛定稿 | `rev>=5` 或评论 `/finalize` | 自动产出 `PLAN_FINAL` 并打 `bot:plan-final` |
| S4_定稿触发实现 | 进入 `plan-final` 状态 | 自动触发 implement，创建 PR 与 PR marker |
| S5_仅验证不合并 | 链路验证完成 | PR 可关闭并清理测试分支，issue 可手动关闭 |

## 4. 执行步骤与观测点

### Step 1: 标签触发草案（S1）

操作：为 issue `#138` 添加标签 `bot:fix`。

观测点：
- issue 评论区出现 `PLAN_DRAFT`
- issue 标签出现 `bot:needs-discussion`
- workflow run 状态为成功（或可解释失败并重试成功）

记录证据：
- `PLAN_DRAFT` 评论链接
- 标签变更截图或链接
- 对应 workflow run 链接与 run id

### Step 2: 评论驱动讨论（S2）

操作：连续发布人工评论（建议每轮 1 条），触发 discussion。

观测点：
- 每轮产生或更新 `DISCUSSION_SUMMARY`
- `rev` 按轮次递增（`rev=N+1`）
- discussion 阶段由评论事件持续推进（无 schedule 也可推进）

记录证据：
- 每轮人工评论链接
- 对应 `DISCUSSION_SUMMARY` 评论链接
- `rev` 递增记录（例如 `1 -> 2 -> 3`）

### Step 3: 收敛并定稿（S3）

操作（满足其一即可）：
- 讨论轮次达到 `rev>=5`
- 或人工评论 `/finalize`

观测点：
- issue 评论区出现 `PLAN_FINAL`
- issue 标签出现 `bot:plan-final`

记录证据：
- 触发收敛的评论链接（第 5 轮或 `/finalize`）
- `PLAN_FINAL` 评论链接
- `bot:plan-final` 标签变更链接

### Step 4: 定稿触发实现并建 PR（S4）

操作：等待 `plan-final` 后自动链路执行 implement。

观测点：
- 自动创建 PR（分支名可追踪到 issue `#138`）
- 生成 PR marker（如 `BOT:PR_CREATED`）
- issue 与 PR 正确关联

记录证据：
- PR 链接
- PR marker 评论链接
- implement workflow run 链接与 run id

### Step 5: 验证 review/iterate 可识别，不合并（S5）

操作：
- 在 PR 中添加评论或标签，验证 review/iterate 触发条件被识别
- 不执行 merge

观测点：
- PR 侧自动链路对评论/标签事件有响应
- PR 保持可关闭状态

记录证据：
- PR 评论链接（触发 review/iterate）
- 对应 workflow run 链接

## 5. 验收标准

- S1-S5 全部满足预期
- 关键产物齐全：`PLAN_DRAFT`、`DISCUSSION_SUMMARY(rev 递增)`、`PLAN_FINAL`、PR、PR marker
- 无生产代码改动，仅文档变更

## 6. 证据采集清单（执行时填写）

| 步骤 | 时间戳(UTC+8) | GitHub Event ID / Run ID | 证据链接 | 结果 | 备注 |
|---|---|---|---|---|---|
| Step 1 / S1 |  |  |  | 通过/失败 |  |
| Step 2 / S2 rev=1 |  |  |  | 通过/失败 |  |
| Step 2 / S2 rev=2 |  |  |  | 通过/失败 |  |
| Step 2 / S2 rev=3 |  |  |  | 通过/失败 |  |
| Step 2 / S2 rev=4 |  |  |  | 通过/失败 |  |
| Step 3 / S3 rev=5 或 /finalize |  |  |  | 通过/失败 |  |
| Step 4 / S4 |  |  |  | 通过/失败 |  |
| Step 5 / S5 |  |  |  | 通过/失败 |  |

## 7. 回滚与清理

链路验证结束后按以下顺序清理：

1. 关闭测试 PR（不合并）。
2. 删除测试分支（本地与远端）。
3. 从 issue 移除测试标签（如需要保留可不移除）。
4. 在 issue 留存最终验证结论并手动关闭 issue。

## 8. 执行结论模板（执行后填写）

```md
### niuma e2e smoke 结论

- 执行时间：
- 执行人：
- 结果：通过 / 部分通过 / 失败

#### 场景结果
- S1：
- S2：
- S3：
- S4：
- S5：

#### 关键证据
- PLAN_DRAFT：
- DISCUSSION_SUMMARY：
- PLAN_FINAL：
- PR：
- PR marker：

#### 异常与改进
- 
```

