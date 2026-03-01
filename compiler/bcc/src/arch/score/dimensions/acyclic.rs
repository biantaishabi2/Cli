//! 无循环依赖维度
//!
//! 评估模块间是否存在循环依赖（双向依赖）。

use super::{DimensionResult, ScoringDimension};
use crate::arch::score::config::AcyclicDimensionConfig;
use crate::arch::score::context::ScoringContext;
use crate::arch::score::models::{Issue, Metric, Severity};

/// 无循环依赖维度
pub struct AcyclicDimension {
    weight: f64,
    blocking: bool,
    config: AcyclicDimensionConfig,
}

impl AcyclicDimension {
    /// 创建新的无循环依赖维度
    pub fn new(weight: f64, blocking: bool, config: AcyclicDimensionConfig) -> Self {
        Self {
            weight,
            blocking,
            config,
        }
    }

    /// 收集问题
    fn collect_issues(&self, ctx: &ScoringContext) -> Vec<Issue> {
        let mut issues = Vec::new();
        let pair_count = ctx.structure.bidirectional_pair_count;

        if pair_count > 0 {
            let severity = if pair_count > self.config.max_pairs as i64 {
                Severity::Error
            } else {
                Severity::Warning
            };

            issues.push(Issue {
                severity,
                message: format!(
                    "Found {} bidirectional dependency pairs (limit: {})",
                    pair_count, self.config.max_pairs
                ),
                metric: Some("bidirectional_pair_count".to_string()),
                actual: Some(pair_count as f64),
                threshold: Some(self.config.max_pairs as f64),
                location: None,
            });

            // 添加具体的问题对
            for (pair, ab, ba, total) in &ctx.structure.bidirectional_pairs_top {
                issues.push(Issue {
                    severity: Severity::Warning,
                    message: format!(
                        "Bidirectional dependency: {} ({} + {} = {} total)",
                        pair, ab, ba, total
                    ),
                    metric: None,
                    actual: None,
                    threshold: None,
                    location: Some(pair.clone()),
                });
            }
        }

        issues
    }

    /// 生成建议
    fn generate_suggestions(&self, ctx: &ScoringContext) -> Vec<String> {
        let mut suggestions = Vec::new();
        let pair_count = ctx.structure.bidirectional_pair_count;

        if pair_count > 0 {
            suggestions
                .push("Identify the root cause of each bidirectional dependency".to_string());
            suggestions.push(
                "Consider introducing an abstraction (interface/trait) to break the cycle"
                    .to_string(),
            );
            suggestions
                .push("Evaluate if both directions of dependency are truly necessary".to_string());
            suggestions
                .push("Apply the Dependency Inversion Principle to decouple modules".to_string());
        } else {
            suggestions.push("No circular dependencies detected. Excellent!".to_string());
        }

        suggestions
    }
}

impl ScoringDimension for AcyclicDimension {
    fn name(&self) -> &str {
        "acyclic"
    }

    fn display_name(&self) -> &str {
        "Acyclic Dependencies"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        let pair_count = ctx.structure.bidirectional_pair_count as f64;
        let max_pairs = self.config.max_pairs as f64;

        // 计算可能的模块对数
        let total_pairs = ctx.total_module_pairs() as f64;

        // 评分逻辑
        let score = if pair_count == 0.0 {
            100.0
        } else if total_pairs > 0.0 {
            // 基于对数计算，前几对影响最大
            let deduction = (pair_count * self.config.deduction_per_pair).min(100.0);
            100.0 - deduction
        } else {
            100.0
        };

        // 通过条件：不超过最大允许对数
        let passed = pair_count <= max_pairs;

        DimensionResult {
            score,
            passed,
            metrics: vec![
                Metric {
                    name: "bidirectional_pair_count".to_string(),
                    display_name: Some("Bidirectional Pairs".to_string()),
                    value: pair_count,
                    threshold: Some(max_pairs),
                    unit: "pairs".to_string(),
                    passed: Some(passed),
                },
                Metric {
                    name: "total_module_pairs".to_string(),
                    display_name: Some("Total Possible Pairs".to_string()),
                    value: total_pairs,
                    threshold: None,
                    unit: "pairs".to_string(),
                    passed: None,
                },
                Metric {
                    name: "acyclic_ratio".to_string(),
                    display_name: Some("Acyclic Ratio".to_string()),
                    value: if total_pairs > 0.0 {
                        (total_pairs - pair_count) / total_pairs * 100.0
                    } else {
                        100.0
                    },
                    threshold: Some(100.0),
                    unit: "%".to_string(),
                    passed: Some(pair_count == 0.0),
                },
            ],
            issues: self.collect_issues(ctx),
            suggestions: self.generate_suggestions(ctx),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_context(pairs: i64, modules: i64) -> ScoringContext {
        ScoringContext {
            structure: crate::arch::score::context::StructureData {
                bidirectional_pair_count: pairs,
                modules_count: modules,
                ..Default::default()
            },
            ..Default::default()
        }
    }

    #[test]
    fn test_no_cycles() {
        let ctx = create_context(0, 10);
        let dim = AcyclicDimension::new(0.10, true, AcyclicDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert_eq!(result.score, 100.0);
        assert!(result.passed);
        assert!(result.issues.is_empty());
    }

    #[test]
    fn test_with_cycles() {
        let ctx = create_context(5, 10);
        let dim = AcyclicDimension::new(0.10, true, AcyclicDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert!(result.score < 100.0);
        assert!(!result.passed);
        assert!(!result.issues.is_empty());
    }

    #[test]
    fn test_within_limit() {
        let ctx = create_context(3, 10);
        let dim = AcyclicDimension::new(
            0.10,
            true,
            AcyclicDimensionConfig {
                max_pairs: 5,
                deduction_per_pair: 10.0,
            },
        );
        let result = dim.calculate(&ctx);

        assert!(result.passed); // 3 <= 5
        assert!(result.score < 100.0);
    }

    #[test]
    fn test_exceeds_limit() {
        let ctx = create_context(10, 20);
        let dim = AcyclicDimension::new(
            0.10,
            true,
            AcyclicDimensionConfig {
                max_pairs: 5,
                deduction_per_pair: 10.0,
            },
        );
        let result = dim.calculate(&ctx);

        assert!(!result.passed); // 10 > 5
    }
}
