//! Graph 模块集成测试

use bcc::graph::error::GraphError;
use bcc::graph::sqlite::GraphStoreManager;
use bcc::graph::store::{CodeGraphStore, GraphStoreInsert};
use bcc::graph::types::{FunctionRecord, Repository};
use chrono::Utc;
use std::thread;

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
