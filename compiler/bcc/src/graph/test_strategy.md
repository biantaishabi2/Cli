# BCC Graph 测试策略

## 测试分层

| 类型 | 标记 | CI | 速度 | 场景 |
|-----|------|-----|------|------|
| 单元测试 | `#[test]` | ✅ PR | <10ms | 纯内存，无 IO |
| 集成测试 | `#[test]` + in-memory SQLite | ✅ PR | <100ms | SQLite :memory: |
| BDD | `tests/graph_bdd.rs` | ✅ master | <1s | 临时文件系统 |
| E2E | `tests/graph_e2e.rs` | ❌ 手动 | >10s | 真实仓库 |

## 必须测试的场景

### Phase 1: Repository Layer

```rust
// 1. 并发安全（必须）
#[test]
fn test_concurrent_build_same_repo() {
    // 两线程同时 build 同一仓库，应排队或报错
    let store = Arc::new(Mutex::new(SqliteGraphStore::new_in_memory().unwrap()));
    // 线程1: build
    // 线程2: build（应等待或返回错误）
}

// 2. 多仓库隔离
#[test]
fn test_multi_repo_isolation() {
    // repo1 和 repo2 的函数 ID 相同，但数据不冲突
}
```

### Phase 2: Class + Inheritance

```rust
// 3. 循环继承检测（必须）
#[test]
fn test_circular_inheritance() {
    // class A extends B, B extends A
    // 应检测并返回错误，防死循环
    let result = indexer.build_from_snapshot(&circular_snapshot, "commit1");
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("circular inheritance"));
}

// 4. 多层继承查询
#[test]
fn test_inheritance_hierarchy() {
    // A extends B extends C
    // find_class_hierarchy("A") 应返回 [B, C]
}
```

### Phase 3: Search Graph

```rust
// 5. 搜索深度限制（必须）
#[test]
fn test_search_depth_limit() {
    // 递归调用链 depth=100
    // 应截断在 MAX_DEPTH(10)，防栈溢出
    let result = store.find_callers("deep_func", 100);
    assert!(result.len() <= 100); // 截断保护
}

// 6. 多图融合查询
#[test]
fn test_multi_graph_search() {
    // 同时搜索：调用者 + 同文件 + 同模块
}
```

### Phase 4: Arch Validation

```rust
// 7. 分层违规检测
#[test]
fn test_layer_violation() {
    // api → dao（跳过 service）应检测为违规
}

// 8. 领域依赖违规
#[test]
fn test_domain_violation() {
    // core → support（反向依赖）应检测为违规
}
```
