# 跨项目架构对比分析案例

> 展示如何使用 `bcc arch` 对比两个相似项目的架构设计，识别依赖注入与架构契约的区别。

## 案例概述

本案例对比分析两个编码 Agent 项目的架构：

| 项目 | 语言 | 框架 | 特点 |
|-----|------|------|------|
| **Gong** | Elixir | Jido + OTP | 基于行为树的 Agent 框架 |
| **PI-Mono** | TypeScript | 自研框架 | 多包架构的完整系统 |

通过对比，展示如何：
1. 区分**架构契约**与**实现机制**
2. 识别**依赖注入**与**直接依赖**的不同
3. 检查 **Seed 定义完整性**

## 核心洞察

### 洞察 1：依赖注入的架构界定

**问题**：AST 检测到的依赖 = 架构依赖？

**答案**：不完全是。需要区分：
- **架构契约**：模块 A 需要模块 B 的功能（应该定义在 seed 中）
- **实现机制**：如何获取模块 B 的实例（直接 import、框架注入、配置等）

**示例**：
```elixir
# Gong - Jido 框架注入
use Jido.AI.ReActAgent,
  tools: [Gong.Tools.Bash]  # 声明式注入
```
```typescript
// PI-Mono - 直接导入
import { bashTool } from './tools/bash';
```

虽然 AST 都检测到了依赖，但前者是框架管理的依赖注入，后者是直接耦合。

### 洞察 2：Seed 定义完整性

**问题**：为什么 Gong 分析显示有"额外边"？

**答案**：Seed 文件漏定义了正当的架构依赖。

Gong 的原始 seed 只定义了 7 条边，但 AGENT → TOOLS 是正当的架构依赖（Agent 需要调用工具），应该在 seed 中定义。

**修复**：补充 AGENT → TOOLS 到 relations_expected。

### 洞察 3：框架差异的识别

| 维度 | Gong (Elixir) | PI-Mono (TypeScript) |
|-----|---------------|---------------------|
| 模块系统 | OTP Behaviours | ES Modules |
| 依赖注入 | 编译期宏注入 | 运行时服务定位 |
| 监督结构 | 监督树强制分层 | 自由导入 |
| 循环依赖 | 编译期阻止 | 运行时才暴露 |

## 快速开始

### 步骤 1：查看项目配置

```bash
# Gong 项目配置
cat projects/gong/seed.yaml

# PI-Mono 项目配置
cat projects/pi-mono/seed.yaml
```

### 步骤 2：重新生成矩阵（可选）

```bash
# Gong
cd projects/gong
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir ../../analysis/gong-matrix --force

# PI-Mono
cd projects/pi-mono
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir ../../analysis/pi-mono-matrix --force
```

### 步骤 3：查看对比报告

```bash
cat analysis/comparison-report.md
```

### 步骤 4：理解依赖注入

```bash
cat insights/dependency-injection.md
cat insights/seed-completeness.md
```

## 目录结构

```
cross-project-arch-comparison/
├── README.md                    # 本文件
├── projects/                    # 项目配置和数据
│   ├── gong/
│   │   ├── seed.yaml           # Gong 模块定义
│   │   └── ast.json            # Gong AST 提取结果
│   └── pi-mono/
│       ├── seed.yaml           # PI-Mono 模块定义
│       └── ast.json            # PI-Mono AST 提取结果
├── analysis/                    # 分析产物
│   ├── gong-matrix/            # Gong 矩阵输出
│   ├── pi-mono-matrix/         # PI-Mono 矩阵输出
│   └── comparison-report.md    # 对比分析报告
└── insights/                    # 架构洞察
    ├── dependency-injection.md # 依赖注入的架构界定
    └── seed-completeness.md    # Seed 定义完整性
```

## 关键发现

### Gong 架构健康度：✅ 良好

- 期望边：7 条，全部命中
- 额外边分析：
  - AGENT → TOOLS：不是违反，是 seed 漏定义
  - INFRA → PROVIDERS：轻微违反（硬编码）
  - TOOLS → COMPACTION：模块职责不清

### PI-Mono 架构健康度：⚠️ 需要关注

- 期望边：19 条，全部命中
- 额外边：15 条，包括：
  - 循环依赖（CORE ↔ CLI）
  - 反向分层（AI_CORE → PROVIDERS）
  - 模块间高度耦合

## 与现有案例的关系

| 案例 | 场景 | 重点 |
|-----|------|------|
| `openclaw-arch` | 单一项目架构治理 | extract → matrix → validate 闭环 |
| `cross-project-arch-comparison` | 多项目架构对比 | matrix → compare → analyze 洞察 |

两个案例互补：
- `openclaw-arch`：教你如何用 bcc 治理存量项目架构
- `cross-project-arch-comparison`：教你如何用 bcc 对比项目架构，识别设计差异

## 延伸阅读

- [依赖注入的架构界定](./insights/dependency-injection.md)
- [Seed 定义完整性检查](./insights/seed-completeness.md)
- [架构闭环迁移计划](../../docs/架构闭环迁移计划.md)
