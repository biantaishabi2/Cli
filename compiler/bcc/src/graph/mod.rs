//! BCC Code Graph Index Module
//!
//! 提供代码知识图谱的存储和查询能力
//! 支持函数级别的调用关系分析和影响分析

pub mod arch;
pub mod cli;
pub mod error;
pub mod impact;
pub mod indexer;
pub mod query;
pub mod schema;
pub mod sqlite;
pub mod store;
pub mod types;
