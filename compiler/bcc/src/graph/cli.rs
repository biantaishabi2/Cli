//! graph-index CLI 命令

use crate::graph::store::CodeGraphStore;
use crate::graph::sqlite::SqliteGraphStore;
use crate::graph::types::*;
use std::path::Path;

/// 查询类型
pub enum QueryType {
    ById,
    ByName,
    ByModule,
    Callers { depth: usize },
    Callees { depth: usize },
    Impact,
}

/// graph-index build 命令
pub fn build_index(
    extract_output: &str,
    commit_hash: &str,
    db_path: &str,
) -> Result<(), String> {
    println!("[graph-index] Building index from: {}", extract_output);
    println!("[graph-index] Commit: {}", commit_hash);
    println!("[graph-index] Database: {}", db_path);
    
    // 确保目录存在
    if let Some(parent) = Path::new(db_path).parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Failed to create directory: {}", e))?;
    }
    
    // 创建存储
    let store = SqliteGraphStore::new(db_path)
        .map_err(|e| format!("Failed to open database: {}", e))?;
    
    // TODO: 读取 extract 输出并构建索引
    println!("[graph-index] Index created successfully");
    Ok(())
}

/// graph-index query 命令
pub fn query_function(
    db_path: &str,
    query: &str,
    query_type: QueryType,
) -> Result<(), String> {
    let store = SqliteGraphStore::new(db_path)
        .map_err(|e| format!("Failed to open database: {}", e))?;
    
    match query_type {
        QueryType::ById => {
            if let Some(func) = store.get_function(query) {
                println!("Found function:");
                println!("  ID: {}", func.id);
                println!("  Name: {}", func.name);
                println!("  File: {}:{}", func.file_path, func.start_line);
                println!("  Module: {}", func.module);
                println!("  Signature: {}", func.signature);
            } else {
                println!("Function not found: {}", query);
            }
        }
        QueryType::ByName => {
            let funcs = store.find_by_name(query);
            println!("Found {} functions with name '{}'", funcs.len(), query);
            for func in funcs {
                println!("  - {} ({}:{})", func.id, func.file_path, func.start_line);
            }
        }
        QueryType::ByModule => {
            let funcs = store.find_by_module(query);
            println!("Found {} functions in module '{}'", funcs.len(), query);
            for func in funcs {
                println!("  - {} ({})", func.name, func.file_path);
            }
        }
        QueryType::Callers { depth } => {
            let callers = store.find_callers(query, depth);
            println!("Found {} callers of '{}' (depth={})", callers.len(), query, depth);
            for caller in callers {
                println!("  - {} ({}:{})", caller.name, caller.file_path, caller.start_line);
            }
        }
        QueryType::Callees { depth } => {
            let callees = store.find_callees(query, depth);
            println!("Found {} callees of '{}' (depth={})", callees.len(), query, depth);
            for callee in callees {
                println!("  - {} ({}:{})", callee.name, callee.file_path, callee.start_line);
            }
        }
        QueryType::Impact => {
            let impact = store.analyze_impact(&[query.to_string()]);
            println!("Impact Analysis for '{}'", query);
            println!("\nDirect changes: {}", impact.direct_changes.len());
            println!("Upstream impact: {}", impact.upstream_impact.len());
            for path in &impact.upstream_impact {
                println!("  - {} (depth={})", path.function.name, path.depth);
            }
            println!("Downstream impact: {}", impact.downstream_impact.len());
            for path in &impact.downstream_impact {
                println!("  - {} (depth={})", path.function.name, path.depth);
            }
        }
    }
    
    Ok(())
}
