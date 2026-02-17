# 架构整改方案

> 基于架构分析结果，为 Gong 和 PI-Mono 提供具体的修复建议。

## 概述

本文档基于 `bcc arch` 分析结果，为两个项目提供架构整改方案：

| 项目 | 健康度 | 主要问题 | 整改难度 |
|-----|--------|---------|---------|
| **Gong** | ✅ 良好 | 2 条轻微额外边 | 低 |
| **PI-Mono** | ⚠️ 需关注 | 15 条额外边，含循环依赖 | 中-高 |

---

## Gong 整改方案

### 问题清单

| 边 | 严重程度 | 原因 | 整改建议 |
|---|---------|------|---------|
| INFRA → PROVIDERS | ✅ **非违反** | Seed 漏定义 | 补充 seed 定义 |
| TOOLS → COMPACTION | ⚠️ 轻微 | truncate 是通用工具 | 移到 utils |

### 分析：INFRA → PROVIDERS

**重要发现**：这不是架构违反，而是 **BCC 提取问题 + Seed 漏定义**！

**代码分析**:
```elixir
# application.ex
ReqLLM.Providers.register(Gong.Providers.DeepSeek)
```

**依赖关系**:
- `ReqLLM` 是 **外部依赖库**（`{:req_llm, "~> 1.5"}`）
- `Gong.Providers.DeepSeek` 是 **内部模块**

**正确的架构理解**:
```
INFRA (Application) ──> PROVIDERS (Gong.Providers.DeepSeek)
                              │
                              └── use ReqLLM.Provider (外部库)
```

**问题根源**:
1. BCC AST 提取可能混淆了 `ReqLLM.Providers` 和 `gong/providers`
2. Seed 漏定义了 `INFRA → PROVIDERS` 这条正当依赖

**修复方案**：补充 Seed 定义（不是修改代码）

```yaml
# seed.yaml 补充
relations_expected:
  - caller: INFRA
    callee: PROVIDERS
    allowed: true
    rationale: "Application 启动时需要注册 Provider"
```

**可选优化**（配置化，非必须）:
如果希望更灵活，可以将 Provider 配置化：
```elixir
# 从配置读取
providers = Application.get_env(:gong, :providers, [Gong.Providers.DeepSeek])
Enum.each(providers, &ReqLLM.Providers.register/1)
```

但这属于功能增强，不是架构修复。

---

### 修复 2：Truncate 模块位置调整

**问题**: `truncate.ex` 被 TOOLS 使用，但放在 COMPACTION 模块下

**当前结构**:
```
lib/gong/
  compaction.ex
  truncate.ex          # <- 这里
  tools/
    bash.ex            # <- 使用 Truncate
```

**修复后结构**:
```
lib/gong/
  compaction.ex
  utils/
    output.ex          # <- 移到这里，改名
  tools/
    bash.ex            # <- 使用 Utils.Output
```

**代码变更**:
```elixir
# 修复前
import Gong.Truncate

# 修复后
import Gong.Utils.Output
```

**收益**:
- 解除 TOOLS → COMPACTION 的不当依赖
- truncate 作为通用工具，位置更合理

---

### 修复 3：补充 Seed 定义（重要）

**问题**: Seed 漏定义了 AGENT → TOOLS

**当前 seed**:
```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  # 缺少 AGENT -> TOOLS！
```

**修复后 seed**:
```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
    rationale: "Agent 需要动态构建 system prompt"
    
  - caller: AGENT
    callee: TOOLS
    allowed: true
    rationale: "Agent 需要调用工具完成用户任务"
    injection_type: "framework"  # 标注为框架注入
    
  # 其他定义...
```

**收益**:
- 消除"假阳性"的额外边报告
- 文档化架构设计决策

---

## PI-Mono 整改方案

### 问题分类

| 类别 | 问题 | 数量 | 优先级 |
|-----|------|------|--------|
| 循环依赖 | CORE ↔ CLI | 1 | P0 |
| 反向分层 | AI_CORE → PROVIDERS | 1 | P0 |
| 过多依赖 | CORE → 各子模块 | 4 | P1 |
| 交叉依赖 | EXTENSIONS/MODES 互相依赖 | 9 | P2 |

---

### 修复 1：打破循环依赖（P0）

**问题**: `PI_CODING_CORE → PI_CODING_CLI`

**文件**: `coding-agent/src/core/model-resolver.ts`

**当前代码**:
```typescript
// 从 CLI 导入函数
import { isValidThinkingLevel } from "../cli/args.js";
```

