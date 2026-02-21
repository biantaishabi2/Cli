# Gong 架构依赖图 (seed-v4)

> 由 `bcc arch export-mermaid --seed-file seed-v4.yaml` 自动生成

- 节点形状：`([圆角])` = core, `[方框]` = support, `[(圆柱)]` = generic
- 边样式：`-->` = 显式 allowed, `-.->` = 继承 allowed, `<-.->` = 兄弟模块, `-.-x` = forbidden
- 分组：按 layer 分 subgraph，子模块嵌套在父模块内（蓝色边框高亮）

```mermaid
graph TD
  subgraph application["application"]
    subgraph AGENT["Agent 运行时"]
      AGENT_CORE(["Agent 核心逻辑"])
      AGENT_LOOP(["Agent 循环控制"])
    end
    SESSION(["会话协调"])
    PROMPT(["Prompt 系统"])
    HOOK["Hook 系统"]
    COMPACTION["压缩系统"]
    EXTENSIONS["扩展系统"]
    STREAM["流式输出"]
    TOOLS["工具集"]
    DATA["数据层"]
  end
  subgraph infrastructure["infrastructure"]
    PROVIDERS[("LLM Providers")]
    RUNTIME[("运行时支撑")]
    TAPE[("存储层")]
    INFRA[("基础设施")]
    CLI[("命令行入口")]
    BDD[("BDD 测试基础设施")]
  end
  style AGENT fill:#e8f4fd,stroke:#1a73e8,stroke-width:2px,stroke-dasharray:none

  SESSION --> STREAM
  SESSION --> RUNTIME
  SESSION --> PROVIDERS
  COMPACTION --> PROVIDERS
  COMPACTION --> PROMPT
  COMPACTION --> TAPE
  EXTENSIONS --> SESSION
  TOOLS --> DATA
  RUNTIME --> PROVIDERS
  INFRA --> PROVIDERS
  INFRA --> PROMPT
  INFRA --> SESSION
  CLI --> SESSION
  AGENT_CORE -.-> PROMPT
  AGENT_LOOP -.-> PROMPT
  AGENT_CORE -.-> DATA
  AGENT_LOOP -.-> DATA
  AGENT_CORE -.-> TOOLS
  AGENT_LOOP -.-> TOOLS
  AGENT_CORE -.-> HOOK
  AGENT_LOOP -.-> HOOK
  AGENT_CORE -.-> EXTENSIONS
  AGENT_LOOP -.-> EXTENSIONS
  AGENT_CORE -.-> RUNTIME
  AGENT_LOOP -.-> RUNTIME
  AGENT_CORE -.-> STREAM
  AGENT_LOOP -.-> STREAM
  SESSION -.-> AGENT_CORE
  SESSION -.-> AGENT_LOOP
  AGENT_CORE <-.-> AGENT_LOOP
  AGENT_LOOP <-.-> AGENT_CORE
  TAPE -.-x SESSION
  DATA -.-x AGENT_CORE
  DATA -.-x AGENT_LOOP
  TOOLS -.-x AGENT_CORE
  TOOLS -.-x AGENT_LOOP
```
