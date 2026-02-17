# BCC 架构健康度评分工具设计文档

## 1. 概述

### 1.1 目标
设计一个通用的架构评分 CLI 工具 `bcc arch score`，可以集成到 BCC 中或者作为独立工具使用。该工具基于 BCC 的验证结果计算架构健康度评分，支持自定义评分规则和 CI/CD 集成。

### 1.2 核心能力
- 读取 BCC 验证结果（target/transition/gates/actual）
- 计算多维度架构健康度评分
- 支持严格模式（一票否决）和宽松模式
- 输出 JSON 和文本格式报告
- 可集成到 CI/CD 流程
- 支持自定义评分规则和权重

## 2. 评分维度与算法

### 2.1 评分维度

| 维度 | 权重 | 说明 | 一票否决 |
|------|------|------|----------|
| 合规性 (Compliance) | 30% | 允许的依赖边匹配度 | 是 |
| 依赖密度 (Density) | 25% | 模块间依赖密度合理性 | 是 |
| 分层清晰 (Layering) | 25% | 分层架构调用合规性 | 是 |
| 无循环依赖 (Acyclic) | 10% | 双向依赖对数量 | 是 |
| 测试覆盖 (Coverage) | 10% | 架构契约测试覆盖度 | 否 |

### 2.2 评分算法接口

```rust
/// 评分维度 trait - 可扩展的评分规则
trait ScoringDimension: Send + Sync {
    /// 维度名称
    fn name(&self) -> &str;
    
    /// 维度权重 (0.0 - 1.0)
    fn weight(&self) -> f64;
    
    /// 是否为一票否决项
    fn is_blocking(&self) -> bool;
    
    /// 计算维度得分 (0.0 - 100.0)
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult;
    
    /// 获取该维度的配置定义
    fn config_schema(&self) -> serde_json::Value;
}

/// 维度评分结果
struct DimensionResult {
    /// 原始得分 (0.0 - 100.0)
    score: f64,
    /// 是否通过阈值
    passed: bool,
    /// 详细指标
    metrics: Vec<Metric>,
    /// 问题列表
    issues: Vec<Issue>,
    /// 建议
    suggestions: Vec<String>,
}

struct Metric {
    name: String,
    value: f64,
    threshold: Option<f64>,
    unit: String,
}

struct Issue {
    severity: Severity,
    message: String,
    location: Option<String>,
}

enum Severity {
    Info,
    Warning,
    Error,
    Critical,
}
```

### 2.3 各维度具体算法

#### 2.3.1 合规性评分 (Compliance)

```rust
struct ComplianceDimension;

impl ScoringDimension for ComplianceDimension {
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 计算公式：
        // score = (matched_edges / (matched_edges + unexpected_edges + forbidden_edges)) * 100
        // 
        // 加权因素：
        // - 匹配边权重: 1.0
        // - 意外边权重: -1.5 (违规成本)
        // - 禁止边权重: -2.0 (严重违规)
        
        let matched = ctx.scenario.matched_edges_count as f64;
        let unexpected = ctx.scenario.unexpected_edges_count as f64;
        let forbidden = ctx.scenario.forbidden_edges_count as f64;
        
        let total = matched + unexpected * 1.5 + forbidden * 2.0;
        let score = if total > 0.0 {
            (matched / total * 100.0).min(100.0)
        } else {
            100.0
        };
        
        DimensionResult {
            score,
            passed: forbidden == 0.0 && unexpected == 0.0,
            metrics: vec![
                Metric { name: "matched_edges".into(), value: matched, threshold: None, unit: "count".into() },
                Metric { name: "unexpected_edges".into(), value: unexpected, threshold: Some(0.0), unit: "count".into() },
                Metric { name: "forbidden_edges".into(), value: forbidden, threshold: Some(0.0), unit: "count".into() },
            ],
            issues: self.collect_issues(ctx),
            suggestions: self.generate_suggestions(ctx),
        }
    }
}
```

#### 2.3.2 依赖密度评分 (Density)

```rust
struct DensityDimension {
    config: DensityConfig,
}

struct DensityConfig {
    /// 理想密度范围
    ideal_min: f64,  // 默认 10%
    ideal_max: f64,  // 默认 40%
    /// 最大可接受密度
    max_acceptable: f64,  // 默认 75%
}

impl ScoringDimension for DensityDimension {
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        let density = ctx.structure.directed_density_pct;
        
        // 评分逻辑：
        // - 在理想范围内: 100 分
        // - 低于理想最小值: 线性递减，最低 60 分
        // - 高于理想最大值: 线性递减，到 max_acceptable 时为 0 分
        
        let score = if density >= self.config.ideal_min && density <= self.config.ideal_max {
            100.0
        } else if density < self.config.ideal_min {
            60.0 + (density / self.config.ideal_min) * 40.0
        } else if density < self.config.max_acceptable {
            100.0 - ((density - self.config.ideal_max) / 
                     (self.config.max_acceptable - self.config.ideal_max)) * 100.0
        } else {
            0.0
        };
        
        DimensionResult {
            score,
            passed: density <= self.config.max_acceptable,
            metrics: vec![
                Metric { name: "directed_density_pct".into(), value: density, 
                        threshold: Some(self.config.max_acceptable), unit: "%".into() },
                Metric { name: "modules_count".into(), value: ctx.structure.modules_count as f64, 
                        threshold: None, unit: "count".into() },
                Metric { name: "directed_edges_actual".into(), value: ctx.structure.directed_edges_actual as f64, 
                        threshold: None, unit: "count".into() },
            ],
            issues: vec![],
            suggestions: vec![],
        }
    }
}
```

