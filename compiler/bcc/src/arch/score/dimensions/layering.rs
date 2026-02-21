//! 分层清晰维度
//!
//! 评估分层架构的调用合规性，检查是否存在违反分层规则的依赖。

use super::{DimensionResult, ScoringDimension};
use crate::arch::score::config::{LayerDefinition, LayeringDimensionConfig};
use crate::arch::score::context::ScoringContext;
use crate::arch::score::models::{Issue, Metric, Severity};
use std::collections::HashMap;

/// 预计算版本：接收已查找的 from/to precedence 值，避免重复构建 map
///
/// 规则优先级：同层允许 > 显式禁止 > 显式允许 > 默认 precedence 比较
pub fn check_layer_transition_with_precedence(
    from_layer: &str,
    to_layer: &str,
    from_precedence: Option<i32>,
    to_precedence: Option<i32>,
    forbidden_transitions: &[(String, String)],
    allowed_transitions: &[(String, String)],
) -> bool {
    // 同层允许
    if from_layer == to_layer {
        return true;
    }

    // 检查显式禁止的转换
    for (from, to) in forbidden_transitions {
        if from == from_layer && to == to_layer {
            return false;
        }
    }

    // 检查显式允许的转换
    for (from, to) in allowed_transitions {
        if from == from_layer && to == to_layer {
            return true;
        }
    }

    // 默认：上层可以依赖下层（precedence 越小越上层）
    match (from_precedence, to_precedence) {
        (Some(f), Some(t)) => f < t,
        _ => true, // 未知层默认允许
    }
}

/// 独立函数：检查 layer 间的依赖是否允许（内部构建 precedence map，适用于无预计算场景）
pub fn check_layer_transition(
    from_layer: &str,
    to_layer: &str,
    layers: &[LayerDefinition],
    forbidden_transitions: &[(String, String)],
    allowed_transitions: &[(String, String)],
) -> bool {
    let layer_precedence: HashMap<&str, i32> = layers
        .iter()
        .map(|l| (l.name.as_str(), l.precedence))
        .collect();

    check_layer_transition_with_precedence(
        from_layer,
        to_layer,
        layer_precedence.get(from_layer).copied(),
        layer_precedence.get(to_layer).copied(),
        forbidden_transitions,
        allowed_transitions,
    )
}

/// 分层清晰维度
pub struct LayeringDimension {
    weight: f64,
    blocking: bool,
    config: LayeringDimensionConfig,
    layer_precedence: HashMap<String, i32>,
}

impl LayeringDimension {
    /// 创建新的分层清晰维度
    pub fn new(weight: f64, blocking: bool, config: LayeringDimensionConfig) -> Self {
        let layer_precedence: HashMap<String, i32> = config
            .layers
            .iter()
            .map(|l| (l.name.clone(), l.precedence))
            .collect();

        Self {
            weight,
            blocking,
            config,
            layer_precedence,
        }
    }

    /// 检查依赖是否允许（使用预计算的 layer_precedence）
    fn is_allowed_transition(&self, from_layer: &str, to_layer: &str) -> bool {
        check_layer_transition_with_precedence(
            from_layer,
            to_layer,
            self.layer_precedence.get(from_layer).copied(),
            self.layer_precedence.get(to_layer).copied(),
            &self.config.forbidden_transitions,
            &self.config.allowed_transitions,
        )
    }

    /// 分析分层违规
    fn analyze_violations(&self, _ctx: &ScoringContext) -> Vec<LayerViolation> {
        // TODO: 从实际模块依赖数据中分析违规
        // 目前返回空列表，需要结合 module registry 和 actual relations 实现
        Vec::new()
    }

    /// 收集问题
    fn collect_issues(&self, violations: &[LayerViolation]) -> Vec<Issue> {
        let mut issues = Vec::new();

        for v in violations {
            issues.push(Issue {
                severity: Severity::Error,
                message: format!(
                    "Layer violation: {} ({}) -> {} ({})",
                    v.from_module, v.from_layer, v.to_module, v.to_layer
                ),
                metric: Some("layer_violation".to_string()),
                actual: None,
                threshold: Some(0.0),
                location: Some(format!("{} -> {}", v.from_module, v.to_module)),
            });
        }

        issues
    }