**修复方案**:

1. **创建共享 utils**
   ```typescript
   // coding-agent/src/utils/validation.ts
   export function isValidThinkingLevel(level: string): boolean {
     return ["low", "medium", "high"].includes(level);
   }
   ```

2. **更新导入**
   ```typescript
   // core/model-resolver.ts
   import { isValidThinkingLevel } from "../utils/validation.js";
   
   // cli/args.ts
   import { isValidThinkingLevel } from "../utils/validation.js";
   ```

**工作量**: 1-2 天

**收益**: 打破循环依赖，架构更清晰

---

### 修复 2：修复反向分层（P0）

**问题**: `PI_AI_CORE → PI_AI_PROVIDERS`

**文件**: `ai/src/stream.ts`

**当前代码**:
```typescript
import { registerBuiltins } from "./providers/register-builtins.js";

// 直接调用
registerBuiltins();
```

**修复方案**:

1. **定义接口**
   ```typescript
   // ai/src/provider-registry.ts
   export interface ProviderRegistry {
     register(): void;
   }
   ```

2. **修改 stream.ts**
   ```typescript
   // ai/src/stream.ts
   import type { ProviderRegistry } from "./provider-registry.js";
   
   export class AIStream {
     constructor(private registry: ProviderRegistry) {}
     
     init() {
       this.registry.register();
     }
   }
   ```

3. **在应用启动时注入**
   ```typescript
   // main.ts
   import { AIStream } from "./ai/stream.js";
   import { BuiltinProviderRegistry } from "./ai/providers/register-builtins.js";
   
   const registry = new BuiltinProviderRegistry();
   const stream = new AIStream(registry);
   stream.init();
   ```

**工作量**: 2-3 天

**收益**: 
- 正确分层（PROVIDERS → AI_CORE）
- 可测试性提升（可注入 mock registry）

---

### 修复 3：引入依赖注入（P1）

**问题**: `PI_CODING_CORE` 依赖了 TOOLS、COMPACTION、EXTENSIONS、MODES

**当前架构**:
```
CORE -> TOOLS
CORE -> COMPACTION
CORE -> EXTENSIONS
CORE -> MODES
```

**目标架构**:
```
        CORE (定义接口)
         ^
         | 依赖注入
   TOOLS/COMPACTION/EXTENSIONS/MODES (实现接口)
```

**修复步骤**:

1. **定义接口**
   ```typescript
   // coding-agent/src/core/interfaces.ts
   
   export interface Tool {
     name: string;
     execute(args: any): Promise<ToolResult>;
   }
   
   export interface ToolRegistry {
     register(tool: Tool): void;
     get(name: string): Tool | undefined;
   }
   
   export interface CompactionStrategy {
     compact(messages: Message[]): CompactResult;
   }
   
   export interface ExtensionLoader {
     load(extensions: string[]): Promise<Extension[]>;
   }
   ```

2. **CORE 只依赖接口**
   ```typescript
   // coding-agent/src/core/agent-session.ts
   import type { ToolRegistry, CompactionStrategy } from "./interfaces.js";
   
   export class AgentSession {
     constructor(
       private tools: ToolRegistry,
       private compaction: CompactionStrategy,
       // ... 其他接口
     ) {}
     
     async run(input: string) {
       // 使用接口，不关心具体实现
       const tool = this.tools.get("bash");
       const result = await tool.execute({ command: input });
     }
   }
   ```

3. **各模块实现接口**
   ```typescript
   // coding-agent/src/tools/registry.ts
   import type { Tool, ToolRegistry } from "../core/interfaces.js";
   
   export class ToolRegistryImpl implements ToolRegistry {
     private tools = new Map<string, Tool>();
     
     register(tool: Tool) {
       this.tools.set(tool.name, tool);
     }
     
     get(name: string): Tool | undefined {
       return this.tools.get(name);
     }
   }
   
   // coding-agent/src/tools/bash.ts
   import type { Tool } from "../core/interfaces.js";
   
   export class BashTool implements Tool {
     name = "bash";
     async execute(args: { command: string }) {
       // 实现
     }
   }
   ```

4. **应用启动时组装**
   ```typescript
   // coding-agent/src/main.ts
   import { AgentSession } from "./core/agent-session.js";
   import { ToolRegistryImpl } from "./tools/registry.js";
   import { BashTool } from "./tools/bash.js";
   import { CompactionStrategyImpl } from "./compaction/strategy.js";
   // ...
   
   // 组装依赖
   const tools = new ToolRegistryImpl();
   tools.register(new BashTool());
   tools.register(new WriteTool());
   // ...
   
   const compaction = new CompactionStrategyImpl();
   
   const agent = new AgentSession(tools, compaction, ...);
   ```

