---
name: backend-file-analysis
description: 对后端源码文件生成严格遵循模板的分析文档。读源码→提取结构→按模板填充所有章节（职责/行为意图/行为/约束/输入输出/调用链/副作用/异常处理/PRD映射）。
allowed-tools: Read, Grep, Glob, Bash, Write, Edit
user-invocable: true
---

# 后端文件分析文档生成

## Skill 文件结构

```
backend-file-analysis/
├── SKILL.md              # 本文件（执行指南）
├── template.md           # 分析文档模板（严格遵守，所有章节必填）
└── extraction-rules.md   # 各语言提取规则参考
```

- **template.md**：生成文档时必须严格按此模板的章节顺序和结构输出，不可省略任何章节。
- **extraction-rules.md**：按语言列出公开函数、类型签名、错误模式、模块调用、副作用等的匹配模式。

## 目标

对指定的后端源码文件，生成严格遵循模板的分析文档。文档路径与源文件一一对应：`docs/backend-trace/files/<源文件相对路径>.md`。

## 适用范围

- 支持语言：Elixir、Go、Rust、PHP、TypeScript
- 输入：一个或多个源码文件路径（或目录）
- 输出：per-file 分析文档（markdown）

## 用法

```
/backend-file-analysis <源文件路径或目录> [--project-root <项目根目录>] [--docs-dir <文档输出目录>]
```

示例：
```
/backend-file-analysis src/gateway/router.ts
/backend-file-analysis lib/shop/order/
/backend-file-analysis app/controllers/ShopController.php --project-root ~/document/upfit
```

## 模板（严格遵守，不可省略任何章节）

每个文件的分析文档必须包含以下 **全部 10 个章节**，顺序固定：

```markdown
# <源文件相对路径>

## 职责
- 一句话描述核心功能边界、上下游角色。

## 行为意图
- 该文件存在的目的（为什么要做这件事，输入语义/业务触发）
- 与上层能力的关系（如 Gateway/agent/cron/频道/插件/infra 等）

## 行为
- 关键流程（按步骤、时序、分支）
- 对输入的处理策略（参数校验、分发、持久化、失败路径）

## 约束
- 约束条件（权限、环境、依赖、配置前置）
- 失败边界（超时、重试、幂等、降级）
- 安全边界（认证、授权、越权防护、执行审批）

## 输入输出
- 输入对象（必填/可选字段）
- 输出对象（返回值、事件、回调、持久化副作用）

## 调用链位置
- 上游调用者（谁会调用它）
- 下游依赖（调用了哪些内部文件）

### 本地依赖（下游）
- 列出关键依赖文件

### 上游引用
- 列出直接调用方

## 状态与副作用
- FS 读写
- 网络/进程调用
- 事件发布/订阅
- 日志与审计

## 异常处理
- 异常来源（参数错误、权限失败、超时、外部服务异常）
- 处理策略（重试/返回错误码/补偿）

## 与 PRD 需求映射
- 关联 PRD 标签（如 `gateway-routing`、`agent-runtime`、`session-management`）

## 溯源证据
- `source: <源文件路径>`
- 证据时间：`YYYY-MM-DD`
- 关键代码定位（函数名 / 常量 / 分支）
```

## 执行步骤

### 1. 确定文件列表

- 如果输入是文件：直接分析该文件
- 如果输入是目录：递归收集源码文件（排除测试文件、__tests__、node_modules、_build、target 等）
- 自动检测语言：`.ex`/`.exs` → Elixir，`.go` → Go，`.rs` → Rust，`.php` → PHP，`.ts`/`.tsx` → TypeScript

### 2. 逐文件分析（读源码 + 提取结构）

对每个源码文件，执行以下提取：

**a) 结构提取（确定性，直接从代码读取）**

| 提取项 | Elixir | Go | Rust | PHP | TypeScript |
|--------|--------|-----|------|-----|------------|
| 公开函数 | `def` | 大写开头 | `pub fn` | `public function` | `export function/class` |
| 类型签名 | `@spec` | 函数签名 | 返回类型 | PHPDoc | TS 类型标注 |
| 错误模式 | `{:error, _}` | `error` 返回 | `Result::Err` | `throw` | `throw/reject` |
| 模块调用 | `Mod.func()` | `pkg.Func()` | `crate::func()` | `Class::method()` | `import from` |
| 模块文档 | `@moduledoc` | `// Package` | `//!` doc | `/** */` | JSDoc |
| 副作用 | GenServer/Task/HTTP | goroutine/net | async/tokio | curl/file | async/fetch/fs |

**b) 语义分析（需要理解代码逻辑）**

- 行为意图：读函数体、注释、模块上下文，理解"为什么存在"
- 约束：读 guard clause、权限检查、配置依赖
- 行为流程：读控制流（if/case/cond/match），理解处理步骤和分支
- 异常处理策略：读 rescue/catch/try 块，理解恢复逻辑
- PRD 映射：结合项目文档目录中的 PRD/需求文档，关联标签

### 3. 上游引用分析

对每个文件，搜索项目中哪些文件引用了它：
- Elixir：`grep` 搜索 `alias ModuleName` 或 `ModuleName.func`
- Go：`grep` 搜索 `import "pkg/path"` 或 `pkg.Func`
- TypeScript：`grep` 搜索 `from './path'` 或 `require('./path')`
- PHP：`grep` 搜索 `use ClassName` 或 `ClassName::`

### 4. 生成文档

- 输出路径：`<docs-dir>/files/<源文件相对路径>.md`
- 严格按模板顺序填充所有章节
- 无法确定的内容写 `（待补充：<原因>）` 而非留空
- 溯源证据自动填入源文件路径、当前日期、关键函数名

### 5. 质量自检

生成后自检：
- [ ] 10 个章节全部存在
- [ ] 行为意图和约束不是空的或纯 TODO
- [ ] 输入输出有具体的参数/返回类型
- [ ] 调用链有上下游至少各一项
- [ ] 副作用列出了具体类型（而非"无"——如果确实无副作用则写"纯函数，无副作用"）

## 批量模式

处理目录时，先输出文件清单和进度，逐个生成：

```
扫描: src/gateway/ → 12 个源文件
[1/12] src/gateway/router.ts → docs/backend-trace/files/src/gateway/router.ts.md ✓
[2/12] src/gateway/middleware.ts → docs/backend-trace/files/src/gateway/middleware.ts.md ✓
...
完成: 12/12，全部章节齐全
```

## 注意事项

- 不要生成空章节或纯占位章节，每个章节必须有实质内容
- 行为意图要写"为什么"而非"做什么"——"做什么"是行为章节的事
- 约束要区分三类：前置条件（环境/依赖）、失败边界（超时/重试）、安全边界（权限/认证）
- 若源文件是入口/路由文件，行为章节必须写明路由条件和分支触发
- 若涉及副作用，必须写明副作用类型和可观测点（日志/文件/事件/状态）
- PRD 映射如果项目没有 PRD 文档，写 `（项目无 PRD 文档，跳过映射）`
