//! CodeGraphStore trait 定义

use crate::graph::error::Result;
use crate::graph::types::*;

/// 代码图谱存储接口
pub trait CodeGraphStore {
    /// 获取仓库信息
    fn get_repository(&self) -> Option<Repository>;

    /// 更新仓库信息
    fn update_repository(&self, repo: &Repository) -> Result<()>;

    /// 获取语言统计
    fn get_language_stats(&self) -> Vec<LanguageStat>;

    /// 从 extract 输出构建索引
    fn index_from_snapshot(
        &self,
        snapshot: &crate::extract::AstSnapshot,
        commit_hash: &str,
    ) -> Result<()>;

    /// 增量索引（只处理新 commit）
    fn index_incremental(&self, commits: &[CommitInfo]) -> Result<()>;

    /// 标记需要重新索引的函数
    fn mark_stale(&self, file_paths: &[String]) -> Result<()>;

    /// 获取函数信息
    fn get_function(&self, id: &str) -> Option<FunctionRecord>;

    /// 按模块查询函数
    fn find_by_module(&self, module: &str) -> Vec<FunctionRecord>;

    /// 按文件查询函数
    fn find_by_file(&self, file_path: &str) -> Vec<FunctionRecord>;

    /// 按名称查询函数
    fn find_by_name(&self, name: &str) -> Vec<FunctionRecord>;

    /// 查找调用者（支持深度）
    fn find_callers(&self, function_id: &str, depth: usize) -> Result<Vec<FunctionRecord>>;

    /// 查找被调用者（支持深度）
    fn find_callees(&self, function_id: &str, depth: usize) -> Result<Vec<FunctionRecord>>;

    /// 查找 commit 改动的函数
    fn find_changed_by_commit(&self, commit_hash: &str) -> Vec<CommitFunctionRecord>;

    /// 影响分析
    fn analyze_impact(&self, function_ids: &[String]) -> Result<ImpactAnalysis>;

    /// 查找相似 commit
    fn find_similar_commits(&self, commit_hash: &str, limit: usize) -> Vec<CommitSimilarity>;
}

/// Commit 信息（用于增量索引）
#[derive(Debug, Clone)]
pub struct CommitInfo {
    pub hash: String,
    pub changed_files: Vec<String>,
}

/// 为 CodeGraphStore 扩展插入方法（内部使用）
pub trait GraphStoreInsert: CodeGraphStore {
    fn insert_function(&self, func: &FunctionRecord) -> Result<()>;
    fn insert_call_edge(&self, edge: &CallEdge) -> Result<()>;
    fn insert_commit_function(&self, cf: &CommitFunctionRecord) -> Result<()>;
    fn insert_language_stat(&self, stat: &LanguageStat) -> Result<()>;
}