    /// 生成建议
    fn generate_suggestions(&self, violations: &[LayerViolation]) -> Vec<String> {
        let mut suggestions = Vec::new();

        if !violations.is_empty() {
            suggestions.push(
                "Review layer definitions and ensure they align with the architecture".to_string(),
            );
            suggestions.push(
                "Consider introducing intermediate layers or facades to resolve violations".to_string(),
            );
            suggestions.push(
                "Apply the Dependency Inversion Principle for cross-layer dependencies".to_string(),
            );
        } else {
            suggestions.push("Layer architecture is well maintained. Keep it up!".to_string());
        }

        suggestions
    }
}

/// 分层违规
#[derive(Debug, Clone)]
struct LayerViolation {
    from_module: String,
    to_module: String,
    from_layer: String,
    to_layer: String,
}

impl ScoringDimension for LayeringDimension {
    fn name(&self) -> &str {
        "layering"
    }

    fn display_name(&self) -> &str {
        "Layer Clarity"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult {
        // 分析分层违规
        let violations = self.analyze_violations(ctx);
        let violation_count = violations.len() as f64;

        // 获取总依赖数
        let total_deps = ctx.total_dependencies() as f64;

        // 评分计算
        let score = if total_deps > 0.0 {
            let valid = (total_deps - violation_count).max(0.0);
            (valid / total_deps * 100.0).min(100.0)
        } else {
            100.0
        };

        // 通过条件：没有违规
        let passed = violations.is_empty();

        // 构建指标
        let mut metrics = vec![
            Metric {
                name: "layer_violations".to_string(),
                display_name: Some("Layer Violations".to_string()),
                value: violation_count,
                threshold: Some(0.0),
                unit: "count".to_string(),
                passed: Some(passed),
            },
            Metric {
                name: "total_dependencies".to_string(),
                display_name: Some("Total Dependencies".to_string()),
                value: total_deps,
                threshold: None,
                unit: "count".to_string(),
                passed: None,
            },
        ];

        // 添加各层信息
        for layer in &self.config.layers {
            metrics.push(Metric {
                name: format!("layer_{}", layer.name),
                display_name: Some(format!("Layer: {}", layer.name)),
                value: layer.precedence as f64,
                threshold: None,
                unit: "precedence".to_string(),
                passed: None,
            });
        }

        DimensionResult {
            score,
            passed,
            metrics,
            issues: self.collect_issues(&violations),
            suggestions: self.generate_suggestions(&violations),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::arch::score::config::LayerDefinition;

    fn create_config() -> LayeringDimensionConfig {
        LayeringDimensionConfig {
            layers: vec![
                LayerDefinition { name: "api".to_string(), precedence: 1 },
                LayerDefinition { name: "service".to_string(), precedence: 2 },
                LayerDefinition { name: "dao".to_string(), precedence: 3 },
            ],
            allowed_transitions: vec![
                ("api".to_string(), "service".to_string()),
                ("service".to_string(), "dao".to_string()),
            ],
            forbidden_transitions: vec![],
        }
    }

    #[test]
    fn test_allowed_transitions() {
        let config = create_config();
        let dim = LayeringDimension::new(0.25, true, config);

        // 同层允许
        assert!(dim.is_allowed_transition("api", "api"));
        assert!(dim.is_allowed_transition("service", "service"));

        // 上层可以依赖下层
        assert!(dim.is_allowed_transition("api", "service"));
        assert!(dim.is_allowed_transition("service", "dao"));
        assert!(dim.is_allowed_transition("api", "dao"));

        // 下层不能依赖上层
        assert!(!dim.is_allowed_transition("service", "api"));
        assert!(!dim.is_allowed_transition("dao", "service"));
        assert!(!dim.is_allowed_transition("dao", "api"));
    }

    #[test]
    fn test_explicit_allowed() {
        let mut config = create_config();
        config.allowed_transitions.push(("dao".to_string(), "api".to_string()));
        let dim = LayeringDimension::new(0.25, true, config);

        // 显式允许的转换应该通过
        assert!(dim.is_allowed_transition("dao", "api"));
    }

    #[test]
    fn test_explicit_forbidden() {
        let mut config = create_config();
        config.forbidden_transitions.push(("api".to_string(), "dao".to_string()));
        let dim = LayeringDimension::new(0.25, true, config);

        // 显式禁止的转换应该失败，即使默认允许
        assert!(!dim.is_allowed_transition("api", "dao"));
    }
}
