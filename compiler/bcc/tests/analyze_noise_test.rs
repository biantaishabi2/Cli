//! 噪音检测器集成测试：通过 bcc analyze CLI 验证 SmellReport 输出

use std::fs;
use std::process::Command;

fn bcc_bin() -> String {
    env!("CARGO_BIN_EXE_bcc").to_string()
}

fn temp_dir(prefix: &str) -> std::path::PathBuf {
    let dir = std::env::temp_dir().join(format!(
        "bcc_analyze_noise_{}_{}", prefix, std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_millis()
    ));
    fs::create_dir_all(&dir).unwrap();
    dir
}

fn write(path: &std::path::Path, content: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).ok();
    }
    fs::write(path, content).unwrap();
}

/// 构造 Python 源码文件和对应的 FileRecord JSON，运行 bcc analyze，返回输出 JSON
fn run_analyze_with_python_source(
    source: &str,
    prefix: &str,
    imports: &[(&str, &str)],
    calls: &[&str],
) -> serde_json::Value {
    let dir = temp_dir(prefix);
    let source_path = dir.join("test_source.py");
    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    write(&source_path, source);

    // 构造 FileRecord JSON（file_path 指向真实源码文件）
    let import_json: Vec<serde_json::Value> = imports
        .iter()
        .map(|(spec, kind)| {
            serde_json::json!({
                "specifier": spec,
                "kind": kind
            })
        })
        .collect();

    let call_json: Vec<serde_json::Value> = calls
        .iter()
        .map(|callee| {
            serde_json::json!({
                "callee": callee,
                "line": 1
            })
        })
        .collect();

    let ast_json = serde_json::json!([{
        "language": "python",
        "file_path": source_path.to_str().unwrap(),
        "module_doc": null,
        "exports": [],
        "imports": import_json,
        "calls": call_json,
        "side_effects": {
            "hasAsync": false,
            "hasHttp": false,
            "hasGenserver": false,
            "hasFileIo": false,
            "hasPubsub": false
        },
        "loc_lines": source.lines().count(),
        "declarations": 1
    }]);

    write(&ast_path, &serde_json::to_string_pretty(&ast_json).unwrap());

    let output = Command::new(bcc_bin())
        .args([
            "analyze",
            "--ast-file",
            ast_path.to_str().unwrap(),
            "-o",
            out_path.to_str().unwrap(),
        ])
        .output()
        .expect("failed to run bcc analyze");

    assert!(
        output.status.success(),
        "bcc analyze failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let result_str = fs::read_to_string(&out_path).unwrap();
    serde_json::from_str(&result_str).unwrap()
}

#[test]
fn excessive_comments_python() {
    let source = r#"
def heavy_comments():
    # comment line 1
    # comment line 2
    # comment line 3
    # comment line 4
    # comment line 5
    # comment line 6
    x = 1
    y = 2
    return x + y
"#;
    let result = run_analyze_with_python_source(source, "excessive", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    assert!(!reports.is_empty());
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "excessive_comments"),
        "应检出 excessive_comments, got: {:?}",
        smells
    );
}

#[test]
fn todo_comment_python() {
    let source = r#"
# TODO: fix this later
x = 1
"#;
    let result = run_analyze_with_python_source(source, "todo", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "leftover_boilerplate"),
        "应检出 leftover_boilerplate, got: {:?}",
        smells
    );
}

#[test]
fn empty_function_body_python() {
    let source = r#"
def foo():
    pass
"#;
    let result = run_analyze_with_python_source(source, "empty_body", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "empty_function_body"),
        "应检出 empty_function_body, got: {:?}",
        smells
    );
}

#[test]
fn unused_import_python() {
    let source = "import os\nx = 1\n";
    let result = run_analyze_with_python_source(
        source,
        "unused_import",
        &[("os", "import")],
        &[],
    );
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "unused_import"),
        "应检出 unused_import, got: {:?}",
        smells
    );
}

#[test]
fn commented_out_code_python() {
    let source = r#"
x = 1
# def old_function():
#     x = 1
#     return x + 2
y = 2
"#;
    let result = run_analyze_with_python_source(source, "commented_code", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "commented_out_code"),
        "应检出 commented_out_code, got: {:?}",
        smells
    );
}

#[test]
fn dead_branch_python() {
    let source = r#"
if False:
    do_something()
x = 1
"#;
    let result = run_analyze_with_python_source(source, "dead_branch", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "dead_branch"),
        "应检出 dead_branch, got: {:?}",
        smells
    );
}

#[test]
fn unreachable_code_python() {
    let source = r#"
def foo():
    return 1
    x = 2
"#;
    let result = run_analyze_with_python_source(source, "unreachable", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "unreachable_code"),
        "应检出 unreachable_code, got: {:?}",
        smells
    );
}

/// 三级嵌套函数：外层注释应归属外层，嵌套行数不应被双重扣减
#[test]
fn nested_functions_no_double_counting() {
    // 外层 outer 包含 middle，middle 包含 inner
    // 注释只在外层范围、嵌套范围之外，不应触发 excessive_comments 误报
    let source = r#"
def outer():
    x = 1
    y = 2
    z = 3
    w = 4
    a = 5
    b = 6
    c = 7
    d = 8
    def middle():
        m1 = 1
        m2 = 2
        m3 = 3
        def inner():
            i1 = 1
            i2 = 2
            i3 = 3
            return i1
        return m1
    return x
"#;
    let result = run_analyze_with_python_source(source, "nested_no_double", &[], &[]);
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    // 外层函数几乎没有注释，不应被误报 excessive_comments
    let excessive = smells.iter().filter(|s| {
        s["rule"] == "excessive_comments"
            && s["message"].as_str().unwrap_or("").contains("outer")
    }).count();
    assert_eq!(
        excessive, 0,
        "三级嵌套不应对 outer 触发 excessive_comments 误报, got: {:?}",
        smells
    );
}

#[test]
fn rules_filter_excludes_noise() {
    // 使用 defensive 过滤，noise 检测器不应运行
    let dir = temp_dir("rules_filter");
    let source = "# TODO: fix\nx = 1\n";
    let source_path = dir.join("test.py");
    let ast_path = dir.join("ast.json");
    let out_path = dir.join("smells.json");

    write(&source_path, source);

    let ast_json = serde_json::json!([{
        "language": "python",
        "file_path": source_path.to_str().unwrap(),
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
        "loc_lines": 2,
        "declarations": 0
    }]);
    write(&ast_path, &serde_json::to_string_pretty(&ast_json).unwrap());

    let output = Command::new(bcc_bin())
        .args([
            "analyze",
            "--ast-file",
            ast_path.to_str().unwrap(),
            "-o",
            out_path.to_str().unwrap(),
            "--rules",
            "defensive",
        ])
        .output()
        .expect("failed to run bcc analyze");

    assert!(output.status.success());

    let result_str = fs::read_to_string(&out_path).unwrap();
    let reports: Vec<serde_json::Value> = serde_json::from_str(&result_str).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.is_empty(),
        "rules=defensive 应过滤 noise 检测器: {:?}",
        smells
    );
}
