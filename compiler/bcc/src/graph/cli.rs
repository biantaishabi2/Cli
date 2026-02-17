//! graph-index CLI 命令

use crate::graph::arch::ArchValidator;
use crate::graph::error::{GraphError, Result};
use crate::graph::sqlite::GraphStoreManager;
use crate::graph::store::{CodeGraphStore, GraphStoreInsert};
use crate::graph::types::{CallEdge, CallType, FunctionRecord, Repository, SearchInclude, ModuleRecord, ModuleDepEdge, DepType};
use chrono::Utc;
use std::collections::BTreeSet;

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
    
    let languages = detect_snapshot_languages(&snapshot);

    // 创建/更新仓库信息
    let repo = Repository {
        id: repo_id.to_string(),
        name: repo_name.to_string(),
        root_path: root_path.to_string(),
        languages,
        created_at: Utc::now(),
        updated_at: Utc::now(),
    };
    store.update_repository(&repo)?;
    
    // 从 AST 构建索引
    let mut func_count = 0;
    let mut edge_count = 0;
    let mut module_count = 0;
    let mut mod_dep_count = 0;
    
    // 第一步：收集所有函数 ID
    let mut func_ids = std::collections::HashSet::new();
    for record in &snapshot.records {
        let func_id = format!("{}#file#1", record.sourcePath);
        func_ids.insert(func_id);
    }
    
    // 第二步：插入所有模块（文件级节点）
    for record in &snapshot.records {
        let module_id = record.sourcePath.clone();
        let module_name = std::path::Path::new(&record.sourcePath)
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("unknown")
            .to_string();
        let directory = std::path::Path::new(&record.sourcePath)
            .parent()
            .and_then(|p| p.to_str())
            .unwrap_or("")
            .to_string();
        
        let module = ModuleRecord {
            id: module_id,
            name: module_name,
            file_path: record.sourcePath.clone(),
            directory,
            exports_count: record.exports_count,
            imports_count: record.imports_count,
            loc_lines: record.loc_lines,
            language: detect_language_from_path(&record.sourcePath),
        };
        
        GraphStoreInsert::insert_module(&store, &module)?;
        module_count += 1;
    }
    
    // 第三步：插入所有函数
    for record in &snapshot.records {
        let func_id = format!("{}#file#1", record.sourcePath);
        let func = FunctionRecord {
            id: func_id.clone(),
            name: "__file__".to_string(),
            file_path: record.sourcePath.clone(),
            module: extract_module(&record.sourcePath),
            language: detect_language_from_path(&record.sourcePath),
            start_line: 1,
            end_line: record.loc_lines,
            signature: format!("module: {} ({} lines)", record.sourcePath, record.loc_lines),
            content_hash: format!("{:x}", md5::compute(&record.sourcePath)),
            indexed_at: Utc::now(),
        };
        
        GraphStoreInsert::insert_function(&store, &func)?;
        func_count += 1;
    }
    
    // 第四步：创建模块依赖边（localDependencies）
    for record in &snapshot.records {
        let source_id = record.sourcePath.clone();
        
        for dep in &record.localDependencies {
            // 尝试多种路径格式匹配
            let possible_targets = vec![
                dep.clone(),
                format!("./{}", dep),
                dep.trim_start_matches("./").to_string(),
                format!("src/{}", dep),
                dep.trim_start_matches("src/").to_string(),
            ];
            
            // 找到匹配的目标模块
            let matched_target = possible_targets.iter()
                .find(|t| snapshot.records.iter().any(|r| &r.sourcePath == *t));
            
            if let Some(target_id) = matched_target {
                let edge = ModuleDepEdge {
                    source_id: source_id.clone(),
                    target_id: target_id.clone(),
                    dep_type: DepType::Import,
                    symbols: vec![], // 可以从 imports 中提取
                };
                GraphStoreInsert::insert_module_dep_edge(&store, &edge)?;
                mod_dep_count += 1;
            }
        }
    }
    
    // 第五步：创建调用边（只创建指向已索引函数的边）
    for record in &snapshot.records {
        let func_id = format!("{}#file#1", record.sourcePath);
        
        for target in &record.localCallTargets {
            // 尝试多种路径格式匹配
            let possible_targets = vec![
                format!("{}#file#1", target),
                // 去掉 ./src/ 前缀
                format!("{}#file#1", target.trim_start_matches("./src/")),
                // 去掉 src/ 前缀
                format!("{}#file#1", target.trim_start_matches("src/")),
                // 保留原样
                target.clone(),
            ];
            
            // 找到匹配的函数 ID
            let matched_target = possible_targets.iter()
                .find(|t| func_ids.contains(*t));
            
            if let Some(target_func_id) = matched_target {
                let edge = CallEdge {
                    caller_id: func_id.clone(),
                    callee_id: target_func_id.clone(),
                    call_type: CallType::Direct,
                    file_path: Some(record.sourcePath.clone()),
                    line_number: None,
                };
                GraphStoreInsert::insert_call_edge(&store, &edge)?;
                edge_count += 1;
            }
        }
    }
    
    println!("[graph] Indexed {} modules, {} module deps, {} functions, {} call edges", 
             module_count, mod_dep_count, func_count, edge_count);
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

