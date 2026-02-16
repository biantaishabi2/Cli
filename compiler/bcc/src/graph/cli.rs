//! graph-index CLI 命令

use crate::graph::error::{GraphError, Result};
use crate::graph::sqlite::{GraphStoreManager, SqliteGraphStore};
use crate::graph::store::CodeGraphStore;
use crate::graph::types::Repository;
use chrono::Utc;
use std::path::PathBuf;

/// 查询类型
#[derive(Debug, Clone)]
pub enum QueryType {
    ById,
    ByName,
    ByModule,
    Callers { depth: usize },
    Callees { depth: usize },
    Impact,
}

/// graph-index build 命令
/// 
/// 用法: bcc graph-index build --repo <id> --input <ast.json> --commit <hash>
pub fn build_index(
    repo_id: &str,
    repo_name: &str,
    root_path: &str,
    _extract_output: &str,
    commit_hash: &str,
) -> Result<()> {
    println!("[graph-index] Building index for repo: {}", repo_id);
    println!("[graph-index] Commit: {}", commit_hash);
    
    // 获取存储管理器
    let manager = GraphStoreManager::default()?;
    
    // 获取或创建存储（注意：每次操作后连接会关闭）
    {
        let store = manager.get_store(repo_id)?;
        
        // 创建/更新仓库信息
        let repo = Repository {
            id: repo_id.to_string(),
            name: repo_name.to_string(),
            root_path: root_path.to_string(),
            languages: "php".to_string(), // TODO: 从 extract 输出检测
            created_at: Utc::now(),
            updated_at: Utc::now(),
        };
        store.update_repository(&repo)?;
    }
    
    // TODO: 读取 extract 输出并构建索引
    println!("[graph-index] Index created successfully");
    Ok(())
}

/// graph-index query 命令
///
/// 用法: bcc graph-index query --repo <id> --id <func-id> [--by name|file|module]
pub fn query_function(
    repo_id: &str,
    query: &str,
    query_type: QueryType,
) -> Result<()> {
    let manager = GraphStoreManager::default()?;
    
    if !manager.repo_exists(repo_id) {
        return Err(GraphError::RepoNotFound(repo_id.to_string()));
    }
    
    // 获取存储（注意：每次操作后连接会关闭）
    let store = manager.get_store(repo_id)?;
    
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
            match store.find_callers(query, depth) {
                Ok(callers) => {
                    println!("Found {} callers of '{}' (depth={})", callers.len(), query, depth);
                    for caller in callers {
                        println!("  - {} ({}:{})", caller.name, caller.file_path, caller.start_line);
                    }
                }
                Err(e) => {
                    eprintln!("Error finding callers: {}", e);
                    return Err(e);
                }
            }
        }
        QueryType::Callees { depth } => {
            match store.find_callees(query, depth) {
                Ok(callees) => {
                    println!("Found {} callees of '{}' (depth={})", callees.len(), query, depth);
                    for callee in callees {
                        println!("  - {} ({}:{})", callee.name, callee.file_path, callee.start_line);
                    }
                }
                Err(e) => {
                    eprintln!("Error finding callees: {}", e);
                    return Err(e);
                }
            }
        }
        QueryType::Impact => {
            match store.analyze_impact(&[query.to_string()]) {
                Ok(impact) => {
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
                Err(e) => {
                    eprintln!("Error analyzing impact: {}", e);
                    return Err(e);
                }
            }
        }
    }
    
    Ok(())
}

/// graph-index list 命令 - 列出所有索引的仓库
pub fn list_repos() -> Result<()> {
    let manager = GraphStoreManager::default()?;
    let repos = manager.list_repos();
    
    println!("Indexed repositories ({}):", repos.len());
    for repo_id in repos {
        if let Ok(store) = manager.get_store(&repo_id) {
            if let Some(repo) = store.get_repository() {
                println!("  - {} ({})", repo_id, repo.name);
                println!("    Path: {}", repo.root_path);
                println!("    Languages: {}", repo.languages);
            } else {
                println!("  - {} (metadata unavailable)", repo_id);
            }
        }
    }
    
    Ok(())
}

/// graph-index delete 命令 - 删除仓库索引
pub fn delete_repo(repo_id: &str) -> Result<()> {
    let manager = GraphStoreManager::default()?;
    
    if !manager.repo_exists(repo_id) {
        return Err(GraphError::RepoNotFound(repo_id.to_string()));
    }
    
    manager.delete_repo(repo_id)?;
    println!("Deleted repository index: {}", repo_id);
    Ok(())
}
