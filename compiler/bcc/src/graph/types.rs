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
