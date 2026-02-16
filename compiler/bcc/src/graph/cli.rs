//! graph-index CLI 命令

use crate::graph::arch::ArchValidator;
use crate::graph::error::{GraphError, Result};
use crate::graph::sqlite::GraphStoreManager;
use crate::graph::store::{CodeGraphStore, GraphStoreInsert};
use crate::graph::types::{CallEdge, CallType, FunctionRecord, Repository, SearchInclude};
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

/// graph build 命令
/// 
/// 用法: bcc graph build --repo <id> --input <ast.json> --commit <hash>
pub fn build_index(
    repo_id: &str,
    repo_name: &str,
    root_path: &str,
    extract_output: &str,
    commit_hash: &str,
) -> Result<()> {
    println!("[graph] Building index for repo: {}", repo_id);
    println!("[graph] Input: {}", extract_output);
    println!("[graph] Commit: {}", commit_hash);
    
    // 读取 extract 输出
    let ast_json = std::fs::read_to_string(extract_output)
        .map_err(|e| GraphError::Io(e))?;
    
    let snapshot: crate::extract::AstSnapshot = serde_json::from_str(&ast_json)
        .map_err(|e| GraphError::Serialization(e))?;
    
    println!("[graph] Parsed AST snapshot: {} files", snapshot.source_count);
    
    // 获取存储管理器
    let manager = GraphStoreManager::default()?;
    let store = manager.get_store(repo_id)?;
    
    // 创建/更新仓库信息
    let repo = Repository {
        id: repo_id.to_string(),
        name: repo_name.to_string(),
        root_path: root_path.to_string(),
        languages: "python".to_string(), // 从 extract 输出检测
        created_at: Utc::now(),
        updated_at: Utc::now(),
    };
    store.update_repository(&repo)?;
    
    // 从 AST 构建函数索引
    let mut func_count = 0;
    let mut edge_count = 0;
    
    for record in &snapshot.records {
        // 为每个文件创建一个函数记录（简化处理）
        // 实际应该从 AST 详细解析函数定义
        let func_id = format!("{}#file#1", record.sourcePath);
        let func = FunctionRecord {
            id: func_id.clone(),
            name: "__file__".to_string(),
            file_path: record.sourcePath.clone(),
            module: extract_module(&record.sourcePath),
            language: "python".to_string(),
            start_line: 1,
            end_line: record.loc_lines,
            signature: format!("module: {} ({} lines)", record.sourcePath, record.loc_lines),
            content_hash: format!("{:x}", md5::compute(&record.sourcePath)),
            indexed_at: Utc::now(),
        };
        
        GraphStoreInsert::insert_function(&store, &func)?;
        func_count += 1;
        
        // 创建调用边（从 localCallTargets）
        for target in &record.localCallTargets {
            let edge = CallEdge {
                caller_id: func_id.clone(),
                callee_id: target.clone(),
                call_type: CallType::Direct,
                file_path: Some(record.sourcePath.clone()),
                line_number: None,
            };
            GraphStoreInsert::insert_call_edge(&store, &edge)?;
            edge_count += 1;
        }
    }
    
    println!("[graph] Indexed {} functions, {} call edges", func_count, edge_count);
    println!("[graph] Index created successfully");
    Ok(())
}

/// 从文件路径提取模块名
fn extract_module(file_path: &str) -> String {
    std::path::Path::new(file_path)
        .parent()
        .and_then(|p| p.file_name())
        .and_then(|n| n.to_str())
        .unwrap_or("root")
        .to_string()
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

/// graph-index search 命令
///
/// 用法: bcc graph-index search --repo <id> --id <func-id> --depth <n> --include <types>
pub fn search_graph(
    repo_id: &str,
    function_id: &str,
    depth: usize,
    include: &str,
) -> Result<()> {
    let manager = GraphStoreManager::default()?;
    
    if !manager.repo_exists(repo_id) {
        return Err(GraphError::RepoNotFound(repo_id.to_string()));
    }
    
    let store = manager.get_store(repo_id)?;
    
    // 解析包含类型
    let include_types = SearchInclude::parse_list(include);
    if include_types.is_empty() {
        return Err(GraphError::InvalidArgs(
            format!("Invalid include types: {}. Valid: callers,callees,siblings,same-file,same-module", include)
        ));
    }
    
    println!("Searching graph for '{}' (depth={})", function_id, depth);
    println!("Include: {}", include);
    
    let result = store.search_graph(function_id, depth, &include_types)?;
    
    println!("\nFound {} functions:", result.functions.len());
    for func in &result.functions {
        println!("  - {} ({}:{})", func.name, func.file_path, func.start_line);
    }
    
    if !result.classes.is_empty() {
        println!("\nFound {} classes:", result.classes.len());
        for class in &result.classes {
            println!("  - {} ({}:{})", class.name, class.file_path, class.start_line);
        }
    }
    
    Ok(())
}

/// graph-index validate-arch 命令
///
/// 用法: bcc graph-index validate-arch --repo <id> --target <target-matrix.yaml> --output <json>
pub fn validate_arch(
    repo_id: &str,
    target_path: &str,
    output_path: Option<&str>,
) -> Result<()> {
    let manager = GraphStoreManager::default()?;
    
    if !manager.repo_exists(repo_id) {
        return Err(GraphError::RepoNotFound(repo_id.to_string()));
    }
    
    // 加载目标架构
    let validator = ArchValidator::from_yaml(target_path)?;
    let store = manager.get_store(repo_id)?;
    
    println!("Validating architecture for repo: {}", repo_id);
    println!("Target matrix: {}", target_path);
    
    // 执行验证
    let result = validator.validate_function(&store, "")?;
    
    println!("\nValidation result: {}", if result.passed { "PASSED" } else { "FAILED" });
    println!("Total functions: {}", result.stats.total_functions);
    println!("Checked dependencies: {}", result.stats.checked_deps);
    println!("Violations: {}", result.violations.len());
    
    if !result.violations.is_empty() {
        println!("\nViolations:");
        for (i, v) in result.violations.iter().enumerate() {
            println!("  {}. [{}] {}", i + 1, v.violation_type, v.message);
        }
    }
    
    // 保存结果到文件
    if let Some(output) = output_path {
        let json = serde_json::to_string_pretty(&result)?;
        std::fs::write(output, json)?;
        println!("\nResults saved to: {}", output);
    }
    
    Ok(())
}