#### 2.3.3 分层清晰评分 (Layering)

```rust
struct LayeringDimension;

/// 分层定义
enum Layer {
    Api,      // 接口层
    Service,  // 服务层
    Dao,      // 数据访问层
    Core,     // 核心域
    Support,  // 支撑域
}

/// 允许的依赖方向
const ALLOWED_LAYER_DEPS: &[(Layer, Layer)] = &[
    (Layer::Api, Layer::Service),
    (Layer::Api, Layer::Core),
    (Layer::Service, Layer::Dao),
    (Layer::Service, Layer::Core),
    (Layer::Core, Layer::Dao),
    (Layer::Support, Layer::Core),
];

impl ScoringDimension for LayeringDimension {
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 分析每个模块的分层，检查跨层依赖
        let violations = ctx.analyze_layer_violations();
        let total_deps = ctx.total_dependencies() as f64;
        let violation_count = violations.len() as f64;
        
        let score = if total_deps > 0.0 {
            ((total_deps - violation_count) / total_deps * 100.0).max(0.0)
        } else {
            100.0
        };
        
        DimensionResult {
            score,
            passed: violations.is_empty(),
            metrics: vec![
                Metric { name: "layer_violations".into(), value: violation_count, 
                        threshold: Some(0.0), unit: "count".into() },
            ],
            issues: violations.into_iter().map(|v| Issue {
                severity: Severity::Error,
                message: format!("Layer violation: {} -> {}", v.from, v.to),
                location: Some(v.location),
            }).collect(),
            suggestions: vec![],
        }
    }
}
```

#### 2.3.4 无循环依赖评分 (Acyclic)

```rust
struct AcyclicDimension;

impl ScoringDimension for AcyclicDimension {
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        let bidirectional_pairs = ctx.structure.bidirectional_pair_count as f64;
        let total_possible_pairs = ctx.total_module_pairs() as f64;
        
        // 评分逻辑：每对双向依赖扣减分数
        let base_score = 100.0;
        let deduction_per_pair = if total_possible_pairs > 0.0 {
            100.0 / total_possible_pairs.min(10.0)  // 最多扣 100 分，前 10 对影响最大
        } else {
            0.0
        };
        
        let score = (base_score - bidirectional_pairs * deduction_per_pair).max(0.0);
        
        DimensionResult {
            score,
            passed: bidirectional_pairs == 0.0,
            metrics: vec![
                Metric { name: "bidirectional_pair_count".into(), value: bidirectional_pairs, 
                        threshold: Some(0.0), unit: "count".into() },
            ],
            issues: ctx.structure.bidirectional_pairs_top.iter().map(|(pair, _, _, _)| Issue {
                severity: Severity::Warning,
                message: format!("Bidirectional dependency: {}", pair),
                location: None,
            }).collect(),
            suggestions: vec![
                "Consider introducing an abstraction layer to break the cycle".into(),
                "Evaluate if both directions are necessary".into(),
            ],
        }
    }
}
```

#### 2.3.5 测试覆盖评分 (Coverage)

```rust
struct CoverageDimension;

impl ScoringDimension for CoverageDimension {
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 基于 BDD 场景覆盖率和架构契约测试覆盖率
        let bdd_coverage = ctx.bdd_coverage();  // 0.0 - 1.0
        let contract_coverage = ctx.contract_coverage();  // 0.0 - 1.0
        
        // 加权计算
        let score = (bdd_coverage * 0.4 + contract_coverage * 0.6) * 100.0;
        
        DimensionResult {
            score,
            passed: score >= 60.0,  // 60% 为及格线
            metrics: vec![
                Metric { name: "bdd_coverage".into(), value: bdd_coverage * 100.0, 
                        threshold: Some(60.0), unit: "%".into() },
                Metric { name: "contract_coverage".into(), value: contract_coverage * 100.0, 
                        threshold: Some(60.0), unit: "%".into() },
            ],
            issues: vec![],
            suggestions: vec![],
        }
    }
}
```

### 2.4 综合评分计算

