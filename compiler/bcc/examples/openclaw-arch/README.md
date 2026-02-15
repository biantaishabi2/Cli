# OpenClaw 架构分析案例

> 一个完整的 `bcc arch` 使用案例，展示如何从代码提取 → 架构设计 → 验证门禁的完整闭环。

## 案例概述

本案例基于 **OpenClaw** 项目（1685 个 TypeScript 文件）的真实架构分析，演示 `bcc arch` 命令链在存量项目中的典型用法：

```
提取事实 → 定义目标 → 验证偏差 → 导出映射 → 生成场景
```

## 核心文件速览

| 文件 | 用途 | 对应命令 |
|------|------|----------|
| `seed/v3.target-matrix.yaml` | 目标架构：允许/禁止的模块关系 | `arch matrix` 输入 |
| `seed/v3.transition-matrix.yaml` | 过渡规则：迁移期临时放行 | `arch validate` 输入 |
| `seed/v3.gates.yaml` | 门禁阈值：unexpected/forbidden 上限 | `arch validate` 输入 |
| `artifacts/module_map.json` | 文件→模块映射（1685 文件） | `extract` 产物 |
| `artifacts/relation_matrix.actual.json` | 实际依赖关系 | `arch validate` 输入 |
| `versions/v3-draft/gate-evaluation.tsv` | 门禁评估结果 | `arch validate` 输出 |

## 快速开始

### 1. 查看架构设计（Seed）

```bash
# 目标架构：哪些模块可以依赖哪些模块
cat seed/v3.target-matrix.yaml

# 过渡规则：迁移期临时放行的边
cat seed/v3.transition-matrix.yaml

# 门禁规则：允许的最大违规数
cat seed/v3.gates.yaml
```

### 2. 复现验证

```bash
cd compiler/bcc/examples/openclaw-arch

# 验证当前代码是否符合架构设计
bcc arch validate \
  --target seed/v3.target-matrix.yaml \
  --transition seed/v3.transition-matrix.yaml \
  --gates seed/v3.gates.yaml \
  --actual artifacts/relation_matrix.actual.json \
  --out-dir my-validation \
  --fail-on-gate

# 查看结果
cat my-validation/gate-evaluation.tsv
cat my-validation/v3-validation-report.md
```

### 3. 导出给 bugfix 使用

```bash
# 生成 bcc bugfix 可用的模块映射
bcc arch export-module-map \
  --module-map artifacts/module_map.json \
  --module-registry artifacts/module_registry.json \
  --out module_map.bugfix.json

# 使用映射运行 bugfix
bcc bugfix /path/to/openclaw \
  --module-map module_map.bugfix.json \
  --lang typescript
```

## 版本演进

本案例保留了完整的版本演进历史，展示架构治理的迭代过程：

| 版本 | 阶段 | 关键产出 | 说明 |
|------|------|----------|------|
| **v0** | 基线 | `module_map.json` | 从代码反推的初始模块划分 |
| **v1** | 调研 | `edge-investigation.json` | 人工调查关键依赖边 |
| **v2** | 验证 | `scenario-validation.tsv` | 用场景验证架构假设 |
| **v3** | 门禁 | `gate-evaluation.tsv` | 完整的 target/transition/gates 规则 |

### 演进示例：v2 → v3

v2 发现问题：
```yaml
# v2.research-matrix.yaml 中的临时边
agents -> gateway: temporary  # 需要整改
```

v3 整改后纳入门禁：
```yaml
# v3.target-matrix.yaml
relations:
  allowed:
    agents: [memory, tools, config]  # 不再直接依赖 gateway

# v3.transition-matrix.yaml
relations:
  temporary:
    agents: [gateway]  # 给 3 个月迁移期
```

## 目录结构详解

```
openclaw-arch/
├── README.md                          # 本文件
│
├── seed/                              # 架构设计输入（人工维护）
│   ├── v0.module-registry.seed.yaml   # 早期模块定义
│   ├── v1.module-registry.seed.yaml   # 演进后模块定义
│   ├── v2.research-matrix.yaml        # 研究阶段矩阵
│   ├── v3.target-matrix.yaml          # ★ 目标架构（生产用）
│   ├── v3.transition-matrix.yaml      # ★ 过渡规则（生产用）
│   └── v3.gates.yaml                  # ★ 门禁阈值（生产用）
│
├── artifacts/                         # 分析产物（工具生成）
│   ├── module_map.json               # 文件路径 → 模块名
│   ├── module_registry.json          # 模块元数据
│   ├── relation_matrix.actual.json   # 实际依赖关系
│   ├── relation_matrix.expected.json # 预期依赖关系
│   ├── relation_matrix.diff.json     # 差异分析
│   └── summary.md                    # 摘要报告
│
└── versions/                          # 版本演进历史
    ├── v0/                           # 基线版本
    ├── v1/                           # 调研版本
    ├── v2-draft/                     # 验证版本
    │   ├── edge-severity.tsv         # 边风险分级
    │   ├── sampling-evidence.md      # 抽样证据
    │   └── v2-validation-report.md   # 验证报告
    └── v3-draft/                     # 门禁版本
        ├── gate-evaluation.tsv       # 门禁评估
        ├── scenario-validation.tsv   # 场景验证
        └── v3-validation-report.md   # 最终报告
```

## 典型工作流

### 工作流 1：日常门禁（CI）

```bash
# 在 CI 中运行，防止违规依赖进入主干
bcc arch validate \
  --target seed/v3.target-matrix.yaml \
  --transition seed/v3.transition-matrix.yaml \
  --gates seed/v3.gates.yaml \
  --actual artifacts/relation_matrix.actual.json \
  --fail-on-gate \
  --fail-on-forbidden
```

### 工作流 2：架构整改

```bash
# 1. 修改 seed/v3.target-matrix.yaml，添加新的允许关系
# 2. 修改 seed/v3.transition-matrix.yaml，移除已整改的临时边
# 3. 重新验证
bcc arch validate ... --out-dir versions/v3.1-draft

# 4. 对比历史
 diff versions/v3-draft/gate-evaluation.tsv versions/v3.1-draft/gate-evaluation.tsv
```

### 工作流 3：新项目参考

```bash
# 复制 seed 结构作为新项目起点
mkdir my-project-arch
cp openclaw-arch/seed/v3.*.yaml my-project-arch/

# 修改模块定义
vim my-project-arch/v3.target-matrix.yaml

# 运行验证
bcc arch matrix --seed-file my-project-arch/v3.target-matrix.yaml ...
```

## 历史背景

- **原始项目**: OpenClaw（多通道 AI 消息网关）
- **原始分析**: 2026-02 使用 TypeScript 脚本完成
- **工具演进**: 2026-02-15 迁移到 Rust `bcc arch` 命令链
- **分析范围**: 1685 个 TypeScript 文件，56 个模块
- **原始仓库**: `/Users/biantaishabi/openclaw/docs/backend-trace/`

## 相关文档

- [架构闭环迁移计划](../../docs/架构闭环迁移计划.md) - BCC 架构设计文档
- [技术设计文档-后端编译器](../../docs/技术设计文档-后端编译器.md) - bcc 详细设计
