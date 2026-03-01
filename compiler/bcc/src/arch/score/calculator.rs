//! 评分计算引擎

use super::config::ScoringConfig;
use super::context::ScoringContext;
use super::dimensions::{create_dimensions, ScoringDimension};
use super::models::*;

/// 评分模式
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ScoringMode {
    /// 严格模式：任何一票否决项失败则总分失败
    Strict,
    /// 宽松模式：允许部分一票否决项失败，但会大幅降低总分
    Lenient,
    /// 仅警告模式：只计算分数，不失败
    Warning,
}

impl ScoringMode {
    /// 从字符串解析
    pub fn from_str(s: &str) -> Option<Self> {
        match s.to_lowercase().as_str() {
            "strict" => Some(Self::Strict),
            "lenient" => Some(Self::Lenient),
            "warning" => Some(Self::Warning),
            _ => None,
        }
    }

    /// 获取显示名称
    pub fn display_name(&self) -> &'static str {
        match self {
            ScoringMode::Strict => "strict",
            ScoringMode::Lenient => "lenient",
            ScoringMode::Warning => "warning",
        }
    }
}

/// 评分计算器
pub struct ScoreCalculator {
    config: ScoringConfig,
    mode: ScoringMode,
    dimensions: Vec<Box<dyn ScoringDimension>>,
}

impl ScoreCalculator {
    /// 创建新的评分计算器
    pub fn new(config: ScoringConfig, mode: ScoringMode) -> Self {
        let dimensions = create_dimensions(&config);
        Self {
            config,
            mode,
            dimensions,
        }
    }

    /// 动态追加维度
    pub fn add_dimension(&mut self, dim: Box<dyn ScoringDimension>) {
        self.dimensions.push(dim);
    }

    /// 计算架构评分
    pub fn calculate(&self, ctx: &ScoringContext) -> ArchitectureScore {
        let mut dimension_scores = Vec::new();
        let mut total_weighted_score = 0.0;
        let mut total_weight = 0.0;
        let mut blocking_failures = 0usize;

        // 计算各维度得分
        for dim in &self.dimensions {
            let result = dim.calculate(ctx);
            let weight = dim.weight();

            total_weighted_score += result.score * weight;
            total_weight += weight;

            if dim.is_blocking() && !result.passed {
                blocking_failures += 1;
            }

            let contribution = result.score * weight;

            dimension_scores.push(DimensionScore {
                name: dim.name().to_string(),
                display_name: dim.display_name().to_string(),
                weight,
                score: result.score,
                contribution,
                passed: result.passed,
                is_blocking: dim.is_blocking(),
                metrics: result.metrics,
                issues: result.issues,
                suggestions: result.suggestions,
            });
        }

        // 计算原始平均分
        let raw_score = if total_weight > 0.0 {
            total_weighted_score / total_weight
        } else {
            0.0
        };

        // 根据模式调整最终分数
        let final_score = self.apply_mode_adjustments(raw_score, blocking_failures);

        // 判断是否通过
        let passed = self.determine_pass_status(final_score, blocking_failures);

        // 汇总问题统计
        let summary = self.calculate_summary(&dimension_scores, blocking_failures);

        // 生成建议
        let recommendations = self.generate_recommendations(&dimension_scores);

        ArchitectureScore {
            total: final_score,
            grade: Grade::from_score(final_score),
            passed,
            mode: self.mode.display_name().to_string(),
            dimensions: dimension_scores,
            summary,
            recommendations,
        }
    }

    /// 应用模式调整
    fn apply_mode_adjustments(&self, raw_score: f64, blocking_failures: usize) -> f64 {
        match self.mode {
            ScoringMode::Strict => {
                if blocking_failures > 0 {
                    0.0 // 一票否决
                } else {
                    raw_score
                }
            }
            ScoringMode::Lenient => {
                // 每个 blocking 失败扣减 20 分
                let penalty = blocking_failures as f64 * 20.0;
                (raw_score - penalty).max(0.0)
            }
            ScoringMode::Warning => raw_score,
        }
    }

    /// 确定通过状态
    fn determine_pass_status(&self, score: f64, blocking_failures: usize) -> bool {
        match self.mode {
            ScoringMode::Strict => blocking_failures == 0 && score >= self.config.threshold,
            ScoringMode::Lenient => score >= self.config.threshold,
            ScoringMode::Warning => true, // 警告模式总是通过
        }
    }

    /// 计算汇总统计
    fn calculate_summary(
        &self,
        dimensions: &[DimensionScore],
        blocking_failures: usize,
    ) -> ScoreSummary {
        let mut total_issues = 0usize;
        let mut critical_issues = 0usize;
        let mut error_issues = 0usize;
        let mut warning_issues = 0usize;
        let mut info_issues = 0usize;

        for dim in dimensions {
            total_issues += dim.issues.len();
            for issue in &dim.issues {
                match issue.severity {
                    Severity::Critical => critical_issues += 1,
                    Severity::Error => error_issues += 1,
                    Severity::Warning => warning_issues += 1,
                    Severity::Info => info_issues += 1,
                }
            }
        }

        ScoreSummary {
            blocking_failures,
            total_issues,
            critical_issues,
            error_issues,
            warning_issues,
            info_issues,
        }
    }