**工作量**: 1-2 周

**收益**:
- CORE 模块瘦身，职责清晰
- 各模块可独立测试
- 易于扩展新工具/策略

---

### 修复 4：事件驱动解耦（P2）

**问题**: EXTENSIONS、MODES 之间交叉依赖

**修复方案**: 引入 EventBus

1. **定义 EventBus**
   ```typescript
   // coding-agent/src/core/event-bus.ts
   type EventHandler = (event: any) => void;
   
   export class EventBus {
     private handlers = new Map<string, EventHandler[]>();
     
     on(event: string, handler: EventHandler) {
       if (!this.handlers.has(event)) {
         this.handlers.set(event, []);
       }
       this.handlers.get(event)!.push(handler);
     }
     
     emit(event: string, data: any) {
       const handlers = this.handlers.get(event) || [];
       handlers.forEach(h => h(data));
     }
   }
   
   export const eventBus = new EventBus();
   ```

2. **EXTENSIONS 发布事件**
   ```typescript
   // coding-agent/src/extensions/loader.ts
   import { eventBus } from "../core/event-bus.js";
   
   export function loadExtension(ext: Extension) {
     // ...
     eventBus.emit("extension:loaded", { extension: ext });
   }
   ```

3. **MODES 订阅事件**
   ```typescript
   // coding-agent/src/modes/interactive.ts
   import { eventBus } from "../core/event-bus.js";
   
   eventBus.on("extension:loaded", (event) => {
     // 响应扩展加载
     registerExtensionCommands(event.extension);
   });
   ```

**工作量**: 1 周

**收益**:
- 模块间解耦
- 易于添加新功能（只需订阅事件）
- 更好的可测试性

---

## 整改路线图

### Gong（1 周内完成）

| 天数 | 任务 | 产出 |
|-----|------|------|
| 1 | Seed 补充（AGENT→TOOLS, INFRA→PROVIDERS） | PR #1 |
| 2 | Truncate 移动 | PR #2 |
| 3-4 | 验证 & 合并 | 完成 |

**注意**：INFRA→PROVIDERS 不是架构违反，是 Seed 漏定义！

### PI-Mono（1 个月内完成）

| 周 | 任务 | 产出 |
|---|------|------|
| 1 | 打破循环依赖 | PR #1 |
| 1-2 | 修复反向分层 | PR #2 |
| 2-3 | 引入依赖注入 | PR #3 |
| 3-4 | 事件驱动改造 | PR #4 |
| 4 | 验证 & 文档 | 完成 |

---

## 参考：从 Gong 学到的

| 问题 | PI-Mono (TypeScript) | Gong (Elixir) | 借鉴方案 |
|-----|---------------------|---------------|---------|
| 循环依赖 | 需要手动解耦 | OTP 监督树阻止 | 使用 IoC 容器 |
| 依赖注入 | 需自建框架 | Jido 内置 | 使用 `tsyringe` |
| 模块边界 | 靠约定 | Behaviours 强制 | 使用 `interface` |
| 配置驱动 | 部分实现 | 完全配置化 | 统一配置管理 |

### 推荐的 TypeScript IoC 库

1. **tsyringe** (推荐)
   ```typescript
   import { container, injectable, inject } from "tsyringe";
   
   @injectable()
   class AgentSession {
     constructor(@inject("ToolRegistry") private tools: ToolRegistry) {}
   }
   ```

2. **inversify**
   ```typescript
   import { Container, injectable, inject } from "inversify";
   
   @injectable()
   class AgentSession {
     constructor(@inject(TYPES.ToolRegistry) private tools: ToolRegistry) {}
   }
   ```

---

## 验证整改效果

整改后重新运行架构验证：

```bash
# Gong
cd projects/gong
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir ../../analysis/gong-matrix --force

# PI-Mono
cd projects/pi-mono
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir ../../analysis/pi-mono-matrix --force

# 对比整改前后
diff analysis/gong-matrix/v3.transition-matrix.yaml \
     analysis/gong-matrix-fixed/v3.transition-matrix.yaml
```

**期望结果**:
- Gong: 额外边从 3 条减少到 1 条（TOOLS→COMPACTION）
  - INFRA→PROVIDERS 和 AGENT→TOOLS 是 Seed 漏定义，不是违反
- PI-Mono: 额外边从 15 条减少到 5 条以内
