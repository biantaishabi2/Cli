//! 评分配置

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// 评分配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScoringConfig {
    /// 配置版本
    #[serde(default = "default_version")]
    pub version: String,
    /// 配置类型
    #[serde(default = "default_kind")]
    pub kind: String,
    /// 评分模式
    #[serde(default)]
    pub mode: ScoringModeConfig,
    /// 失败阈值
    #[serde(default = "default_threshold")]
    pub threshold: f64,
    /// 维度配置
    #[serde(default)]
    pub dimensions: DimensionsConfig,
    /// 自定义规则
    #[serde(default)]
    pub custom_rules: Vec<CustomRule>,
    /// 输出配置
    #[serde(default)]
    pub output: OutputConfig,
    /// 模块标签
    #[serde(default)]
    pub module_tags: HashMap<String, Vec<String>>,
}

fn default_version() -> String {
    "1.0".to_string()
}

fn default_kind() -> String {
    "architecture_scoring_config".to_string()
}

fn default_threshold() -> f64 {
    60.0
}

/// 评分模式配置
#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ScoringModeConfig {
    /// 严格模式
    #[default]
    Strict,
    /// 宽松模式
    Lenient,
    /// 仅警告模式
    Warning,
}

/// 维度配置集合
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DimensionsConfig {
    /// 合规性维度
    #[serde(default)]
    pub compliance: DimensionConfig<ComplianceDimensionConfig>,
    /// 依赖密度维度
    #[serde(default)]
    pub density: DimensionConfig<DensityDimensionConfig>,
    /// 分层清晰维度
    #[serde(default)]
    pub layering: DimensionConfig<LayeringDimensionConfig>,
    /// 无循环依赖维度
    #[serde(default)]
    pub acyclic: DimensionConfig<AcyclicDimensionConfig>,
    /// 测试覆盖维度
    #[serde(default)]
    pub coverage: DimensionConfig<CoverageDimensionConfig>,
    /// 代码质量维度（需配合 --smell-report 使用）
    #[serde(default)]
    pub code_quality: DimensionConfig<CodeQualityDimensionConfig>,
}

impl Default for DimensionsConfig {
    fn default() -> Self {
        Self {
            compliance: DimensionConfig::enabled(0.30, true, ComplianceDimensionConfig::default()),
            density: DimensionConfig::enabled(0.25, true, DensityDimensionConfig::default()),
            layering: DimensionConfig::enabled(0.25, true, LayeringDimensionConfig::default()),
            acyclic: DimensionConfig::enabled(0.10, true, AcyclicDimensionConfig::default()),
            coverage: DimensionConfig::enabled(0.10, false, CoverageDimensionConfig::default()),
            code_quality: DimensionConfig {
                enabled: false,
                weight: 0.10,
                blocking: true,
                config: CodeQualityDimensionConfig::default(),
            },
        }
    }
}

/// 单个维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DimensionConfig<T> {
    /// 是否启用
    #[serde(default = "default_true")]
    pub enabled: bool,
    /// 权重 (0.0 - 1.0)
    #[serde(default)]
    pub weight: f64,
    /// 是否为一票否决项
    #[serde(default)]
    pub blocking: bool,
    /// 维度特定配置
    #[serde(default)]
    pub config: T,
}

fn default_true() -> bool {
    true
}

impl<T: Default> DimensionConfig<T> {
    /// 创建启用的配置
    pub fn enabled(weight: f64, blocking: bool, config: T) -> Self {
        Self {
            enabled: true,
            weight,
            blocking,
            config,
        }
    }
}

impl<T: Default> Default for DimensionConfig<T> {
    fn default() -> Self {
        Self {
            enabled: true,
            weight: 0.2,
            blocking: false,
            config: T::default(),
        }
    }
}

/// 合规性维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComplianceDimensionConfig {
    /// 意外边的惩罚系数
    #[serde(default = "default_unexpected_penalty")]
    pub unexpected_penalty: f64,
    /// 禁止边的惩罚系数
    #[serde(default = "default_forbidden_penalty")]
    pub forbidden_penalty: f64,
}

fn default_unexpected_penalty() -> f64 {
    1.5
}

fn default_forbidden_penalty() -> f64 {
    2.0
}

impl Default for ComplianceDimensionConfig {
    fn default() -> Self {
        Self {
            unexpected_penalty: default_unexpected_penalty(),
            forbidden_penalty: default_forbidden_penalty(),
        }
    }
}

