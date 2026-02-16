//! graph-index CLI 命令

use crate::graph::store::CodeGraphStore;
use crate::graph::sqlite::SqliteGraphStore;
use crate::graph::indexer::Indexer;
use std::path::Path;

/// graph-index build 命令
pub fn build_index(
    extract_output: &str,
    commit_hash: &str,
    db_path: &str,
) -> Result<(), String> {
    println!("[graph-index] Building index from: {}", extract_output);
    
    // 1. 读取 extract 输出
    let snapshot = read_extract_output(extract_output)?;
    
    // 2. 创建/打开存储
    let store = SqliteGraphStore::new(db_path)
        .map_err(|e| format!("Failed to open database: {}", e))?;
    
    // 3. 构建索引
    let indexer = Indexer::new(&store);
    indexer.build_from_snapshot(&snapshot, commit_hash)
        .map_err(|e| format!("Failed to build index: {}", e))?;
    
    println!("[graph-index] Index built successfully at: {}", db_path);
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
                println!("{}", serde_json::to_string_pretty(&func).unwrap());
            } else {
                println!("Function not found: {}", query);
            }
        }
        QueryType::ByName => {
            let funcs = store.find_by_name(query);
            println!("Found {} functions:", funcs.len());
            for func in funcs {
                println!("  - {} ({}:{})", func.id, func.file_path, func.start_line);
            }
        }
        QueryType::ByModule => {
            let funcs = store.find_by_module(query);
            println!("Found {} functions in module '{}':", funcs.len(), query);
            for func in funcs {
                println!("  - {} ({})", func.name, func.file_path);
            }
        }
        QueryType::Callers { depth } => {
            let callers = store.find_callers(query, depth);
            println!("Found {} callers (depth={}):", callers.len(), depth);
            for caller in callers {
                println!("  - {} ({}:{})", caller.name, caller.file_path, caller.start_line);
            }
        }
        QueryType::Callees { depth } => {
            let callees = store.find_callees(query, depth);
            println!("Found {} callees (depth={}):", callees.len(), depth);
            for callee in callees {
                println!("  - {} ({}:{})", callee.name, callee.file_path, callee.start_line);
            }
        }
        QueryType::Impact => {
            let impact = store.analyze_impact(&[query.to_string()]);
            println!("Impact Analysis for {}:", query);
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

/// 查询类型
pub enum QueryType {
    ById,
    ByName,
    ByModule,
    Callers { depth: usize },
    Callees { depth: usize },
    Impact,
}

/// 读取 extract 输出
fn read_extract_output(path: &str) -> Result<crate::extract::AstSnapshot, String> {
    let content = std::fs::read_to_string(path)
        .map_err(|e| format!("Failed to read {}: {}", path, e))?;
    
    serde_json::from_str(&content)
        .map_err(|e| format!("Failed to parse JSON: {}", e))
}
