//! analyze 端到端集成测试
//!
//! 通过 env!("CARGO_BIN_EXE_bcc") 调用编译好的二进制，
//! 构造含已知气味的源码文件验证输出。

use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn temp_dir(prefix: &str) -> PathBuf {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock")
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("{}_{}_{}", prefix, std::process::id(), ts));
    fs::create_dir_all(&dir).expect("create temp dir");
    dir
}

/// 辅助函数：创建源码文件 + AST JSON，运行 analyze，返回解析后的 SmellReport JSON
fn run_analyze(source_content: &str, filename: &str, lang: &str, rules: Option<&str>) -> serde_json::Value {
    let dir = temp_dir("analyze_bdd");
    let src_path = dir.join(filename);
    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    fs::write(&src_path, source_content).unwrap();

    // 构造最小 FileRecord JSON，file_path 指向真实源码文件
    let ast_json = format!(r#"[{{
        "language": "{}",
        "file_path": "{}",
        "module_doc": null,
        "exports": [],
        "imports": [],
        "calls": [],
        "side_effects": {{
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        }},
        "loc_lines": 10,
        "declarations": 1
    }}]"#, lang, src_path.to_str().unwrap());
    fs::write(&ast_path, &ast_json).unwrap();

    let mut cmd = Command::new(env!("CARGO_BIN_EXE_bcc"));
    cmd.args([
        "analyze",
        "--ast-file", ast_path.to_str().unwrap(),
        "-o", out_path.to_str().unwrap(),
    ]);
    if let Some(r) = rules {
        cmd.args(["--rules", r]);
    }

    let output = cmd.output().expect("run bcc analyze");
    assert!(output.status.success(), "bcc analyze failed: {}", String::from_utf8_lossy(&output.stderr));

    let raw = fs::read_to_string(&out_path).expect("read smells output");
    serde_json::from_str(&raw).expect("parse smells json")
}

// --- 安全检测器端到端测试 ---

#[test]
fn e2e_hardcoded_credential_detected() {
    let result = run_analyze(
        r#"api_key = "sk-proj-abc123def456""#,
        "config.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "hardcoded_credential"),
        "Expected hardcoded_credential smell, got: {:?}", smells);
}