/// 依赖密度维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DensityDimensionConfig {
    /// 理想密度最小值 (%)
    #[serde(default = "default_ideal_min")]
    pub ideal_min: f64,
    /// 理想密度最大值 (%)
    #[serde(default = "default_ideal_max")]
    pub ideal_max: f64,
    /// 最大可接受密度 (%)
    #[serde(default = "default_max_acceptable")]
    pub max_acceptable: f64,
}

fn default_ideal_min() -> f64 {
    10.0
}

fn default_ideal_max() -> f64 {
    40.0
}

fn default_max_acceptable() -> f64 {
    75.0
}

impl Default for DensityDimensionConfig {
    fn default() -> Self {
        Self {
            ideal_min: default_ideal_min(),
            ideal_max: default_ideal_max(),
            max_acceptable: default_max_acceptable(),
        }
    }
}

/// 分层定义
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayerDefinition {
    /// 分层名称
    pub name: String,
    /// 优先级（越小越上层）
    pub precedence: i32,
}

/// 分层清晰维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayeringDimensionConfig {
    /// 分层定义
    #[serde(default = "default_layers")]
    pub layers: Vec<LayerDefinition>,
    /// 允许的跨层依赖 (上层 -> 下层)
    #[serde(default = "default_allowed_transitions")]
    pub allowed_transitions: Vec<(String, String)>,
    /// 禁止的跨层依赖
    #[serde(default)]
    pub forbidden_transitions: Vec<(String, String)>,
}

fn default_layers() -> Vec<LayerDefinition> {
    vec![
        LayerDefinition { name: "api".to_string(), precedence: 1 },
        LayerDefinition { name: "service".to_string(), precedence: 2 },
        LayerDefinition { name: "dao".to_string(), precedence: 3 },
        LayerDefinition { name: "core".to_string(), precedence: 2 },
        LayerDefinition { name: "support".to_string(), precedence: 2 },
    ]
}

fn default_allowed_transitions() -> Vec<(String, String)> {
    vec![
        ("api".to_string(), "service".to_string()),
        ("api".to_string(), "core".to_string()),
        ("service".to_string(), "dao".to_string()),
        ("service".to_string(), "core".to_string()),
        ("core".to_string(), "dao".to_string()),
        ("support".to_string(), "core".to_string()),
    ]
}

impl Default for LayeringDimensionConfig {
    fn default() -> Self {
        Self {
            layers: default_layers(),
            allowed_transitions: default_allowed_transitions(),
            forbidden_transitions: Vec::new(),
        }
    }
}

/// 无循环依赖维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AcyclicDimensionConfig {
    /// 最大允许的双向依赖对数
    #[serde(default)]
    pub max_pairs: usize,
    /// 每对双向依赖的扣减分数
    #[serde(default = "default_deduction_per_pair")]
    pub deduction_per_pair: f64,
}

fn default_deduction_per_pair() -> f64 {
    10.0
}

impl Default for AcyclicDimensionConfig {
    fn default() -> Self {
        Self {
            max_pairs: 0,
            deduction_per_pair: default_deduction_per_pair(),
        }
    }
}

/// 测试覆盖维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CoverageDimensionConfig {
    /// BDD 场景覆盖率阈值 (%)
    #[serde(default = "default_min_coverage")]
    pub min_bdd_coverage: f64,
    /// 架构契约测试覆盖率阈值 (%)
    #[serde(default = "default_min_coverage")]
    pub min_contract_coverage: f64,
    /// BDD 覆盖率权重
    #[serde(default = "default_bdd_weight")]
    pub bdd_weight: f64,
    /// 契约测试覆盖率权重
    #[serde(default = "default_contract_weight")]
    pub contract_weight: f64,
}

fn default_min_coverage() -> f64 {
    60.0
}

fn default_bdd_weight() -> f64 {
    0.4
}

fn default_contract_weight() -> f64 {
    0.6
}

impl Default for CoverageDimensionConfig {
    fn default() -> Self {
        Self {
            min_bdd_coverage: default_min_coverage(),
            min_contract_coverage: default_min_coverage(),
            bdd_weight: default_bdd_weight(),
            contract_weight: default_contract_weight(),
        }
    }
}

/// 代码质量维度配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CodeQualityDimensionConfig {
    /// 通过阈值
    #[serde(default = "default_code_quality_threshold")]
    pub threshold: f64,
}

