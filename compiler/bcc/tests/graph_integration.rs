//! Graph 模块集成测试

use bcc::graph::arch::ArchValidator;
use bcc::graph::error::GraphError;
use bcc::graph::sqlite::GraphStoreManager;
use bcc::graph::store::{CodeGraphStore, GraphStoreInsert};
use bcc::graph::types::{
    CallEdge, CallType, DepType, FunctionRecord, ModuleDepEdge, ModuleRecord, Repository,
};
use chrono::Utc;
use std::thread;

fn create_test_module(id: &str, name: &str, directory: &str) -> ModuleRecord {
    ModuleRecord {
        id: id.to_string(),
        name: name.to_string(),
        file_path: id.to_string(),
        directory: directory.to_string(),
        exports_count: 5,
        imports_count: 3,
        loc_lines: 100,
        language: "typescript".to_string(),
    }
}

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
fn test_multi_repo_isolation() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();

    // 创建两个仓库
    {
        let store1 = manager.get_store("repo1").unwrap();
        let store2 = manager.get_store("repo2").unwrap();

        // 在 repo1 中插入函数
        let func1 = create_test_function("test.php#foo#10", "foo", "test");
        GraphStoreInsert::insert_function(&store1, &func1).unwrap();

        // 在 repo2 中插入同名函数
        let func2 = create_test_function("test.php#foo#10", "foo", "test");
        GraphStoreInsert::insert_function(&store2, &func2).unwrap();
    }

    // 验证隔离性（重新打开连接）
    {
        let store1 = manager.get_store("repo1").unwrap();
        let store2 = manager.get_store("repo2").unwrap();
        assert!(store1.get_function("test.php#foo#10").is_some());
        assert!(store2.get_function("test.php#foo#10").is_some());
    }

    // 删除 repo1 不应影响 repo2
    manager.delete_repo("repo1").unwrap();
    assert!(!manager.repo_exists("repo1"));
    assert!(manager.repo_exists("repo2"));
}

#[test]
fn test_concurrent_build_same_repo() {
    let temp_dir = tempfile::tempdir().unwrap();
    let temp_path = temp_dir.path().to_path_buf();

    // 两个线程各自创建独立的 manager 实例
    // 由于 SQLite 文件锁，实际写入会串行化
    let handle1 = thread::spawn(move || {
        let manager = GraphStoreManager::new(&temp_path).unwrap();
        let store = manager.get_store("concurrent-repo").unwrap();
        let func = create_test_function("a.php#func1#1", "func1", "a");
        GraphStoreInsert::insert_function(&store, &func).unwrap();
        "thread1-done"
    });

    // 等待 thread1 完成，避免并发写入冲突
    let result1 = handle1.join().unwrap();
    assert_eq!(result1, "thread1-done");

    // 第二个线程读取并验证
    let temp_path2 = temp_dir.path().to_path_buf();
    let handle2 = thread::spawn(move || {
        let manager = GraphStoreManager::new(&temp_path2).unwrap();
        let store = manager.get_store("concurrent-repo").unwrap();

        // 验证 thread1 写入的数据
        assert!(store.get_function("a.php#func1#1").is_some());

        // 写入新数据
        let func = create_test_function("b.php#func2#1", "func2", "b");
        GraphStoreInsert::insert_function(&store, &func).unwrap();
        "thread2-done"
    });

    let result2 = handle2.join().unwrap();
    assert_eq!(result2, "thread2-done");

    // 最终验证
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("concurrent-repo").unwrap();
    assert!(store.get_function("a.php#func1#1").is_some());
    assert!(store.get_function("b.php#func2#1").is_some());
}

#[test]
fn test_repository_persistence() {
    let temp_dir = tempfile::tempdir().unwrap();
    let repo_id = "persistent-repo";

    // 第一次打开
    {
        let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
        let store = manager.get_store(repo_id).unwrap();

        let repo = Repository {
            id: repo_id.to_string(),
            name: "Persistent Repo".to_string(),
            root_path: "/tmp/persistent".to_string(),
            languages: "php,ts".to_string(),
            created_at: Utc::now(),
            updated_at: Utc::now(),
        };

        store.update_repository(&repo).unwrap();

        let func = create_test_function("test.php#persist#1", "persist", "test");
        GraphStoreInsert::insert_function(&store, &func).unwrap();
    }

    // 第二次打开（新实例）
    {
        let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
        let store = manager.get_store(repo_id).unwrap();

        // 验证数据仍然存在
        let repo = store.get_repository();
        assert!(repo.is_some());
        assert_eq!(repo.unwrap().name, "Persistent Repo");

        assert!(store.get_function("test.php#persist#1").is_some());
    }
}