```rust
struct ArchitectureScore {
    /// 总分 (0.0 - 100.0)
    total: f64,
    /// 等级
    grade: Grade,
    /// 是否通过（严格模式/宽松模式）
    passed: bool,
    /// 各维度得分
    dimensions: Vec<DimensionScore>,
    /// 汇总信息
    summary: ScoreSummary,
}

enum Grade {
    A,  // 90-100
    B,  // 80-89
    C,  // 70-79
    D,  // 60-69
    F,  // < 60
}

struct ScoreCalculator {
    dimensions: Vec<Box<dyn ScoringDimension>>,
    mode: ScoringMode,
}

enum ScoringMode {
    /// 严格模式：任何一票否决项失败则总分失败
    Strict,
    /// 宽松模式：允许部分一票否决项失败，但会大幅降低总分
    Lenient,
    /// 仅警告模式：只计算分数，不失败
    Warning,
}

impl ScoreCalculator {
    fn calculate(&self, ctx: &ScoringContext) -> ArchitectureScore {
        let mut dimension_scores = Vec::new();
        let mut total_weighted_score = 0.0;
        let mut total_weight = 0.0;
        let mut blocking_failures = 0;
        
        for dim in &self.dimensions {
            let result = dim.calculate(ctx);
            let weight = dim.weight();
            
            total_weighted_score += result.score * weight;
            total_weight += weight;
            
            if dim.is_blocking() && !result.passed {
                blocking_failures += 1;
            }
            
            dimension_scores.push(DimensionScore {
                name: dim.name().to_string(),
                weight,
                score: result.score,
                passed: result.passed,
                is_blocking: dim.is_blocking(),
                metrics: result.metrics,
                issues: result.issues,
                suggestions: result.suggestions,
            });
        }
        
        let raw_score = total_weighted_score / total_weight;
        
        // 根据模式调整最终分数
        let final_score = match self.mode {
            ScoringMode::Strict => {
                if blocking_failures > 0 {
                    0.0  // 一票否决
                } else {
                    raw_score
                }
            }
            ScoringMode::Lenient => {
                // 每个 blocking 失败扣减 20 分
                (raw_score - blocking_failures as f64 * 20.0).max(0.0)
            }
            ScoringMode::Warning => raw_score,
        };
        
        let passed = match self.mode {
            ScoringMode::Strict => blocking_failures == 0 && final_score >= 60.0,
            ScoringMode::Lenient => final_score >= 60.0,
            ScoringMode::Warning => true,
        };
        
        ArchitectureScore {
            total: final_score,
            grade: Grade::from_score(final_score),
            passed,
            dimensions: dimension_scores,
            summary: ScoreSummary {
                blocking_failures,
                total_issues: dimension_scores.iter().map(|d| d.issues.len()).sum(),
                critical_issues: dimension_scores.iter()
                    .flat_map(|d| &d.issues)
                    .filter(|i| matches!(i.severity, Severity::Critical))
                    .count(),
            },
        }
    }
}
```

## 3. CLI 命令结构

### 3.1 命令定义

```rust
/// bcc arch score 子命令
#[derive(Subcommand)]
enum ArchAction {
    /// 计算架构健康度评分
    Score {
        /// 验证结果目录（包含 summary.json, scenario-validation.tsv, gate-evaluation.tsv）
        #[arg(long)]
        input: String,
        
        /// 评分配置文件路径
        #[arg(long)]
        config: Option<String>,
        
        /// 评分模式
        #[arg(long, default_value = "strict")]
        mode: String,  // strict | lenient | warning
        
        /// 输出格式
        #[arg(long, default_value = "text")]
        format: String,  // text | json | markdown
        
        /// 输出文件路径（默认 stdout）
        #[arg(short, long)]
        output: Option<String>,
        
        /// 失败阈值（低于此分数返回非零退出码）
        #[arg(long, default_value_t = 60.0)]
        threshold: f64,
        
        /// 详细输出
        #[arg(short, long)]
        verbose: bool,
    },
    
    /// 生成默认配置文件
    InitConfig {
        /// 输出文件路径
        #[arg(short, long, default_value = "arch-score.yaml")]
        output: String,
        
        /// 配置模板类型
        #[arg(long, default_value = "default")]
        template: String,  // default | strict | lenient | minimal
    },
    
    /// 对比多个版本的评分
    Compare {
        /// 多个输入目录（至少两个）
        #[arg(long, num_args = 2..)]
        inputs: Vec<String>,
        
        /// 版本标签
        #[arg(long, num_args = 2..)]
        labels: Vec<String>,
        
        /// 输出格式
        #[arg(long, default_value = "markdown")]
        format: String,
        
        /// 输出文件路径
        #[arg(short, long)]
        output: Option<String>,
    },
}
```

### 3.2 使用示例

```bash
# 基本评分
bcc arch score --input versions/v3-draft/

# 使用自定义配置
bcc arch score --input versions/v3-draft/ --config my-scoring.yaml

# 宽松模式，JSON 输出
bcc arch score --input versions/v3-draft/ --mode lenient --format json --output score.json

# 生成配置文件模板
bcc arch score init-config --output my-scoring.yaml --template strict

# 对比多个版本
bcc arch score compare \
  --inputs versions/v1/ versions/v2/ versions/v3-draft/ \
  --labels v1 v2 v3 \
  --format markdown --output comparison.md
```

