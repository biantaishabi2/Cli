//! 评分维度实现

pub mod acyclic;
pub mod code_quality;
pub mod compliance;
pub mod coverage;
pub mod density;
pub mod layering;
pub use layering::{check_layer_transition, check_layer_transition_with_precedence};

use super::config::ScoringConfig;
use super::context::ScoringContext;
use super::models::DimensionResult;

/// 评分维度 trait
pub trait ScoringDimension: Send + Sync {
    /// 维度名称
    fn name(&self) -> &str;

    /// 显示名称
    fn display_name(&self) -> &str;

    /// 维度权重 (0.0 - 1.0)
    fn weight(&self) -> f64;

    /// 是否为一票否决项
    fn is_blocking(&self) -> bool;

    /// 计算维度得分 (0.0 - 100.0)
    fn calculate(&self, ctx: &ScoringContext) -> DimensionResult;
}

/// 创建所有启用的维度
pub fn create_dimensions(config: &ScoringConfig) -> Vec<Box<dyn ScoringDimension>> {
    let mut dimensions: Vec<Box<dyn ScoringDimension>> = Vec::new();

    // 合规性维度
    if config.dimensions.compliance.enabled {
        dimensions.push(Box::new(compliance::ComplianceDimension::new(
            config.dimensions.compliance.weight,
            config.dimensions.compliance.blocking,
            config.dimensions.compliance.config.clone(),
        )));
    }

    // 依赖密度维度
    if config.dimensions.density.enabled {
        dimensions.push(Box::new(density::DensityDimension::new(
            config.dimensions.density.weight,
            config.dimensions.density.blocking,
            config.dimensions.density.config.clone(),
        )));
    }

    // 分层清晰维度
    if config.dimensions.layering.enabled {
        dimensions.push(Box::new(layering::LayeringDimension::new(
            config.dimensions.layering.weight,
            config.dimensions.layering.blocking,
            config.dimensions.layering.config.clone(),
        )));
    }

    // 无循环依赖维度
    if config.dimensions.acyclic.enabled {
        dimensions.push(Box::new(acyclic::AcyclicDimension::new(
            config.dimensions.acyclic.weight,
            config.dimensions.acyclic.blocking,
            config.dimensions.acyclic.config.clone(),
        )));
    }

    // 测试覆盖维度
    if config.dimensions.coverage.enabled {
        dimensions.push(Box::new(coverage::CoverageDimension::new(
            config.dimensions.coverage.weight,
            config.dimensions.coverage.blocking,
            config.dimensions.coverage.config.clone(),
        )));
    }

    dimensions
}