#[test]
fn e2e_env_var_no_false_positive() {
    let result = run_analyze(
        r#"config = os.environ.get("API_KEY")"#,
        "config.py",
        "python",
        Some("security"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Expected no smells for env var, got: {:?}", smells);
}

#[test]
fn e2e_injection_risk_detected() {
    let result = run_analyze(
        r#"cursor.execute("SELECT * WHERE id=" + user_id)"#,
        "db.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "injection_risk"),
        "Expected injection_risk smell, got: {:?}", smells);
}

#[test]
fn e2e_parameterized_query_no_false_positive() {
    let result = run_analyze(
        r#"cursor.execute("SELECT * WHERE id=%s", (user_id,))"#,
        "db.py",
        "python",
        Some("security"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().all(|s| s["rule"] != "injection_risk"),
        "Expected no injection_risk, got: {:?}", smells);
}

#[test]
fn e2e_unsafe_deserialization_detected() {
    let result = run_analyze(
        "data = pickle.load(f)",
        "utils.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "unsafe_deserialization"),
        "Expected unsafe_deserialization smell, got: {:?}", smells);
}

#[test]
fn e2e_safe_load_no_false_positive() {
    let result = run_analyze(
        "data = yaml.safe_load(f)",
        "config.py",
        "python",
        Some("security"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().all(|s| s["rule"] != "unsafe_deserialization"),
        "Expected no unsafe_deserialization, got: {:?}", smells);
}

#[test]
fn e2e_weak_crypto_detected() {
    let result = run_analyze(
        "h = hashlib.md5(data)",
        "hash.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "weak_crypto"),
        "Expected weak_crypto smell, got: {:?}", smells);
}

#[test]
fn e2e_sha256_no_false_positive() {
    let result = run_analyze(
        "h = hashlib.sha256(data)",
        "hash.py",
        "python",
        Some("security"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Expected no smells for sha256, got: {:?}", smells);
}

#[test]
fn e2e_sensitive_data_log_detected() {
    let result = run_analyze(
        r#"logger.info(f"token={api_token}")"#,
        "auth.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "sensitive_data_log"),
        "Expected sensitive_data_log smell, got: {:?}", smells);
}

#[test]
fn e2e_normal_log_no_false_positive() {
    let result = run_analyze(
        r#"logger.info(f"user={username}")"#,
        "auth.py",
        "python",
        Some("security"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Expected no smells for normal log, got: {:?}", smells);
}

// --- 错误处理检测器端到端测试 ---

#[test]
fn e2e_swallowed_error_detected() {
    let result = run_analyze(
        "try:\n    risky()\nexcept:\n    pass\n",
        "main.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "swallowed_error"),
        "Expected swallowed_error smell, got: {:?}", smells);
}

#[test]
fn e2e_broad_catch_detected() {
    let result = run_analyze(
        "try:\n    risky()\nexcept Exception as e:\n    log(e)\n",
        "main.py",
        "python",
        None,
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "broad_catch"),
        "Expected broad_catch smell, got: {:?}", smells);
}

#[test]
fn e2e_specific_exception_no_false_positive() {
    let result = run_analyze(
        "try:\n    risky()\nexcept ValueError as e:\n    handle(e)\n",
        "main.py",
        "python",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Expected no smells for specific exception, got: {:?}", smells);
}

// --- 边界回归测试 ---

#[test]
fn e2e_swallowed_error_no_false_positive_on_empty_line() {
    // except: 后跟空行再跟有意义代码 → 不应误报 swallowed_error
    let result = run_analyze(
        "try:\n    risky()\nexcept:\n\n    log(e)\n",
        "main.py",
        "python",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().all(|s| s["rule"] != "swallowed_error"),
        "Empty line after except should not trigger swallowed_error, got: {:?}", smells);
}

#[test]
fn e2e_broad_catch_multiline_elixir_rescue() {
    // 多行 rescue\n  _ -> :ok 应检出 broad_catch
    let result = run_analyze(
        "try do\n  risky()\nrescue\n  _ -> :ok\nend\n",
        "main.ex",
        "elixir",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "broad_catch"),
        "Multi-line Elixir rescue _ should trigger broad_catch, got: {:?}", smells);
}

#[test]
fn e2e_broad_catch_js_catch_no_param() {
    // JS/TS catch {} 无参数应检出 broad_catch
    let result = run_analyze(
        "try { risky() } catch { handleError() }",
        "main.ts",
        "typescript",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "broad_catch"),
        "JS catch without parameter should trigger broad_catch, got: {:?}", smells);
}

#[test]
fn e2e_js_catch_with_param_no_broad_catch() {
    // JS/TS catch (e) 有参数不应检出 broad_catch
    let result = run_analyze(
        "try { risky() } catch (e) { handleError(e) }",
        "main.ts",
        "typescript",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().all(|s| s["rule"] != "broad_catch"),
        "JS catch with parameter should not trigger broad_catch, got: {:?}", smells);
}

// --- Rust 错误处理端到端测试 ---

#[test]
fn e2e_rust_swallowed_error_let_discard() {
    let result = run_analyze(
        "fn main() {\n    let _ = fs::remove_file(\"tmp.txt\");\n}\n",
        "main.rs",
        "rust",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "swallowed_error"),
        "Expected swallowed_error for Rust let _ =, got: {:?}", smells);
}

#[test]
fn e2e_rust_no_broad_catch() {
    // Rust 无异常捕获机制，broad_catch 不适用
    let result = run_analyze(
        "fn main() {\n    match result {\n        Err(_) => {}\n        _ => {}\n    }\n}\n",
        "main.rs",
        "rust",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().all(|s| s["rule"] != "broad_catch"),
        "Rust should not trigger broad_catch, got: {:?}", smells);
}

// --- 空文件应正常处理 ---

#[test]
fn e2e_empty_source_file_succeeds() {
    let dir = temp_dir("analyze_bdd_empty");
    let src_path = dir.join("empty.py");
    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    fs::write(&src_path, "").unwrap(); // 合法的空文件

    let ast_json = format!(r#"[{{
        "language": "python",
        "file_path": "{}",
        "module_doc": null,
        "exports": [],
        "imports": [],
        "calls": [],
        "side_effects": {{
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        }},
        "loc_lines": 0,
        "declarations": 0
    }}]"#, src_path.to_str().unwrap());
    fs::write(&ast_path, &ast_json).unwrap();

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", ast_path.to_str().unwrap(),
            "-o", out_path.to_str().unwrap(),
        ])
        .output()
        .expect("run bcc analyze");
    assert!(output.status.success(),
        "Empty source file should not cause failure: {}", String::from_utf8_lossy(&output.stderr));

    let raw = fs::read_to_string(&out_path).unwrap();
    let result: serde_json::Value = serde_json::from_str(&raw).unwrap();
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Empty file should produce no smells");
}

// --- Python comment+pass 应检出 swallowed_error ---

#[test]
fn e2e_swallowed_error_comment_then_pass() {
    // except: 后跟注释再跟 pass → 应检出 swallowed_error
    let result = run_analyze(
        "try:\n    risky()\nexcept:\n    # ignore error\n    pass\n",
        "main.py",
        "python",
        Some("error_handling"),
    );
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.iter().any(|s| s["rule"] == "swallowed_error"),
        "Comment followed by pass should trigger swallowed_error, got: {:?}", smells);
}

