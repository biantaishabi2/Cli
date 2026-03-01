//! 合规性维度
//!
//! 评估架构契约的遵守情况，包括允许的依赖边匹配度、意外边和禁止边。

use super::{DimensionResult, ScoringDimension};
use crate::arch::score::config::ComplianceDimensionConfig;
use crate::arch::score::context::ScoringContext;
use crate::arch::score::models::{Issue, Metric, Severity};

/// 合规性维度
pub struct ComplianceDimension {
    weight: f64,
    blocking: bool,
    config: ComplianceDimensionConfig,
}

impl ComplianceDimension {
    /// 创建新的合规性维度
    pub fn new(weight: f64, blocking: bool, config: ComplianceDimensionConfig) -> Self {
        Self {
            weight,
            blocking,
            config,
        }
    }

    /// 收集问题
    fn collect_issues(&self, ctx: &ScoringContext) -> Vec<Issue> {
        let mut issues = Vec::new();

        // 检查禁止边
        if ctx.scenario.forbidden_edges_count > 0 {
            issues.push(Issue {
                severity: Severity::Critical,
                message: format!(
                    "Found {} forbidden dependencies",
                    ctx.scenario.forbidden_edges_count
                ),
                metric: Some("forbidden_edges_count".to_string()),
                actual: Some(ctx.scenario.forbidden_edges_count as f64),
                threshold: Some(0.0),
                location: None,
            });
        }

        // 检查意外边
        if ctx.scenario.unexpected_edges_count > 0 {
            issues.push(Issue {
                severity: Severity::Error,
                message: format!(
                    "Found {} unexpected dependencies not in target contract",
                    ctx.scenario.unexpected_edges_count
                ),
                metric: Some("unexpected_edges_count".to_string()),
                actual: Some(ctx.scenario.unexpected_edges_count as f64),
                threshold: Some(0.0),
                location: None,
            });
        }

        // 检查缺失边
        if ctx.scenario.missing_edges_count > 0 {
            issues.push(Issue {
                severity: Severity::Warning,
                message: format!(
                    "Found {} expected dependencies missing",
                    ctx.scenario.missing_edges_count
                ),
                metric: Some("missing_edges_count".to_string()),
                actual: Some(ctx.scenario.missing_edges_count as f64),
                threshold: Some(0.0),
                location: None,
            });
        }

        issues
    }

    /// 生成建议
    fn generate_suggestions(&self, ctx: &ScoringContext) -> Vec<String> {
        let mut suggestions = Vec::new();

        if ctx.scenario.forbidden_edges_count > 0 {
            suggestions.push("Review and refactor forbidden dependencies immediately".to_string());
            suggestions.push(
                "Consider introducing abstraction layers to break forbidden dependencies"
                    .to_string(),
            );
        }

        if ctx.scenario.unexpected_edges_count > 0 {
            suggestions
                .push("Update target contract to include legitimate new dependencies".to_string());
            suggestions.push("Refactor code to remove unnecessary dependencies".to_string());
        }

        if ctx.scenario.missing_edges_count > 0 {
            suggestions.push("Verify if expected dependencies are still required".to_string());
            suggestions
                .push("Update target contract if dependencies are no longer needed".to_string());
        }

        if suggestions.is_empty() {
            suggestions
                .push("Architecture compliance is excellent. Keep up the good work!".to_string());
        }

        suggestions
    }
}

impl ScoringDimension for ComplianceDimension {
    fn name(&self) -> &str {
        "compliance"
    }

