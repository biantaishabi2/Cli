---
name: taskctl-research
description: 使用 taskctl research 工具进行假设驱动的研究推理。声明假设、收集证据、贝叶斯更新后验概率，最终收敛到可行动的结论。
---

# taskctl research — 假设驱动的研究推理

## 核心思想

做研究就像做科学：**靠直觉（gut feeling）→ 大胆假设 → 小心求证 → 更新判断 → 收敛到结论**。

你（机器人）只需要做两件事：
1. **声明假设**：对问题先下一个判断（prior）
2. **添加证据**：每次发现新信息，声明它支持还是反对假设

工具会自动帮你：
- 用贝叶斯推理更新后验概率（posterior）
- 给出行动建议（verdict）：act / investigate / contested

**你不需要关心公式，只需要诚实地声明你发现了什么。**

## 化学键比喻（证据强度）

来自 Seed 论文的分子结构理论，不同来源的证据有不同的"键能"：

| bond_type | 比喻 | 权重 | 含义 | 什么时候用 |
|-----------|------|------|------|-----------|
| `deduction` | 共价键 | ×1.0 | 逻辑推导、代码确认、测试验证 | 你读了代码/跑了测试，确定性很高 |
| `verification` | 氢键 | ×0.7 | 回检校验、交叉引用、文档印证 | 你从文档/日志/第二来源确认了信息 |
| `exploration` | 范德华力 | ×0.3 | 试探猜测、直觉推断、模式匹配 | 你有一个感觉，但还没确认 |

**原则**：强推导 > 交叉验证 > 弱猜测。exploration 的证据不管多少条都不会压过一条 deduction。

## Verdict 决策阈值

| posterior 范围 | verdict | 含义 |
|---------------|---------|------|
| >= 0.7 | `act` | 足够确信，可以行动 |
| <= 0.3 | `investigate` | 信心不足，需要更多证据 |
| 有 supports 又有 conflicts | `contested` | 证据冲突，需要厘清 |
| 其他 | `investigate` | 中间地带，继续调查 |

## 完整工作流

### 第一步：声明假设

遇到不确定的问题时，先下一个直觉判断：

```bash
# 我怀疑是内存泄漏（70% 确信）
taskctl research hypothesis add --id memory-leak --prior 0.7

# 也可能是连接池耗尽（40% 确信）
taskctl research hypothesis add --id pool-exhaustion --prior 0.4

# 完全不确定，五五开
taskctl research hypothesis add --id config-error --prior 0.5
```

### 第二步：逐步添加证据

每次你发现了新信息（读了代码、跑了测试、看了日志），立即声明：

```bash
# 读代码发现 buffer 没有释放 → 强力支持内存泄漏假设
taskctl research add \
  --evidence-id e1 \
  --conclusion-id memory-leak \
  --relation supports \
  --confidence 0.9 \
  --bond deduction

# 日志显示内存曲线持续上升 → 交叉验证
taskctl research add \
  --evidence-id e2 \
  --conclusion-id memory-leak \
  --relation supports \
  --confidence 0.8 \
  --bond verification

# 但连接池监控显示正常 → 反对连接池耗尽假设
taskctl research add \
  --evidence-id e3 \
  --conclusion-id pool-exhaustion \
  --relation conflicts \
  --confidence 0.85 \
  --bond deduction
```

**每次 add，工具自动输出更新后的 posterior 和 verdict：**

```json
{
  "ok": true,
  "evidence_id": "e1",
  "conclusion_id": "memory-leak",
  "posterior": 0.87,
  "verdict": "act"
}
```

### 第三步：检查当前状态

```bash
# 查看所有假设的后验概率
taskctl research hypothesis list

# 查看所有假设 + 证据详情
taskctl research list

# 聚合分析（带 verdict 排序）
taskctl research reduce
```

### 第四步：根据 verdict 行动