fn detect_snapshot_languages(snapshot: &crate::extract::AstSnapshot) -> String {
    let mut langs = BTreeSet::new();
    for record in &snapshot.records {
        langs.insert(detect_language_from_path(&record.sourcePath));
    }
    if langs.is_empty() {
        "unknown".to_string()
    } else {
        langs.into_iter().collect::<Vec<_>>().join(",")
    }
}

fn detect_language_from_path(file_path: &str) -> String {
    let path = file_path.to_ascii_lowercase();
    if path.ends_with(".py") {
        "python".to_string()
    } else if path.ends_with(".ts") || path.ends_with(".tsx") {
        "typescript".to_string()
    } else if path.ends_with(".js") || path.ends_with(".jsx") {
        "javascript".to_string()
    } else if path.ends_with(".rs") {
        "rust".to_string()
    } else if path.ends_with(".php") {
        "php".to_string()
    } else if path.ends_with(".ex") || path.ends_with(".exs") {
        "elixir".to_string()
    } else if path.ends_with(".go") {
        "go".to_string()
    } else {
        "unknown".to_string()
    }
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

/// 模块依赖查询类型
#[derive(Debug, Clone)]
pub enum ModuleQueryType {
    ById,
    Deps { depth: usize },
    Dependents { depth: usize },
    Circular,
}

/// graph-index module 命令 - 查询模块依赖
///
/// 用法: bcc graph-index module --repo <id> --id <module-id> [--deps|--dependents|--circular]
pub fn query_module(
    repo_id: &str,
    module_id: &str,
    query_type: ModuleQueryType,
) -> Result<()> {
    let manager = GraphStoreManager::default()?;
    
    if !manager.repo_exists(repo_id) {
        return Err(GraphError::RepoNotFound(repo_id.to_string()));
    }
    
    let store = manager.get_store(repo_id)?;
    
    match query_type {
        ModuleQueryType::ById => {
            if let Some(module) = store.get_module(module_id) {
                println!("Found module:");
                println!("  ID: {}", module.id);
                println!("  Name: {}", module.name);
                println!("  Directory: {}", module.directory);
                println!("  Lines: {}", module.loc_lines);
                println!("  Exports: {}", module.exports_count);
                println!("  Imports: {}", module.imports_count);
            } else {
                println!("Module not found: {}", module_id);
            }
        }
        ModuleQueryType::Deps { depth } => {
            match store.find_module_deps(module_id, depth) {
                Ok(deps) => {
                    println!("Found {} dependencies of '{}' (depth={})", deps.len(), module_id, depth);
                    for dep in deps {
                        println!("  - {} ({})", dep.name, dep.file_path);
                    }
                }
                Err(e) => {
                    eprintln!("Error finding dependencies: {}", e);
                    return Err(e);
                }
            }
        }
        ModuleQueryType::Dependents { depth } => {
            match store.find_module_dependents(module_id, depth) {
                Ok(dependents) => {
                    println!("Found {} dependents of '{}' (depth={})", dependents.len(), module_id, depth);
                    for dep in dependents {
                        println!("  - {} ({})", dep.name, dep.file_path);
                    }
                }
                Err(e) => {
                    eprintln!("Error finding dependents: {}", e);
                    return Err(e);
                }
            }
        }
        ModuleQueryType::Circular => {
            match store.detect_circular_deps(module_id) {
                Ok(Some(cycle)) => {
                    println!("Circular dependency detected:");
                    for (i, m) in cycle.iter().enumerate() {
                        println!("  {}. {}", i + 1, m);
                    }
                }
                Ok(None) => {
                    println!("No circular dependencies found for '{}'", module_id);
                }
                Err(e) => {
                    eprintln!("Error detecting circular deps: {}", e);
                    return Err(e);
                }
            }
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
    
    // 执行仓库级验证
    let result = validator.validate(&store)?;
    
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
