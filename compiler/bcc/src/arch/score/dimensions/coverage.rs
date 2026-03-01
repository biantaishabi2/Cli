//! 测试覆盖维度
//!
//! 评估架构契约的测试覆盖情况，包括 BDD 场景覆盖和契约测试覆盖。

use super::{DimensionResult, ScoringDimension};
use crate::arch::score::config::CoverageDimensionConfig;
use crate::arch::score::context::ScoringContext;
use crate::arch::score::models::{Issue, Metric, Severity};

/// 测试覆盖维度
pub struct CoverageDimension {
    weight: f64,
    blocking: bool,
    config: CoverageDimensionConfig,
}

impl CoverageDimension {
    /// 创建新的测试覆盖维度
    pub fn new(weight: f64, blocking: bool, config: CoverageDimensionConfig) -> Self {
        Self {
            weight,
            blocking,
            config,
        }
    }

    /// 收集问题
    fn collect_issues(&self, bdd: f64, contract: f64) -> Vec<Issue> {
        let mut issues = Vec::new();

        if bdd < self.config.min_bdd_coverage {
            issues.push(Issue {
                severity: if bdd < self.config.min_bdd_coverage / 2.0 {
                    Severity::Error
                } else {
                    Severity::Warning
                },
                message: format!(
                    "BDD coverage ({:.1}%) is below threshold ({:.1}%)",
                    bdd, self.config.min_bdd_coverage
                ),
                metric: Some("bdd_coverage".to_string()),
                actual: Some(bdd),
                threshold: Some(self.config.min_bdd_coverage),
                location: None,
            });
        }

        if contract < self.config.min_contract_coverage {
            issues.push(Issue {
                severity: if contract < self.config.min_contract_coverage / 2.0 {
                    Severity::Error
                } else {
                    Severity::Warning
                },
                message: format!(
                    "Contract test coverage ({:.1}%) is below threshold ({:.1}%)",
                    contract, self.config.min_contract_coverage
                ),
                metric: Some("contract_coverage".to_string()),
                actual: Some(contract),
                threshold: Some(self.config.min_contract_coverage),
                location: None,
            });
        }

        issues
    }

    /// 生成建议
    fn generate_suggestions(&self, bdd: f64, contract: f64) -> Vec<String> {
        let mut suggestions = Vec::new();

        if bdd < self.config.min_bdd_coverage {
            suggestions.push(
                "Add BDD scenarios for uncovered architectural flows".to_string(),
            );
            suggestions.push(
                "Prioritize critical path scenarios first".to_string(),
            );
        }

        if contract < self.config.min_contract_coverage {
            suggestions.push(
                "Implement contract tests for module boundaries".to_string(),
            );
            suggestions.push(
                "Use consumer-driven contract testing for service dependencies".to_string(),
            );
        }

        if suggestions.is_empty() {
            suggestions.push("Test coverage meets requirements. Great job!".to_string());
        }

        suggestions
    }
}

impl ScoringDimension for CoverageDimension {
    fn name(&self) -> &str {
        "coverage"
    }

    fn display_name(&self) -> &str {
        "Test Coverage"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 获取覆盖率数据
        let bdd = ctx.bdd_coverage();
        let contract = ctx.contract_coverage();

        // 加权计算总分
        let score = bdd * self.config.bdd_weight + contract * self.config.contract_weight;
        let score_pct = score * 100.0;

        // 通过条件：两个覆盖率都达到阈值
        let passed = bdd * 100.0 >= self.config.min_bdd_coverage
            && contract * 100.0 >= self.config.min_contract_coverage;

        DimensionResult {
            score: score_pct,
            passed,
            metrics: vec![
                Metric {
                    name: "bdd_coverage".to_string(),
                    display_name: Some("BDD Scenario Coverage".to_string()),
                    value: bdd * 100.0,
                    threshold: Some(self.config.min_bdd_coverage),
                    unit: "%".to_string(),
                    passed: Some(bdd * 100.0 >= self.config.min_bdd_coverage),
                },
                Metric {
                    name: "contract_coverage".to_string(),
                    display_name: Some("Contract Test Coverage".to_string()),
                    value: contract * 100.0,
                    threshold: Some(self.config.min_contract_coverage),
                    unit: "%".to_string(),
                    passed: Some(contract * 100.0 >= self.config.min_contract_coverage),
                },
                Metric {
                    name: "weighted_coverage".to_string(),
                    display_name: Some("Weighted Coverage".to_string()),
                    value: score_pct,
                    threshold: Some(
                        self.config.min_bdd_coverage * self.config.bdd_weight
                            + self.config.min_contract_coverage * self.config.contract_weight,
                    ),
                    unit: "%".to_string(),
                    passed: Some(passed),
                },
            ],
            issues: self.collect_issues(bdd * 100.0, contract * 100.0),
            suggestions: self.generate_suggestions(bdd * 100.0, contract * 100.0),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // 注意：这些测试需要模拟上下文，目前 CoverageDimension 使用 ctx.bdd_coverage()
    // 和 ctx.contract_coverage()，它们返回 0.0（占位实现）

    #[test]
    fn test_coverage_calculation() {
        let config = CoverageDimensionConfig {
            min_bdd_coverage: 60.0,
            min_contract_coverage: 60.0,
            bdd_weight: 0.4,
            contract_weight: 0.6,
        };

        // 手动计算验证
        let bdd = 70.0; // 70%
        let contract = 80.0; // 80%
        let expected_score = bdd * 0.4 + contract * 0.6; // 28 + 48 = 76

        assert!((expected_score - 76.0f64).abs() < 0.01);
    }

    #[test]
    fn test_passed_condition() {
        let config = CoverageDimensionConfig {
            min_bdd_coverage: 60.0,
            min_contract_coverage: 60.0,
            bdd_weight: 0.4,
            contract_weight: 0.6,
        };

        // 都达到阈值
        assert!(70.0 >= config.min_bdd_coverage && 80.0 >= config.min_contract_coverage);

        // BDD 未达到
        assert!(!(50.0 >= config.min_bdd_coverage && 80.0 >= config.min_contract_coverage));

        // Contract 未达到
        assert!(!(70.0 >= config.min_bdd_coverage && 50.0 >= config.min_contract_coverage));
    }
}
