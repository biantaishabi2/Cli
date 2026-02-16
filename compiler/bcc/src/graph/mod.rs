//! BCC Code Graph Index Module
//!
//! 提供代码知识图谱的存储和查询能力
//! 支持函数级别的调用关系分析和影响分析

pub mod arch;
pub mod error;
pub mod schema;
pub mod store;
pub mod sqlite;
pub mod indexer;
pub mod query;
pub mod impact;
pub mod types;
pub mod cli;
