# BCC 代码知识图谱索引设计方案

> Issue: #38  
> 状态: 设计阶段  
> 更新日期: 2026-02-16

## 1. 背景与目标

### 1.1 当前问题

BCC 的 bugfix 分析存在以下限制：

1. **函数信息临时提取**：`context` 阶段解析函数体但不持久化，每次重新解析
2. **缺乏代码关系**：无法查询"谁调用了这个函数"、"改动影响了哪些上游"
3. **Commit 孤立**：无法找到"历史上类似位置的 bugfix"
4. **无检索能力**：用户必须手动指定 commit，不能语义查询

### 1.2 目标

建立代码知识图谱索引，支持：
- 函数级别的代码关系持久化
- 跨 commit 的关联查询
- 影响分析和相似 bugfix 发现

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     BCC Code Graph Index                    │
├─────────────────────────────────────────────────────────────┤
│  Layer 3: Query Interface (Rust trait)                      │
│  ├── CodeGraphStore                                         │
│  ├── ImpactAnalyzer                                         │
│  └── SimilaritySearcher                                     │
├─────────────────────────────────────────────────────────────┤
│  Layer 2: Storage Layer                                     │
│  ├── SQLite (起步)                                          │
│  └── PostgreSQL + AGE (未来扩展)                            │
├─────────────────────────────────────────────────────────────┤
│  Layer 1: Data Model                                        │
│  ├── Entity: Function, Commit, Module, File                 │
│  └── Relation: Calls, Changes, Depends                      │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 与现有模块集成

```
bcc extract ──┬──► AstSnapshot ──┬──► index.db (新增)
              │                  │
              │                  └──► raw/contexts/ (现有)
              │
bcc arch ─────┘──► module deps ──┘
```

**关键设计**：复用 `extract` 模块的 `AstSnapshot`，在其基础上增加调用关系提取。

## 3. 数据模型

### 3.1 实体定义

#### Function（函数）

```rust
pub struct FunctionRecord {
    /// 唯一标识: file_path#function_name#start_line
    pub id: String,
    pub name: String,
    pub file_path: String,
    pub module: String,
    pub language: String,
    pub start_line: usize,
    pub end_line: usize,
    pub signature: String,
    /// 代码内容的 SHA256，用于检测变更
    pub content_hash: String,
    /// 索引时间戳
    pub indexed_at: DateTime<Utc>,
}
```

#### CommitFunction（Commit 与函数的关联）

```rust
pub struct CommitFunctionRecord {
    pub commit_hash: String,
    pub function_id: String,
    /// 变更类型: add/modify/delete
    pub change_type: ChangeType,
    /// 变更前内容 hash
    pub before_hash: Option<String>,
    /// 变更后内容 hash
    pub after_hash: Option<String>,
    pub indexed_at: DateTime<Utc>,
}
```

### 3.2 关系定义

#### CallEdge（函数调用关系）

```rust
pub struct CallEdge {
    pub caller_id: String,
    pub callee_id: String,
    /// 调用类型: direct/indirect/virtual
    pub call_type: CallType,
    /// 调用位置（可选）
    pub call_site: Option<CallSite>,
}

pub struct CallSite {
    pub file_path: String,
    pub line_number: usize,
    pub column: usize,
}
```

### 3.3 数据库 Schema

```sql
-- 函数表
CREATE TABLE functions (
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
CREATE TABLE call_edges (
    caller_id TEXT NOT NULL,
    callee_id TEXT NOT NULL,
    call_type TEXT DEFAULT 'direct',
    file_path TEXT,
    line_number INTEGER,
    PRIMARY KEY (caller_id, callee_id),
    FOREIGN KEY (caller_id) REFERENCES functions(id),
    FOREIGN KEY (callee_id) REFERENCES functions(id)
);

-- commit 与函数的关联
CREATE TABLE commit_functions (
    commit_hash TEXT NOT NULL,
    function_id TEXT NOT NULL,
    change_type TEXT NOT NULL,  -- add/modify/delete
    before_hash TEXT,
    after_hash TEXT,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commit_hash, function_id),
    FOREIGN KEY (function_id) REFERENCES functions(id)
);

-- 索引
CREATE INDEX idx_functions_module ON functions(module);
CREATE INDEX idx_functions_file ON functions(file_path);
CREATE INDEX idx_functions_name ON functions(name);
CREATE INDEX idx_call_edges_caller ON call_edges(caller_id);
CREATE INDEX idx_call_edges_callee ON call_edges(callee_id);
CREATE INDEX idx_commit_functions_hash ON commit_functions(commit_hash);
CREATE INDEX idx_commit_functions_func ON commit_functions(function_id);
```

