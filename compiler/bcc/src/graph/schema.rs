//! 数据库 Schema 定义

/// 创建所有表的 SQL
pub const CREATE_SCHEMA_SQL: &str = r#"
-- 仓库表
CREATE TABLE IF NOT EXISTS repositories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    root_path TEXT NOT NULL,
    languages TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 语言统计表
CREATE TABLE IF NOT EXISTS language_stats (
    repo_id TEXT NOT NULL,
    language TEXT NOT NULL,
    file_count INTEGER DEFAULT 0,
    line_count INTEGER DEFAULT 0,
    PRIMARY KEY (repo_id, language),
    FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE
);

-- 函数表
CREATE TABLE IF NOT EXISTS functions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    module TEXT,
    language TEXT,
    start_line INTEGER,
    end_line INTEGER,
    signature TEXT,
    content_hash TEXT,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 函数调用关系（图边）
CREATE TABLE IF NOT EXISTS call_edges (
    caller_id TEXT NOT NULL,
    callee_id TEXT NOT NULL,
    call_type TEXT DEFAULT 'direct',
    file_path TEXT,
    line_number INTEGER,
    PRIMARY KEY (caller_id, callee_id),
    FOREIGN KEY (caller_id) REFERENCES functions(id) ON DELETE CASCADE,
    FOREIGN KEY (callee_id) REFERENCES functions(id) ON DELETE CASCADE
);

-- commit 与函数的关联
CREATE TABLE IF NOT EXISTS commit_functions (
    commit_hash TEXT NOT NULL,
    function_id TEXT NOT NULL,
    change_type TEXT NOT NULL,
    before_hash TEXT,
    after_hash TEXT,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commit_hash, function_id),
    FOREIGN KEY (function_id) REFERENCES functions(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_functions_module ON functions(module);
CREATE INDEX IF NOT EXISTS idx_functions_file ON functions(file_path);
CREATE INDEX IF NOT EXISTS idx_functions_name ON functions(name);
CREATE INDEX IF NOT EXISTS idx_call_edges_caller ON call_edges(caller_id);
CREATE INDEX IF NOT EXISTS idx_call_edges_callee ON call_edges(callee_id);
CREATE INDEX IF NOT EXISTS idx_commit_functions_hash ON commit_functions(commit_hash);
CREATE INDEX IF NOT EXISTS idx_commit_functions_func ON commit_functions(function_id);
"#;

/// 删除所有表的 SQL（用于测试）
pub const DROP_SCHEMA_SQL: &str = r#"
DROP TABLE IF EXISTS commit_functions;
DROP TABLE IF EXISTS call_edges;
DROP TABLE IF EXISTS functions;
DROP TABLE IF EXISTS language_stats;
DROP TABLE IF EXISTS repositories;
"#;
