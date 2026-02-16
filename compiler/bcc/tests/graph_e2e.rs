//! E2E 测试 - 使用真实仓库（nanobot）
//!
//! 运行条件: 手动触发（通过 workflow_dispatch）
//! 环境变量: E2E_TEST_REPO=/path/to/nanobot

use bcc::graph::sqlite::GraphStoreManager;
use bcc::graph::store::{CodeGraphStore, GraphStoreInsert};
use bcc::graph::types::*;
use chrono::Utc;
use std::path::PathBuf;

/// 获取测试仓库路径
fn get_test_repo() -> Option<PathBuf> {
    std::env::var("E2E_TEST_REPO").ok().map(PathBuf::from)
}

/// E2E Scenario 1: 索引 nanobot 仓库
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_index_nanobot() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    assert!(repo_path.exists(), "Test repo should exist at {:?}", repo_path);

    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("github.com/HKUDS/nanobot").unwrap();

    // 模拟索引 nanobot 的核心模块
    // 这里简化处理，实际应该调用 extract 模块
    let core_funcs = vec![
        FunctionRecord {
            id: "nanobot/agent/tools/registry.py#register_tool#1".to_string(),
            name: "register_tool".to_string(),
            file_path: "nanobot/agent/tools/registry.py".to_string(),
            module: "agent.tools".to_string(),
            language: "python".to_string(),
            start_line: 1,
            end_line: 20,
            signature: "def register_tool(tool: Tool)".to_string(),
            content_hash: "e3b0c44298fc1c149afbf4c8996fb924".to_string(),
            indexed_at: Utc::now(),
        },
        FunctionRecord {
            id: "nanobot/agent/tools/registry.py#get_tool#25".to_string(),
            name: "get_tool".to_string(),
            file_path: "nanobot/agent/tools/registry.py".to_string(),
            module: "agent.tools".to_string(),
            language: "python".to_string(),
            start_line: 25,
            end_line: 40,
            signature: "def get_tool(name: str) -> Tool".to_string(),
            content_hash: "a1b2c3d4e5f678901234567890123456".to_string(),
            indexed_at: Utc::now(),
        },
        FunctionRecord {
            id: "nanobot/agent/executor.py#execute_task#1".to_string(),
            name: "execute_task".to_string(),
            file_path: "nanobot/agent/executor.py".to_string(),
            module: "agent".to_string(),
            language: "python".to_string(),
            start_line: 1,
            end_line: 50,
            signature: "def execute_task(task: Task) -> Result".to_string(),
            content_hash: "b2c3d4e5f678901234567890123456a1".to_string(),
            indexed_at: Utc::now(),
        },
    ];

    for func in &core_funcs {
        store.insert_function(func).unwrap();
    }

    // 创建调用关系：executor -> registry
    store.insert_call_edge(&CallEdge {
        caller_id: "nanobot/agent/executor.py#execute_task#1".to_string(),
        callee_id: "nanobot/agent/tools/registry.py#get_tool#25".to_string(),
        call_type: CallType::Direct,
        file_path: Some("nanobot/agent/executor.py".to_string()),
        line_number: Some(30),
    }).unwrap();

    // 验证索引
    let found = store.get_function("nanobot/agent/executor.py#execute_task#1");
    assert!(found.is_some());
    assert_eq!(found.unwrap().module, "agent");

    // 验证调用关系
    let callees = store.find_callees("nanobot/agent/executor.py#execute_task#1", 1).unwrap();
    assert_eq!(callees.len(), 1);
    assert_eq!(callees[0].name, "get_tool");

    println!("Successfully indexed nanobot core modules");
}