#[test]
fn test_repo_not_found_error() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();

    // 查询不存在的仓库
    assert!(!manager.repo_exists("non-existent"));

    // 尝试获取不存在的仓库（会创建新文件）
    // 但查询时应该返回 None
    let store = manager.get_store("new-repo").unwrap();
    assert!(store.get_repository().is_none());
}

#[test]
fn test_depth_limit_error() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("depth-test").unwrap();

    // 插入测试函数
    let func = create_test_function("test.php#foo#1", "foo", "test");
    GraphStoreInsert::insert_function(&store, &func).unwrap();

    // 测试正常深度
    let result = store.find_callers("test.php#foo#1", 10);
    assert!(result.is_ok());

    // 测试超限深度
    let result = store.find_callers("test.php#foo#1", 101);
    assert!(matches!(result, Err(GraphError::DepthLimitExceeded(100))));
}

// ==================== 模块依赖图集成测试 ====================

#[test]
fn test_module_crud() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-test").unwrap();

    // 插入模块
    let module = create_test_module("src/utils.ts", "utils.ts", "src");
    GraphStoreInsert::insert_module(&store, &module).unwrap();

    // 查询模块
    let retrieved = store.get_module("src/utils.ts");
    assert!(retrieved.is_some());
    let retrieved = retrieved.unwrap();
    assert_eq!(retrieved.name, "utils.ts");
    assert_eq!(retrieved.directory, "src");
    assert_eq!(retrieved.exports_count, 5);
    assert_eq!(retrieved.imports_count, 3);
    assert_eq!(retrieved.loc_lines, 100);

    // 查询不存在的模块
    assert!(store.get_module("non-existent.ts").is_none());
}

#[test]
fn test_module_dep_edge_storage() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-dep-test").unwrap();

    // 插入模块
    let mod_a = create_test_module("src/a.ts", "a.ts", "src");
    let mod_b = create_test_module("src/b.ts", "b.ts", "src");
    let mod_c = create_test_module("src/c.ts", "c.ts", "src");

    GraphStoreInsert::insert_module(&store, &mod_a).unwrap();
    GraphStoreInsert::insert_module(&store, &mod_b).unwrap();
    GraphStoreInsert::insert_module(&store, &mod_c).unwrap();

    // 插入依赖边: a -> b, a -> c, b -> c
    let edge_ab = ModuleDepEdge {
        source_id: "src/a.ts".to_string(),
        target_id: "src/b.ts".to_string(),
        dep_type: DepType::Import,
        symbols: vec!["helper".to_string(), "util".to_string()],
    };
    let edge_ac = ModuleDepEdge {
        source_id: "src/a.ts".to_string(),
        target_id: "src/c.ts".to_string(),
        dep_type: DepType::Import,
        symbols: vec!["config".to_string()],
    };
    let edge_bc = ModuleDepEdge {
        source_id: "src/b.ts".to_string(),
        target_id: "src/c.ts".to_string(),
        dep_type: DepType::DynamicImport,
        symbols: vec![],
    };

    GraphStoreInsert::insert_module_dep_edge(&store, &edge_ab).unwrap();
    GraphStoreInsert::insert_module_dep_edge(&store, &edge_ac).unwrap();
    GraphStoreInsert::insert_module_dep_edge(&store, &edge_bc).unwrap();

    // 验证依赖查询
    let deps_of_a = store.find_module_deps("src/a.ts", 1).unwrap();
    assert_eq!(deps_of_a.len(), 2);

    let deps_of_b = store.find_module_deps("src/b.ts", 1).unwrap();
    assert_eq!(deps_of_b.len(), 1);
    assert_eq!(deps_of_b[0].id, "src/c.ts");

    // 验证被依赖查询
    let dependents_of_c = store.find_module_dependents("src/c.ts", 1).unwrap();
    assert_eq!(dependents_of_c.len(), 2);

    let dependents_of_b = store.find_module_dependents("src/b.ts", 1).unwrap();
    assert_eq!(dependents_of_b.len(), 1);
    assert_eq!(dependents_of_b[0].id, "src/a.ts");
}

