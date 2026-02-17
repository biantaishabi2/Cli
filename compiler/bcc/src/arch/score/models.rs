//! 架构评分数据模型

use serde::{Deserialize, Serialize};

/// 架构健康度总分
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArchitectureScore {
    /// 总分 (0.0 - 100.0)
    pub total: f64,
    /// 等级
    pub grade: Grade,
    /// 是否通过
    pub passed: bool,
    /// 评分模式
    pub mode: String,
    /// 各维度得分
    pub dimensions: Vec<DimensionScore>,
    /// 汇总信息
    pub summary: ScoreSummary,
    /// 建议
    pub recommendations: Vec<Recommendation>,
}

/// 等级
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum Grade {
    A, // 90-100
    B, // 80-89
    C, // 70-79
    D, // 60-69
    F, // < 60
}

impl Grade {
    /// 从分数计算等级
    pub fn from_score(score: f64) -> Self {
        match score {
            s if s >= 90.0 => Grade::A,
            s if s >= 80.0 => Grade::B,
            s if s >= 70.0 => Grade::C,
            s if s >= 60.0 => Grade::D,
            _ => Grade::F,
        }
    }

    /// 获取等级的显示名称
    pub fn display_name(&self) -> &'static str {
        match self {
            Grade::A => "A (Excellent)",
            Grade::B => "B (Good)",
            Grade::C => "C (Fair)",
            Grade::D => "D (Poor)",
            Grade::F => "F (Fail)",
        }
    }

    /// 获取等级的 emoji 表示
    pub fn emoji(&self) -> &'static str {
        match self {
            Grade::A => "🌟",
            Grade::B => "✅",
            Grade::C => "⚠️",
            Grade::D => "❗",
            Grade::F => "❌",
        }
    }
}

/// 维度得分
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DimensionScore {
    /// 维度名称（英文标识）
    pub name: String,
    /// 显示名称
    pub display_name: String,
    /// 权重 (0.0 - 1.0)
    pub weight: f64,
    /// 原始得分 (0.0 - 100.0)
    pub score: f64,
    /// 对总分的贡献
    pub contribution: f64,
    /// 是否通过阈值
    pub passed: bool,
    /// 是否为一票否决项
    pub is_blocking: bool,
    /// 指标列表
    pub metrics: Vec<Metric>,
    /// 问题列表
    pub issues: Vec<Issue>,
    /// 建议
    pub suggestions: Vec<String>,
}

/// 指标
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Metric {
    /// 指标名称
    pub name: String,
    /// 显示名称
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display_name: Option<String>,
    /// 指标值
    pub value: f64,
    /// 阈值
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f64>,
    /// 单位
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub unit: String,
    /// 是否通过
    #[serde(skip_serializing_if = "Option::is_none")]
    pub passed: Option<bool>,
}

/// 问题严重级别
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Severity {
    Info,
    Warning,
    Error,
    Critical,
}

impl Severity {
    /// 获取 emoji 表示
    pub fn emoji(&self) -> &'static str {
        match self {
            Severity::Info => "ℹ️",
            Severity::Warning => "⚠️",
            Severity::Error => "❌",
            Severity::Critical => "🚨",
        }
    }

    /// 获取文本表示
    pub fn display_name(&self) -> &'static str {
        match self {
            Severity::Info => "Info",
            Severity::Warning => "Warning",
            Severity::Error => "Error",
            Severity::Critical => "Critical",
        }
    }
}

/// 问题
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Issue {
    /// 严重级别
    pub severity: Severity,
    /// 问题消息
    pub message: String,
    /// 相关指标
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metric: Option<String>,
    /// 实际值
    #[serde(skip_serializing_if = "Option::is_none")]
    pub actual: Option<f64>,
    /// 阈值
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f64>,
    /// 位置信息
    #[serde(skip_serializing_if = "Option::is_none")]
    pub location: Option<String>,
}

/// 评分汇总
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScoreSummary {
    /// 一票否决失败数
    pub blocking_failures: usize,
    /// 总问题数
    pub total_issues: usize,
    /// 严重问题数
    pub critical_issues: usize,
    /// 错误数
    pub error_issues: usize,
    /// 警告数
    pub warning_issues: usize,
    /// 信息数
    pub info_issues: usize,
}

/// 建议
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Recommendation {
    /// 优先级
    pub priority: Priority,
    /// 分类
    pub category: String,
    /// 建议内容
    pub message: String,
    /// 受影响的模块
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub affected_modules: Vec<String>,
}

/// 优先级
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Priority {
    High,
    Medium,
    Low,
}

impl Priority {
    pub fn display_name(&self) -> &'static str {
        match self {
            Priority::High => "High",
            Priority::Medium => "Medium",
            Priority::Low => "Low",
        }
    }
}

/// 维度计算结果（内部使用）
#[derive(Debug, Clone)]
pub struct DimensionResult {
    /// 原始得分 (0.0 - 100.0)
    pub score: f64,
    /// 是否通过阈值
    pub passed: bool,
    /// 详细指标
    pub metrics: Vec<Metric>,
    /// 问题列表
    pub issues: Vec<Issue>,
    /// 建议
    pub suggestions: Vec<String>,
}

impl DimensionResult {
    /// 创建一个新的结果
    pub fn new(score: f64) -> Self {
        Self {
            score: score.clamp(0.0, 100.0),
            passed: score >= 60.0,
            metrics: Vec::new(),
            issues: Vec::new(),
            suggestions: Vec::new(),
        }
    }

    /// 添加指标
    pub fn with_metric(mut self, metric: Metric) -> Self {
        self.metrics.push(metric);
        self
    }

    /// 添加问题
    pub fn with_issue(mut self, issue: Issue) -> Self {
        self.issues.push(issue);
        self
    }

    /// 添加建议
    pub fn with_suggestion(mut self, suggestion: impl Into<String>) -> Self {
        self.suggestions.push(suggestion.into());
        self
    }
}