    /// 生成建议
    fn generate_recommendations(&self, dimensions: &[DimensionScore]) -> Vec<Recommendation> {
        let mut recommendations = Vec::new();

        // 按优先级排序维度问题
        let mut dim_with_issues: Vec<_> =
            dimensions.iter().filter(|d| !d.issues.is_empty()).collect();
        dim_with_issues.sort_by(|a, b| {
            // 优先处理 blocking 且分数低的维度
            let a_priority = if a.is_blocking { 1000.0 } else { 0.0 } - a.score;
            let b_priority = if b.is_blocking { 1000.0 } else { 0.0 } - b.score;
            b_priority.partial_cmp(&a_priority).unwrap()
        });

        // 为每个有问题的维度生成建议
        for dim in dim_with_issues.iter().take(5) {
            // 最多 5 条建议
            let priority = if dim.is_blocking && dim.score < 60.0 {
                Priority::High
            } else if dim.score < 60.0 {
                Priority::Medium
            } else {
                Priority::Low
            };

            // 收集受影响的模块
            let affected_modules: Vec<String> = dim
                .issues
                .iter()
                .filter_map(|i| i.location.clone())
                .take(5)
                .collect();

            if let Some(first_suggestion) = dim.suggestions.first() {
                recommendations.push(Recommendation {
                    priority,
                    category: dim.name.clone(),
                    message: format!("[{}] {}", dim.display_name, first_suggestion),
                    affected_modules,
                });
            }
        }

        recommendations
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::arch::score::config::ScoringConfig;

    fn create_test_context() -> ScoringContext {
        ScoringContext {
            scenario: crate::arch::score::context::ScenarioData {
                matched_edges_count: 80,
                unexpected_edges_count: 10,
                forbidden_edges_count: 5,
                ..Default::default()
            },
            structure: crate::arch::score::context::StructureData {
                modules_count: 10,
                directed_density_pct: 50.0,
                bidirectional_pair_count: 2,
                ..Default::default()
            },
            ..Default::default()
        }
    }

    #[test]
    fn test_calculator_creation() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Strict);

        assert!(!calculator.dimensions.is_empty());
    }

    #[test]
    fn test_strict_mode_blocking() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Strict);

        // 模拟一个有 blocking 失败的情况
        let score = calculator.apply_mode_adjustments(80.0, 1);
        assert_eq!(score, 0.0);

        // 没有 blocking 失败
        let score = calculator.apply_mode_adjustments(80.0, 0);
        assert_eq!(score, 80.0);
    }

    #[test]
    fn test_lenient_mode_penalty() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Lenient);

        // 1 个 blocking 失败扣 20 分
        let score = calculator.apply_mode_adjustments(80.0, 1);
        assert_eq!(score, 60.0);

        // 2 个 blocking 失败扣 40 分
        let score = calculator.apply_mode_adjustments(80.0, 2);
        assert_eq!(score, 40.0);

        // 不会低于 0
        let score = calculator.apply_mode_adjustments(50.0, 3);
        assert_eq!(score, 0.0);
    }

    #[test]
    fn test_warning_mode_no_penalty() {
        let config = ScoringConfig::default();
        let calculator = ScoreCalculator::new(config, ScoringMode::Warning);

        // 警告模式不受 blocking 影响
        let score = calculator.apply_mode_adjustments(80.0, 5);
        assert_eq!(score, 80.0);
    }

    #[test]
    fn test_pass_status() {
        let config = ScoringConfig::default();

        // Strict 模式：需要没有 blocking 失败且分数达标
        let calc_strict = ScoreCalculator::new(config.clone(), ScoringMode::Strict);
        assert!(!calc_strict.determine_pass_status(80.0, 1)); // 有 blocking 失败
        assert!(calc_strict.determine_pass_status(80.0, 0)); // 无 blocking 失败
        assert!(!calc_strict.determine_pass_status(50.0, 0)); // 分数不达标

        // Lenient 模式：只看分数
        let calc_lenient = ScoreCalculator::new(config.clone(), ScoringMode::Lenient);
        assert!(calc_lenient.determine_pass_status(80.0, 1)); // 有 blocking 但分数达标
        assert!(!calc_lenient.determine_pass_status(50.0, 0)); // 分数不达标

        // Warning 模式：总是通过
        let calc_warning = ScoreCalculator::new(config, ScoringMode::Warning);
        assert!(calc_warning.determine_pass_status(0.0, 10));
    }

    #[test]
    fn test_grade_calculation() {
        assert!(matches!(Grade::from_score(95.0), Grade::A));
        assert!(matches!(Grade::from_score(85.0), Grade::B));
        assert!(matches!(Grade::from_score(75.0), Grade::C));
        assert!(matches!(Grade::from_score(65.0), Grade::D));
        assert!(matches!(Grade::from_score(55.0), Grade::F));
    }
}
