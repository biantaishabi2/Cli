# niuma 🐮🐴

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/niuma-ci.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/niuma-ci.yml)

**负重前行，代码自动生成**

AI 驱动的全自动开发机器人：Issue → Plan → Code → PR → Iterate → **Control（多 Issue 协调）**

## 定位

`niuma`（牛马）是 Cli 工具链的自动化层，负责：

**单 Issue 全流程（Phase 2.5/2.6 已就绪）**：
- 接收 Issue（bug/feature/refactor）
- 自动分析并输出方案（含测试场景）
- 自动改代码、加测试
- 自动提 PR
- 根据 Review 意见自动迭代

**多 Issue 协调（Phase 3 开发中）**：
- 扫描所有带 `bot:fix` 标签的 Issue
- AI 分析 Issue 间依赖关系
- 调用 taskctl 构建 DAG（有依赖的自动编排执行顺序）
- 构建 Integration 分支（批量 PR 联合验证）
- 按拓扑序批量合并到 master

**人只做三件事**：建 issue + 加 `bot:fix` 标签 + 最终批准合并。其他全部 AI 自动化。

## 目录结构

```
automation/niuma/
├── cmd/niuma/           # CLI 入口
│   └── main.go
├── cmd/niumad/          # 服务入口（可选）
│   └── main.go
├── pkg/
│   ├── agent/           # 核心逻辑（状态机/计划/实现/迭代）
│   ├── control/         # 【Phase 3】多 Issue 协调控制层
│   │   ├── taskctl.go   # taskctl CLI 封装
│   │   ├── analyzer.go  # AI 依赖分析
│   │   ├── integration.go # Integration 分支构建
│   │   └── controller.go # 协调循环核心
│   ├── github/          # GitHub API 封装
│   ├── ai/              # AI Provider 抽象（支持 Kimi/OpenCode/Codex 等多后端）
│   ├── state/           # Label 状态机
│   └── marker/          # 幂等 Marker 管理
├── templates/           # Final Plan 模板
├── .github/workflows/   # 4 个自动化 workflow
└── README.md
```

## 核心流程

### 单 Issue 流程（Phase 2.5/2.6）

```
Issue 创建
    ↓ (自动加 label: bot:fix)
Draft Plan（草案方案）
    ↓ (信息不足则进入讨论态)
Discussion（收敛讨论）
    ↓ (由 orchestration-loop-core 多轮推进；/finalize > /hold > should_finish > 静默预警；达到轮次上限仅提醒不自动定稿)
Final Plan（最终方案 + 测试场景）
    ↓
Implement（改代码 + 测试）
    ↓
PR Created
    ↓ (人 Review)
Iterate（根据意见自动修复）
    ↓
Merged
```

### 多 Issue 协调流程（Phase 3）

```
扫描所有 bot:fix Issues
    ↓
解析显式 depends-on（人工声明优先）
    ↓
对未声明 depends-on 的 issue 做统一 AI 依赖补全（含仅有 parent 的 sub issue）
    ↓
写入 taskctl blocked_by（先落盘）
    ↓
计算并推进 ready tasks（后放行）
    ↓
收集 PRs → 构建 Integration 分支
    ↓
CI 联合验证（检测冲突）
    ↓
人批准 → 按拓扑序批量合并
    ↓
全部 Merged
```

依赖语义约束：
- 执行依赖优先级：`depends-on` > AI 推断（AI 仅补全未声明项，不覆盖人工声明）
- `parent` 仅表示结构归属与收口关系，不隐式作为执行依赖边
- `ready` 判定前必须完成 `blocked_by` 写入；写入失败时本轮跳过放行，等待下轮重试

### Control 命令

```bash
niuma control run      # 执行一次完整协调循环
niuma control status   # 查看全局状态（DAG + 各 task 进度）
niuma control merge --issues 40,41,42  # 人批准后批量合并
```

## 状态机（Labels）

| Label | 含义 |
|-------|------|
| `bot:fix` | 请求机器人介入 |
| `bot:plan-draft` | 草案方案已输出 |
| `bot:needs-discussion` | 信息不足/冲突，进入讨论态 |
| `bot:plan-final` | **最终方案定稿**（含测试场景） |
| `bot:implementing` | 正在改代码 |
| `bot:pr-created` | PR 已创建，等待自检 |
| `bot:pr-reviewable` | 自检通过，可人工审核 |
| `bot:pr-needs-fix` | 自检/审核失败，需修复 |
| `bot:iterating` | 根据 Review 意见迭代 |
| `bot:done` | 合并/关闭 |

## Discussion 协议（当前）

- 讨论模式：仅 `debate_ab`
- 每轮模型输出：自然语言正文 + 末尾最小 JSON `{"should_finish": true|false}`
- 收敛优先级：`/finalize` > `/hold` > `should_finish` > 静默预警/超时
- 达到 discuss 最大轮次：写入轮次上限提醒，保持 `bot:needs-discussion`，不自动进入 `bot:plan-final`

