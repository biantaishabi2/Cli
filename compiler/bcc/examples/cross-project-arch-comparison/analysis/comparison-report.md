# Gong vs PI-Mono 架构对比分析报告

> 生成时间：2026-02-17  
> 工具版本：bcc v3

## 执行摘要

| 指标 | Gong (Elixir) | PI-Mono (TypeScript) |
|-----|---------------|---------------------|
| 文件数 | 78 | 385 |
| 模块数 | 8 | 18 |
| 期望边 | 7 (v1) → **9 (v2)** | 19 |
| 实际命中 | 7 (100%) → **9 (100%)** | 19 (100%) |
| 额外边 | 3 (v1) → **1 (v2)** | 15 |
| 架构健康度 | ✅ **优秀** (v2) | ⚠️ 需要关注 |

> **重要更新**：通过完善 Seed 定义（v1 → v2），Gong 的额外边从 3 条减少到 1 条。
> 
> 详细对比参见：[Gong Seed v1 vs v2 对比报告](./gong-seed-v1-v2-comparison.md)

## 模块映射对比

### 核心模块对应关系

| PI-Mono | Gong | 对应状态 | 说明 |
|---------|------|---------|------|
| PI_CODING_CORE | AGENT + PROMPT | ⚠️ 拆分 | Gong 将 Agent 核心和 Prompt 分开 |
| PI_CODING_TOOLS | TOOLS | ✅ 一致 | 都是工具实现 |
| PI_CODING_COMPACTION | COMPACTION | ✅ 一致 | 都是上下文压缩 |
| PI_CODING_EXTENSIONS | EXTENSIONS | ✅ 一致 | 都是扩展系统 |
| PI_AI_CORE + PI_AI_PROVIDERS | PROVIDERS | ⚠️ 合并 | Gong 简化了 AI 层 |
| PI_CODING_CLI | INFRA | ⚠️ 扩大 | INFRA 还包含 Application |
| PI_CODING_MODES | (无) | ❌ 缺失 | Gong 没有 TUI 交互模式 |
| (无显式定义) | DATA | ✅ 新增 | Gong 显式定义了数据层 |

### 架构差异说明

**Gong 缺少的模块**：
- TUI 组件层（符合设计意图，Gong 是 headless agent）
- 交互模式层（Gong 只有简单的 Agent Loop）

**Gong 新增的模块**：
- DATA 层（显式定义 tool_result 等数据，更好的设计）

## 依赖边详细对比

### Gong 的依赖边

#### 期望边（全部命中）

| 边 | 对应 PI-Mono | 状态 |
|---|-------------|------|
| AGENT → PROMPT | PI_CODING_CORE → system-prompt | ✅ |
| AGENT → DATA | (无显式对应) | ✅ 新增 |
| COMPACTION → PROVIDERS | PI_CODING_COMPACTION → PI_AI_CORE | ✅ |
| EXTENSIONS → AGENT | PI_CODING_EXTENSIONS → PI_CODING_CORE | ✅ |
| INFRA → EXTENSIONS | (无显式对应) | ✅ 启动依赖 |
| INFRA → COMPACTION | (无显式对应) | ✅ 启动依赖 |
| TOOLS → DATA | (无显式对应) | ✅ 数据依赖 |

#### 额外边分析（基于 v1 Seed）

| 边 | 分析 | 判定 | 建议 |
|---|------|------|------|
| AGENT → TOOLS | Seed 漏定义，正当依赖 | ✅ **非违反** | 补充 seed 定义 |
| INFRA → PROVIDERS | Seed 漏定义 + BCC 提取问题 | ✅ **非违反** | 补充 seed 定义 |
| TOOLS → COMPACTION | truncate 是通用工具 | ⚠️ 轻微违反 | 移到 utils |

**重要发现**：Gong 的 3 条"额外边"中，**2 条是 Seed 漏定义，不是架构违反**！

- **AGENT → TOOLS**: Agent 需要调用工具，正当依赖
- **INFRA → PROVIDERS**: Application 启动时注册 Provider，正当依赖（ReqLLM 是外部库）
- **TOOLS → COMPACTION**: 唯一真正的轻微违反（模块职责不清）

#### v2 Seed 改进结果

通过补充 2 条漏定义的依赖，Gong 的架构健康度显著提升：

| 版本 | 期望边 | 额外边 | 架构健康度 |
|-----|--------|--------|----------|
| v1 | 7 条 | 3 条 | ⚠️ 有 2 条假阳性 |
| **v2** | **9 条** | **1 条** | ✅ **优秀** |

**详细对比**：[Gong Seed v1 vs v2 对比报告](./gong-seed-v1-v2-comparison.md)

### PI-Mono 的依赖边

#### 期望边（全部命中）

参见 `pi-mono-matrix/v3.target-matrix.yaml`

#### 额外边分析（15 条）

| 边 | 严重程度 | 分析 |
|---|---------|------|
| PI_AI_CORE → PI_AI_PROVIDERS | ❌❌ 严重 | 反向分层，核心层不应依赖实现层 |
| PI_CODING_CORE → PI_CODING_CLI | ❌❌ 严重 | 循环依赖，CORE 不应知道 CLI |
| PI_CODING_CORE → PI_CODING_MODES | ❌ 违反 | CORE 不应依赖具体交互模式 |
| PI_CODING_CORE → PI_CODING_TOOLS | ⚠️ 轻微 | 应该通过抽象层访问 |
| 其他 11 条 EXTENSIONS/MODES 相关 | ⚠️ 中等 | 模块间耦合度过高 |

