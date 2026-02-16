//! 代码图谱核心类型定义

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// 仓库信息
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Repository {
    /// 唯一标识（如 github.com#HKUDS#nanobot）
    pub id: String,
    /// 仓库名称
    pub name: String,
    /// 根目录路径
    pub root_path: String,
    /// 主要编程语言（逗号分隔）
    pub languages: String,
    /// 创建时间
    pub created_at: DateTime<Utc>,
    /// 最后更新时间
    pub updated_at: DateTime<Utc>,
}

/// 语言统计
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LanguageStat {
    pub repo_id: String,
    pub language: String,
    pub file_count: usize,
    pub line_count: usize,
}

/// 函数记录
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionRecord {
    /// 唯一标识: file_path#function_name#start_line
    pub id: String,
    pub name: String,
    pub file_path: String,
    pub module: String,
    pub language: String,
    pub start_line: usize,
    pub end_line: usize,
    pub signature: String,
    /// 代码内容的 SHA256
    pub content_hash: String,
    /// 索引时间戳
    pub indexed_at: DateTime<Utc>,
}

/// Commit 与函数的关联
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommitFunctionRecord {
    pub commit_hash: String,
    pub function_id: String,
    pub change_type: ChangeType,
    pub before_hash: Option<String>,
    pub after_hash: Option<String>,
    pub indexed_at: DateTime<Utc>,
}

/// 变更类型
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ChangeType {
    Add,
    Modify,
    Delete,
}

impl std::fmt::Display for ChangeType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ChangeType::Add => write!(f, "add"),
            ChangeType::Modify => write!(f, "modify"),
            ChangeType::Delete => write!(f, "delete"),
        }
    }
}

/// 函数调用关系
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CallEdge {
    pub caller_id: String,
    pub callee_id: String,
    pub call_type: CallType,
    pub file_path: Option<String>,
    pub line_number: Option<usize>,
}

/// 调用类型
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CallType {
    Direct,
    Indirect,
    Virtual,
}

/// 影响分析结果
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImpactAnalysis {
    /// 直接改动的函数
    pub direct_changes: Vec<FunctionRecord>,
    /// 上游调用者（被影响）
    pub upstream_impact: Vec<ImpactPath>,
    /// 下游被调用者（依赖）
    pub downstream_impact: Vec<ImpactPath>,
}

/// 影响路径
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImpactPath {
    pub function: FunctionRecord,
    /// 调用链（函数 ID 列表）
    pub path: Vec<String>,
    pub depth: usize,
}

/// Commit 相似度
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommitSimilarity {
    pub commit_hash: String,
    pub similarity_score: f64,
    pub reason: String,
}

// ==================== Phase 2: Class + Inheritance ====================

/// 类/结构体记录（支持 OOP 语言）
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClassRecord {
    /// 唯一标识: file_path#class_name
    pub id: String,
    pub name: String,
    pub file_path: String,
    pub module: String,
    pub language: String,
    /// 父类 ID（继承）
    pub parent_id: Option<String>,
    /// 实现的接口 ID 列表
    pub interface_ids: Vec<String>,
    /// 类开始行
    pub start_line: usize,
    /// 类结束行
    pub end_line: usize,
    /// 索引时间戳
    pub indexed_at: DateTime<Utc>,
}

/// 继承边（类继承关系）
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InheritEdge {
    pub child_id: String,
    pub parent_id: String,
    /// 继承类型
    pub edge_type: InheritType,
}

/// 继承类型
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum InheritType {
    /// 类继承（extends）
    Extends,
    /// 接口实现（implements）
    Implements,
    /// Trait 混入
    Trait,
}

impl std::fmt::Display for InheritType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            InheritType::Extends => write!(f, "extends"),
            InheritType::Implements => write!(f, "implements"),
            InheritType::Trait => write!(f, "trait"),
        }
    }
}

// ==================== Phase 3: Search Graph ====================

/// 搜索结果
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchResult {
    /// 函数记录
    pub functions: Vec<FunctionRecord>,
    /// 类记录
    pub classes: Vec<ClassRecord>,
    /// 搜索元数据
    pub meta: SearchMeta,
}