## 4. 配置文件格式

### 4.1 配置文件结构

```yaml
# arch-score.yaml - 架构评分配置
version: "1.0"
kind: architecture_scoring_config

# 评分模式
mode: strict  # strict | lenient | warning

# 总分失败阈值
threshold: 60.0

# 维度配置
dimensions:
  # 合规性维度
  compliance:
    enabled: true
    weight: 0.30
    blocking: true
    config:
      unexpected_penalty: 1.5
      forbidden_penalty: 2.0
      
  # 依赖密度维度
  density:
    enabled: true
    weight: 0.25
    blocking: true
    config:
      ideal_min: 10.0
      ideal_max: 40.0
      max_acceptable: 75.0
      
  # 分层清晰维度
  layering:
    enabled: true
    weight: 0.25
    blocking: true
    config:
      layers:
        - name: api
          precedence: 1
        - name: service
          precedence: 2
        - name: dao
          precedence: 3
        - name: core
          precedence: 2
        - name: support
          precedence: 2
      allowed_transitions:
        - [api, service]
        - [api, core]
        - [service, dao]
        - [service, core]
        - [core, dao]
        - [support, core]
        
  # 无循环依赖维度
  acyclic:
    enabled: true
    weight: 0.10
    blocking: true
    config:
      max_pairs: 0
      
  # 测试覆盖维度
  coverage:
    enabled: true
    weight: 0.10
    blocking: false
    config:
      min_bdd_coverage: 60.0
      min_contract_coverage: 60.0

# 自定义规则（高级）
custom_rules:
  - name: "critical_module_isolation"
    description: "Critical modules should not depend on non-critical modules"
    severity: error
    condition: |
      module.tags.contains("critical") && 
      dependency.module.tags.contains("non-critical")
    
  - name: "api_stability"
    description: "API modules should have stable dependencies"
    severity: warning
    condition: |
      module.layer == "api" && 
      dependency.stability_score < 0.5

# 输出配置
output:
  text:
    show_details: true
    show_suggestions: true
    max_issues: 20
  json:
    pretty: true
    include_metrics: true
  markdown:
    template: default  # default | detailed | minimal
```

### 4.2 配置验证

```rust
impl ScoringConfig {
    /// 验证配置有效性
    fn validate(&self) -> Result<(), ConfigError> {
        // 检查权重总和
        let total_weight: f64 = self.dimensions.iter()
            .filter(|d| d.enabled)
            .map(|d| d.weight)
            .sum();
        
        if (total_weight - 1.0).abs() > 0.001 {
            return Err(ConfigError::InvalidWeights {
                total: total_weight,
                expected: 1.0,
            });
        }
        
        // 检查各维度配置
        for dim in &self.dimensions {
            if dim.enabled {
                dim.validate()?;
            }
        }
        
        Ok(())
    }
}
```

## 5. 输出报告格式

### 5.1 文本格式输出

```
================================================================================
                         Architecture Health Score
================================================================================

Overall Score: 72.5/100 (Grade: C)
Status: ⚠️  PASSED (with warnings)
Mode: strict

--------------------------------------------------------------------------------
                              Dimension Breakdown
--------------------------------------------------------------------------------

✅ Compliance        85.0/100  [███████░░░]  weight: 30%  contribution: 25.5
❌ Density           45.0/100  [████░░░░░░]  weight: 25%  contribution: 11.3  [BLOCKING]
✅ Layering         100.0/100  [██████████]  weight: 25%  contribution: 25.0
✅ Acyclic           90.0/100  [█████████░]  weight: 10%  contribution:  9.0
⚠️  Coverage          20.0/100  [██░░░░░░░░]  weight: 10%  contribution:  2.0

--------------------------------------------------------------------------------
                                 Issues Summary
--------------------------------------------------------------------------------

Critical: 0 | Error: 1 | Warning: 3 | Info: 5

[BLOCKING] Density (45.0/100):
  ❌ directed_density_pct: 75% (threshold: 40%)
  
  Suggestions:
    - Consider refactoring high-coupling modules
    - Introduce abstraction layers to reduce direct dependencies
    - Review module boundaries for potential splitting

Coverage (20.0/100):
  ⚠️ bdd_coverage: 15% (threshold: 60%)
  ⚠️ contract_coverage: 25% (threshold: 60%)
  
  Suggestions:
    - Add BDD scenarios for uncovered architectural edges
    - Implement contract tests for critical module interactions

--------------------------------------------------------------------------------
                                 Top Issues
--------------------------------------------------------------------------------

1. [ERROR] Layer violation: api.order -> dao.payment
   Location: src/api/order.ts -> src/dao/payment.ts
   
2. [WARNING] Bidirectional dependency: billing <-> payment
   Suggestion: Consider introducing a BillingService abstraction
   
3. [WARNING] High coupling: account module has 15 outgoing dependencies

================================================================================
```