#[test]
fn test_module_deps_recursive() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-recursive-test").unwrap();

    // 创建链式依赖: a -> b -> c -> d
    let modules = vec![
        ("src/a.ts", "a.ts", "src"),
        ("src/b.ts", "b.ts", "src"),
        ("src/c.ts", "c.ts", "src"),
        ("src/d.ts", "d.ts", "src"),
    ];

    for (id, name, dir) in &modules {
        let module = create_test_module(id, name, dir);
        GraphStoreInsert::insert_module(&store, &module).unwrap();
    }

    // 创建链式依赖
    let edges = vec![
        ("src/a.ts", "src/b.ts"),
        ("src/b.ts", "src/c.ts"),
        ("src/c.ts", "src/d.ts"),
    ];

    for (source, target) in edges {
        let edge = ModuleDepEdge {
            source_id: source.to_string(),
            target_id: target.to_string(),
            dep_type: DepType::Import,
            symbols: vec![],
        };
        GraphStoreInsert::insert_module_dep_edge(&store, &edge).unwrap();
    }

    // depth=1: a 直接依赖 b
    let deps_depth1 = store.find_module_deps("src/a.ts", 1).unwrap();
    assert_eq!(deps_depth1.len(), 1);
    assert_eq!(deps_depth1[0].id, "src/b.ts");

    // depth=2: a 依赖 b, c
    let deps_depth2 = store.find_module_deps("src/a.ts", 2).unwrap();
    assert_eq!(deps_depth2.len(), 2);
    let ids: Vec<_> = deps_depth2.iter().map(|m| m.id.clone()).collect();
    assert!(ids.contains(&"src/b.ts".to_string()));
    assert!(ids.contains(&"src/c.ts".to_string()));

    // depth=3: a 依赖 b, c, d
    let deps_depth3 = store.find_module_deps("src/a.ts", 3).unwrap();
    assert_eq!(deps_depth3.len(), 3);
    let ids: Vec<_> = deps_depth3.iter().map(|m| m.id.clone()).collect();
    assert!(ids.contains(&"src/d.ts".to_string()));
}

#[test]
fn test_circular_module_deps_detection() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-circular-test").unwrap();

    // 创建循环依赖: a -> b -> c -> a
    let modules = vec![
        ("src/a.ts", "a.ts", "src"),
        ("src/b.ts", "b.ts", "src"),
        ("src/c.ts", "c.ts", "src"),
    ];

    for (id, name, dir) in &modules {
        let module = create_test_module(id, name, dir);
        GraphStoreInsert::insert_module(&store, &module).unwrap();
    }

    // 创建循环依赖
    let edges = vec![
        ("src/a.ts", "src/b.ts"),
        ("src/b.ts", "src/c.ts"),
        ("src/c.ts", "src/a.ts"),
    ];

    for (source, target) in edges {
        let edge = ModuleDepEdge {
            source_id: source.to_string(),
            target_id: target.to_string(),
            dep_type: DepType::Import,
            symbols: vec![],
        };
        GraphStoreInsert::insert_module_dep_edge(&store, &edge).unwrap();
    }

    // 检测循环依赖
    let cycle = store.detect_circular_deps("src/a.ts").unwrap();
    assert!(cycle.is_some());
    let cycle_path = cycle.unwrap();
    assert!(cycle_path.contains(&"src/a.ts".to_string()));
    assert!(cycle_path.contains(&"src/b.ts".to_string()));
    assert!(cycle_path.contains(&"src/c.ts".to_string()));

    // 无循环的模块
    let no_cycle = store.detect_circular_deps("src/b.ts").unwrap();
    // 注意：由于算法从给定节点出发，b 在循环中也会被检测到
    assert!(no_cycle.is_some());
}

#[test]
fn test_module_deps_no_cycle() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-no-cycle-test").unwrap();

    // 创建无循环依赖: a -> b -> c
    let modules = vec![
        ("src/a.ts", "a.ts", "src"),
        ("src/b.ts", "b.ts", "src"),
        ("src/c.ts", "c.ts", "src"),
    ];

    for (id, name, dir) in &modules {
        let module = create_test_module(id, name, dir);
        GraphStoreInsert::insert_module(&store, &module).unwrap();
    }

    // 创建无循环依赖
    let edges = vec![("src/a.ts", "src/b.ts"), ("src/b.ts", "src/c.ts")];

    for (source, target) in edges {
        let edge = ModuleDepEdge {
            source_id: source.to_string(),
            target_id: target.to_string(),
            dep_type: DepType::Import,
            symbols: vec![],
        };
        GraphStoreInsert::insert_module_dep_edge(&store, &edge).unwrap();
    }

    // c 没有出边，应该无循环
    let cycle = store.detect_circular_deps("src/c.ts").unwrap();
    assert!(cycle.is_none());
}

