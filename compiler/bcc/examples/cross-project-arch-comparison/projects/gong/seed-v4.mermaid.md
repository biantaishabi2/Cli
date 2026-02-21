# Gong 架构依赖图 (seed-v4)

> 由 `bcc arch export-mermaid --seed-file seed-v4.yaml` 自动生成

- 节点形状：`([圆角])` = core, `[方框]` = support, `[(圆柱)]` = generic
- 边颜色：🔵 蓝色 = 允许依赖, 🟠 橙色 = 继承依赖, 🟢 绿色 = 兄弟模块, 🔴 红色 = 禁止依赖
- 边标注：中文说明依赖用途（来自 seed 的 rationale 字段）
- 分组：按 layer 分 subgraph，子模块嵌套在父模块内（蓝色边框高亮）

## 总览图（父/顶层模块）

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

  AGENT -->|"Agent 构建 system prompt"| PROMPT
  AGENT -->|"Agent 处理 tool_result 等数据"| DATA
  AGENT -->|"Agent 通过 Jido tools 参数注入工具"| TOOLS
  AGENT -->|"AgentLoop 调用 HookRunner 执行 gate/pipe"| HOOK
  AGENT -->|"AgentLoop 调用 Extension setup/teardown"| EXTENSIONS
  AGENT -->|"AgentLoop 使用 retry/steering/abort"| RUNTIME
  AGENT -->|"AgentLoop 发射流式事件"| STREAM
  SESSION -->|"Session 驱动 AgentLoop 执行"| AGENT
  SESSION -->|"Session 广播流式事件给订阅者"| STREAM
  SESSION -->|"Session 使用 steering/retry/settings"| RUNTIME
  SESSION -->|"Session 构建 LLM backend"| PROVIDERS
  COMPACTION -->|"Compaction 调用 LLM 生成摘要"| PROVIDERS
  COMPACTION -->|"Summarizer 构建压缩 prompt"| PROMPT
  COMPACTION -->|"Compaction 持久化压缩结果"| TAPE
  EXTENSIONS -->|"ExtensionIntegration 注册扩展命令"| SESSION
  TOOLS -->|"Tools 返回 ToolResult 数据"| DATA
  RUNTIME -->|"Auth 模块查询 ModelRegistry 做认证校验"| PROVIDERS
  INFRA -->|"Application 启动时注册 Provider"| PROVIDERS
  INFRA -->|"Application 启动时调用 PromptTemplate.init()"| PROMPT
  INFRA -->|"Application 启动 SessionRegistry 和 SessionSupervisor"| SESSION
  CLI -->|"CLI 创建/管理 Session"| SESSION
  DATA -.->|"⛔ 数据层不能反向依赖 Agent"| AGENT
  TOOLS -.->|"⛔ 工具不能反向依赖 Agent"| AGENT
  TAPE -.->|"⛔ 存储层不能反向依赖 Session"| SESSION
  linkStyle 0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20 stroke:#2196F3,stroke-width:2px
  linkStyle 21,22,23 stroke:#F44336,stroke-width:2px,stroke-dasharray:5
```

## Agent 运行时 子模块详情

```mermaid
graph TD
  subgraph AGENT["Agent 运行时"]
    AGENT_CORE(["Agent 核心逻辑"])
    AGENT_LOOP(["Agent 循环控制"])
  end
  style AGENT fill:#e8f4fd,stroke:#1a73e8,stroke-width:2px,stroke-dasharray:none
  DATA["数据层"]
  EXTENSIONS["扩展系统"]
  HOOK["Hook 系统"]
  PROMPT(["Prompt 系统"])
  RUNTIME[("运行时支撑")]
  SESSION(["会话协调"])
  STREAM["流式输出"]
  TOOLS["工具集"]

  AGENT -.->|"Agent 构建 system prompt"| PROMPT
  AGENT -.->|"AgentLoop 调用 HookRunner 执行 gate/pipe"| HOOK
  AGENT -.->|"Agent 处理 tool_result 等数据"| DATA
  AGENT -.->|"AgentLoop 使用 retry/steering/abort"| RUNTIME
  AGENT -.->|"Agent 通过 Jido tools 参数注入工具"| TOOLS
  AGENT -.->|"AgentLoop 发射流式事件"| STREAM
  AGENT -.->|"AgentLoop 调用 Extension setup/teardown"| EXTENSIONS
  SESSION -.->|"Session 驱动 AgentLoop 执行"| AGENT
  AGENT_CORE <-.->|"兄弟模块"| AGENT_LOOP
  AGENT_LOOP <-.->|"兄弟模块"| AGENT_CORE
  DATA -.->|"⛔ 数据层不能反向依赖 Agent"| AGENT
  TOOLS -.->|"⛔ 工具不能反向依赖 Agent"| AGENT
  linkStyle 0,1,2,3,4,5,6,7 stroke:#FF9800,stroke-width:2px
  linkStyle 8,9 stroke:#4CAF50,stroke-width:2px
  linkStyle 10,11 stroke:#F44336,stroke-width:2px
```