### 5.2 JSON 格式输出

```json
{
  "version": "1.0",
  "generated_at": "2026-02-16T15:30:00Z",
  "summary": {
    "total_score": 72.5,
    "grade": "C",
    "passed": true,
    "mode": "strict",
    "threshold": 60.0,
    "issue_counts": {
      "critical": 0,
      "error": 1,
      "warning": 3,
      "info": 5
    }
  },
  "dimensions": [
    {
      "name": "compliance",
      "display_name": "Compliance",
      "weight": 0.30,
      "score": 85.0,
      "passed": true,
      "is_blocking": true,
      "contribution": 25.5,
      "metrics": [
        {
          "name": "matched_edges",
          "value": 27,
          "unit": "count"
        },
        {
          "name": "unexpected_edges",
          "value": 0,
          "threshold": 0,
          "unit": "count"
        },
        {
          "name": "forbidden_edges",
          "value": 5,
          "threshold": 0,
          "unit": "count"
        }
      ],
      "issues": [],
      "suggestions": [
        "Review forbidden edges for potential refactoring"
      ]
    },
    {
      "name": "density",
      "display_name": "Dependency Density",
      "weight": 0.25,
      "score": 45.0,
      "passed": false,
      "is_blocking": true,
      "contribution": 11.3,
      "metrics": [
        {
          "name": "directed_density_pct",
          "value": 75.0,
          "threshold": 40.0,
          "unit": "%"
        }
      ],
      "issues": [
        {
          "severity": "error",
          "message": "Dependency density exceeds threshold",
          "metric": "directed_density_pct",
          "actual": 75.0,
          "threshold": 40.0
        }
      ],
      "suggestions": [
        "Consider refactoring high-coupling modules",
        "Introduce abstraction layers to reduce direct dependencies"
      ]
    }
  ],
  "recommendations": [
    {
      "priority": "high",
      "category": "density",
      "message": "Reduce dependency density by introducing service layer abstractions",
      "affected_modules": ["account", "billing", "payment"]
    },
    {
      "priority": "medium",
      "category": "coverage",
      "message": "Increase BDD scenario coverage for critical paths",
      "affected_modules": ["order", "inventory"]
    }
  ]
}
```

### 5.3 Markdown 格式输出

```markdown
# Architecture Health Score Report

**Generated:** 2026-02-16T15:30:00Z  
**Overall Score:** 72.5/100 (Grade: C)  
**Status:** ⚠️ PASSED (with warnings)  
**Mode:** strict

## Summary

| Metric | Value |
|--------|-------|
| Total Score | 72.5/100 |
| Grade | C |
| Passed | ✅ |
| Critical Issues | 0 |
| Errors | 1 |
| Warnings | 3 |

## Dimension Breakdown

| Dimension | Score | Weight | Contribution | Status |
|-----------|-------|--------|--------------|--------|
| Compliance | 85.0 | 30% | 25.5 | ✅ |
| Density | 45.0 | 25% | 11.3 | ❌ |
| Layering | 100.0 | 25% | 25.0 | ✅ |
| Acyclic | 90.0 | 10% | 9.0 | ✅ |
| Coverage | 20.0 | 10% | 2.0 | ⚠️ |

## Issues

### Density (Score: 45.0)

| Severity | Metric | Actual | Threshold | Status |
|----------|--------|--------|-----------|--------|
| ❌ Error | directed_density_pct | 75% | 40% | FAIL |

**Suggestions:**
- Consider refactoring high-coupling modules
- Introduce abstraction layers to reduce direct dependencies

## Recommendations

1. **High Priority:** Reduce dependency density by introducing service layer abstractions
   - Affected modules: account, billing, payment

2. **Medium Priority:** Increase BDD scenario coverage for critical paths
   - Affected modules: order, inventory
```

## 6. 代码结构

### 6.1 目录结构

```
compiler/bcc/src/
├── main.rs                 # CLI 入口
├── lib.rs                  # 库入口
├── arch.rs                 # 现有 arch 模块
├── arch/
│   ├── mod.rs              # arch 子模块入口
│   ├── matrix.rs           # matrix 命令实现（从 arch.rs 迁移）
│   ├── validate.rs         # validate 命令实现（从 arch.rs 迁移）
│   ├── report.rs           # report 命令实现（从 arch.rs 迁移）
│   └── score/              # 新增评分模块
│       ├── mod.rs          # 评分模块入口
│       ├── cli.rs          # CLI 命令处理
│       ├── config.rs       # 配置文件解析
│       ├── calculator.rs   # 评分计算引擎
│       ├── context.rs      # 评分上下文
│       ├── dimensions/     # 各维度实现
│       │   ├── mod.rs
│       │   ├── compliance.rs
│       │   ├── density.rs
│       │   ├── layering.rs
│       │   ├── acyclic.rs
│       │   └── coverage.rs
│       ├── output/         # 输出格式化
│       │   ├── mod.rs
│       │   ├── text.rs
│       │   ├── json.rs
│       │   └── markdown.rs
│       └── models.rs       # 数据模型
├── extract/
├── graph/
└── ...
```