    fn display_name(&self) -> &str {
        "Compliance"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        let matched = ctx.scenario.matched_edges_count as f64;
        let unexpected = ctx.scenario.unexpected_edges_count as f64;
        let forbidden = ctx.scenario.forbidden_edges_count as f64;

        // 计算公式：
        // 基础分 = matched / (matched + unexpected * penalty + forbidden * penalty)
        // 禁止边和意外边有更高的惩罚系数
        let weighted_violations =
            unexpected * self.config.unexpected_penalty + forbidden * self.config.forbidden_penalty;

        let total = matched + weighted_violations;

        let score = if total > 0.0 {
            (matched / total * 100.0).min(100.0)
        } else {
            100.0 // 没有依赖时默认满分
        };

        // 通过条件：没有禁止边和意外边
        let passed = forbidden == 0.0 && unexpected == 0.0;

        DimensionResult {
            score,
            passed,
            metrics: vec![
                Metric {
                    name: "matched_edges".to_string(),
                    display_name: Some("Matched Edges".to_string()),
                    value: matched,
                    threshold: None,
                    unit: "count".to_string(),
                    passed: None,
                },
                Metric {
                    name: "matched_total_edges".to_string(),
                    display_name: Some("Matched Total Edge Weight".to_string()),
                    value: ctx.scenario.matched_total_edges as f64,
                    threshold: None,
                    unit: "edges".to_string(),
                    passed: None,
                },
                Metric {
                    name: "unexpected_edges".to_string(),
                    display_name: Some("Unexpected Edges".to_string()),
                    value: unexpected,
                    threshold: Some(0.0),
                    unit: "count".to_string(),
                    passed: Some(unexpected == 0.0),
                },
                Metric {
                    name: "unexpected_total_edges".to_string(),
                    display_name: Some("Unexpected Total Edge Weight".to_string()),
                    value: ctx.scenario.unexpected_total_edges as f64,
                    threshold: Some(0.0),
                    unit: "edges".to_string(),
                    passed: Some(ctx.scenario.unexpected_total_edges == 0),
                },
                Metric {
                    name: "forbidden_edges".to_string(),
                    display_name: Some("Forbidden Edges".to_string()),
                    value: forbidden,
                    threshold: Some(0.0),
                    unit: "count".to_string(),
                    passed: Some(forbidden == 0.0),
                },
                Metric {
                    name: "forbidden_total_edges".to_string(),
                    display_name: Some("Forbidden Total Edge Weight".to_string()),
                    value: ctx.scenario.forbidden_total_edges as f64,
                    threshold: Some(0.0),
                    unit: "edges".to_string(),
                    passed: Some(ctx.scenario.forbidden_total_edges == 0),
                },
                Metric {
                    name: "missing_edges".to_string(),
                    display_name: Some("Missing Edges".to_string()),
                    value: ctx.scenario.missing_edges_count as f64,
                    threshold: Some(0.0),
                    unit: "count".to_string(),
                    passed: Some(ctx.scenario.missing_edges_count == 0),
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

    fn create_context(matched: i64, unexpected: i64, forbidden: i64) -> ScoringContext {
        ScoringContext {
            scenario: crate::arch::score::context::ScenarioData {
                matched_edges_count: matched,
                matched_total_edges: matched,
                unexpected_edges_count: unexpected,
                unexpected_total_edges: unexpected,
                forbidden_edges_count: forbidden,
                forbidden_total_edges: forbidden,
                missing_edges_count: 0,
                ..Default::default()
            },
            ..Default::default()
        }
    }

    #[test]
    fn test_perfect_compliance() {
        let ctx = create_context(100, 0, 0);
        let dim = ComplianceDimension::new(0.3, true, ComplianceDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert_eq!(result.score, 100.0);
        assert!(result.passed);
        assert!(result.issues.is_empty());
    }

    #[test]
    fn test_with_forbidden_edges() {
        let ctx = create_context(80, 0, 5);
        let dim = ComplianceDimension::new(0.3, true, ComplianceDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert!(result.score < 100.0);
        assert!(!result.passed);
        assert!(!result.issues.is_empty());
    }

    #[test]
    fn test_with_unexpected_edges() {
        let ctx = create_context(80, 10, 0);
        let dim = ComplianceDimension::new(0.3, true, ComplianceDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert!(result.score < 100.0);
        assert!(!result.passed);
    }

    #[test]
    fn test_empty_dependencies() {
        let ctx = create_context(0, 0, 0);
        let dim = ComplianceDimension::new(0.3, true, ComplianceDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert_eq!(result.score, 100.0);
        assert!(result.passed);
    }
}
