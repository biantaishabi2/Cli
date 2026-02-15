# niuma 🐮🐴

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/niuma-ci.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/niuma-ci.yml)

**负重前行，代码自动生成**

AI 驱动的全自动开发机器人：Issue → Plan → Code → PR → Iterate

## 定位

`niuma`（牛马）是 Cli 工具链的自动化层，负责：

- 接收 Issue（bug/feature/refactor）
- 自动分析并输出方案（含测试场景）
- 自动改代码、加测试
- 自动提 PR
- 根据 Review 意见自动迭代

人只在 PR Review 阶段介入，其他全部自动化。

## 目录结构

```
automation/niuma/
├── cmd/niuma/           # CLI 入口
│   └── main.go
├── cmd/niumad/          # 服务入口（可选）
│   └── main.go
├── pkg/
│   ├── agent/           # 核心逻辑（状态机/计划/实现）
│   ├── github/          # GitHub API 封装
│   ├── codex/           # 内网 AI 调用
│   ├── state/           # Label 状态机
│   └── marker/          # 幂等 Marker 管理
├── templates/           # Final Plan 模板
├── .github/workflows/   # 4 个自动化 workflow
└── README.md
```

## 核心流程

```
Issue 创建
    ↓ (自动加 label: bot:fix)
Draft Plan（草案方案）
    ↓ (信息不足则进入讨论态)
Discussion（收敛讨论）
    ↓ (静默窗口 10min 或轮次上限)
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

## 状态机（Labels）

| Label | 含义 |
|-------|------|
| `bot:fix` | 请求机器人介入 |
| `bot:plan-draft` | 草案方案已输出 |
| `bot:needs-discussion` | 信息不足/冲突，进入讨论态 |
| `bot:plan-final` | **最终方案定稿**（含测试场景） |
| `bot:implementing` | 正在改代码 |
| `bot:pr-ready` | PR 已创建，等待 Review |
| `bot:iterating` | 根据 Review 意见迭代 |
| `bot:done` | 合并/关闭 |

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
# 设置内网 Codex 地址
export NIUMACODEX_URL="http://your-codex-server:8080"

# 设置 GitHub Token（需有 repo 权限）
export GITHUB_TOKEN="ghp_xxx"
```

### 3. 手动触发（调试）

```bash
# 为 Issue #123 生成 Draft Plan
niuma plan draft --repo owner/repo --issue 123

# 收敛讨论并生成 Final Plan
niuma plan final --repo owner/repo --issue 123

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
- 明确不做什�

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

4 个 workflow 实现全自动（跑在 self-hosted runner）：

| Workflow | 触发 | 职责 |
|----------|------|------|
| `niuma-plan-draft.yml` | Issue labeled `bot:fix` | 生成 Draft Plan |
| `niuma-discuss.yml` | Issue 评论 / Schedule | 收敛讨论 → Final Plan |
| `niuma-implement.yml` | Issue labeled `bot:plan-final` | 改代码 → 提 PR |
| `niuma-iterate.yml` | PR Review / 评论 | 根据意见迭代 |

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
./niuma plan draft --repo biantaishabi2/Cli --issue 123
```

## 许可

MIT

---

🐮🐴 **niuma** - 让机器人做牛马，人做决策
