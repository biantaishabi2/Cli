---
name: bcc-seed-authoring
description: 为 bcc arch 命令创建和维护 seed 文件。分析代码结构，定义模块边界，识别依赖关系，避免漏定义和过度定义。
allowed-tools: Read, Grep, Glob, Bash, Write, Edit
user-invocable: true
---

# BCC Seed 文件编写

## Skill 文件结构

```
bcc-seed-authoring/
├── SKILL.md              # 本文件（执行指南）
├── patterns.md           # 常见框架依赖注入模式识别
└── checklist.md          # Seed 完整性检查清单
```

## 目标

为 `bcc arch matrix/validate` 命令创建高质量的 seed 文件：
- 准确反映架构设计意图
- 避免漏定义（导致假阳性"额外边"）
- 避免过度定义（掩盖真正的架构违反）

## 适用范围

- 新项目：从零创建 seed 文件
- 存量项目：根据代码反推完善 seed
- 迭代维护：根据 BCC 反馈更新 seed

## 用法

```
/bcc-seed-authoring <项目路径> [--lang <语言>] [--reference <参考项目>]
```

示例：
```
/bcc-seed-authoring /home/wangbo/document/gong --lang elixir
/bcc-seed-authoring /home/wangbo/document/pi-mono --lang typescript --reference openclaw-arch
```

## Seed 文件结构

```yaml
version: "0.1.0"
source_of_truth: <文档来源或设计说明>

modules:
  - module_id: <模块ID>
    display_name: <显示名称>
    domain_kind: business | infrastructure
    layer: application | infrastructure
    precedence: <数字，小优先>
    path_rules:
      include:
        - "<glob模式>"
      exclude:
        - "<glob模式>"
    symbol_anchors:          # 可选：关键符号锚点
      - file: "<文件路径>"
        tag: "<标签>"

relations_expected:
  - caller: <模块ID>
    callee: <模块ID>
    allowed: true
    rationale: "<为什么需要这个依赖>"
    injection_type: "direct | framework | constructor | config"  # 可选
```

## 执行步骤

### 1. 项目结构分析

```bash
# 分析项目结构
ls -la <项目路径>/lib 或 src
find <项目路径> -name "*.ex" -o -name "*.ts" | head -50

# 识别主要模块
grep -r "defmodule\|export class" <项目路径>/lib --include="*.ex" | head -30
```

### 2. 模块定义（modules）

**原则**：
- 一个模块对应一个**业务/技术领域**
- 模块边界应该**稳定**（不随文件重构而频繁变化）
- 粒度适中（5-15 个模块为宜）

**常见模块划分**：

| 类型 | 示例 |
|-----|------|
| 业务模块 | AGENT, TOOLS, COMPACTION, EXTENSIONS |
| 基础设施 | INFRA, PROVIDERS, DATA |
| 接入层 | CLI, API, WEB |

### 3. 依赖定义（relations_expected）

**关键原则**：

```
定义"功能依赖"，不是"实现机制"

✅ 正确：AGENT 需要 TOOLS 的功能
❌ 错误：AGENT 通过 Jido 注入工具

✅ 正确：INFRA 需要 PROVIDERS 的功能
❌ 错误：INFRA 调用 ReqLLM.Providers.register
```

**依赖类型标注**（可选但推荐）：

```yaml
injection_type: "framework"  # 框架注入（Jido, Spring等）
injection_type: "constructor"  # 构造函数注入
injection_type: "config"  # 配置注入
injection_type: "direct"  # 直接导入（默认）
```

### 4. 常见漏定义模式检查

#### 模式 1：框架注入依赖

```elixir
# 代码中
use Jido.AI.ReActAgent,
  tools: [Gong.Tools.Bash]

# Seed 中容易漏定义
relations_expected:
  - caller: AGENT
    callee: TOOLS          # ← 容易漏！
    allowed: true
    injection_type: "framework"
```

#### 模式 2：外部库包装的内部模块

```elixir
# 代码中
ReqLLM.Providers.register(Gong.Providers.DeepSeek)
# 外部库 ^    ^ 内部模块

# Seed 中容易漏定义
relations_expected:
  - caller: INFRA
    callee: PROVIDERS      # ← 容易漏！
    allowed: true
    rationale: "Application 启动时注册 Provider"
```

#### 模式 3：启动时初始化依赖

```elixir
# 代码中
def start(_type, _args) do
  Extension.Loader.load_all()  # 启动时加载
end

# Seed 中容易漏定义
relations_expected:
  - caller: INFRA
    callee: EXTENSIONS     # ← 容易漏！
    allowed: true
    rationale: "Application 启动时初始化 Extension"
```

### 5. BCC 反馈迭代

```bash
# 1. 生成初步 seed
# 2. 运行 matrix
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir matrix

# 3. 检查额外边
cat matrix/v3.transition-matrix.yaml

# 4. 分析每条额外边
# - 正当依赖但漏定义 → 补充到 seed
# - 真正的架构违反 → 保持额外边，规划整改
# - BCC 提取问题 → 理解上下文，可能忽略

# 5. 迭代更新 seed
```

## 质量检查清单

生成 seed 后自检：

- [ ] 模块划分符合业务/技术边界
- [ ] 每个模块有明确的 path_rules
- [ ] 检查框架注入依赖（Jido, Spring, NestJS 等）
- [ ] 检查外部库调用的内部模块注册
- [ ] 检查启动时初始化依赖
- [ ] 运行 BCC matrix，分析额外边
- [ ] 参考类似项目的 seed（如 openclaw-arch）

## 示例：Gong Seed 演进

### v1：初步定义（有漏定义）

```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  - caller: AGENT
    callee: DATA
    allowed: true
  # 漏了：AGENT -> TOOLS
  # 漏了：INFRA -> PROVIDERS
```

### v2：BCC 反馈后完善

```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  - caller: AGENT
    callee: DATA
    allowed: true
  - caller: AGENT
    callee: TOOLS          # ← 补充
    allowed: true
    injection_type: "framework"
  - caller: INFRA
    callee: EXTENSIONS
    allowed: true
  - caller: INFRA
    callee: COMPACTION
    allowed: true
  - caller: INFRA
    callee: PROVIDERS      # ← 补充
    allowed: true
  - caller: TOOLS
    callee: DATA
    allowed: true
```

## 与代码的关系

Seed 是**架构设计意图**的文档化：

```
架构设计 → Seed 文件 → 代码实现
    ↑__________________________|
         BCC 验证反馈
```

- **Seed 先于代码**：新项目的架构设计
- **Seed 后于代码**：存量项目的架构梳理
- **Seed 与代码同步**：迭代维护

## 注意事项

- **不要过度定义**：只定义"功能依赖"，不定义"实现细节"
- **不要漏定义**：特别关注框架注入、外部库包装、启动初始化
- **保持简洁**：seed 是架构契约，不是代码文档
- **迭代完善**：先用 BCC 跑一遍，根据反馈调整
- **参考先例**：参考 `openclaw-arch` 等成熟 seed