## 4. 存储层接口

### 4.1 Core Trait

```rust
/// 代码图谱存储接口
pub trait CodeGraphStore {
    /// 索引管理
    fn index_from_snapshot(&self, snapshot: &AstSnapshot, commit_hash: &str) -> Result<()>;
    fn index_incremental(&self, commits: &[Commit]) -> Result<()>;
    fn mark_stale(&self, file_paths: &[String]) -> Result<()>;
    
    /// 函数查询
    fn get_function(&self, id: &str) -> Option<FunctionRecord>;
    fn find_by_module(&self, module: &str) -> Vec<FunctionRecord>;
    fn find_by_file(&self, file_path: &str) -> Vec<FunctionRecord>;
    
    /// 关系查询
    fn find_callers(&self, function_id: &str, depth: usize) -> Vec<FunctionRecord>;
    fn find_callees(&self, function_id: &str, depth: usize) -> Vec<FunctionRecord>;
    fn find_changed_by_commit(&self, commit_hash: &str) -> Vec<CommitFunctionRecord>;
    
    /// 高级分析
    fn analyze_impact(&self, function_ids: &[String]) -> ImpactAnalysis;
    fn find_similar_commits(&self, commit_hash: &str, limit: usize) -> Vec<CommitSimilarity>;
}

/// 影响分析结果
pub struct ImpactAnalysis {
    /// 直接改动的函数
    pub direct_changes: Vec<FunctionRecord>,
    /// 上游调用者（被影响）
    pub upstream_impact: Vec<ImpactPath>,
    /// 下游被调用者（依赖）
    pub downstream_impact: Vec<ImpactPath>,
}

pub struct ImpactPath {
    pub function: FunctionRecord,
    pub path: Vec<String>,  // 调用链
    pub depth: usize,
}
```

### 4.2 SQLite 实现

```rust
pub struct SqliteGraphStore {
    conn: Connection,
}

impl SqliteGraphStore {
    pub fn new(db_path: &Path) -> Result<Self>;
    
    /// 递归查询调用者（使用 CTE）
    fn find_callers_recursive(&self, function_id: &str, depth: usize) -> Result<Vec<FunctionRecord>> {
        let sql = r#"
            WITH RECURSIVE callers AS (
                -- 直接调用者
                SELECT caller_id, 1 as depth, ARRAY[caller_id] as path
                FROM call_edges
                WHERE callee_id = ?1
                
                UNION ALL
                
                -- 递归
                SELECT e.caller_id, c.depth + 1, c.path || e.caller_id
                FROM call_edges e
                JOIN callers c ON e.callee_id = c.caller_id
                WHERE c.depth < ?2
                  AND NOT e.caller_id = ANY(c.path)  -- 防止循环
            )
            SELECT DISTINCT caller_id FROM callers
        "#;
        // ...
    }
}
```

## 5. 与 Extract 模块集成

### 5.1 扩展现有 AstSnapshot

```rust
// 在 compiler/bcc/src/extract/mod.rs 中扩展

#[derive(Debug, Serialize, Deserialize)]
pub struct FunctionInfo {
    pub name: String,
    pub line: usize,
    pub signature: String,
    pub calls: Vec<String>,  // 新增：调用的函数名列表
}

#[derive(Debug, Serialize, Deserialize)]
pub struct FileRecord {
    pub file_path: String,
    pub language: String,
    pub exports: Vec<FunctionInfo>,
    pub imports: Vec<ImportInfo>,
}
```

### 5.2 调用关系提取

```rust
/// 在现有提取器基础上，增加调用关系提取
pub fn enrich_with_calls(file_record: &mut FileRecord, content: &str, lang: &str) {
    // 使用 tree-sitter 查询调用表达式
    // 匹配: function_call(arg1, arg2)
    // 提取被调用的函数名，添加到 FunctionInfo.calls
}
```

### 5.3 索引构建流程