## 架构健康度评估

### Gong：✅ 优秀（v2）

**优点**：
1. 分层清晰，没有循环依赖
2. Elixir/OTP 监督树天然限制不良依赖
3. Jido 框架提供良好的依赖注入机制
4. 显式定义 DATA 层，职责清晰
5. **完善 Seed 后，只有 1 条轻微违反**（v2）

**问题**（v1 → v2 改进）：
1. ~~Seed 漏定义 AGENT → TOOLS~~ ✅ v2 已补充
2. ~~Seed 漏定义 INFRA → PROVIDERS~~ ✅ v2 已补充
3. truncate 模块位置不当（唯一轻微违反）

**关键洞察**：
Gong 的架构实际上**非常健康**。v1 的 3 条"额外边"中，2 条是 Seed 漏定义，1 条是轻微违反。
通过完善 Seed，架构健康度从"良好"提升到"优秀"。

参见：[Gong Seed v1 vs v2 详细对比](./gong-seed-v1-v2-comparison.md)

### PI-Mono：⚠️ 需要关注

**优点**：
1. 模块化程度高，18 个模块划分细致
2. 功能完整，包含 TUI、WebUI 等
3. 所有期望边都已实现

**问题**：
1. 循环依赖：CORE ↔ CLI
2. 反向分层：AI_CORE → PROVIDERS
3. 模块间耦合度过高，15 条额外边
4. TypeScript 灵活性导致隐式依赖多

## 关键洞察

### 洞察 1：依赖注入 vs 直接依赖

**Gong 的 Jido 注入**：
```elixir
use Jido.AI.ReActAgent,
  tools: [Gong.Tools.Bash]
```
- 框架管理依赖生命周期
- Agent 和 Tool 松耦合
- 不是架构违反

**PI-Mono 的直接依赖**：
```typescript
import { bashTool } from './tools/bash';
```
- 代码直接引用
- 紧耦合
- 容易形成循环依赖

### 洞察 2：Seed 定义完整性

Gong 的"额外边" AGENT → TOOLS 实际上是：
- **不是架构违反**
- **是 seed 漏定义**

修复：补充 AGENT → TOOLS 到 relations_expected

### 洞察 3：语言/框架对架构的影响

| 维度 | Elixir/OTP | TypeScript/Node |
|-----|------------|-----------------|
| 循环依赖 | 编译期阻止 | 运行时才暴露 |
| 模块系统 | Behaviours 强制接口 | 自由导入 |
| 监督结构 | 监督树强制分层 | 无强制 |
| 依赖注入 | 宏/行为支持 | 需额外框架 |

## 建议

### 对 Gong

**已完成的改进（v1 → v2）**：

参见：[Gong Seed v1 vs v2 对比报告](./gong-seed-v1-v2-comparison.md)

1. **✅ 已修复：补充 Seed 定义**
   - AGENT → TOOLS（Jido 框架注入）
   - INFRA → PROVIDERS（启动时注册）

2. **可选优化：Provider 配置化**
   - 从配置读取，不要硬编码
   - 当前硬编码也是可接受的

3. **建议修复：整理 truncate 模块**
   - 移到 utils 或单独的工具模块
   - 这是唯一的轻微架构违反

### 对 PI-Mono

1. **解除循环依赖**
   - CORE 不应依赖 CLI
   - 使用依赖注入或事件机制

2. **修复反向分层**
   - AI_CORE 不应依赖 PROVIDERS
   - 使用接口/抽象层

3. **降低模块耦合**
   - 梳理 EXTENSIONS/MODES 的依赖关系
   - 引入依赖注入框架

### 对 bcc 工具

1. **识别依赖注入模式**
   - Elixir: `use Framework, tools: [...]`
   - TypeScript: `@Injectable()`

2. **Seed 完整性检查**
   - 检测可能的漏定义依赖
   - 给出补充建议

3. **架构健康度评分**
   - 循环依赖检测
   - 反向分层检测
   - 耦合度分析

## 附录

### 生成命令

```bash
# Gong
cd projects/gong
bcc arch matrix \
  --seed-file seed.yaml \
  --ast-file ast.json \
  --out-dir ../../analysis/gong-matrix \
  --force

# PI-Mono
cd projects/pi-mono
bcc arch matrix \
  --seed-file seed.yaml \
  --ast-file ast.json \
  --out-dir ../../analysis/pi-mono-matrix \
  --force
```

### 参考文件

#### Gong
- `gong-matrix/v3.target-matrix.yaml` (v1)
- `gong-matrix/v3.transition-matrix.yaml` (v1: 3 条额外边)
- `gong-matrix-v2/v3.target-matrix.yaml` (v2)
- `gong-matrix-v2/v3.transition-matrix.yaml` (v2: 1 条额外边)
- [Gong Seed v1 vs v2 对比报告](./gong-seed-v1-v2-comparison.md)

#### PI-Mono
- `pi-mono-matrix/v3.target-matrix.yaml`
- `pi-mono-matrix/v3.transition-matrix.yaml`

#### Seed 文件
- `projects/gong/seed.yaml` (v1)
- `projects/gong/seed-v2.yaml` (v2)
- `projects/pi-mono/seed.yaml`