- **act** → 后验概率足够高，按此假设行动（修代码、提 PR）
- **investigate** → 还不够确定，继续调查，多加证据
- **contested** → 有矛盾证据，优先厘清冲突点

## 关键原则

### 1. 先假设再求证

不要等到 100% 确定才声明假设。prior = 0.5 表示"完全不确定"，这也是一个合法的起点。工具的价值在于帮你**从模糊到清晰**。

### 2. 诚实声明 confidence

- 0.9+ = 你非常确定（测试跑过、代码读过）
- 0.7~0.8 = 你比较确定（日志/文档支撑）
- 0.5~0.6 = 你有点感觉
- 0.3~0.4 = 弱线索

### 3. 正确选择 bond_type

这直接影响证据权重：
- **deduction**：你亲自验证了（读代码、跑测试、查数据库）
- **verification**：你从第二来源确认了（文档、日志、监控）
- **exploration**：你只是猜测或模式匹配（"这看起来像是..."）

### 4. 收敛后行动

当某假设 posterior >= 0.7 且 verdict = act，说明证据链已经足够支撑行动。不需要继续无限收集证据。

### 5. 矛盾时优先解决

如果 verdict = contested，不要继续无脑添加支持证据。先搞清楚矛盾的原因——可能假设本身需要细化。

## 命令速查

| 操作 | 命令 |
|------|------|
| 声明假设 | `taskctl research hypothesis add --id <ID> --prior <0.0~1.0>` |
| 移除假设 | `taskctl research hypothesis remove --id <ID>` |
| 列出假设 | `taskctl research hypothesis list` |
| 添加证据 | `taskctl research add --evidence-id <EID> --conclusion-id <HID> --relation <supports/conflicts> --confidence <0.0~1.0> --bond <deduction/verification/exploration>` |
| 移除证据 | `taskctl research remove --evidence-id <EID>` |
| 列出全部 | `taskctl research list` |
| 聚合分析 | `taskctl research reduce` |

## Store 文件

所有数据持久化在 `--store` 指定的 JSON 文件中（默认 `./taskctl-store.json`），和 task DAG 共用同一个 store。

## 典型场景示例

### Bug 定位

```bash
# 1. 直觉：可能是并发竞态
taskctl research hypothesis add --id race-condition --prior 0.6

# 2. 读代码发现没加锁 → 强支持
taskctl research add --evidence-id read-code --conclusion-id race-condition \
  --relation supports --confidence 0.9 --bond deduction
# → posterior ≈ 0.87, verdict: act

# 3. 直接修复
```

### 架构决策

```bash
# 1. 假设：应该用 WebSocket 而不是轮询
taskctl research hypothesis add --id use-websocket --prior 0.5

# 2. 性能测试显示轮询延迟太高 → 支持
taskctl research add --evidence-id perf-test --conclusion-id use-websocket \
  --relation supports --confidence 0.8 --bond deduction

# 3. 但运维团队说 WS 难维护 → 反对
taskctl research add --evidence-id ops-concern --conclusion-id use-websocket \
  --relation conflicts --confidence 0.6 --bond verification
# → verdict: contested → 需要进一步讨论

# 4. 找到轻量 WS 库解决运维痛点 → 支持
taskctl research add --evidence-id lightweight-lib --conclusion-id use-websocket \
  --relation supports --confidence 0.7 --bond verification
# → posterior 回升, verdict: act
```

### 需求调研

```bash
# 1. 假设：用户需要导出功能
taskctl research hypothesis add --id need-export --prior 0.5

# 2. 用户访谈中 3/5 提到了 → 支持
taskctl research add --evidence-id interview --conclusion-id need-export \
  --relation supports --confidence 0.7 --bond verification

# 3. 竞品都有导出 → 弱支持（只是猜测市场需要）
taskctl research add --evidence-id competitor --conclusion-id need-export \
  --relation supports --confidence 0.6 --bond exploration
# → exploration 权重低，不会主导判断
```
