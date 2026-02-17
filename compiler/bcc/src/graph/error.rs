//! Graph 模块错误类型定义

use thiserror::Error;

/// Graph 模块错误类型
#[derive(Debug, Error)]
pub enum GraphError {
    #[error("仓库不存在: {0}")]
    RepoNotFound(String),

    #[error("数据库损坏: {0}")]
    SqliteCorrupted(String),

    #[error("继承链断裂: 父类 {0} 未索引")]
    InheritanceBroken(String),

    #[error("循环继承检测: {0}")]
    CircularInheritance(String),

    #[error("搜索深度超限: 最大 {0}")]
    DepthLimitExceeded(usize),

    #[error("架构违规: {0}")]
    ArchViolation(String),

    #[error("参数错误: {0}")]
    InvalidArgs(String),

    #[error("数据库错误: {0}")]
    Database(#[from] rusqlite::Error),

    #[error("IO错误: {0}")]
    Io(#[from] std::io::Error),

    #[error("序列化错误: {0}")]
    Serialization(#[from] serde_json::Error),
}

impl GraphError {
    /// 获取退出码
    pub fn exit_code(&self) -> i32 {
        match self {
            GraphError::RepoNotFound(_) => 1,
            GraphError::SqliteCorrupted(_) => 2,
            GraphError::InheritanceBroken(_) => 3,
            GraphError::CircularInheritance(_) => 4,
            GraphError::DepthLimitExceeded(_) => 5,
            GraphError::ArchViolation(_) => 6,
            GraphError::InvalidArgs(_) => 10,
            GraphError::Database(_) => 20,
            GraphError::Io(_) => 21,
            GraphError::Serialization(_) => 22,
        }
    }
}

pub type Result<T> = std::result::Result<T, GraphError>;