// --- 源码读取失败应返回非零退出码 ---

#[test]
fn e2e_unreadable_source_fails() {
    let dir = temp_dir("analyze_bdd_unreadable");
    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    // file_path 指向不存在的文件
    let ast_json = r#"[{
        "language": "python",
        "file_path": "/nonexistent/path/missing.py",
        "module_doc": null,
        "exports": [],
        "imports": [],
        "calls": [],
        "side_effects": {
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        },
        "loc_lines": 5,
        "declarations": 0
    }]"#;
    fs::write(&ast_path, &ast_json).unwrap();

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", ast_path.to_str().unwrap(),
            "-o", out_path.to_str().unwrap(),
        ])
        .output()
        .expect("run bcc analyze");
    assert!(!output.status.success(),
        "bcc analyze should fail when source file is unreadable, stderr: {}",
        String::from_utf8_lossy(&output.stderr));
}

// --- 并发访问同一资源测试 ---

#[test]
fn e2e_concurrent_analyze_same_source() {
    use std::thread;

    let dir = temp_dir("analyze_bdd_concurrent");
    let src_path = dir.join("vuln.py");
    let ast_path = dir.join("ast.json");

    // 构造含多种气味的源码
    let source = r#"api_key = "sk-proj-abc123def456"
h = hashlib.md5(data)
try:
    risky()
except:
    pass
"#;
    fs::write(&src_path, source).unwrap();

    let ast_json = format!(r#"[{{
        "language": "python",
        "file_path": "{}",
        "module_doc": null,
        "exports": [],
        "imports": [],
        "calls": [],
        "side_effects": {{
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        }},
        "loc_lines": 10,
        "declarations": 1
    }}]"#, src_path.to_str().unwrap());
    fs::write(&ast_path, &ast_json).unwrap();

    // 多线程同时对同一输入运行 analyze，验证无竞态
    let handles: Vec<_> = (0..4)
        .map(|i| {
            let ast = ast_path.clone();
            let out = dir.join(format!("smells_{}.json", i));
            thread::spawn(move || {
                let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
                    .args([
                        "analyze",
                        "--ast-file", ast.to_str().unwrap(),
                        "-o", out.to_str().unwrap(),
                    ])
                    .output()
                    .expect("run bcc analyze");
                assert!(output.status.success(),
                    "Concurrent run {} failed: {}", i, String::from_utf8_lossy(&output.stderr));
                let raw = fs::read_to_string(&out).unwrap();
                let result: serde_json::Value = serde_json::from_str(&raw).unwrap();
                result
            })
        })
        .collect();

    // 所有线程结果应一致
    let results: Vec<serde_json::Value> = handles.into_iter().map(|h| h.join().unwrap()).collect();
    let baseline_smells = results[0][0]["smells"].as_array().unwrap().len();
    assert!(baseline_smells > 0, "Should detect smells in concurrent test");
    for (i, r) in results.iter().enumerate().skip(1) {
        let count = r[0]["smells"].as_array().unwrap().len();
        assert_eq!(count, baseline_smells,
            "Concurrent run {} produced {} smells, expected {} (same as run 0)", i, count, baseline_smells);
    }
}

