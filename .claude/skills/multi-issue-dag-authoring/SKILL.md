---
name: multi-issue-dag-authoring
description: 为复杂需求生成父 issue + sub-issues 的 DAG 化写作模板与创建步骤，默认使用 Sub-Issue 模式（parent/depends-on），并提供降低并行冲突的拆分规则。
---

# Multi-Issue DAG Authoring

用于“需求较大、需要并行但有依赖”的 issue 设计与创建。

## 适用场景

- 需求跨多个模块，单 issue 难以追踪
- 存在明确先后关系（依赖链）
- 需要并行开发但要控制冲突风险

## 默认模式

- 默认：`Sub-Issue` 模式（父 issue + 子 issue）
- 子 issue body 使用：
  - `parent: #<parent_issue_number>`
  - 可选 `depends-on: #<issue1>, #<issue2>`
- 仅在历史任务补录时使用“标签批量模式（bot:orchestrate）”

## 写作原则（仅 issue 设计，不含运行时合并策略）

1. 父 issue 只定义目标、边界、验收总标准。
2. 子 issue 只放“单一可交付单元”，避免一条里混多个模块。
3. 每个子 issue 必须写测试场景（输入/预期/边界）。
4. 每个子 issue 建议写 `affected_files`，用于降低并行冲突。

## 降冲突拆分规则

1. 先按“文件/模块边界”切分，再按阶段切分。
2. 高重叠文件的任务不要同层并行，改为显式依赖。
3. 公共接口变更放前置节点，业务改动依赖该节点。
4. 纯文档/测试任务可并行放在末层。

## 父 Issue 模板

```markdown
## 背景
...

## 目标
...

## 非目标
...

## DAG 结构（概要）
- L0: #A #B
- L1: #C(depends-on A), #D(depends-on B)
- L2: #E(depends-on C,D)

## Sub-Issues
- [ ] #<sub1>
- [ ] #<sub2>
- [ ] #<sub3>

## 总体验收标准
- [ ] 所有 sub issue 完成并关闭
- [ ] 关键链路测试通过
- [ ] 无未决阻塞依赖
```

## 子 Issue 模板

```markdown
## 背景
...

## 任务定义
...

## 依赖
parent: #<parent>
depends-on: #<optional_dep_1>, #<optional_dep_2>

## 影响范围
- affected_files:
  - `path/a`
  - `path/b`

## 测试场景
1. 输入: ...
   预期: ...
2. 边界: ...
   预期: ...

## 验收标准
- [ ] 功能完成
- [ ] 测试通过
```

## 创建步骤（gh CLI）

1. 先创建父 issue，记录编号 `P`。
2. 逐个创建子 issue，body 中写 `parent: #P` 与可选 `depends-on`。
3. 回填父 issue 的 task list：`- [ ] #<sub>`。
4. 需要触发自动流程时，再给子 issue 增加 `bot:fix`（按项目策略）。

## 快速命令示例

```bash
# 创建父 issue
gh issue create --title "feat: <parent-title>" --body-file /tmp/parent.md --label enhancement

# 创建子 issue（示例）
gh issue create --title "sub: <task-title>" --body-file /tmp/sub1.md --label enhancement
```
