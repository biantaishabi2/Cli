//! 从 extract 输出构建索引

use crate::graph::error::Result;
use crate::graph::store::CodeGraphStore;

/// 索引构建器
pub struct Indexer<'a> {
    store: &'a dyn CodeGraphStore,
}

impl<'a> Indexer<'a> {
    pub fn new(store: &'a dyn CodeGraphStore) -> Self {
        Self { store }
    }

    /// 从 extract 输出构建完整索引
    /// TODO: 需要与 extract 模块集成，等待 AstSnapshot 结构确定
    pub fn build_from_snapshot(&self, _commit_hash: &str) -> Result<()> {
        // 暂时空实现，避免编译警告
        Ok(())
    }
}