### 6.2 核心模块实现

#### 6.2.1 评分模块入口 (arch/score/mod.rs)

```rust
//! 架构健康度评分模块

pub mod cli;
pub mod config;
pub mod calculator;
pub mod context;
pub mod dimensions;
pub mod output;
pub mod models;

pub use calculator::{ScoreCalculator, ScoringMode};
pub use config::ScoringConfig;
pub use context::ScoringContext;
pub use models::{ArchitectureScore, DimensionScore, Grade};

use std::path::Path;

/// 执行评分命令的主入口
pub fn score(
    input: &str,
    config: Option<&str>,
    mode: &str,
    format: &str,
    output: Option<&str>,
    threshold: f64,
    verbose: bool,
) {
    if let Err(e) = score_impl(input, config, mode, format, output, threshold, verbose) {
        eprintln!("[score] error: {}", e);
        std::process::exit(1);
    }
}

fn score_impl(
    input: &str,
    config: Option<&str>,
    mode: &str,
    format: &str,
    output: Option<&str>,
    threshold: f64,
    verbose: bool,
) -> Result<(), String> {
    // 加载配置
    let config = match config {
        Some(path) => ScoringConfig::from_file(path)?,
        None => ScoringConfig::default(),
    };
    
    // 解析模式
    let scoring_mode = match mode {
        "strict" => ScoringMode::Strict,
        "lenient" => ScoringMode::Lenient,
        "warning" => ScoringMode::Warning,
        _ => return Err(format!("invalid mode: {}", mode)),
    };
    
    // 构建评分上下文
    let ctx = ScoringContext::from_input_dir(input)?;
    
    // 创建评分器
    let calculator = ScoreCalculator::new(config, scoring_mode);
    
    // 计算评分
    let score = calculator.calculate(&ctx);
    
    // 格式化输出
    let output_content = match format {
        "json" => output::json::format(&score, verbose)?,
        "markdown" | "md" => output::markdown::format(&score, verbose)?,
        "text" | _ => output::text::format(&score, verbose)?,
    };
    
    // 输出结果
    match output {
        Some(path) => {
            std::fs::write(path, output_content)
                .map_err(|e| format!("write output failed: {}", e))?;
            println!("score_report_written={}", path);
        }
        None => {
            println!("{}", output_content);
        }
    }
    
    // 根据阈值决定退出码
    if !score.passed || score.total < threshold {
        std::process::exit(2);
    }
    
    Ok(())
}

/// 生成默认配置
pub fn init_config(output: &str, template: &str) {
    if let Err(e) = init_config_impl(output, template) {
        eprintln!("[score] error: {}", e);
        std::process::exit(1);
    }
}

fn init_config_impl(output: &str, template: &str) -> Result<(), String> {
    let config = match template {
        "strict" => ScoringConfig::strict_template(),
        "lenient" => ScoringConfig::lenient_template(),
        "minimal" => ScoringConfig::minimal_template(),
        _ => ScoringConfig::default(),
    };
    
    let yaml = serde_yaml::to_string(&config)
        .map_err(|e| format!("serialize config failed: {}", e))?;
    
    std::fs::write(output, yaml)
        .map_err(|e| format!("write config failed: {}", e))?;
    
    println!("config_template_written={}", output);
    Ok(())
}

/// 对比多个版本
pub fn compare(
    inputs: &[String],
    labels: &[String],
    format: &str,
    output: Option<&str>,
) {
    if let Err(e) = compare_impl(inputs, labels, format, output) {
        eprintln!("[score] error: {}", e);
        std::process::exit(1);
    }
}

fn compare_impl(
    inputs: &[String],
    labels: &[String],
    format: &str,
    output: Option<&str>,
) -> Result<(), String> {
    if inputs.len() < 2 {
        return Err("at least 2 inputs required for comparison".into());
    }
    if !labels.is_empty() && labels.len() != inputs.len() {
        return Err("labels count must match inputs count".into());
    }
    
    let config = ScoringConfig::default();
    let calculator = ScoreCalculator::new(config, ScoringMode::Strict);
    
    let mut results = Vec::new();
    for (i, input) in inputs.iter().enumerate() {
        let label = labels.get(i)
            .cloned()
            .unwrap_or_else(|| format!("v{}", i + 1));
        let ctx = ScoringContext::from_input_dir(input)?;
        let score = calculator.calculate(&ctx);
        results.push((label, score));
    }
    
    let content = match format {
        "json" => output::json::format_comparison(&results)?,
        _ => output::markdown::format_comparison(&results)?,
    };
    
    match output {
        Some(path) => {
            std::fs::write(path, content)
                .map_err(|e| format!("write output failed: {}", e))?;
            println!("comparison_report_written={}", path);
        }
        None => println!("{}", content),
    }
    
    Ok(())
}
```