// --- 深度/大文件限制测试 ---

#[test]
fn e2e_deeply_nested_source_no_stack_overflow() {
    // 构造深度嵌套的 try/except（200 层）+ 大量重复行，验证不会栈溢出或超时
    let mut source = String::new();
    for i in 0..200 {
        let indent = "    ".repeat(i);
        source.push_str(&format!("{}try:\n{}    risky{}()\n{}except:\n{}    pass\n",
            indent, indent, i, indent, indent));
    }

    let result = run_analyze(&source, "deep_nested.py", "python", None);
    let smells = result[0]["smells"].as_array().unwrap();
    // 应能正常完成分析，且检出多条 swallowed_error
    assert!(smells.iter().filter(|s| s["rule"] == "swallowed_error").count() >= 100,
        "Deep nesting should still detect swallowed_error, got {} total smells", smells.len());
}

#[test]
fn e2e_large_file_many_lines() {
    // 构造含 5000 行的大文件，每 100 行插入一个气味
    let mut lines: Vec<String> = Vec::new();
    for i in 0..5000 {
        if i % 100 == 0 {
            lines.push(format!(r#"secret_{} = "sk-proj-{}""#, i, i));
        } else {
            lines.push(format!("x_{} = {}", i, i));
        }
    }
    let source = lines.join("\n");

    let result = run_analyze(&source, "large.py", "python", Some("security"));
    let smells = result[0]["smells"].as_array().unwrap();
    let cred_count = smells.iter().filter(|s| s["rule"] == "hardcoded_credential").count();
    // 5000 行 / 100 = 50 个凭证
    assert_eq!(cred_count, 50,
        "Large file should detect all 50 hardcoded credentials, got {}", cred_count);
}

// --- 过滤功能端到端测试 ---

#[test]
fn e2e_rules_filter_works() {
    // 源码同时有安全和错误处理问题
    let source = "api_key = \"sk-proj-abc123def456\"\ntry:\n    risky()\nexcept:\n    pass\n";
    let result = run_analyze(source, "mixed.py", "python", Some("security"));
    let smells = result[0]["smells"].as_array().unwrap();
    // 只应有 security 类别
    for s in smells {
        assert_eq!(s["category"], "security", "Expected only security smells with rules filter");
    }
}

#[test]
fn e2e_test_file_excluded() {
    let dir = temp_dir("analyze_bdd_test_excl");
    // 源码放在 test 目录下
    let src_path = dir.join("test").join("config_test.py");
    fs::create_dir_all(src_path.parent().unwrap()).unwrap();
    fs::write(&src_path, r#"api_key = "sk-proj-abc123def456""#).unwrap();

    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    let ast_json = format!(r#"[{{
        "language": "python",
        "file_path": "test/config_test.py",
        "module_doc": null,
        "exports": [],
        "imports": [],
        "calls": [],
        "side_effects": {{
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        }},
        "loc_lines": 1,
        "declarations": 0
    }}]"#);
    fs::write(&ast_path, &ast_json).unwrap();

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", ast_path.to_str().unwrap(),
            "-o", out_path.to_str().unwrap(),
        ])
        .output()
        .expect("run bcc analyze");
    assert!(output.status.success());

    let raw = fs::read_to_string(&out_path).unwrap();
    let result: serde_json::Value = serde_json::from_str(&raw).unwrap();
    let smells = result[0]["smells"].as_array().unwrap();
    assert!(smells.is_empty(), "Test files should be excluded, got: {:?}", smells);
}