/// 搜索元数据
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchMeta {
    /// 查询的函数ID
    pub query_id: String,
    /// 搜索深度
    pub depth: usize,
    /// 包含的关系类型
    pub include_types: Vec<String>,
    /// 总结果数
    pub total_count: usize,
}

/// 搜索包含类型
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SearchInclude {
    /// 调用者（上游）
    Callers,
    /// 被调用者（下游）
    Callees,
    /// 兄弟函数（同一类的其他方法）
    Siblings,
    /// 同文件函数
    SameFile,
    /// 同模块函数
    SameModule,
}

impl SearchInclude {
    /// 从字符串解析
    pub fn from_str(s: &str) -> Option<Self> {
        match s {
            "callers" => Some(Self::Callers),
            "callees" => Some(Self::Callees),
            "siblings" => Some(Self::Siblings),
            "same-file" => Some(Self::SameFile),
            "same-module" => Some(Self::SameModule),
            _ => None,
        }
    }

    /// 解析逗号分隔的列表
    pub fn parse_list(s: &str) -> Vec<Self> {
        s.split(',')
            .map(|s| s.trim())
            .filter_map(Self::from_str)
            .collect()
    }
}

impl std::fmt::Display for SearchInclude {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SearchInclude::Callers => write!(f, "callers"),
            SearchInclude::Callees => write!(f, "callees"),
            SearchInclude::Siblings => write!(f, "siblings"),
            SearchInclude::SameFile => write!(f, "same-file"),
            SearchInclude::SameModule => write!(f, "same-module"),
        }
    }
}

// ==================== Phase 4: Arch Integration ====================

/// 架构层定义
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ArchLayer {
    /// API 层（控制器/路由）
    Api,
    /// Service 层（业务逻辑）
    Service,
    /// DAO 层（数据访问）
    Dao,
    /// 工具层
    Util,
}

impl std::fmt::Display for ArchLayer {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ArchLayer::Api => write!(f, "api"),
            ArchLayer::Service => write!(f, "service"),
            ArchLayer::Dao => write!(f, "dao"),
            ArchLayer::Util => write!(f, "util"),
        }
    }
}

impl std::str::FromStr for ArchLayer {
    type Err = String;

    fn from_str(s: &str) -> std::result::Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "api" => Ok(ArchLayer::Api),
            "service" => Ok(ArchLayer::Service),
            "dao" => Ok(ArchLayer::Dao),
            "util" => Ok(ArchLayer::Util),
            _ => Err(format!("Unknown layer: {}", s)),
        }
    }
}

/// 目标架构矩阵（从 YAML 加载）
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TargetMatrix {
    /// 层定义
    pub layers: Vec<LayerDef>,
    /// 允许的依赖规则
    pub allowed_deps: Vec<DepRule>,
}

/// 层定义
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayerDef {
    pub name: String,
    /// 模块匹配模式（如 "app/Controllers/*"）
    pub patterns: Vec<String>,
}

/// 依赖规则
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DepRule {
    /// 源层
    pub from: String,
    /// 目标层
    pub to: String,
}

/// 架构违规
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArchViolation {
    /// 违规类型
    pub violation_type: ViolationType,
    /// 源函数
    pub source_func: FunctionRecord,
    /// 目标函数
    pub target_func: FunctionRecord,
    /// 源层
    pub source_layer: String,
    /// 目标层
    pub target_layer: String,
    /// 描述
    pub message: String,
}

/// 违规类型
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ViolationType {
    /// 跳过层（如 api -> dao 跳过 service）
    SkipLayer,
    /// 反向依赖（如 dao -> service）
    ReverseDep,
    /// 未知层
    UnknownLayer,
}

impl std::fmt::Display for ViolationType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ViolationType::SkipLayer => write!(f, "skip_layer"),
            ViolationType::ReverseDep => write!(f, "reverse_dep"),
            ViolationType::UnknownLayer => write!(f, "unknown_layer"),
        }
    }
}

/// 架构验证结果
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArchValidationResult {
    /// 是否通过
    pub passed: bool,
    /// 违规列表
    pub violations: Vec<ArchViolation>,
    /// 统计信息
    pub stats: ValidationStats,
}

/// 验证统计
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ValidationStats {
    pub total_functions: usize,
    pub checked_deps: usize,
    pub violation_count: usize,
}