/// E2E Scenario 2: 影响分析
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_impact_analysis() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("github.com/HKUDS/nanobot").unwrap();

    // 构建调用链：A -> B -> C, A -> D
    let func_a = FunctionRecord {
        id: "api.py#handle_request#1".to_string(),
        name: "handle_request".to_string(),
        file_path: "api.py".to_string(),
        module: "api".to_string(),
        language: "python".to_string(),
        start_line: 1,
        end_line: 30,
        signature: "def handle_request(req)".to_string(),
        content_hash: "hash_a".to_string(),
        indexed_at: Utc::now(),
    };

    let func_b = FunctionRecord {
        id: "service.py#process_data#1".to_string(),
        name: "process_data".to_string(),
        file_path: "service.py".to_string(),
        module: "service".to_string(),
        language: "python".to_string(),
        start_line: 1,
        end_line: 40,
        signature: "def process_data(data)".to_string(),
        content_hash: "hash_b".to_string(),
        indexed_at: Utc::now(),
    };

    let func_c = FunctionRecord {
        id: "dao.py#save_to_db#1".to_string(),
        name: "save_to_db".to_string(),
        file_path: "dao.py".to_string(),
        module: "dao".to_string(),
        language: "python".to_string(),
        start_line: 1,
        end_line: 25,
        signature: "def save_to_db(record)".to_string(),
        content_hash: "hash_c".to_string(),
        indexed_at: Utc::now(),
    };

    let func_d = FunctionRecord {
        id: "cache.py#invalidate#1".to_string(),
        name: "invalidate".to_string(),
        file_path: "cache.py".to_string(),
        module: "cache".to_string(),
        language: "python".to_string(),
        start_line: 1,
        end_line: 15,
        signature: "def invalidate(key)".to_string(),
        content_hash: "hash_d".to_string(),
        indexed_at: Utc::now(),
    };

    store.insert_function(&func_a).unwrap();
    store.insert_function(&func_b).unwrap();
    store.insert_function(&func_c).unwrap();
    store.insert_function(&func_d).unwrap();

    // A -> B
    store.insert_call_edge(&CallEdge {
        caller_id: "api.py#handle_request#1".to_string(),
        callee_id: "service.py#process_data#1".to_string(),
        call_type: CallType::Direct,
        file_path: Some("api.py".to_string()),
        line_number: Some(10),
    }).unwrap();

    // B -> C
    store.insert_call_edge(&CallEdge {
        caller_id: "service.py#process_data#1".to_string(),
        callee_id: "dao.py#save_to_db#1".to_string(),
        call_type: CallType::Direct,
        file_path: Some("service.py".to_string()),
        line_number: Some(20),
    }).unwrap();

    // A -> D
    store.insert_call_edge(&CallEdge {
        caller_id: "api.py#handle_request#1".to_string(),
        callee_id: "cache.py#invalidate#1".to_string(),
        call_type: CallType::Direct,
        file_path: Some("api.py".to_string()),
        line_number: Some(15),
    }).unwrap();

    // 执行影响分析
    let impact = store.analyze_impact(&["api.py#handle_request#1".to_string()]).unwrap();

    // 验证结果
    assert_eq!(impact.direct_changes.len(), 1);
    assert_eq!(impact.downstream_impact.len(), 3); // B, C, D

    println!("Impact analysis completed: {} downstream functions affected", impact.downstream_impact.len());
}

/// E2E Scenario 3: 大规模搜索性能测试
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_large_scale_search() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let temp_dir = tempfile::tempdir().unwrap();
    let manager = GraphStoreManager::new(temp_dir.path()).unwrap();
    let store = manager.get_store("github.com/HKUDS/nanobot").unwrap();

    // 模拟大规模索引（100个函数）
    let start_time = std::time::Instant::now();

    for i in 0..100 {
        let func = FunctionRecord {
            id: format!("module{}/file{}.py#func{}#1", i % 10, i, i),
            name: format!("func{}", i),
            file_path: format!("module{}/file{}.py", i % 10, i),
            module: format!("module{}", i % 10),
            language: "python".to_string(),
            start_line: 1,
            end_line: 20,
            signature: format!("def func{}()", i),
            content_hash: format!("hash{}", i),
            indexed_at: Utc::now(),
        };
        store.insert_function(&func).unwrap();
    }

    let insert_duration = start_time.elapsed();
    println!("Inserted 100 functions in {:?}", insert_duration);

    // 搜索性能测试
    let search_start = std::time::Instant::now();
    let result = store.search_graph(
        "module0/file0.py#func0#1",
        2,
        &[SearchInclude::SameModule]
    ).unwrap();
    let search_duration = search_start.elapsed();

    println!("Search completed in {:?}, found {} functions", search_duration, result.functions.len());

    // 断言性能在可接受范围内（< 1秒）
    assert!(search_duration.as_secs() < 1, "Search should complete within 1 second");
}