fn default_code_quality_threshold() -> f64 {
    60.0
}

impl Default for CodeQualityDimensionConfig {
    fn default() -> Self {
        Self {
            threshold: default_code_quality_threshold(),
        }
    }
}

/// 自定义规则
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CustomRule {
    /// 规则名称
    pub name: String,
    /// 规则描述
    pub description: String,
    /// 严重级别
    pub severity: String,
    /// 条件表达式（简化版）
    pub condition: String,
    /// 消息模板
    #[serde(default)]
    pub message: String,
}

/// 输出配置
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct OutputConfig {
    /// 文本输出配置
    #[serde(default)]
    pub text: TextOutputConfig,
    /// JSON 输出配置
    #[serde(default)]
    pub json: JsonOutputConfig,
    /// Markdown 输出配置
    #[serde(default)]
    pub markdown: MarkdownOutputConfig,
}

/// 文本输出配置
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TextOutputConfig {
    #[serde(default = "default_true")]
    pub show_details: bool,
    #[serde(default = "default_true")]
    pub show_suggestions: bool,
    #[serde(default = "default_max_issues")]
    pub max_issues: usize,
    #[serde(default = "default_true")]
    pub show_progress: bool,
}

fn default_max_issues() -> usize {
    20
}

impl Default for TextOutputConfig {
    fn default() -> Self {
        Self {
            show_details: true,
            show_suggestions: true,
            max_issues: default_max_issues(),
            show_progress: true,
        }
    }
}

/// JSON 输出配置
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct JsonOutputConfig {
    #[serde(default = "default_true")]
    pub pretty: bool,
    #[serde(default = "default_true")]
    pub include_metrics: bool,
    #[serde(default = "default_true")]
    pub include_suggestions: bool,
}

/// Markdown 输出配置
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MarkdownOutputConfig {
    #[serde(default)]
    pub template: String,
    #[serde(default)]
    pub include_charts: bool,
    #[serde(default)]
    pub include_comparison: bool,
}

impl ScoringConfig {
    /// 从文件加载配置
    pub fn from_file(path: &str) -> Result<Self, String> {
        let content =
            std::fs::read_to_string(path).map_err(|e| format!("read config file failed: {}", e))?;
        serde_yaml::from_str(&content).map_err(|e| format!("parse config failed: {}", e))
    }

    /// 验证配置
    pub fn validate(&self) -> Result<(), String> {
        // 检查权重总和
        let mut total_weight = 0.0;
        let mut enabled_count = 0;

        if self.dimensions.compliance.enabled {
            total_weight += self.dimensions.compliance.weight;
            enabled_count += 1;
            if self.dimensions.compliance.weight <= 0.0 {
                return Err("weight for dimension 'compliance' must be positive".into());
            }
        }
        if self.dimensions.density.enabled {
            total_weight += self.dimensions.density.weight;
            enabled_count += 1;
            if self.dimensions.density.weight <= 0.0 {
                return Err("weight for dimension 'density' must be positive".into());
            }
        }
        if self.dimensions.layering.enabled {
            total_weight += self.dimensions.layering.weight;
            enabled_count += 1;
            if self.dimensions.layering.weight <= 0.0 {
                return Err("weight for dimension 'layering' must be positive".into());
            }
        }
        if self.dimensions.acyclic.enabled {
            total_weight += self.dimensions.acyclic.weight;
            enabled_count += 1;
            if self.dimensions.acyclic.weight <= 0.0 {
                return Err("weight for dimension 'acyclic' must be positive".into());
            }
        }
        if self.dimensions.coverage.enabled {
            total_weight += self.dimensions.coverage.weight;
            enabled_count += 1;
            if self.dimensions.coverage.weight <= 0.0 {
                return Err("weight for dimension 'coverage' must be positive".into());
            }
        }
        if self.dimensions.code_quality.enabled {
            total_weight += self.dimensions.code_quality.weight;
            enabled_count += 1;
            if self.dimensions.code_quality.weight <= 0.0 {
                return Err("weight for dimension 'code_quality' must be positive".into());
            }
        }

        if enabled_count > 0 && (total_weight - 1.0).abs() > 0.001 {
            return Err(format!(
                "total weight of enabled dimensions must be 1.0, got {:.2}",
                total_weight
            ));
        }

        Ok(())
    }

