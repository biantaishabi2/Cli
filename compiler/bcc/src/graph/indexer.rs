//! 从 extract 输出构建索引

use crate::graph::store::{CodeGraphStore, Result};
use crate::graph::types::*;
use chrono::Utc;
use sha2::{Digest, Sha256};

/// 索引构建器
pub struct Indexer<'a> {
    store: &'a dyn CodeGraphStore,
}

impl<'a> Indexer<'a> {
    pub fn new(store: &'a dyn CodeGraphStore) -> Self {
        Self { store }
    }

    /// 从 extract 输出构建完整索引
    /// TODO: 需要与 extract 模块集成
    pub fn build_from_snapshot(&self, _commit_hash: &str) -> Result<()> {
        // TODO: 实现从 AstSnapshot 构建索引
        // 需要与 extract 模块协调数据结构
        Ok(())
    }

    /// 计算内容 hash
    fn compute_hash(&self, content: &str) -> String {
        let mut hasher = Sha256::new();
        hasher.update(content.as_bytes());
        format!("{:x}", hasher.finalize())[..16].to_string()
    }
}