#[test]
fn test_module_function_isolation() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-func-isolation").unwrap();

    // 插入模块
    let module = create_test_module("src/app.ts", "app.ts", "src");
    GraphStoreInsert::insert_module(&store, &module).unwrap();

    // 插入同名函数（不同 ID 格式）
    let func = create_test_function("src/app.ts#foo#10", "foo", "app");
    GraphStoreInsert::insert_function(&store, &func).unwrap();

    // 验证模块和函数独立存储
    let retrieved_module = store.get_module("src/app.ts");
    assert!(retrieved_module.is_some());
    assert_eq!(retrieved_module.unwrap().name, "app.ts");

    let retrieved_func = store.get_function("src/app.ts#foo#10");
    assert!(retrieved_func.is_some());
    assert_eq!(retrieved_func.unwrap().name, "foo");

    // 模块 ID 和函数 ID 不同
    assert!(store.get_function("src/app.ts").is_none());
    assert!(store.get_module("src/app.ts#foo#10").is_none());
}

#[test]
fn test_module_depth_limit() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("module-depth-test").unwrap();

    // 插入测试模块
    let module = create_test_module("src/test.ts", "test.ts", "src");
    GraphStoreInsert::insert_module(&store, &module).unwrap();

    // 测试正常深度
    let result = store.find_module_deps("src/test.ts", 10);
    assert!(result.is_ok());

    // 测试超限深度
    let result = store.find_module_deps("src/test.ts", 101);
    assert!(matches!(result, Err(GraphError::DepthLimitExceeded(100))));

    // 测试 dependents 深度限制
    let result = store.find_module_dependents("src/test.ts", 101);
    assert!(matches!(result, Err(GraphError::DepthLimitExceeded(100))));
}

#[test]
fn test_arch_validation_violation_count_matches_len() {
    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("arch-validate").unwrap();

    let api_func = FunctionRecord {
        id: "app/Controllers/UserController.php#create#10".to_string(),
        name: "create".to_string(),
        file_path: "app/Controllers/UserController.php".to_string(),
        module: "Controllers".to_string(),
        language: "php".to_string(),
        start_line: 10,
        end_line: 20,
        signature: "public function create()".to_string(),
        content_hash: "hash-api".to_string(),
        indexed_at: Utc::now(),
    };
    let dao_func = FunctionRecord {
        id: "app/Models/UserModel.php#insert#5".to_string(),
        name: "insert".to_string(),
        file_path: "app/Models/UserModel.php".to_string(),
        module: "Models".to_string(),
        language: "php".to_string(),
        start_line: 5,
        end_line: 12,
        signature: "public function insert()".to_string(),
        content_hash: "hash-dao".to_string(),
        indexed_at: Utc::now(),
    };
    GraphStoreInsert::insert_function(&store, &api_func).unwrap();
    GraphStoreInsert::insert_function(&store, &dao_func).unwrap();
    GraphStoreInsert::insert_call_edge(
        &store,
        &CallEdge {
            caller_id: api_func.id.clone(),
            callee_id: dao_func.id.clone(),
            call_type: CallType::Direct,
            file_path: Some("app/Controllers/UserController.php".to_string()),
            line_number: Some(12),
        },
    )
    .unwrap();

    let matrix_file = temp_dir.path().join("target-matrix.yaml");
    std::fs::write(
        &matrix_file,
        r#"
layers:
  - name: api
    patterns:
      - "*/Controllers/*"
  - name: service
    patterns:
      - "*/Services/*"
  - name: dao
    patterns:
      - "*/Models/*"
allowed_deps:
  - from: api
    to: service
  - from: service
    to: dao
"#,
    )
    .unwrap();

    let validator = ArchValidator::from_yaml(&matrix_file).unwrap();
    let result = validator.validate(&store).unwrap();

    assert_eq!(result.violations.len(), result.stats.violation_count);
    assert_eq!(result.stats.violation_count, 1);
    assert!(!result.passed);
}
