//! 依赖密度维度
//!
//! 评估模块间依赖密度的合理性，密度过高表示耦合度过高。

use super::{DimensionResult, ScoringDimension};
use crate::arch::score::config::DensityDimensionConfig;
use crate::arch::score::context::ScoringContext;
use crate::arch::score::models::{Issue, Metric, Severity};

/// 依赖密度维度
pub struct DensityDimension {
    weight: f64,
    blocking: bool,
    config: DensityDimensionConfig,
}

impl DensityDimension {
    /// 创建新的依赖密度维度
    pub fn new(weight: f64, blocking: bool, config: DensityDimensionConfig) -> Self {
        Self {
            weight,
            blocking,
            config,
        }
    }

    /// 收集问题
    fn collect_issues(&self, ctx: &ScoringContext) -> Vec<Issue> {
        let mut issues = Vec::new();
        let density = ctx.structure.directed_density_pct;

        if density > self.config.max_acceptable {
            issues.push(Issue {
                severity: Severity::Error,
                message: format!(
                    "Dependency density ({:.1}%) exceeds maximum acceptable ({:.1}%)",
                    density, self.config.max_acceptable
                ),
                metric: Some("directed_density_pct".to_string()),
                actual: Some(density),
                threshold: Some(self.config.max_acceptable),
                location: None,
            });
        } else if density > self.config.ideal_max {
            issues.push(Issue {
                severity: Severity::Warning,
                message: format!(
                    "Dependency density ({:.1}%) exceeds ideal maximum ({:.1}%)",
                    density, self.config.ideal_max
                ),
                metric: Some("directed_density_pct".to_string()),
                actual: Some(density),
                threshold: Some(self.config.ideal_max),
                location: None,
            });
        } else if density < self.config.ideal_min && ctx.structure.modules_count > 5 {
            // 只有模块数足够多时才警告密度过低
            issues.push(Issue {
                severity: Severity::Info,
                message: format!(
                    "Dependency density ({:.1}%) is below ideal minimum ({:.1}%)",
                    density, self.config.ideal_min
                ),
                metric: Some("directed_density_pct".to_string()),
                actual: Some(density),
                threshold: Some(self.config.ideal_min),
                location: None,
            });
        }

        issues
    }

    /// 生成建议
    fn generate_suggestions(&self, ctx: &ScoringContext) -> Vec<String> {
        let mut suggestions = Vec::new();
        let density = ctx.structure.directed_density_pct;

        if density > self.config.max_acceptable {
            suggestions.push(
                "Consider refactoring high-coupling modules into smaller, more focused units"
                    .to_string(),
            );
            suggestions.push(
                "Introduce abstraction layers (interfaces/traits) to reduce direct dependencies"
                    .to_string(),
            );
            suggestions.push(
                "Review module boundaries and consider splitting overly connected modules"
                    .to_string(),
            );
        } else if density > self.config.ideal_max {
            suggestions
                .push("Monitor dependency growth and establish dependency budgets".to_string());
            suggestions.push("Consider applying the Dependency Inversion Principle".to_string());
        }

        if density < self.config.ideal_min && ctx.structure.modules_count > 5 {
            suggestions
                .push("Very low dependency density may indicate over-modularization".to_string());
            suggestions.push(
                "Consider consolidating related modules to reduce maintenance overhead".to_string(),
            );
        }

        if suggestions.is_empty() {
            suggestions
                .push("Dependency density is within ideal range. Good architecture!".to_string());
        }

        suggestions
    }
}

impl ScoringDimension for DensityDimension {
    fn name(&self) -> &str {
        "density"
    }

    fn display_name(&self) -> &str {
        "Dependency Density"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        let density = ctx.structure.directed_density_pct;

        // 评分逻辑：
        // - 在理想范围内: 100 分
        // - 低于理想最小值: 线性递减，最低 60 分
        // - 高于理想最大值: 线性递减，到 max_acceptable 时为 0 分
        let score = if density >= self.config.ideal_min && density <= self.config.ideal_max {
            100.0
        } else if density < self.config.ideal_min {
            // 密度过低：60-100 分
            let ratio = density / self.config.ideal_min;
            60.0 + ratio * 40.0
        } else if density < self.config.max_acceptable {
            // 密度偏高：0-100 分
            let range = self.config.max_acceptable - self.config.ideal_max;
            let excess = density - self.config.ideal_max;
            100.0 - (excess / range) * 100.0
        } else {
            0.0
        };

        // 通过条件：不超过最大可接受密度
        let passed = density <= self.config.max_acceptable;

        DimensionResult {
            score,
            passed,
            metrics: vec![
                Metric {
                    name: "directed_density_pct".to_string(),
                    display_name: Some("Dependency Density".to_string()),
                    value: density,
                    threshold: Some(self.config.max_acceptable),
                    unit: "%".to_string(),
                    passed: Some(passed),
                },
                Metric {
                    name: "modules_count".to_string(),
                    display_name: Some("Modules Count".to_string()),
                    value: ctx.structure.modules_count as f64,
                    threshold: None,
                    unit: "count".to_string(),
                    passed: None,
                },
                Metric {
                    name: "directed_edges_actual".to_string(),
                    display_name: Some("Actual Dependencies".to_string()),
                    value: ctx.structure.directed_edges_actual as f64,
                    threshold: None,
                    unit: "edges".to_string(),
                    passed: None,
                },
                Metric {
                    name: "directed_edges_possible".to_string(),
                    display_name: Some("Possible Dependencies".to_string()),
                    value: ctx.structure.directed_edges_possible as f64,
                    threshold: None,
                    unit: "edges".to_string(),
                    passed: None,
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

    fn create_context(density: f64, modules: i64) -> ScoringContext {
        ScoringContext {
            structure: crate::arch::score::context::StructureData {
                directed_density_pct: density,
                modules_count: modules,
                directed_edges_possible: modules * (modules - 1),
                directed_edges_actual: ((modules * (modules - 1)) as f64 * density / 100.0) as i64,
                ..Default::default()
            },
            ..Default::default()
        }
    }

    #[test]
    fn test_ideal_density() {
        let ctx = create_context(25.0, 10);
        let dim = DensityDimension::new(0.25, true, DensityDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert_eq!(result.score, 100.0);
        assert!(result.passed);
    }

    #[test]
    fn test_high_density() {
        let ctx = create_context(80.0, 10);
        let dim = DensityDimension::new(0.25, true, DensityDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert_eq!(result.score, 0.0);
        assert!(!result.passed);
    }

    #[test]
    fn test_low_density() {
        let ctx = create_context(5.0, 10);
        let dim = DensityDimension::new(0.25, true, DensityDimensionConfig::default());
        let result = dim.calculate(&ctx);

        assert!(result.score < 100.0);
        assert!(result.score >= 60.0);
        assert!(result.passed);
    }

    #[test]
    fn test_boundary_density() {
        let config = DensityDimensionConfig::default();
        let dim = DensityDimension::new(0.25, true, config.clone());

        // 正好在边界上
        let ctx = create_context(config.max_acceptable, 10);
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 0.0);
        assert!(result.passed); // 边界值算通过
    }
}