```rust
pub fn build_index_from_extract_output(
    store: &dyn CodeGraphStore,
    extract_output: &Path,
    commit_hash: &str,
) -> Result<()> {
    // 1. 读取 AstSnapshot
    let snapshot: AstSnapshot = read_json(extract_output)?;
    
    // 2. 提取调用关系
    let enriched = enrich_snapshot_with_calls(snapshot)?;
    
    // 3. 写入索引
    store.index_from_snapshot(&enriched, commit_hash)?;
    
    Ok(())
}
```

## 6. PoC 计划

### 6.1 目标

验证核心流程：SQLite 表结构 → 复用 extract 输出 → 写入索引 → 基础查询

### 6.2 范围（1 周）

| 天数 | 任务 | 产出 |
|-----|------|------|
| 1-2 | SQLite Schema + 基础 CRUD | `src/graph/schema.rs`, `src/graph/store.rs` |
| 3 | 复用 extract 输出 | `src/graph/indexer.rs` |
| 4 | 基础查询接口 | `find_callers`, `find_callees`, `analyze_impact` |
| 5 | 集成测试 | `tests/graph_integration_test.rs` |

### 6.3 验收标准

```rust
#[test]
fn test_poc() {
    // 1. 能建索引
    let store = SqliteGraphStore::new(":memory:").unwrap();
    store.index_from_snapshot(&snapshot, "abc123").unwrap();
    
    // 2. 能查询函数
    let func = store.get_function("order.php#create_order#42");
    assert!(func.is_some());
    
    // 3. 能查调用关系
    let callers = store.find_callers("order.php#create_order#42", 2);
    assert!(!callers.is_empty());
    
    // 4. 能分析影响
    let impact = store.analyze_impact(&["order.php#create_order#42"]);
    assert!(!impact.upstream_impact.is_empty());
}
```

### 6.4 测试项目

使用 `compiler/bcc/examples/openclaw-arch` 作为测试数据：
- 代码量：适中（约 100 个函数）
- 有 arch 模块输出，可验证模块映射
- 有实际 bugfix 历史

## 7. 未来扩展

### 7.1 短期（PoC 后）

- [ ] 增量更新（只索引新 commit）
- [ ] 相似度计算（基于代码变更模式）
- [ ] CLI 命令：`bcc graph-index build`, `bcc graph-index query`

### 7.2 中期（3-4 周）

- [ ] 集成到 bugfix 流水线（自动索引）
- [ ] 影响分析命令：`bcc analyze-impact <commit>`
- [ ] 相似 bugfix 发现：`bcc find-related <commit>`

### 7.3 长期（按需）

- [ ] PostgreSQL + AGE 迁移
- [ ] 语义向量索引（结合 embedding）
- [ ] 实时索引更新（监听 git hooks）

## 8. 风险与应对

| 风险 | 影响 | 应对 |
|-----|------|------|
| 调用关系提取不准确 | 中 | 先用简单启发式，逐步优化；提供置信度分数 |
| 数据库性能瓶颈 | 低 | PoC 验证；SQLite 支持 10万+ 函数无压力 |
| 与现有流程冲突 | 低 | 增量开发，不破坏现有 bugfix 流水线 |
| 多语言差异 | 中 | 复用现有 extract 模块，语言特定逻辑已封装 |

## 9. 参考

- [FastCode Graph Builder](https://github.com/HKUDS/FastCode/blob/main/fastcode/graph_builder.py)
- [nanobot Tool Registry](https://github.com/HKUDS/nanobot/blob/main/nanobot/agent/tools/registry.py)
- [SQLite Recursive CTE](https://www.sqlite.org/lang_with.html)
- [Apache AGE](https://age.apache.org/)

---

## 附录：文件结构

```
compiler/bcc/src/
├── graph/
│   ├── mod.rs           # 模块入口
│   ├── schema.rs        # 数据库 Schema
│   ├── store.rs         # CodeGraphStore trait
│   ├── sqlite.rs        # SQLite 实现
│   ├── indexer.rs       # 从 extract 输出构建索引
│   ├── query.rs         # 查询接口
│   └── impact.rs        # 影响分析
├── extract/
│   └── mod.rs           # 扩展：增加 calls 字段
└── ...

docs/
└── code-graph-index.md  # 本文件
```
