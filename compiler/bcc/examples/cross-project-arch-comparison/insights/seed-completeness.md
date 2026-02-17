# Seed 定义完整性检查

> 为什么 Gong 的"额外边"实际上是漏定义的正当依赖？

## 问题发现

在对比分析 Gong 和 PI-Mono 时，发现 Gong 有 3 条"额外边"：

1. AGENT → TOOLS
2. INFRA → PROVIDERS  
3. TOOLS → COMPACTION

经过分析，**AGENT → TOOLS 不是架构违反，而是 seed 漏定义了**。

## 分析过程

### 1. 查看原始 Seed 定义

```yaml
# gong-seed.yaml (原始)
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  - caller: AGENT
    callee: DATA
    allowed: true
  # 缺少 AGENT -> TOOLS！
```

### 2. 查看实际代码依赖

```elixir
# agent.ex
use Jido.AI.ReActAgent,
  tools: [
    Gong.Tools.Read,
    Gong.Tools.Write,
    Gong.Tools.Bash,
    ...
  ]
```

Agent 明确需要 Tools 的功能！

### 3. 架构合理性判断

| 检查项 | 结果 | 说明 |
|-------|------|------|
| 依赖方向 | ✅ 正确 | AGENT → TOOLS 符合分层 |
| 功能必要性 | ✅ 必要 | Agent 必须调用工具 |
| 实现方式 | ✅ 健康 | Jido 框架注入，松耦合 |

**结论**：这是正当的架构依赖，应该在 seed 中定义。

## 修复方案

### 补充 Seed 定义

```yaml
# gong-seed.yaml (修复后)
relations_expected:
  # 已有定义
  - caller: AGENT
    callee: PROMPT
    allowed: true
    rationale: "Agent 需要动态构建 system prompt"
    
  - caller: AGENT
    callee: DATA
    allowed: true
    rationale: "Agent 需要处理 tool_result"
    
  # 补充定义
  - caller: AGENT
    callee: TOOLS
    allowed: true
    rationale: "Agent 需要调用工具完成用户任务"
    injection_type: "framework"  # 标注为框架注入
    
  # 其他定义...
```

### 修复后的验证结果

修复 seed 后重新运行验证：

```bash
bcc arch matrix --seed-file gong-seed-fixed.yaml --ast-file gong-ast.json --out-dir gong-matrix-fixed
```

**结果**：
- 期望边：8 条（原 7 条 + 新增的 AGENT → TOOLS）
- 实际命中：8 条 ✅
- 额外边：2 条（原 3 条 - AGENT → TOOLS）

## 如何检查 Seed 完整性

### 方法 1：代码审查

检查每个模块的代码，确认是否需要其他模块的功能：

```bash
# 查看 AGENT 模块的依赖
grep -r "Gong\.Tools\." lib/gong/agent*.ex

# 查看 COMPACTION 模块的依赖
grep -r "ReqLLM\|Provider" lib/gong/compaction*.ex
```

### 方法 2：对比类似项目

参考 PI-Mono 的 seed 定义：

```yaml
# pi-mono seed
relations_expected:
  - caller: PI_CODING_CORE
    callee: PI_CODING_TOOLS  # PI-Mono 明确定义了 CORE -> TOOLS
    allowed: true
```

### 方法 3：运行时分析

通过实际运行确认依赖关系：

```elixir
# 在 iex 中检查 Agent 的工具列表
iex> Gong.Agent.__jido_actions__()
# 应该返回工具列表
```

## 常见 Seed 漏定义场景

### 场景 1：框架注入的依赖

```elixir
# 代码中通过框架注入，不明显
use SomeFramework,
  plugins: [ModuleA, ModuleB]
```

**容易漏定义**：因为不是直接的 `import` 或 `require`

### 场景 2：宏生成的依赖

```elixir
# 宏在编译期生成依赖
defmacro __using__(opts) do
  quote do
    import ModuleX  # 生成的依赖
  end
end
```

**容易漏定义**：AST 能看到，但人工审查容易忽略

### 场景 3：动态调用的依赖

```elixir
# 动态模块调用
module = String.to_atom("Elixir.Gong.#{name}")
apply(module, :function, [args])
```

**容易漏定义**：静态分析难以检测

## Seed 完整性检查清单

### 创建 Seed 时

- [ ] 列出所有模块
- [ ] 检查每个模块的代码，确认功能依赖
- [ ] 参考类似项目的架构设计
- [ ] 运行 matrix 命令，检查"额外边"
- [ ] 分析每条"额外边"，判断是违反还是漏定义

### 维护 Seed 时

- [ ] 代码变更后重新运行验证
- [ ] 新增模块时补充依赖定义
- [ ] 定期审查"临时边"，判断是否可以转正或删除

## 对 Gong 的最终评估

### 修复前

| 指标 | 数值 | 说明 |
|-----|------|------|
| 期望边 | 7 条 | 定义在 seed 中 |
| 实际命中 | 7 条 | ✅ |
| 额外边 | 3 条 | 包含 1 条漏定义 |

### 修复后

| 指标 | 数值 | 说明 |
|-----|------|------|
| 期望边 | 8 条 | 补充 AGENT → TOOLS |
| 实际命中 | 8 条 | ✅ |
| 额外边 | 2 条 | 真正的额外依赖 |

### 真正的额外边分析

1. **INFRA → PROVIDERS**
   - 原因：application.ex 硬编码注册 DeepSeek
   - 性质：轻微违反，建议改为配置驱动

2. **TOOLS → COMPACTION**
   - 原因：tools 引用 truncate.ex 进行输出截断
   - 性质：模块职责不清，truncate 应该移到 utils

## 总结

### 核心教训

**"额外边" ≠ "架构违反"**

需要分析每条额外边：
1. **是否正当依赖** → 补充 seed 定义
2. **是否架构违反** → 规划整改
3. **是否临时需要** → 添加到 transition

### Seed 定义的最佳实践

1. **先定义架构契约**：模块间的功能依赖关系
2. **不关注实现机制**：直接 import 还是框架注入
3. **定期审查验证**：根据"额外边"反馈完善 seed
4. **参考类似项目**：借鉴成熟项目的架构设计

### 对 bcc 工具的改进建议

可以增加 seed 完整性检查功能：

```bash
# 检查 seed 可能漏定义的依赖
bcc arch check-seed \
  --seed-file seed.yaml \
  --ast-file ast.json \
  --suggest-missing
```

输出：
```
可能漏定义的依赖：
- AGENT -> TOOLS (检测到代码引用，但 seed 未定义)
  证据：agent.ex:12 使用了 Gong.Tools.Bash
  建议：补充到 relations_expected
```
