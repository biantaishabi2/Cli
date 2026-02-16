//! SQLite 实现

use crate::graph::schema::CREATE_SCHEMA_SQL;
use crate::graph::store::{CodeGraphStore, Result};
use crate::graph::types::*;
use chrono::Utc;
use rusqlite::{params, Connection, OptionalExtension};
use std::path::Path;

/// SQLite 图存储实现
pub struct SqliteGraphStore {
    conn: Connection,
}

impl SqliteGraphStore {
    /// 创建新的存储实例
    pub fn new<P: AsRef<Path>>(db_path: P) -> Result<Self> {
        let conn = Connection::open(db_path)?;
        let store = Self { conn };
        store.init_schema()?;
        Ok(store)
    }

    /// 创建内存存储（用于测试）
    pub fn new_in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory()?;
        let store = Self { conn };
        store.init_schema()?;
        Ok(store)
    }

    /// 初始化数据库 Schema
    fn init_schema(&self) -> Result<()> {
        self.conn.execute_batch(CREATE_SCHEMA_SQL)?;
        Ok(())
    }

    /// 插入函数记录（内部方法）
    pub fn insert_function(&self, func: &FunctionRecord) -> Result<()> {
        self.conn.execute(
            r#"
            INSERT OR REPLACE INTO functions 
            (id, name, file_path, module, language, start_line, end_line, signature, content_hash, indexed_at)
            VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)
            "#,
            params![
                func.id,
                func.name,
                func.file_path,
                func.module,
                func.language,
                func.start_line as i64,
                func.end_line as i64,
                func.signature,
                func.content_hash,
                func.indexed_at.to_rfc3339(),
            ],
        )?;
        Ok(())
    }

    /// 插入调用关系（内部方法）
    pub fn insert_call_edge(&self, edge: &CallEdge) -> Result<()> {
        self.conn.execute(
            r#"
            INSERT OR REPLACE INTO call_edges 
            (caller_id, callee_id, call_type, file_path, line_number)
            VALUES (?1, ?2, ?3, ?4, ?5)
            "#,
            params![
                edge.caller_id,
                edge.callee_id,
                format!("{:?}", edge.call_type).to_lowercase(),
                edge.file_path,
                edge.line_number.map(|n| n as i64),
            ],
        )?;
        Ok(())
    }

    /// 插入 commit 函数关联（内部方法）
    pub fn insert_commit_function(&self, cf: &CommitFunctionRecord) -> Result<()> {
        self.conn.execute(
            r#"
            INSERT OR REPLACE INTO commit_functions 
            (commit_hash, function_id, change_type, before_hash, after_hash, indexed_at)
            VALUES (?1, ?2, ?3, ?4, ?5, ?6)
            "#,
            params![
                cf.commit_hash,
                cf.function_id,
                cf.change_type.to_string(),
                cf.before_hash,
                cf.after_hash,
                cf.indexed_at.to_rfc3339(),
            ],
        )?;
        Ok(())
    }

    /// 从数据库行解析函数记录
    fn parse_function_row(row: &rusqlite::Row) -> rusqlite::Result<FunctionRecord> {
        Ok(FunctionRecord {
            id: row.get(0)?,
            name: row.get(1)?,
            file_path: row.get(2)?,
            module: row.get(3)?,
            language: row.get(4)?,
            start_line: row.get::<_, i64>(5)? as usize,
            end_line: row.get::<_, i64>(6)? as usize,
            signature: row.get(7)?,
            content_hash: row.get(8)?,
            indexed_at: row.get::<_, String>(9)?.parse().unwrap_or_else(|_| Utc::now()),
        })
    }
}

impl CodeGraphStore for SqliteGraphStore {
    fn index_from_snapshot(
        &self,
        _snapshot: &crate::extract::AstSnapshot,
        _commit_hash: &str,
    ) -> Result<()> {
        // TODO: 实现从 AstSnapshot 构建索引
        Ok(())
    }

