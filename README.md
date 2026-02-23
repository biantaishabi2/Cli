# Cli

[![CI](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/biantaishabi2/Cli/actions/workflows/ci.yml)
[![Release](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml/badge.svg)](https://github.com/biantaishabi2/Cli/actions/workflows/release.yml)

> **LLM 时代的软件工程操作系统**：taskctl 编排复杂工作流，BCC 编译代码结构为可验证的知识图谱，niuma 实现从需求到合并的全自动开发，BDDC 执行行为驱动测试。人定义规则和目标，机器处理执行和验证。
>
> 📖 详细哲学阐述见 [`PHILOSOPHY.md`](PHILOSOPHY.md)

## 需求即测试，代码即文档

`需求即测试`：需求必须通过行为契约/DSL直接生成测试代码，需求不是独立于测试的文本描述。

`代码即文档`：代码实现通过架构和实现的契约进入编译器/校验器检查。

代码行为由测试代码验证；结构与约束由契约校验。两条验证链共同形成闭环。

```mermaid
flowchart TB
    subgraph DSL层["📜 DSL 层"]
        BEHAVIOR[("行为契约 DSL")]
        ARCH[("架构实现 DSL")]
    end

    subgraph 自研工具层["🔧 自研工具层"]
        BDDC[("BDDC<br/>行为测试生成器")]
        BCC[("BCC<br/>架构契约编译器")]
        NIUMA[("🐂 牛马<br/>LLM代码生成")]
    end

    GEN_IMPL[("实现代码")]

    subgraph CI门禁["⛔ CI 门禁"]
        direction TB
        style CI门禁 fill:#f5f5f5,stroke:#666,stroke-width:2px,stroke-dasharray: 5 5

        IN[/"入口"/]

        subgraph 并行验证["并行验证"]
            direction LR
            RUN_TEST[/执行测试/]
            CHECK[/架构契约校验/]
        end

        OUT[/"出口<br/>统一结果"/]

        IN --> RUN_TEST
        IN --> CHECK
        RUN_TEST --> OUT
        CHECK --> OUT
    end

    PASS{通过?}
    MERGE[("合并入库")]

    BEHAVIOR --> BDDC
    ARCH --> BCC

    BEHAVIOR --> NIUMA
    ARCH --> NIUMA

    BDDC -.->|生成测试代码| RUN_TEST
    BCC -.->|编译校验| CHECK

    NIUMA --> GEN_IMPL
    GEN_IMPL --> IN

    OUT --> PASS
    PASS -->|是| MERGE
    PASS -->|否| NIUMA

    MERGE -.->|持续迭代<br/>更新DSL| DSL层

    style BEHAVIOR fill:#e3f2fd
    style ARCH fill:#e8f5e9
    style BDDC fill:#fff3e0
    style BCC fill:#fce4ec
    style NIUMA fill:#ffccbc
    style GEN_IMPL fill:#f3e5f5
    style CI门禁 fill:#fafafa
```

## 文档导航

- BCC：[`compiler/bcc/README.md`](compiler/bcc/README.md)
- BDDC：[`compiler/bddc/README.md`](compiler/bddc/README.md)
- Niuma：[`automation/niuma/README.md`](automation/niuma/README.md)
- Taskctl：[`orchestration/taskctl/README.md`](orchestration/taskctl/README.md)
