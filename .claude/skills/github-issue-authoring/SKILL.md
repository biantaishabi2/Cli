---
name: github-issue-authoring
description: 根据 niuma 工作流要求，创建结构化的 GitHub Issue，便于后续 AI 自动化处理
---

# GitHub Issue Authoring Skill

根据 niuma 工作流要求，创建结构化的 GitHub Issue，便于后续 AI 自动化处理。

## 使用方式

```
/github-issue-authoring
```

或带初始信息：

```
/github-issue-authoring --title "功能请求：XXX" --type feature
```

## 工作流程

1. **收集信息** - 通过对话收集必要字段
2. **结构化组织** - 按照 niuma Final Plan 模板组织内容
3. **生成 Issue** - 输出可直接粘贴到 GitHub 的 Markdown
4. **创建 Issue** - 调用 GitHub API/CLI 创建 Issue
5. **添加标签** - 如需要，添加 `bot:fix` 标签触发 niuma

## 创建 Issue 的方式

### 方式 1：GitHub CLI（推荐）

```bash
# 创建 Issue
gh issue create \
  --title "feat: 添加用户认证模块" \
  --body-file issue-body.md \
  --label "enhancement"

# 或直接在命令行写 body
gh issue create \
  --title "fix: 修复内存泄漏" \
  --body "## 背景..." \
  --label "bug"
```

### 方式 2：GitHub API

```bash
# 使用 curl 调用 API
curl -X POST \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/{owner}/{repo}/issues \
  -d '{
    "title": "feat: 添加用户认证模块",
    "body": "## 背景...",
    "labels": ["enhancement"]
  }'
```

### 方式 3：使用 gh 交互式

```bash
gh issue create
# 然后按提示输入标题、body、标签等
```

## 触发 niuma 自动化

创建 Issue 后，添加 `bot:fix` 标签触发 niuma：

```bash
# 获取刚创建的 issue 号
ISSUE_NUM=$(gh issue list --limit 1 --json number --jq '.[0].number')

# 添加 bot:fix 标签
gh issue edit $ISSUE_NUM --add-label "bot:fix"
```

或在创建时直接加标签：

```bash
gh issue create \
  --title "..." \
  --body "..." \
  --label "enhancement,bot:fix"
```

## 信息收集清单

### 基础信息（必须）

| 字段 | 说明 | 示例 |
|------|------|------|
| `title` | Issue 标题 | "feat: 添加用户认证模块" |
| `type` | 类型 | feature / bug / refactor / docs |
| `background` | 背景/动机 | 为什么要做这个功能 |
| `problem` | 问题描述 | 当前存在的问题 |
| `expected` | 期望行为 | 应该是什么样的 |

### 详细方案（推荐）

| 字段 | 说明 | 对应 Final Plan 章节 |
|------|------|---------------------|
| `approach` | 解决方案概述 | 3. 修复策略 |
| `affected_files` | 涉及文件/模块 | 4. 改动清单 |
| `test_scenarios` | 测试场景 | 6. 测试场景 |
| `risks` | 潜在风险 | 5. 风险与回滚 |
| `compatibility` | 兼容性影响 | 4. 改动清单 |

### 测试策略（niuma 必需）

```yaml
test_strategy:
  unit_tests:
    - name: "测试名"
      scenario: "测试场景"
      assertions: ["断言1", "断言2"]
  integration_tests:
    - name: "集成测试名"
      setup: "前置条件"
      verification: "验证点"
  bdd_tests:
    - name: "BDD 场景"
      given: "前置条件"
      when: "操作"
      then: "预期结果"
```

## Issue 模板结构

生成的 Issue 遵循以下结构：

```markdown
## 背景

{{background}}

## 问题描述

{{problem}}

## 期望行为

{{expected}}

## 方案草案（可选）

### 解决方案
{{approach}}

### 涉及模块/文件
{{affected_files}}

### 测试策略
{{test_strategy}}

### 风险与回滚
{{risks}}

## 验收标准

- [ ] {{criteria_1}}
- [ ] {{criteria_2}}
```

## 类型特定模板

### Feature（新功能）