## 快速开始

### 1. 安装

```bash
# 从源码构建
cd automation/niuma
go build -o niuma ./cmd/niuma

# 安装到系统 PATH
mv niuma /usr/local/bin/
```

### 2. 配置

```bash
# 配置文件指定 AI provider（默认读 .niuma.yml）
# 或通过环境变量覆盖
export NIUMA_AI_PROVIDER="kimi"

# 设置 GitHub Token（需有 repo 权限）
export GITHUB_TOKEN="ghp_xxx"
```

集成测试仓配置（用于 `go test -tags integration`）：

- 默认测试仓：`biantaishabi2/Cli-niuma-test`
- 可通过 `NIUMA_TEST_REPO` 覆盖测试目标仓库
- Token 优先级：`NIUMA_TEST_TOKEN` > `GITHUB_TOKEN`

```bash
export NIUMA_TEST_REPO="biantaishabi2/Cli-niuma-test"
export NIUMA_TEST_TOKEN="ghp_xxx"
```

### 3. 手动触发（调试）

```bash
# 为 Issue #123 生成 Draft Plan
niuma plan-draft --repo owner/repo --issue 123

# 收敛讨论并生成 Final Plan
niuma plan-final --repo owner/repo --issue 123

# 执行修复并提 PR
niuma fix --repo owner/repo --issue 123

# 根据 PR Review 迭代
niuma iterate --repo owner/repo --pr 456
```

### 4. 全自动模式（推荐）

直接给 Issue 加 label `bot:fix`，GitHub Actions 会自动触发完整流程。

## Final Plan 模板

机器人只有在能产出以下结构时，才允许打 `bot:plan-final`：

```markdown
## Final Plan

### 1. 目标与非目标
- 要做什么
- 明确不做什么

### 2. 根因分析（证据链）
- 问题根因
- 证据/日志/代码位置

### 3. 修复策略
- 技术方案
- 改动范围

### 4. 改动清单
- 文件/模块/接口
- 兼容性影响
- 开关/降级策略

### 5. 风险与回滚
- 最坏情况
- 回滚方案

### 6. 测试场景（必须）
#### 6.1 复现测试
- 输入/前置条件/期望输出

#### 6.2 回归测试
- 同模块关键路径
- 上下游调用链

#### 6.3 边界与异常
- 空值/超长/非法输入
- 超时/重试/并发

#### 6.4 非功能（NFR）
- 性能不退化
- 资源不泄露
- 可观测性

### 7. 观测与验收
- 日志/指标/告警

### 8. 发布策略（可选）
- 灰度/全量
```

## 测试要求

Final Plan 必须包含可执行的测试：

| 类型 | 要求 |
|------|------|
| 复现 | 用 Issue 中的最小复现步骤 |
| 回归 | 覆盖同模块关键路径 + 上下游 |
| 边界 | 空值/超长/非法/超时/并发 |
| 非功能 | 性能/资源/可观测性 |

## GitHub Actions Workflows

统一 workflow 实现全自动（跑在 self-hosted runner），根据当前 label 决定执行阶段：

| 触发事件 | 执行阶段 |
|----------|---------|
| Issue labeled `bot:fix` | Draft Plan |
| Issue 评论 / Schedule (5min) | Discussion 收敛检查 → Final Plan |
| Issue labeled `bot:plan-final` | Implement → 提 PR |
| PR created (`bot:pr-created`) | Self-Check（测试 + 规范） |
| PR Review (`changes_requested`) | Iterate（自动修复意见） |

## 幂等机制

用 **Marker 注释** 保证不重复执行：

```markdown
<!-- BOT:PLAN_DRAFT issue=123 rev=1 -->
<!-- BOT:DISCUSSION_SUMMARY issue=123 rev=2 -->
<!-- BOT:PLAN_FINAL issue=123 rev=1 -->
<!-- BOT:PR_CREATED issue=123 pr=456 -->
```

每次执行前检查同类 marker，找到则更新或退出。

## 与 Cli 其他工具的关系

```
Cli/
├── orchestration/taskctl/   # 任务编排（人主动）
├── compiler/bcc/            # 代码分析/编译（人主动）
└── automation/niuma/        # 全自动开发（机器人驱动）
```

`niuma` 可以调用 `bcc` 做代码分析，也可以创建 `taskctl` 任务追踪复杂工作。

## 开发

```bash
cd automation/niuma

# 测试
go test ./...

# 构建
go build -o niuma ./cmd/niuma

# 本地调试（不触发 Actions）
./niuma plan-draft --repo biantaishabi2/Cli --issue 123
```

## 许可

MIT

---

🐮🐴 **niuma** - 让机器人做牛马，人做决策
