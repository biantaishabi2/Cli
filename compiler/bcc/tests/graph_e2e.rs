//! E2E 测试 - 使用真实仓库（nanobot）
//!
//! 运行条件: 手动触发（通过 workflow_dispatch）
//! 环境变量: E2E_TEST_REPO=/path/to/nanobot
//!
//! 注意: 这些测试需要完整的工具链：
//! 1. bcc extract - 从源码提取 AST
//! 2. bcc graph-index build - 从 AST 构建索引
//! 3. 验证索引结果

use std::path::PathBuf;
use std::process::Command;

/// 获取测试仓库路径
fn get_test_repo() -> Option<PathBuf> {
    std::env::var("E2E_TEST_REPO").ok().map(PathBuf::from)
}

/// 运行 bcc 命令
fn run_bcc(args: &[&str]) -> Result<String, String> {
    let output = Command::new("cargo")
        .args(&["run", "--bin", "bcc", "--"])
        .args(args)
        .current_dir("/Users/biantaishabi/Cli-graph-68") // 确保在正确目录
        .output()
        .map_err(|e| format!("Failed to run bcc: {}", e))?;

    if output.status.success() {
        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).to_string())
    }
}

/// E2E Scenario 1: 完整流程 - extract + index + query
/// 
/// 测试步骤:
/// 1. 使用 bcc extract 提取 nanobot 的 AST
/// 2. 使用 bcc graph-index build 构建索引
/// 3. 使用 bcc graph-index query 查询函数
/// 4. 验证结果
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable and full bcc toolchain"]
fn e2e_full_workflow_nanobot() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    assert!(repo_path.exists(), "Test repo should exist at {:?}", repo_path);
    
    let repo_id = "github.com/HKUDS/nanobot";
    let temp_output = tempfile::tempdir().unwrap();
    let ast_output = temp_output.path().join("ast.json");

    // Step 1: Extract AST from nanobot
    println!("Step 1: Extracting AST from {:?}", repo_path);
    // TODO: 实现 bcc extract 命令
    // run_bcc(&["extract", &repo_path.to_string_lossy(), "-o", &ast_output.to_string_lossy()])
    //     .expect("Failed to extract AST");

    // Step 2: Build index
    println!("Step 2: Building index for {}", repo_id);
    // TODO: 实现 bcc graph-index build
    // run_bcc(&[
    //     "graph-index", "build",
    //     "--repo", repo_id,
    //     "--name", "nanobot",
    //     "--path", &repo_path.to_string_lossy(),
    //     "--input", &ast_output.to_string_lossy(),
    //     "--commit", "HEAD"
    // ]).expect("Failed to build index");

    // Step 3: Query functions
    println!("Step 3: Querying indexed functions");
    // TODO: 实现查询验证
    // let result = run_bcc(&[
    //     "graph-index", "query",
    //     "--repo", repo_id,
    //     "--id", "some_function_id",
    //     "--by", "id"
    // ]).expect("Failed to query");

    // 临时：验证仓库路径存在
    println!("E2E test setup complete. Repo: {:?}", repo_path);
}

/// E2E Scenario 2: 索引性能测试
/// 
/// 测试大规模仓库的索引性能
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_index_performance() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    // 统计仓库规模
    let output = Command::new("find")
        .args(&[
            &repo_path.to_string_lossy(),
            "-name", "*.py",
            "-o", "-name", "*.php",
            "-o", "-name", "*.ts",
            "-o", "-name", "*.js",
        ])
        .output()
        .expect("Failed to count source files");

    let file_count = String::from_utf8_lossy(&output.stdout)
        .lines()
        .count();

    println!("Repository has {} source files", file_count);

    // TODO: 测量索引时间
    // let start = std::time::Instant::now();
    // run_bcc(&[...]).expect("Index failed");
    // let duration = start.elapsed();
    // println!("Indexed {} files in {:?}", file_count, duration);
    
    // 断言性能指标
    // assert!(duration.as_secs() < 60, "Indexing should complete within 60 seconds");
}

/// E2E Scenario 3: 架构验证
/// 
/// 测试 nanobot 的架构合规性
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable and target-matrix.yaml"]
fn e2e_arch_validation() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "github.com/HKUDS/nanobot";

    // 创建临时 target-matrix.yaml
    let temp_dir = tempfile::tempdir().unwrap();
    let matrix_path = temp_dir.path().join("target-matrix.yaml");
    std::fs::write(&matrix_path, r#"
layers:
  - name: api
    patterns:
      - "*/api/*"
      - "*/routes/*"
  - name: service
    patterns:
      - "*/services/*"
      - "*/core/*"
  - name: dao
    patterns:
      - "*/models/*"
      - "*/db/*"

allowed_deps:
  - from: api
    to: service
  - from: service
    to: dao
"#).expect("Failed to write target matrix");

    println!("Running arch validation for {}", repo_id);
    
    // TODO: 运行架构验证
    // run_bcc(&[
    //     "graph-index", "validate-arch",
    //     "--repo", repo_id,
    //     "--target", &matrix_path.to_string_lossy(),
    //     "--output", "/tmp/violations.json"
    // ]).expect("Arch validation failed");

    println!("Arch validation setup complete");
}

/// E2E Scenario 4: 并发索引测试
/// 
/// 测试多仓库同时索引的场景
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_concurrent_indexing() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    // 模拟同时索引同一个仓库的不同版本
    let repo_ids = vec![
        "github.com/HKUDS/nanobot#v1",
        "github.com/HKUDS/nanobot#v2",
        "github.com/HKUDS/nanobot#v3",
    ];

    println!("Testing concurrent indexing for {} versions", repo_ids.len());

    // TODO: 使用多线程并发索引
    // use std::thread;
    // let handles: Vec<_> = repo_ids.into_iter().map(|repo_id| {
    //     thread::spawn(move || {
    //         run_bcc(&["graph-index", "build", "--repo", repo_id, ...])
    //     })
    // }).collect();
    //
    // for handle in handles {
    //     handle.join().expect("Thread panicked");
    // }

    println!("Concurrent indexing test setup complete");
}

/// 辅助函数：验证 nanobot 仓库结构
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_verify_nanobot_structure() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    // 验证 nanobot 的关键文件存在
    let key_files = vec![
        "nanobot/agent/tools/registry.py",
        "nanobot/agent/executor.py",
        "nanobot/__init__.py",
    ];

    for file in key_files {
        let full_path = repo_path.join(file);
        assert!(
            full_path.exists(),
            "Expected nanobot file not found: {:?}",
            full_path
        );
        println!("✓ Found: {}", file);
    }

    println!("Nanobot structure verified successfully");
}
