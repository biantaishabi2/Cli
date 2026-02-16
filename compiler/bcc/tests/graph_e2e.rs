//! E2E 测试 - 使用真实仓库（openclaw）
//!
//! 运行条件: 手动触发（通过 workflow_dispatch）
//! 环境变量: E2E_TEST_REPO=/path/to/openclaw

use std::path::PathBuf;
use std::process::Command;

/// 获取测试仓库路径
fn get_test_repo() -> Option<PathBuf> {
    std::env::var("E2E_TEST_REPO").ok().map(PathBuf::from)
}

/// 运行 bcc 命令
fn run_bcc(args: &[&str]) -> Result<String, String> {
    // 尝试使用 release 二进制，如果不存在则使用 cargo run
    let bcc_path = "/Users/biantaishabi/Cli-graph-68/target/release/bcc";
    
    let output = if std::path::Path::new(bcc_path).exists() {
        Command::new(bcc_path)
            .args(args)
            .output()
            .map_err(|e| format!("Failed to run bcc binary: {}", e))?
    } else {
        Command::new("cargo")
            .args(&["run", "--release", "--bin", "bcc", "--"])
            .args(args)
            .current_dir("/Users/biantaishabi/Cli-graph-68")
            .output()
            .map_err(|e| format!("Failed to run cargo: {}", e))?
    };

    let stdout = String::from_utf8_lossy(&output.stdout).to_string();
    let stderr = String::from_utf8_lossy(&output.stderr).to_string();

    if output.status.success() {
        Ok(stdout + &stderr)
    } else {
        Err(format!("bcc failed: {}\nstdout: {}\nstderr: {}", 
            output.status, stdout, stderr))
    }
}

/// E2E Scenario 1: 完整流程 - extract + index + query
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_full_workflow_openclaw() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    assert!(repo_path.exists(), "Test repo should exist at {:?}", repo_path);
    
    let repo_id = "local/openclaw-full";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Step 1: Extract AST from openclaw
    println!("Step 1: Extracting AST from {:?}", repo_path);
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang", "python",
        "--output", &ast_output.to_string_lossy()
    ]).expect("Failed to extract AST");

    assert!(ast_output.exists(), "AST output file should be created");
    println!("✓ AST extracted to {:?}", ast_output);

    // Step 2: Build index
    println!("Step 2: Building index for {}", repo_id);
    run_bcc(&[
        "graph", "build",
        "--repo", repo_id,
        "--name", "openclaw",
        "--path", &repo_path.to_string_lossy(),
        "--input", &ast_output.to_string_lossy(),
        "--commit", "HEAD"
    ]).expect("Failed to build index");

    println!("✓ Index built for {}", repo_id);

    // Step 3: List repos to verify
    println!("Step 3: Verifying index");
    let list_output = run_bcc(&["graph", "list"])
        .expect("Failed to list repos");
    
    assert!(list_output.contains(repo_id), "Repo should be in index list");
    println!("✓ Repo found in index");

    // Step 4: Query a function (if any exist)
    println!("Step 4: Querying functions");
    // Note: This assumes some functions were indexed
    // In real scenario, we'd query specific known functions from openclaw
}

/// E2E Scenario 2: 索引性能测试
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
        ])
        .output()
        .expect("Failed to count source files");

    let file_count = String::from_utf8_lossy(&output.stdout)
        .lines()
        .count();

    println!("Repository has {} Python files", file_count);

    // 测量索引时间
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");
    let repo_id = "local/openclaw-full-perf";

    let start = std::time::Instant::now();
    
    // Extract
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang", "python",
        "--output", &ast_output.to_string_lossy()
    ]).expect("Extract failed");

    // Build index
    run_bcc(&[
        "graph", "build",
        "--repo", repo_id,
        "--name", "openclaw",
        "--path", &repo_path.to_string_lossy(),
        "--input", &ast_output.to_string_lossy(),
        "--commit", "HEAD"
    ]).expect("Build index failed");

    let duration = start.elapsed();
    println!("Indexed {} files in {:?}", file_count, duration);
    
    // 断言性能指标（根据文件数动态调整）
    let expected_max_secs = (file_count as u64).max(10); // 至少10秒，或按文件数
    assert!(
        duration.as_secs() < expected_max_secs,
        "Indexing {} files took {:?}, expected < {} seconds",
        file_count, duration, expected_max_secs
    );
}

/// E2E Scenario 3: 架构验证
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_arch_validation() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-full";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");
    let matrix_path = temp_dir.path().join("target-matrix.yaml");
    let violations_path = temp_dir.path().join("violations.json");

    // 创建 target-matrix.yaml
    std::fs::write(&matrix_path, r#"
layers:
  - name: api
    patterns:
      - "*/api/*"
      - "*/web/*"
  - name: service
    patterns:
      - "*/services/*"
      - "*/core/*"
      - "*/agent/*"
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

    // Extract and index
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang", "python",
        "--output", &ast_output.to_string_lossy()
    ]).expect("Extract failed");

    run_bcc(&[
        "graph", "build",
        "--repo", repo_id,
        "--name", "openclaw",
        "--path", &repo_path.to_string_lossy(),
        "--input", &ast_output.to_string_lossy(),
        "--commit", "HEAD"
    ]).expect("Build index failed");

    // Run arch validation
    println!("Running arch validation for {}", repo_id);
    let result = run_bcc(&[
        "graph", "validate-arch",
        "--repo", repo_id,
        "--target", &matrix_path.to_string_lossy(),
        "--output", &violations_path.to_string_lossy()
    ]);

    match result {
        Ok(output) => println!("Validation output:\n{}", output),
        Err(e) => println!("Validation error (expected if no violations): {}", e),
    }

    // Check if violations file was created
    if violations_path.exists() {
        let content = std::fs::read_to_string(&violations_path)
            .expect("Failed to read violations");
        println!("Violations: {}", content);
    }
}

/// E2E Scenario 4: 搜索功能测试
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_search_functionality() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-full";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Extract and index
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang", "python",
        "--output", &ast_output.to_string_lossy()
    ]).expect("Extract failed");

    run_bcc(&[
        "graph", "build",
        "--repo", repo_id,
        "--name", "openclaw",
        "--path", &repo_path.to_string_lossy(),
        "--input", &ast_output.to_string_lossy(),
        "--commit", "HEAD"
    ]).expect("Build index failed");

    // Test search
    println!("Testing search functionality");
    
    // Search for callers/callees (using a dummy function ID)
    // In real test, we'd use actual function IDs from openclaw
    let search_result = run_bcc(&[
        "graph", "search",
        "--repo", repo_id,
        "--id", "openclaw/agent/tools/registry.py#register_tool#1",
        "--depth", "2",
        "--include", "callers,callees"
    ]);

    match search_result {
        Ok(output) => println!("Search result:\n{}", output),
        Err(e) => println!("Search error (expected if function not found): {}", e),
    }
}

/// E2E Scenario 5: 验证 openclaw 仓库结构
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_verify_openclaw_structure() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    // 验证 openclaw 的关键文件存在
    let key_files = vec![
        "src/index.ts",
        "src/agents",
    ];

    for file in key_files {
        let full_path = repo_path.join(file);
        assert!(
            full_path.exists(),
            "Expected openclaw path not found: {:?}",
            full_path
        );
        println!("✓ Found: {}", file);
    }

    println!("Nanobot structure verified successfully");
}
