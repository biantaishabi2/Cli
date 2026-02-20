//! 重复检测器集成测试：通过 bcc analyze CLI 验证聚类结果

use std::fs;
use std::process::Command;

fn bcc_bin() -> String {
    env!("CARGO_BIN_EXE_bcc").to_string()
}

fn temp_dir(prefix: &str) -> std::path::PathBuf {
    let dir = std::env::temp_dir().join(format!(
        "bcc_analyze_dup_{}_{}", prefix, std::time::SystemTime::now()
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

fn run_analyze_python(source: &str, prefix: &str) -> serde_json::Value {
    let dir = temp_dir(prefix);
    let source_path = dir.join("test_source.py");
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
        "loc_lines": source.lines().count(),
        "declarations": 2
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
            "duplication",
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
fn structural_duplication_detected() {
    // 两个函数仅变量名不同
    let source = r#"
def process_user(user):
    result = validate(user)
    if result:
        save(result)
    return result

def process_order(order):
    result = validate(order)
    if result:
        save(result)
    return result
"#;
    let result = run_analyze_python(source, "structural_dup");
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "structural_duplication"),
        "应检出 structural_duplication, got: {:?}",
        smells
    );
    // 聚类应该报告 2 个函数
    let dup_smells: Vec<_> = smells
        .iter()
        .filter(|s| s["rule"] == "structural_duplication")
        .collect();
    assert_eq!(
        dup_smells.len(),
        2,
        "应有 2 条 structural_duplication 记录（每个函数一条）"
    );
}

#[test]
fn boilerplate_skeleton_detected() {
    // 两个函数骨架相同但叶子节点（函数调用名）不同
    let source = r#"
def fetch_users(db):
    items = db.query("users")
    for item in items:
        process(item)
    return items

def fetch_orders(db):
    items = db.execute("orders")
    for item in items:
        transform(item)
    return items
"#;
    let result = run_analyze_python(source, "skeleton_dup");
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        smells.iter().any(|s| s["rule"] == "boilerplate_skeleton"),
        "应检出 boilerplate_skeleton, got: {:?}",
        smells
    );
}

#[test]
fn no_duplication_for_different_functions() {
    let source = r#"
def add(a, b):
    return a + b

def multiply(a, b):
    result = a * b
    print(result)
    return result
"#;
    let result = run_analyze_python(source, "no_dup");
    let reports: Vec<serde_json::Value> = serde_json::from_value(result).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    let dup_smells: Vec<_> = smells
        .iter()
        .filter(|s| {
            s["rule"] == "structural_duplication" || s["rule"] == "boilerplate_skeleton"
        })
        .collect();
    assert!(
        dup_smells.is_empty(),
        "不同函数不应检出重复: {:?}",
        dup_smells
    );
}

#[test]
fn cross_file_no_duplicate_counting() {
    // 回归测试：duplication 检测器仅在跨文件阶段执行一次，不应重复计数
    let dir = temp_dir("no_dup_count");
    let source = r#"
def process_user(user):
    result = validate(user)
    if result:
        save(result)
    return result

def process_order(order):
    result = validate(order)
    if result:
        save(result)
    return result
"#;
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
        "loc_lines": source.lines().count(),
        "declarations": 2
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
            "duplication",
        ])
        .output()
        .expect("failed to run bcc analyze");

    assert!(
        output.status.success(),
        "bcc analyze failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let result_str = fs::read_to_string(&out_path).unwrap();
    let reports: Vec<serde_json::Value> = serde_json::from_str(&result_str).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();

    // 每个函数应只有 1 条 structural_duplication，不能因单文件+跨文件重复执行而出现 2 条
    let dup_smells: Vec<_> = smells
        .iter()
        .filter(|s| s["rule"] == "structural_duplication")
        .collect();
    assert_eq!(
        dup_smells.len(),
        2,
        "2 个函数应恰好产生 2 条 structural_duplication（每函数 1 条），实际 {}，smells: {:?}",
        dup_smells.len(),
        dup_smells
    );

    // summary 中 total_smells 应与实际 smells 数一致（无重复膨胀）
    let total = reports[0]["summary"]["total_smells"].as_u64().unwrap() as usize;
    assert_eq!(
        total,
        smells.len(),
        "summary.total_smells ({}) 应等于 smells.len() ({})",
        total,
        smells.len()
    );
}

#[test]
fn duplication_rules_filter() {
    // 使用 rules=noise 应过滤掉 duplication 检测器
    let dir = temp_dir("dup_filter");
    let source = r#"
def process_user(user):
    result = validate(user)
    if result:
        save(result)
    return result

def process_order(order):
    result = validate(order)
    if result:
        save(result)
    return result
"#;
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
        "loc_lines": source.lines().count(),
        "declarations": 2
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
            "noise",
        ])
        .output()
        .expect("failed to run bcc analyze");

    assert!(output.status.success());

    let result_str = fs::read_to_string(&out_path).unwrap();
    let reports: Vec<serde_json::Value> = serde_json::from_str(&result_str).unwrap();
    let smells = reports[0]["smells"].as_array().unwrap();
    assert!(
        !smells.iter().any(|s| s["rule"] == "structural_duplication"),
        "rules=noise 应过滤 duplication: {:?}",
        smells
    );
}