    fn index_incremental(&self, _commits: &[crate::graph::store::CommitInfo]) -> Result<()> {
        // TODO: 实现增量索引
        Ok(())
    }

    fn mark_stale(&self, _file_paths: &[String]) -> Result<()> {
        // TODO: 实现标记 stale
        Ok(())
    }

    fn get_function(&self, id: &str) -> Option<FunctionRecord> {
        self.conn
            .query_row(
                "SELECT * FROM functions WHERE id = ?1",
                params![id],
                Self::parse_function_row,
            )
            .optional()
            .ok()
            .flatten()
    }

    fn find_by_module(&self, module: &str) -> Vec<FunctionRecord> {
        let mut stmt = match self.conn.prepare(
            "SELECT * FROM functions WHERE module = ?1 ORDER BY file_path, start_line"
        ) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = match stmt.query_map(params![module], Self::parse_function_row) {
            Ok(rows) => rows,
            Err(_) => return vec![],
        };

        rows.filter_map(|r| r.ok()).collect()
    }

    fn find_by_file(&self, file_path: &str) -> Vec<FunctionRecord> {
        let mut stmt = match self.conn.prepare(
            "SELECT * FROM functions WHERE file_path = ?1 ORDER BY start_line"
        ) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = match stmt.query_map(params![file_path], Self::parse_function_row) {
            Ok(rows) => rows,
            Err(_) => return vec![],
        };

        rows.filter_map(|r| r.ok()).collect()
    }

    fn find_by_name(&self, name: &str) -> Vec<FunctionRecord> {
        let mut stmt = match self.conn.prepare(
            "SELECT * FROM functions WHERE name = ?1 ORDER BY file_path, start_line"
        ) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = match stmt.query_map(params![name], Self::parse_function_row) {
            Ok(rows) => rows,
            Err(_) => return vec![],
        };

        rows.filter_map(|r| r.ok()).collect()
    }

    fn find_callers(&self, function_id: &str, depth: usize) -> Vec<FunctionRecord> {
        if depth == 0 {
            return vec![];
        }

        let sql = r#"
            WITH RECURSIVE caller_chain(caller_id, depth, path) AS (
                SELECT caller_id, 1, caller_id || ','
                FROM call_edges
                WHERE callee_id = ?1
                
                UNION
                
                SELECT e.caller_id, c.depth + 1, c.path || e.caller_id || ','
                FROM call_edges e
                JOIN caller_chain c ON e.callee_id = c.caller_id
                WHERE c.depth < ?2
                  AND c.path NOT LIKE '%' || e.caller_id || ',%'
            )
            SELECT DISTINCT f.* FROM functions f
            JOIN caller_chain c ON f.id = c.caller_id
            ORDER BY c.depth, f.file_path, f.start_line
        "#;

        let mut stmt = match self.conn.prepare(sql) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = match stmt.query_map(params![function_id, depth as i64], Self::parse_function_row) {
            Ok(rows) => rows,
            Err(_) => return vec![],
        };

        rows.filter_map(|r| r.ok()).collect()
    }

    fn find_callees(&self, function_id: &str, depth: usize) -> Vec<FunctionRecord> {
        if depth == 0 {
            return vec![];
        }

        let sql = r#"
            WITH RECURSIVE callee_chain(callee_id, depth, path) AS (
                SELECT callee_id, 1, callee_id || ','
                FROM call_edges
                WHERE caller_id = ?1
                
                UNION
                
                SELECT e.callee_id, c.depth + 1, c.path || e.callee_id || ','
                FROM call_edges e
                JOIN callee_chain c ON e.caller_id = c.callee_id
                WHERE c.depth < ?2
                  AND c.path NOT LIKE '%' || e.callee_id || ',%'
            )
            SELECT DISTINCT f.* FROM functions f
            JOIN callee_chain c ON f.id = c.callee_id
            ORDER BY c.depth, f.file_path, f.start_line
        "#;

        let mut stmt = match self.conn.prepare(sql) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = match stmt.query_map(params![function_id, depth as i64], Self::parse_function_row) {
            Ok(rows) => rows,
            Err(_) => return vec![],
        };

        rows.filter_map(|r| r.ok()).collect()
    }

