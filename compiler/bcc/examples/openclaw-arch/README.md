# OpenClaw 架构分析案例

本目录包含 OpenClaw 项目的架构分析案例，用于演示 `bcc arch` 命令的使用。

## 目录结构

```
.
├── seed/                    # 架构设计输入（种子文件）
│   ├── v0.module-registry.seed.yaml    # v0 模块注册表
│   ├── v1.module-registry.seed.yaml    # v1 模块注册表
│   ├── v2.research-matrix.yaml         # v2 研究矩阵
│   ├── v3.target-matrix.yaml           # v3 目标矩阵
│   ├── v3.transition-matrix.yaml       # v3 过渡矩阵
│   └── v3.gates.yaml                   # v3 门禁规则
│
├── artifacts/               # 分析产物
│   ├── module_map.json                 # 文件→模块映射
│   ├── module_registry.json            # 模块注册表
│   ├── relation_matrix.actual.json     # 实际关系矩阵
│   ├── relation_matrix.expected.json   # 预期关系矩阵
│   └── relation_matrix.diff.json       # 差异分析
│
└── versions/                # 版本演进
    ├── v0/                  # v0 基线版本
    ├── v1/                  # v1 调研版本
    ├── v2-draft/            # v2 验证版本
    └── v3-draft/            # v3 门禁版本
```

## 使用方式

### 1. 生成架构矩阵

```bash
bcc arch matrix \
  --seed-file seed/v3.target-matrix.yaml \
  --ast-file artifacts/module_registry.json \
  --out-dir versions/v4 \
  --version v4
```

### 2. 验证架构

```bash
bcc arch validate \
  --target seed/v3.target-matrix.yaml \
  --transition seed/v3.transition-matrix.yaml \
  --gates seed/v3.gates.yaml \
  --actual artifacts/relation_matrix.actual.json \
  --out-dir versions/v4-draft
```

### 3. 导出模块映射

```bash
bcc arch export-module-map \
  --module-map artifacts/module_map.json \
  --module-registry artifacts/module_registry.json \
  --out module_map.bugfix.json
```

## 版本演进说明

- **v0**: 初始基线，从代码反推的模块划分
- **v1**: 调研版本，增加边缘调查
- **v2**: 验证版本，场景验证和修复待办
- **v3**: 门禁版本，完整的 target/transition/gates 规则

## 历史背景

本案例源自 OpenClaw 项目的后端架构分析。原始分析使用 TypeScript 脚本完成，
现已迁移到 Rust 实现的 `bcc arch` 命令链。

- 原始分析仓库: `/Users/biantaishabi/openclaw/docs/backend-trace/`
- 迁移时间: 2026-02-15
