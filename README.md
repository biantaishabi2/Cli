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

## 工具介绍

- [**BCC（Architecture Compiler）**](compiler/bcc/README.md)：把架构与行为契约编译为可校验工件，覆盖模块层级、分层规则、`flow/boundary/event` 三视图；并产出供 BDDC 生成测试代码的输入。BCC 自身的架构契约校验也是 CI 门禁的一部分。
- [**BDDC（BDD Test Runtime）**](compiler/bddc/README.md)：基于 BCC 输入生成测试代码并执行；测试通过后形成验收结果，作为 CI 门禁结果的一部分。
- [**Niuma（自动化研发引擎）**](automation/niuma/README.md)：单 Issue 流程包含前期讨论收敛与方案定稿，自动推进实现与 PR 迭代；PR review 阶段由人参与决策；同时支持多 Issue DAG 协调。
- [**taskctl（Research/Plan/Execute Orchestration）**](orchestration/taskctl/README.md)：不仅提供执行期 DAG 编排，还支持研究期与/或图建模，用于从研究到执行的统一任务编排。
