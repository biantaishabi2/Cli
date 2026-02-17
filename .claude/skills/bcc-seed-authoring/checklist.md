# Seed 完整性检查清单

## 创建 Seed 时检查

### 模块定义检查

- [ ] 模块数量适中（5-15 个）
- [ ] 每个模块有明确的业务/技术边界
- [ ] 模块 ID 命名一致（大写/小写）
- [ ] path_rules.include 能正确匹配文件
- [ ] path_rules.exclude 排除了测试/生成文件

### 依赖定义检查

- [ ] 所有功能依赖都已定义
- [ ] 没有定义实现细节依赖
- [ ] 依赖方向符合分层架构
- [ ] rationale 说明了依赖原因

## 常见漏定义检查

### 框架注入依赖

- [ ] **Elixir**: 检查 `use Jido.*`, `use GenServer` 等
- [ ] **TypeScript**: 检查 `@Injectable`, 构造函数注入
- [ ] **Go**: 检查接口类型、结构体嵌入
- [ ] **Java**: 检查 `@Autowired`, `@Component`

### 外部库包装

- [ ] 检查 `ExternalLib.register(MyModule)` 模式
- [ ] 检查 `ReqLLM.Providers.register(Gong.Providers.*)`
- [ ] 检查 `Plugin.register(MyExtension)`

### 启动初始化

- [ ] 检查 `Application.start()` / `main()` 中的初始化
- [ ] 检查 `init()` 函数
- [ ] 检查 `bootstrap()` 函数
- [ ] 检查 `load_all()`, `register()` 调用

### 配置驱动

- [ ] 检查配置文件中的模块列表
- [ ] 检查动态加载的模块

## BCC 反馈后检查

### 分析额外边

```bash
# 运行 BCC matrix
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir matrix

# 查看额外边
cat matrix/v3.transition-matrix.yaml
```

对每条额外边：

- [ ] **是正当依赖？** → 补充到 seed
- [ ] **是架构违反？** → 保持额外边，规划整改
- [ ] **是 BCC 提取问题？** → 理解上下文，可能忽略

### 验证修复

补充 seed 后重新运行：

```bash
bcc arch matrix --seed-file seed.yaml --ast-file ast.json --out-dir matrix-fixed

# 对比
diff matrix/v3.transition-matrix.yaml matrix-fixed/v3.transition-matrix.yaml
```

## 参考对比

### 对比类似项目

- [ ] 参考 `openclaw-arch` 的模块划分
- [ ] 参考 `cross-project-arch-comparison` 的依赖定义
- [ ] 对比同语言项目的 seed 结构

### 对比架构文档

- [ ] 如果项目有架构文档，对比是否一致
- [ ] 检查 PRD 中的模块划分
- [ ] 检查技术设计文档

## 质量指标

### 健康 Seed 的标准

| 指标 | 目标 | 检查方法 |
|-----|------|---------|
| 额外边数量 | 0-2 条 | 运行 BCC matrix |
| 漏定义比例 | < 10% | 代码审查 |
| 模块粒度 | 5-15 个 | 人工检查 |
| 依赖方向 | 符合分层 | 架构审查 |

### 警告信号

- [ ] 额外边 > 5 条（可能大量漏定义）
- [ ] 模块 > 20 个（粒度过细）
- [ ] 模块 < 3 个（粒度过粗）
- [ ] 存在循环依赖（架构设计问题）

## 迭代流程

```
1. 初步定义 seed
      ↓
2. 运行 BCC matrix
      ↓
3. 分析额外边
   ├── 正当依赖 → 补充 seed
   ├── 架构违反 → 规划整改
   └── 提取问题 → 理解上下文
      ↓
4. 更新 seed
      ↓
5. 重复 2-4 直到满意
```

## 示例：Gong Seed 检查

### 初始版本问题

```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  # ❌ 漏了：AGENT -> TOOLS (Jido 注入)
  # ❌ 漏了：INFRA -> PROVIDERS (外部库注册)
```

### BCC 反馈

```yaml
temporary_allow_edges:
  - caller: AGENT
    callee: TOOLS  # ← 额外边
  - caller: INFRA
    callee: PROVIDERS  # ← 额外边
```

### 修复后版本

```yaml
relations_expected:
  - caller: AGENT
    callee: PROMPT
    allowed: true
  - caller: AGENT
    callee: TOOLS          # ✅ 补充
    allowed: true
    injection_type: "framework"
  - caller: INFRA
    callee: PROVIDERS      # ✅ 补充
    allowed: true
```

### 验证结果

```bash
$ bcc arch matrix ...
mapped_files=20
actual_edges=7
# ✅ 额外边从 3 条减少到 1 条（TOOLS -> COMPACTION 是真正的轻微违反）
```