#### 6.2.2 CLI 集成 (arch/score/cli.rs)

```rust
//! CLI 命令定义和处理

use clap::Subcommand;

#[derive(Subcommand)]
pub enum ScoreAction {
    /// 计算架构健康度评分
    Score {
        /// 验证结果目录
        #[arg(long)]
        input: String,
        
        /// 评分配置文件路径
        #[arg(long)]
        config: Option<String>,
        
        /// 评分模式: strict | lenient | warning
        #[arg(long, default_value = "strict")]
        mode: String,
        
        /// 输出格式: text | json | markdown
        #[arg(long, default_value = "text")]
        format: String,
        
        /// 输出文件路径
        #[arg(short, long)]
        output: Option<String>,
        
        /// 失败阈值
        #[arg(long, default_value_t = 60.0)]
        threshold: f64,
        
        /// 详细输出
        #[arg(short, long)]
        verbose: bool,
    },
    
    /// 生成默认配置文件
    InitConfig {
        /// 输出文件路径
        #[arg(short, long, default_value = "arch-score.yaml")]
        output: String,
        
        /// 配置模板: default | strict | lenient | minimal
        #[arg(long, default_value = "default")]
        template: String,
    },
    
    /// 对比多个版本的评分
    Compare {
        /// 多个输入目录
        #[arg(long, num_args = 2..)]
        inputs: Vec<String>,
        
        /// 版本标签
        #[arg(long, num_args = 2..)]
        labels: Vec<String>,
        
        /// 输出格式
        #[arg(long, default_value = "markdown")]
        format: String,
        
        /// 输出文件路径
        #[arg(short, long)]
        output: Option<String>,
    },
}

impl ScoreAction {
    pub fn execute(self) {
        match self {
            ScoreAction::Score { input, config, mode, format, output, threshold, verbose } => {
                super::score(&input, config.as_deref(), &mode, &format, output.as_deref(), threshold, verbose);
            }
            ScoreAction::InitConfig { output, template } => {
                super::init_config(&output, &template);
            }
            ScoreAction::Compare { inputs, labels, format, output } => {
                super::compare(&inputs, &labels, &format, output.as_deref());
            }
        }
    }
}
```

#### 6.2.3 主入口集成 (main.rs 修改)

```rust
// 在 ArchAction 枚举中添加 score 子命令
#[derive(Subcommand)]
enum ArchAction {
    // ... 现有命令
    
    /// 架构健康度评分
    Score {
        #[command(subcommand)]
        action: arch::score::cli::ScoreAction,
    },
}

// 在 match 语句中添加处理
Some(Commands::Arch { action }) => match action {
    // ... 现有命令处理
    
    ArchAction::Score { action } => {
        action.execute();
    }
},
```

## 7. CI/CD 集成

### 7.1 GitHub Actions 示例

```yaml
# .github/workflows/architecture-score.yml
name: Architecture Health Score

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  score:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup BCC
        run: |
          cargo build --release -p bcc
          sudo ln -sf $(pwd)/target/release/bcc /usr/local/bin/bcc
      
      - name: Extract Architecture
        run: |
          bcc extract src/ --batch --lang typescript --output ast.json
      
      - name: Validate Architecture
        run: |
          bcc arch validate \
            --target seed/v3.target-matrix.yaml \
            --transition seed/v3.transition-matrix.yaml \
            --gates seed/v3.gates.yaml \
            --actual ast.json \
            --out-dir validation-results/
      
      - name: Calculate Architecture Score
        run: |
          bcc arch score \
            --input validation-results/ \
            --config .arch-score.yaml \
            --mode strict \
            --format markdown \
            --output arch-score-report.md \
            --threshold 70.0 \
            --verbose
      
      - name: Upload Score Report
        uses: actions/upload-artifact@v4
        with:
          name: architecture-score
          path: arch-score-report.md
      
      - name: Comment PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const report = fs.readFileSync('arch-score-report.md', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: report
            });
```

### 7.2 预提交钩子示例

```yaml
# .pre-commit-hooks.yml
- id: architecture-score
  name: Architecture Health Score
  entry: bcc arch score
  language: system
  files: '\.(ts|tsx|ex|exs|php)$'
  pass_filenames: false
  args:
    - --input
    - validation-results/
    - --mode
    - strict
    - --threshold
    - "60.0"
```

## 8. 测试策略

### 8.1 单元测试

