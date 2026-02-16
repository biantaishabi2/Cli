//! 从 extract 输出构建索引

use crate::extract::{AstSnapshot, FileRecord, FunctionInfo};
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

    /// 从 AstSnapshot 构建完整索引
    pub fn build_from_snapshot(&self, snapshot: &AstSnapshot, commit_hash: &str) -> Result<()> {
        // 1. 提取所有函数
        let functions = self.extract_functions(snapshot);
        
        // 2. 提取调用关系
        let call_edges = self.extract_call_edges(snapshot, &functions);
        
        // 3. 写入存储
        for func in &functions {
            self.store.insert_function(func)?;
        }
        
        for edge in &call_edges {
            self.store.insert_call_edge(edge)?;
        }
        
        // 4. 记录 commit 与函数的关联
        for func in &functions {
            let cf = CommitFunctionRecord {
                commit_hash: commit_hash.to_string(),
                function_id: func.id.clone(),
                change_type: ChangeType::Add, // TODO: 根据实际变更类型判断
                before_hash: None,
                after_hash: Some(func.content_hash.clone()),
                indexed_at: Utc::now(),
            };
            self.store.insert_commit_function(&cf)?;
        }
        
        Ok(())
    }

    /// 从 AstSnapshot 提取函数列表
    fn extract_functions(&self, snapshot: &AstSnapshot) -> Vec<FunctionRecord> {
        let mut functions = vec![];
        
        for file in &snapshot.records {
            for func_info in &file.exports {
                let func = self.convert_function_info(func_info, file);
                functions.push(func);
            }
        }
        
        functions
    }

    /// 转换 FunctionInfo 为 FunctionRecord
    fn convert_function_info(&self, info: &FunctionInfo, file: &FileRecord) -> FunctionRecord {
        // 生成唯一 ID: file_path#function_name#start_line
        let id = format!("{}#{}#{}", file.file_path, info.name, info.line);
        
        // 计算内容 hash（基于签名）
        let content_hash = self.compute_hash(&info.signature);
        
        // 推断模块（从文件路径）
        let module = self.infer_module(&file.file_path);
        
        FunctionRecord {
            id,
            name: info.name.clone(),
            file_path: file.file_path.clone(),
            module,
            language: file.language.clone(),
            start_line: info.line,
            end_line: info.line + 10, // TODO: 从 AstSnapshot 获取实际结束行
            signature: info.signature.clone(),
            content_hash,
            indexed_at: Utc::now(),
        }
    }

    /// 提取调用关系
    fn extract_call_edges(&self, snapshot: &AstSnapshot, functions: &[FunctionRecord]) -> Vec<CallEdge> {
        let mut edges = vec![];
        
        // 构建函数名到 ID 的映射
        let name_to_id: std::collections::HashMap<String, String> = functions
            .iter()
            .map(|f| (f.name.clone(), f.id.clone()))
            .collect();
        
        for file in &snapshot.records {
            for func_info in &file.exports {
                let caller_id = format!("{}#{}#{}", file.file_path, func_info.name, func_info.line);
                
                // 提取函数体内的调用（从代码内容解析）
                let calls = self.extract_calls_from_code(&func_info.signature);
                
                for callee_name in calls {
                    if let Some(callee_id) = name_to_id.get(&callee_name) {
                        edges.push(CallEdge {
                            caller_id: caller_id.clone(),
                            callee_id: callee_id.clone(),
                            call_type: CallType::Direct,
                            file_path: Some(file.file_path.clone()),
                            line_number: Some(func_info.line),
                        });
                    }
                }
            }
        }
        
        edges
    }

    /// 从代码内容提取调用（简单启发式）
    fn extract_calls_from_code(&self, code: &str) -> Vec<String> {
        let mut calls = vec![];
        
        // 简单正则匹配：function_name(
        // 实际应该用 tree-sitter 解析
        for line in code.lines() {
            // 匹配可能的函数调用模式
            if let Some(pos) = line.find('(') {
                let before = &line[..pos];
                // 提取最后一个标识符作为函数名
                if let Some(name) = before.split_whitespace().last() {
                    let name = name.trim();
                    if !name.is_empty() && !name.contains(".") && !name.contains("$") {
                        calls.push(name.to_string());
                    }
                }
            }
        }
        
        calls
    }

    /// 从文件路径推断模块
    fn infer_module(&self, file_path: &str) -> String {
        // 简单启发式：从路径提取模块名
        // 例如：app/controllers/order_controller.php -> order
        // 例如：lib/shop/order/cart.ex -> order
        
        let path = std::path::Path::new(file_path);
        
        // 尝试从文件名提取
        if let Some(stem) = path.file_stem() {
            let name = stem.to_string_lossy().to_string();
            
            // 去掉常见后缀
            let suffixes = ["Controller", "Service", "Model", "Trait", 
                           "_controller", "_service", "_model"];
            for suffix in &suffixes {
                if name.ends_with(suffix) {
                    return name[..name.len() - suffix.len()].to_lowercase();
                }
            }
            
            // 从目录结构推断
            let parts: Vec<&str> = file_path.split('/').collect();
            if parts.len() >= 2 {
                // 跳过通用目录名
                let skip = ["app", "lib", "src", "controllers", "models", "services"];
                for part in parts.iter().rev().skip(1) {
                    if !skip.contains(part) && !part.is_empty() {
                        return part.to_lowercase();
                    }
                }
            }
            
            return name.to_lowercase();
        }
        
        "unknown".to_string()
    }

    /// 计算内容 hash
    fn compute_hash(&self, content: &str) -> String {
        let mut hasher = Sha256::new();
        hasher.update(content.as_bytes());
        format!("{:x}", hasher.finalize())[..16].to_string()
    }
}

/// 为 CodeGraphStore 扩展 insert 方法
trait GraphStoreInsert {
    fn insert_function(&self, func: &FunctionRecord) -> Result<()>;
    fn insert_call_edge(&self, edge: &CallEdge) -> Result<()>;
    fn insert_commit_function(&self, cf: &CommitFunctionRecord) -> Result<()>;
}

// 在 sqlite.rs 中实现这个 trait