    /// 严格模式模板
    pub fn strict_template() -> Self {
        Self {
            version: "1.0".to_string(),
            kind: "architecture_scoring_config".to_string(),
            mode: ScoringModeConfig::Strict,
            threshold: 80.0,
            dimensions: DimensionsConfig {
                compliance: DimensionConfig::enabled(
                    0.30,
                    true,
                    ComplianceDimensionConfig::default(),
                ),
                density: DimensionConfig::enabled(
                    0.25,
                    true,
                    DensityDimensionConfig {
                        ideal_min: 10.0,
                        ideal_max: 30.0,
                        max_acceptable: 50.0,
                    },
                ),
                layering: DimensionConfig::enabled(
                    0.25,
                    true,
                    LayeringDimensionConfig::default(),
                ),
                acyclic: DimensionConfig::enabled(
                    0.10,
                    true,
                    AcyclicDimensionConfig {
                        max_pairs: 0,
                        deduction_per_pair: 20.0,
                    },
                ),
                coverage: DimensionConfig::enabled(
                    0.10,
                    false,
                    CoverageDimensionConfig {
                        min_bdd_coverage: 80.0,
                        min_contract_coverage: 80.0,
                        bdd_weight: 0.4,
                        contract_weight: 0.6,
                    },
                ),
                code_quality: DimensionConfig::enabled(
                    0.10,
                    true,
                    CodeQualityDimensionConfig { threshold: 60.0 },
                ),
            },
            custom_rules: Vec::new(),
            output: OutputConfig::default(),
            module_tags: HashMap::new(),
        }
    }

    /// 宽松模式模板
    pub fn lenient_template() -> Self {
        Self {
            version: "1.0".to_string(),
            kind: "architecture_scoring_config".to_string(),
            mode: ScoringModeConfig::Lenient,
            threshold: 60.0,
            dimensions: DimensionsConfig {
                compliance: DimensionConfig::enabled(
                    0.30,
                    false, // 非一票否决
                    ComplianceDimensionConfig::default(),
                ),
                density: DimensionConfig::enabled(
                    0.25,
                    false,
                    DensityDimensionConfig {
                        ideal_min: 10.0,
                        ideal_max: 50.0,
                        max_acceptable: 80.0,
                    },
                ),
                layering: DimensionConfig::enabled(0.25, false, LayeringDimensionConfig::default()),
                acyclic: DimensionConfig::enabled(
                    0.10,
                    false,
                    AcyclicDimensionConfig {
                        max_pairs: 5,
                        deduction_per_pair: 5.0,
                    },
                ),
                coverage: DimensionConfig::enabled(
                    0.10,
                    false,
                    CoverageDimensionConfig {
                        min_bdd_coverage: 40.0,
                        min_contract_coverage: 40.0,
                        bdd_weight: 0.4,
                        contract_weight: 0.6,
                    },
                ),
                code_quality: DimensionConfig::enabled(
                    0.10,
                    true,
                    CodeQualityDimensionConfig { threshold: 60.0 },
                ),
            },
            custom_rules: Vec::new(),
            output: OutputConfig::default(),
            module_tags: HashMap::new(),
        }
    }

    /// 最小配置模板
    pub fn minimal_template() -> Self {
        Self {
            version: "1.0".to_string(),
            kind: "architecture_scoring_config".to_string(),
            mode: ScoringModeConfig::Warning,
            threshold: 0.0,
            dimensions: DimensionsConfig {
                compliance: DimensionConfig::enabled(
                    1.0,
                    false,
                    ComplianceDimensionConfig::default(),
                ),
                density: DimensionConfig {
                    enabled: false,
                    ..Default::default()
                },
                layering: DimensionConfig {
                    enabled: false,
                    ..Default::default()
                },
                acyclic: DimensionConfig {
                    enabled: false,
                    ..Default::default()
                },
                coverage: DimensionConfig {
                    enabled: false,
                    ..Default::default()
                },
                code_quality: DimensionConfig {
                    enabled: false,
                    ..Default::default()
                },
            },
            custom_rules: Vec::new(),
            output: OutputConfig::default(),
            module_tags: HashMap::new(),
        }
    }
}

impl Default for ScoringConfig {
    fn default() -> Self {
        Self {
            version: default_version(),
            kind: default_kind(),
            mode: ScoringModeConfig::default(),
            threshold: default_threshold(),
            dimensions: DimensionsConfig::default(),
            custom_rules: Vec::new(),
            output: OutputConfig::default(),
            module_tags: HashMap::new(),
        }
    }
}