```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_compliance_dimension_perfect_score() {
        let ctx = ScoringContext::builder()
            .matched_edges(100)
            .unexpected_edges(0)
            .forbidden_edges(0)
            .build();
        
        let dim = ComplianceDimension;
        let result = dim.calculate(&ctx);
        
        assert_eq!(result.score, 100.0);
        assert!(result.passed);
    }
    
    #[test]
    fn test_compliance_dimension_with_violations() {
        let ctx = ScoringContext::builder()
            .matched_edges(80)
            .unexpected_edges(10)
            .forbidden_edges(5)
            .build();
        
        let dim = ComplianceDimension;
        let result = dim.calculate(&ctx);
        
        assert!(result.score < 100.0);
        assert!(!result.passed);
    }
    
    #[test]
    fn test_strict_mode_blocking() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Strict);
        
        let ctx = ScoringContext::builder()
            .with_blocking_failure()
            .build();
        
        let score = calculator.calculate(&ctx);
        
        assert_eq!(score.total, 0.0);
        assert!(!score.passed);
    }
    
    #[test]
    fn test_lenient_mode_penalty() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Lenient);
        
        let ctx = ScoringContext::builder()
            .with_blocking_failure()
            .with_base_score(80.0)
            .build();
        
        let score = calculator.calculate(&ctx);
        
        assert!(score.total > 0.0);
        assert!(score.total < 80.0);
    }
}
```

### 8.2 集成测试

```rust
// tests/arch_score_bdd.rs
#[test]
fn test_score_end_to_end() {
    let temp_dir = tempfile::tempdir().unwrap();
    
    // 创建测试输入
    create_test_validation_results(temp_dir.path());
    
    // 执行评分
    let output = temp_dir.path().join("score.json");
    bcc::arch::score::score(
        temp_dir.path().to_str().unwrap(),
        None,
        "strict",
        "json",
        Some(output.to_str().unwrap()),
        60.0,
        false,
    );
    
    // 验证输出
    let result: serde_json::Value = serde_json::from_str(
        &std::fs::read_to_string(&output).unwrap()
    ).unwrap();
    
    assert!(result["summary"]["total_score"].as_f64().unwrap() > 0.0);
    assert!(result["dimensions"].as_array().unwrap().len() > 0);
}
```

## 9. 扩展性设计

### 9.1 添加新维度

```rust
// 1. 创建维度实现
pub struct CustomDimension {
    config: CustomConfig,
}

impl ScoringDimension for CustomDimension {
    fn name(&self) -> &str { "custom" }
    fn weight(&self) -> f64 { self.config.weight }
    fn is_blocking(&self) -> bool { self.config.blocking }
    
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 自定义计算逻辑
    }
}

// 2. 在配置中添加
#[derive(Debug, Deserialize)]
struct ScoringConfig {
    // ... 现有维度
    custom: Option<DimensionConfig<CustomConfig>>,
}

// 3. 在计算器注册
impl ScoreCalculator {
    fn register_default_dimensions(&mut self) {
        // ... 现有维度
        if let Some(config) = &self.config.custom {
            if config.enabled {
                self.dimensions.push(Box::new(CustomDimension {
                    config: config.settings.clone(),
                }));
            }
        }
    }
}
```

### 9.2 插件系统（未来）

```rust
/// 维度插件 trait
trait DimensionPlugin: Send + Sync {
    fn metadata(&self) -> PluginMetadata;
    fn create_dimension(&self, config: &serde_json::Value) -> Box<dyn ScoringDimension>;
}

/// 插件注册表
struct PluginRegistry {
    plugins: HashMap<String, Box<dyn DimensionPlugin>>,
}

impl PluginRegistry {
    fn load_plugin(&mut self, path: &str) -> Result<(), PluginError> {
        // 动态加载插件
    }
}
```

## 10. 里程碑与实现计划

### Phase 1: 核心框架 (MVP)
- [ ] 评分模块基础结构
- [ ] 配置解析
- [ ] 评分上下文构建
- [ ] 基础维度实现（合规性、密度）
- [ ] 文本输出格式

### Phase 2: 完整功能
- [ ] 全维度实现
- [ ] JSON/Markdown 输出
- [ ] 配置文件生成
- [ ] 版本对比功能
- [ ] CI/CD 集成示例

### Phase 3: 增强功能
- [ ] 自定义规则引擎
- [ ] 趋势分析
- [ ] 插件系统
- [ ] Web 报告可视化
- [ ] 历史数据追踪

## 11. 附录

### 11.1 与现有 BCC 命令的关系

```
bcc arch matrix      → 生成 target/transition/gates
         validate    → 验证并生成 summary.json
         report      → 生成架构债务报告
         score       → [新增] 计算健康度评分
```

### 11.2 数据流

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  arch       │ → │  validate   │ → │  score      │
│  matrix     │    │  (验证)     │    │  (评分)     │
└─────────────┘    └─────────────┘    └─────────────┘
                           ↓                ↓
                    ┌─────────────┐    ┌─────────────┐
                    │ summary.json│    │ score.json  │
                    │ *.tsv       │    │ report.md   │
                    └─────────────┘    └─────────────┘
```

### 11.3 命名约定

- 命令: `bcc arch score`
- 配置文件: `.arch-score.yaml` 或 `arch-score.yaml`
- 输出文件: `arch-score-report.{json,md}`
- 环境变量前缀: `BCC_SCORE_`