```markdown
## 用户故事
作为 {{role}}，我希望 {{desire}}，以便 {{benefit}}

## 功能描述
{{feature_description}}

## 接口设计（如适用）
```

### Bug（缺陷修复）

```markdown
## 复现步骤
1. {{step_1}}
2. {{step_2}}

## 实际行为
{{actual_behavior}}

## 环境信息
- 版本: {{version}}
- 平台: {{platform}}
```

### Refactor（重构）

```markdown
## 技术债务
{{technical_debt}}

## 重构目标
{{refactor_goal}}

## 影响范围
{{impact_scope}}
```

## 与 niuma 工作流集成

当 Issue 被标记为 `bot:fix` 时，niuma 会：

1. **Draft 阶段** - 根据 Issue 内容生成方案草案
2. **Discussion 阶段** - 如需补充信息，进入讨论
3. **Final Plan 阶段** - 生成符合模板的最终方案
4. **Implement 阶段** - 自动实现代码

因此，Issue 中的以下字段会被 niuma 直接使用：

- `background` → 目标与非目标
- `problem` + `approach` → 根因分析 + 修复策略
- `affected_files` → 改动清单
- `test_scenarios` → 测试场景
- `risks` → 风险与回滚

## 完整示例

### 步骤 1：生成 Issue 内容

```markdown
## 背景

用户需要将架构验证结果导出为不同格式便于分享和存档。

## 问题描述

目前 `bcc arch validate` 只能查看控制台输出，无法保存结果。

## 期望行为

支持通过 `--format` 和 `--output` 参数导出：
- JSON: 结构化数据，便于程序处理
- Markdown: 便于阅读和人肉 review
- HTML: 便于在浏览器中查看

## 方案草案

### 解决方案

1. 添加 `--format <json|markdown|html>` 参数
2. 添加 `--output <path>` 参数
3. 实现对应格式的 Reporter

### 涉及模块/文件

- `compiler/bcc/src/commands/validate.rs`: 添加参数解析
- `compiler/bcc/src/reporters/mod.rs`: 定义 Reporter trait
- `compiler/bcc/src/reporters/json.rs`: JSON 格式实现
- `compiler/bcc/src/reporters/markdown.rs`: Markdown 格式实现
- `compiler/bcc/src/reporters/html.rs`: HTML 格式实现

### 测试策略

**单元测试:**
- 验证各格式输出结构正确
- 验证文件写入成功

**集成测试:**
- 端到端验证导出功能

### 风险与回滚

- 风险: 新增依赖（HTML 生成库）
- 回滚: 移除 reporter 模块，恢复原有输出

## 验收标准

- [ ] `--format json` 输出有效 JSON
- [ ] `--format markdown` 输出有效 Markdown
- [ ] `--format html` 输出有效 HTML
- [ ] `--output` 指定文件路径时写入文件
- [ ] 未指定 `--output` 时输出到 stdout
```

### 步骤 2：保存到文件并创建 Issue

```bash
# 保存 body 到文件
cat > /tmp/issue-body.md << 'EOF'
## 背景
...
EOF

# 创建 Issue
gh issue create \
  --title "feat: BCC 添加架构验证报告导出功能" \
  --body-file /tmp/issue-body.md \
  --label "enhancement"

# 获取 issue 号并添加 bot:fix 标签
ISSUE_NUM=$(gh issue list --limit 1 --json number --jq '.[0].number')
gh issue edit $ISSUE_NUM --add-label "bot:fix"
```

## 最佳实践

1. **标题规范**: `<type>: <description>`
   - feat: 新功能
   - fix: 修复 bug
   - refactor: 重构
   - docs: 文档
   - test: 测试
   - chore: 构建/工具

2. **背景要清晰**: 让不熟悉项目的人也能理解为什么要做

3. **问题要具体**: 包含错误信息、日志、截图等

4. **方案要可行**: 不需要完美，但要方向正确

5. **测试要明确**: niuma 会根据测试策略生成测试代码

6. **文件路径要准确**: 帮助 niuma 定位修改范围

7. **触发自动化**: 创建后记得加 `bot:fix` 标签让 niuma 接手