    fn find_changed_by_commit(&self, commit_hash: &str) -> Vec<CommitFunctionRecord> {
        let mut stmt = match self.conn.prepare(
            r#"
            SELECT cf.*, f.* 
            FROM commit_functions cf
            JOIN functions f ON cf.function_id = f.id
            WHERE cf.commit_hash = ?1
            ORDER BY f.file_path, f.start_line
            "#
        ) {
            Ok(s) => s,
            Err(_) => return vec![],
        };

        let rows = stmt.query_map(params![commit_hash], |row| {
            Ok(CommitFunctionRecord {
                commit_hash: row.get(0)?,
                function_id: row.get(1)?,
                change_type: match row.get::<_, String>(2)?.as_str() {
                    "add" => ChangeType::Add,
                    "modify" => ChangeType::Modify,
                    "delete" => ChangeType::Delete,
                    _ => ChangeType::Modify,
                },
                before_hash: row.get(3)?,
                after_hash: row.get(4)?,
                indexed_at: row.get::<_, String>(5)?.parse().unwrap_or_else(|_| Utc::now()),
            })
        });

        match rows {
            Ok(rows) => rows.filter_map(|r| r.ok()).collect(),
            Err(_) => vec![],
        }
    }

    fn analyze_impact(&self, function_ids: &[String]) -> ImpactAnalysis {
        let mut direct_changes = vec![];
        let mut upstream_impact = vec![];
        let mut downstream_impact = vec![];

        for id in function_ids {
            if let Some(func) = self.get_function(id) {
                direct_changes.push(func);
            }

            let callers = self.find_callers(id, 3);
            for caller in callers {
                upstream_impact.push(ImpactPath {
                    function: caller.clone(),
                    path: vec![caller.id.clone(), id.clone()],
                    depth: 1,
                });
            }

            let callees = self.find_callees(id, 3);
            for callee in callees {
                downstream_impact.push(ImpactPath {
                    function: callee.clone(),
                    path: vec![id.clone(), callee.id.clone()],
                    depth: 1,
                });
            }
        }

        ImpactAnalysis {
            direct_changes,
            upstream_impact,
            downstream_impact,
        }
    }

    fn find_similar_commits(&self, _commit_hash: &str, _limit: usize) -> Vec<CommitSimilarity> {
        vec![]
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_function(id: &str, name: &str, module: &str) -> FunctionRecord {
        FunctionRecord {
            id: id.to_string(),
            name: name.to_string(),
            file_path: format!("src/{}.php", module),
            module: module.to_string(),
            language: "php".to_string(),
            start_line: 10,
            end_line: 20,
            signature: format!("function {}()", name),
            content_hash: "abc123".to_string(),
            indexed_at: Utc::now(),
        }
    }

    #[test]
    fn test_sqlite_store_creation() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        let _ = store;
    }

    #[test]
    fn test_function_crud() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        let func = create_test_function("test.php#foo#10", "foo", "test");
        store.insert_function(&func).unwrap();
        
        let found = store.get_function("test.php#foo#10");
        assert!(found.is_some());
        assert_eq!(found.unwrap().name, "foo");
    }

    #[test]
    fn test_find_by_module() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        let func1 = create_test_function("order.php#create#10", "create", "order");
        let func2 = create_test_function("order.php#update#20", "update", "order");
        let func3 = create_test_function("user.php#login#10", "login", "user");
        
        store.insert_function(&func1).unwrap();
        store.insert_function(&func2).unwrap();
        store.insert_function(&func3).unwrap();
        
        let order_funcs = store.find_by_module("order");
        assert_eq!(order_funcs.len(), 2);
        
        let user_funcs = store.find_by_module("user");
        assert_eq!(user_funcs.len(), 1);
    }
}
