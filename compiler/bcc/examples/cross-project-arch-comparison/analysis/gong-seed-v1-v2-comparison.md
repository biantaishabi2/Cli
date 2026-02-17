# Gong Seed v1 vs v2 对比报告

> 展示如何通过补充漏定义的依赖，消除"假阳性"额外边

## 对比概览

| 指标 | v1 (原始) | v2 (补充后) | 改进 |
|-----|----------|------------|------|
| 期望边 | 7 条 | 9 条 | +2 |
| 实际命中 | 7 条 | 9 条 | +2 |
| 额外边 | 3 条 | 1 条 | -2 ✅ |
| 架构健康度 | ⚠️ 3 条问题 | ✅ 1 条轻微问题 | 显著提升 |

## 详细对比

### v1 额外边（3 条）

```yaml
temporary_allow_edges:
  - caller: AGENT
    callee: TOOLS
    reason: "derived from actual but not in target"
    
  - caller: INFRA
    callee: PROVIDERS
    reason: "derived from actual but not in target"
    
  - caller: TOOLS
    callee: COMPACTION
    reason: "derived from actual but not in target"
```

**分析**：
- AGENT → TOOLS：❌ **漏定义**（正当依赖）
- INFRA → PROVIDERS：❌ **漏定义**（正当依赖）
- TOOLS → COMPACTION：⚠️ **轻微违反**（模块职责不清）

### v2 额外边（1 条）

```yaml
temporary_allow_edges:
  - caller: TOOLS
    callee: COMPACTION
    reason: "derived from actual but not in target"
```

**分析**：
- TOOLS → COMPACTION：⚠️ **唯一真正的轻微违反**

## v2 新增的定义

### 新增 1：AGENT → TOOLS

```yaml
relations_expected:
  - caller: AGENT
    callee: TOOLS
    allowed: true
    rationale: "Agent 需要调用工具完成用户任务"
    injection_type: "framework"
    notes: "通过 Jido.AI.ReActAgent 的 tools 参数注入"
```

**为什么 v1 漏了？**
- Jido 的 `tools: [...]` 看起来是配置，不像依赖
- 架构设计时没意识到这是模块间的功能依赖

### 新增 2：INFRA → PROVIDERS

```yaml
relations_expected:
  - caller: INFRA
    callee: PROVIDERS
    allowed: true
    rationale: "Application 启动时需要注册 LLM Provider"
    injection_type: "registration"
    notes: "通过 ReqLLM.Providers.register 注册内部 Provider 模块"
```

**为什么 v1 漏了？**
- `ReqLLM.Providers.register` 看起来是外部库调用
- 没意识到这是在注册内部模块 `Gong.Providers.DeepSeek`
- BCC 提取也混淆了外部库和内部模块

## 唯一真正的额外边

### TOOLS → COMPACTION

**现状**：
- `tools/bash.ex` 调用 `Gong.Truncate.truncate/2`
- `truncate.ex` 位于 `compaction/` 目录下

**问题**：
- truncate 是一个通用工具函数
- 不应该放在 compaction 模块下
- 导致工具模块依赖了压缩模块

**修复建议**：
```
当前：
  lib/gong/
    compaction/
      truncate.ex    <- 通用工具不应该在这里
    tools/
      bash.ex        <- 调用 Truncate

修复后：
  lib/gong/
    utils/
      output.ex      <- 移动到这里，改名
    tools/
      bash.ex        <- 调用 Utils.Output
```

## 关键洞察

### 1. "额外边" ≠ "架构违反"

| 边 | v1 判定 | 实际 | 修复方式 |
|---|--------|------|---------|
| AGENT → TOOLS | 额外边 | 正当依赖 | 补充 seed |
| INFRA → PROVIDERS | 额外边 | 正当依赖 | 补充 seed |
| TOOLS → COMPACTION | 额外边 | 轻微违反 | 代码重构 |

**结论**：3 条"额外边"中，2 条是 seed 漏定义，1 条是真正的轻微违反。

### 2. BCC 的价值

BCC 不是来告诉你"代码有问题"，而是来帮你：
1. **发现漏定义的依赖**（补充 seed）
2. **识别真正的架构违反**（规划整改）
3. **验证 seed 完整性**（迭代完善）

### 3. Seed 编写的最佳实践

```
1. 写初步 seed（基于架构设计）
      ↓
2. 运行 BCC matrix
      ↓
3. 分析额外边
   ├── 正当依赖 → 补充 seed（如 AGENT→TOOLS）
   ├── 架构违反 → 规划整改（如 TOOLS→COMPACTION）
   └── 提取问题 → 理解上下文（如 INFRA→PROVIDERS）
      ↓
4. 迭代更新 seed
      ↓
5. 直到额外边只剩真正的违反
```

## 命令对比

### v1 验证

```bash
cd projects/gong
bcc arch matrix \
  --seed-file seed.yaml \
  --ast-file ast.json \
  --out-dir ../../analysis/gong-matrix \
  --force

# 结果：3 条额外边
cat ../../analysis/gong-matrix/v3.transition-matrix.yaml
```

### v2 验证

```bash
cd projects/gong
bcc arch matrix \
  --seed-file seed-v2.yaml \
  --ast-file ast.json \
  --out-dir ../../analysis/gong-matrix-v2 \
  --force

# 结果：1 条额外边 ✅
cat ../../analysis/gong-matrix-v2/v3.transition-matrix.yaml
```

## 总结

### Gong 架构健康度

- **v1**: ⚠️ 3 条额外边（2 条漏定义 + 1 条轻微违反）
- **v2**: ✅ 1 条额外边（仅 1 条轻微违反）

**结论**：Gong 的架构实际上非常健康，只有 1 个轻微的模块职责问题。

### 对 PI-Mono 的启示

如果 PI-Mono 也进行类似的 seed 完善：
- 15 条额外边中，可能有多条是"漏定义"而非"违反"
- 需要逐一分析，区分"补充 seed"和"代码整改"
- 真正的架构违反可能少于 15 条

### 最终建议

1. **对于新项目**：参考 v2 的 seed 结构，避免常见漏定义
2. **对于存量项目**：运行 BCC，分析额外边，迭代完善 seed
3. **对于架构治理**：区分"seed 完善"和"代码整改"，避免过度工程
