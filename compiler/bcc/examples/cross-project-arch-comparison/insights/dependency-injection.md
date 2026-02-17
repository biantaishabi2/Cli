# 依赖注入的架构界定

> 为什么 Jido 的工具注入不是架构违反？

## 问题的核心

当 AST 分析检测到 `agent.ex` 引用了 `Gong.Tools.Bash`，这是否意味着架构违反？

**答案取决于**：这是**直接依赖**还是**依赖注入**。

## 架构契约 vs 实现机制

### 架构契约（应该定义在 seed 中）

```
┌─────────────────────────────────────────┐
│  AGENT 模块 ────────> TOOLS 模块        │
│  (需要工具功能)      (提供工具功能)      │
└─────────────────────────────────────────┘
              ↑
        这是架构契约！
        "Agent 需要调用工具"
```

### 实现机制（实现细节，不影响架构）

| 机制 | 代码示例 | 说明 |
|-----|---------|------|
| **直接导入** | `import { Bash } from './tools'` | 代码直接依赖 |
| **构造函数注入** | `new Agent({ tools: [Bash] })` | 外部传入依赖 |
| **框架注入** | `use Jido, tools: [Bash]` | 框架管理依赖 |
| **配置注入** | 从配置文件读取工具列表 | 运行时动态加载 |
| **服务发现** | 从注册表查找工具 | 运行时服务定位 |

## Jido 工具注入详解

### 代码表现

```elixir
defmodule Gong.Agent do
  use Jido.AI.ReActAgent,
    tools: [
      Gong.Tools.Bash,
      Gong.Tools.Write,
      ...
    ]
end
```

### 运行时架构

```
┌─────────────────────────────────────────┐
│  Agent.ex (你的代码)                    │
│  - 声明：我需要这些工具                 │
│  - 不直接实例化工具                     │
│  - 不直接调用工具                       │
└──────────────┬──────────────────────────┘
               │ use Jido.AI.ReActAgent
               │ "框架，请帮我管理这些工具"
               ▼
┌─────────────────────────────────────────┐
│  Jido 框架                              │
│  - 实例化工具                           │
│  - 管理工具生命周期                     │
│  - 调度 Action 执行                     │
│  - 处理工具结果                         │
└──────────────┬──────────────────────────┘
               │ 框架调用
               ▼
┌─────────────────────────────────────────┐
│  Tools.Bash (工具实现)                  │
│  - 实现 Jido.Action 行为                │
│  - 被框架调用，不依赖 Agent             │
└─────────────────────────────────────────┘
```

### 关键特征

| 维度 | 直接依赖 | Jido 注入 |
|-----|---------|----------|
| **谁实例化** | Agent 自己 | Jido 框架 |
| **谁调用** | Agent 直接调用 | Jido 调度 |
| **结果处理** | Agent 自己处理 | Jido 回调 |
| **可替换性** | 需修改代码 | 配置即可 |
| **耦合度** | 紧耦合 | 松耦合 |

## 为什么 AST 检测到了但不是违反？

### AST 检测的是什么？

AST 分析检测的是**代码层面的引用关系**：
```elixir
# AST 看到：agent.ex 引用了 Gong.Tools.Bash
tools: [Gong.Tools.Bash]
```

### 架构验证应该关注什么？

架构验证应该关注**功能依赖关系**：
```
AGENT 模块是否需要 TOOLS 模块的功能？
→ 是的，Agent 需要调用工具
→ 这是正当的架构依赖
```

### 结论

**AST 检测到的 ≠ 架构违反**

需要结合上下文判断：
1. **依赖方向是否正确**：AGENT → TOOLS ✅
2. **是否通过框架隔离**：Jido 框架 ✅
3. **是否可替换**：配置即可替换 ✅

## 如何在 seed 中体现？

### 方案 1：标注注入类型（推荐）

```yaml
relations_expected:
  - caller: AGENT
    callee: TOOLS
    allowed: true
    rationale: "Agent 需要调用工具完成用户任务"
    injection_type: "framework"  # framework | constructor | config
    framework: "Jido"
```

### 方案 2：分层定义

```yaml
# 架构契约层
architecture_contract:
  AGENT:
    depends_on: [PROMPT, TOOLS, DATA]
    
# 实现机制层
implementation_details:
  AGENT:
    tools:
      injection: "Jido.AI.ReActAgent"
      list: ["Read", "Write", "Bash", ...]
```

## 对比：直接依赖 vs 依赖注入

### 直接依赖（架构违反风险高）

```typescript
// PI-Mono 风格的直接依赖
import { bashTool } from './tools/bash';

class Agent {
  constructor() {
    this.bash = bashTool;  // 直接引用
  }
  
  async run() {
    const result = await this.bash.execute(cmd);  // 直接调用
    // 自己处理结果
  }
}
```

**问题**：
- Agent 和 Tool 紧耦合
- 替换工具需要修改 Agent 代码
- 测试时需要 mock 具体工具

### 依赖注入（架构健康）

```elixir
# Gong/Jido 风格的依赖注入
defmodule Gong.Agent do
  use Jido.AI.ReActAgent,
    tools: [Gong.Tools.Bash]  # 声明式注入
    
  # Agent 不直接调用工具
  # Jido 框架负责调度
end
```

**优点**：
- Agent 和 Tool 解耦
- 替换工具只需改配置
- 测试时框架提供 mock

## 总结

### 核心原则

1. **架构契约**：模块 A 是否需要模块 B 的功能？
   - 如果是，应该在 seed 中定义
   - 与实现机制无关

2. **实现机制**：如何获取模块 B 的实例？
   - 直接 import：紧耦合
   - 依赖注入：松耦合
   - 框架注入：最松耦合

3. **验证重点**：
   - 依赖方向是否正确
   - 是否存在循环依赖
   - 分层架构是否被违反

### 对 Gong 的重新评估

**AGENT → TOOLS 不是架构违反**，因为：
1. 架构上：Agent 确实需要 Tools 的功能
2. 实现上：通过 Jido 框架注入，松耦合
3. 方向上：AGENT → TOOLS 是正确的分层

**真正的问题**：Seed 文件漏定义了这条正当依赖！
