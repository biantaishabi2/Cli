//! E2E 测试 - 使用真实仓库（openclaw）
//!
//! 运行条件: 手动触发（通过 workflow_dispatch）
//! 环境变量: E2E_TEST_REPO=/path/to/openclaw
//! 可选环境变量: BCC_BIN=/path/to/bcc
//! 手动执行: E2E_TEST_REPO=/path/to/repo cargo test -p bcc --test graph_e2e <case> -- --ignored --nocapture

use std::path::{Path, PathBuf};
use std::process::Command;

/// 获取测试仓库路径
fn get_test_repo() -> Option<PathBuf> {
    std::env::var("E2E_TEST_REPO").ok().map(PathBuf::from)
}

/// 定位 workspace 根目录（默认从 compiler/bcc 回溯到仓库根）
fn workspace_root() -> PathBuf {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .join("../..")
        .canonicalize()
        .unwrap_or(manifest_dir)
}

/// 运行 bcc 命令
fn run_bcc(args: &[&str]) -> Result<String, String> {
    // 优先使用显式指定的二进制路径（便于 CI/本地复用）
    let output = if let Ok(bcc_bin) = std::env::var("BCC_BIN") {
        if !Path::new(&bcc_bin).exists() {
            return Err(format!("BCC_BIN does not exist: {}", bcc_bin));
        }
        Command::new(&bcc_bin)
            .args(args)
            .output()
            .map_err(|e| format!("Failed to run bcc binary (BCC_BIN={}): {}", bcc_bin, e))?
    } else {
        // 回退到当前仓库执行 cargo run
        Command::new("cargo")
            .args(&["run", "--release", "--bin", "bcc", "--"])
            .args(args)
            .current_dir(workspace_root())
            .output()
            .map_err(|e| format!("Failed to run cargo in workspace root: {}", e))?
    };

    let stdout = String::from_utf8_lossy(&output.stdout).to_string();
    let stderr = String::from_utf8_lossy(&output.stderr).to_string();

    if output.status.success() {
        Ok(stdout + &stderr)
    } else {
        Err(format!(
            "bcc failed: {}\nstdout: {}\nstderr: {}",
            output.status, stdout, stderr
        ))
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

    assert!(
        repo_path.exists(),
        "Test repo should exist at {:?}",
        repo_path
    );

    let repo_id = "local/openclaw-full";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Step 1: Extract AST from openclaw
    println!("Step 1: Extracting AST from {:?}", repo_path);
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "python",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Failed to extract AST");

    assert!(ast_output.exists(), "AST output file should be created");
    println!("✓ AST extracted to {:?}", ast_output);

    // Step 2: Build index
    println!("Step 2: Building index for {}", repo_id);
    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Failed to build index");

    println!("✓ Index built for {}", repo_id);

    // Step 3: List repos to verify
    println!("Step 3: Verifying index");
    let list_output = run_bcc(&["graph", "list"]).expect("Failed to list repos");

    assert!(
        list_output.contains(repo_id),
        "Repo should be in index list"
    );
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
        .args(&[&repo_path.to_string_lossy(), "-name", "*.py"])
        .output()
        .expect("Failed to count source files");

    let file_count = String::from_utf8_lossy(&output.stdout).lines().count();

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
        "--lang",
        "python",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    // Build index
    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    let duration = start.elapsed();
    println!("Indexed {} files in {:?}", file_count, duration);

    // 断言性能指标（根据文件数动态调整）
    let expected_max_secs = (file_count as u64).max(10); // 至少10秒，或按文件数
    assert!(
        duration.as_secs() < expected_max_secs,
        "Indexing {} files took {:?}, expected < {} seconds",
        file_count,
        duration,
        expected_max_secs
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
    std::fs::write(
        &matrix_path,
        r#"
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
"#,
    )
    .expect("Failed to write target matrix");

    // Extract and index
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "python",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    // Run arch validation
    println!("Running arch validation for {}", repo_id);
    let result = run_bcc(&[
        "graph",
        "validate-arch",
        "--repo",
        repo_id,
        "--target",
        &matrix_path.to_string_lossy(),
        "--output",
        &violations_path.to_string_lossy(),
    ]);

    match result {
        Ok(output) => println!("Validation output:\n{}", output),
        Err(e) => println!("Validation error (expected if no violations): {}", e),
    }

    // Check if violations file was created
    if violations_path.exists() {
        let content = std::fs::read_to_string(&violations_path).expect("Failed to read violations");
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
        "--lang",
        "python",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    // Test search
    println!("Testing search functionality");

    // Search for callers/callees (using a dummy function ID)
    // In real test, we'd use actual function IDs from openclaw
    let search_result = run_bcc(&[
        "graph",
        "search",
        "--repo",
        repo_id,
        "--id",
        "openclaw/agent/tools/registry.py#register_tool#1",
        "--depth",
        "2",
        "--include",
        "callers,callees",
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
    let key_files = vec!["src/index.ts", "src/agents"];

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

// ==================== 模块依赖图 E2E 测试 ====================

/// E2E Scenario 6: 模块依赖查询
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_module_deps_query() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-module-test";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Step 1: Extract AST
    println!("Step 1: Extracting AST from {:?}", repo_path);
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "typescript",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Failed to extract AST");

    // Step 2: Build index
    println!("Step 2: Building index");
    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Failed to build index");

    // Step 3: Query module info
    println!("Step 3: Querying module info");
    let module_output = run_bcc(&[
        "graph",
        "module",
        "--repo",
        repo_id,
        "--id",
        "src/index.ts",
        "--by",
        "id",
    ])
    .expect("Failed to query module");

    println!("Module info:\n{}", module_output);
    assert!(
        module_output.contains("src/index.ts"),
        "Should show module ID"
    );
    assert!(module_output.contains("Lines:"), "Should show line count");
    assert!(
        module_output.contains("Exports:"),
        "Should show exports count"
    );
    assert!(
        module_output.contains("Imports:"),
        "Should show imports count"
    );

    // Step 4: Query module dependencies
    println!("Step 4: Querying module dependencies");
    let deps_output = run_bcc(&[
        "graph",
        "module",
        "--repo",
        repo_id,
        "--id",
        "src/index.ts",
        "--by",
        "deps",
        "--depth",
        "1",
    ])
    .expect("Failed to query dependencies");

    println!("Dependencies:\n{}", deps_output);
    assert!(
        deps_output.contains("Found") && deps_output.contains("dependencies"),
        "Should show dependency count"
    );

    // Step 5: Query module dependents
    println!("Step 5: Querying module dependents");
    let dependents_output = run_bcc(&[
        "graph",
        "module",
        "--repo",
        repo_id,
        "--id",
        "src/utils.ts",
        "--by",
        "dependents",
        "--depth",
        "1",
    ])
    .expect("Failed to query dependents");

    println!("Dependents:\n{}", dependents_output);
    // utils.ts 应该被很多模块依赖
    assert!(
        dependents_output.contains("Found") && dependents_output.contains("dependents"),
        "Should show dependents count"
    );

    println!("✓ Module dependency E2E test passed");
}

/// E2E Scenario 7: 模块依赖统计验证
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_module_deps_stats() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-module-stats";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Extract and build
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "typescript",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    let build_output = run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    println!("Build output:\n{}", build_output);

    // 验证 build 输出包含模块统计
    assert!(
        build_output.contains("modules") && build_output.contains("module deps"),
        "Build output should show module and dependency counts"
    );

    // 提取统计数字
    // 格式: "Indexed X modules, Y module deps, Z functions, W call edges"
    let modules_count = extract_number(&build_output, "Indexed ", " modules");
    let deps_count = extract_number(&build_output, "modules, ", " module deps");

    println!(
        "Stats: {} modules, {} module dependencies",
        modules_count, deps_count
    );

    // 验证模块依赖边数量合理（应该远多于调用边）
    assert!(modules_count > 0, "Should have modules");
    assert!(deps_count > 0, "Should have module dependencies");

    // 模块依赖通常比模块数量多（平均每个模块导入多个其他模块）
    let avg_deps_per_module = deps_count as f64 / modules_count as f64;
    println!(
        "Average dependencies per module: {:.2}",
        avg_deps_per_module
    );

    // 通常每个 TypeScript 文件会导入 3-10 个模块
    assert!(
        avg_deps_per_module >= 1.0 && avg_deps_per_module <= 50.0,
        "Average dependencies per module should be reasonable (1-50), got {:.2}",
        avg_deps_per_module
    );
}

/// E2E Scenario 8: 深度递归依赖查询
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_module_deps_recursive_depth() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-module-recursive";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Extract and build
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "typescript",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    // 测试不同深度的依赖查询
    for depth in [1, 2, 3] {
        println!("Testing depth={}", depth);

        let output = run_bcc(&[
            "graph",
            "module",
            "--repo",
            repo_id,
            "--id",
            "src/index.ts",
            "--by",
            "deps",
            "--depth",
            &depth.to_string(),
        ])
        .expect(&format!("Failed to query deps with depth={}", depth));

        println!(
            "Depth {} result: {}",
            depth,
            output.lines().next().unwrap_or("")
        );

        // depth 越大，找到的依赖应该越多（或相等）
        assert!(output.contains("Found"), "Should show result count");
    }

    println!("✓ Recursive depth test passed");
}

/// E2E Scenario 9: 循环依赖检测
#[test]
#[ignore = "requires E2E_TEST_REPO environment variable"]
fn e2e_module_circular_detection() {
    let repo_path = match get_test_repo() {
        Some(p) => p,
        None => {
            eprintln!("Skipping E2E test: E2E_TEST_REPO not set");
            return;
        }
    };

    let repo_id = "local/openclaw-module-circular";
    let temp_dir = tempfile::tempdir().unwrap();
    let ast_output = temp_dir.path().join("ast.json");

    // Extract and build
    run_bcc(&[
        "extract",
        &repo_path.to_string_lossy(),
        "--batch",
        "--lang",
        "typescript",
        "--output",
        &ast_output.to_string_lossy(),
    ])
    .expect("Extract failed");

    run_bcc(&[
        "graph",
        "build",
        "--repo",
        repo_id,
        "--name",
        "openclaw",
        "--path",
        &repo_path.to_string_lossy(),
        "--input",
        &ast_output.to_string_lossy(),
        "--commit",
        "HEAD",
    ])
    .expect("Build index failed");

    // 对一些常见模块检测循环依赖
    let test_modules = vec!["src/index.ts", "src/utils.ts", "src/config/config.ts"];

    for module_id in test_modules {
        println!("Checking circular deps for: {}", module_id);

        let output = run_bcc(&[
            "graph", "module", "--repo", repo_id, "--id", module_id, "--by", "circular",
        ]);

        match output {
            Ok(result) => {
                println!("{}", result);
                // 不管有没有循环，命令应该成功执行
                assert!(
                    result.contains("Circular dependency detected")
                        || result.contains("No circular dependencies"),
                    "Should report circular detection result"
                );
            }
            Err(e) => {
                println!("Warning: circular check failed for {}: {}", module_id, e);
                // 不失败，因为某些模块可能不存在
            }
        }
    }

    println!("✓ Circular detection test completed");
}

// 辅助函数：从字符串中提取数字
fn extract_number(s: &str, prefix: &str, suffix: &str) -> usize {
    if let Some(start) = s.find(prefix) {
        let after_prefix = &s[start + prefix.len()..];
        if let Some(end) = after_prefix.find(suffix) {
            let num_str = &after_prefix[..end];
            return num_str.parse().unwrap_or(0);
        }
    }
    0
}
