use regex::Regex;
use serde_json::Value;
use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitStatus};
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

fn write(path: &Path, content: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).expect("create parent");
    }
    fs::write(path, content).expect("write file");
}

fn run_arch_generate(seed: &Path, emit: &str, output: &Path) -> ExitStatus {
    run_arch_generate_with_options(seed, emit, output, false, "error-on-conflict")
}

fn run_arch_generate_with_options(
    seed: &Path,
    emit: &str,
    output: &Path,
    emit_runtime_bridge: bool,
    conflict_strategy: &str,
) -> ExitStatus {
    Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "generate",
            "--seed-file",
            &seed.to_string_lossy(),
            "--emit",
            emit,
            "--emit-runtime-bridge",
            if emit_runtime_bridge { "true" } else { "false" },
            "--conflict-strategy",
            conflict_strategy,
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run arch generate")
}

fn api_contract_schema_path() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("schemas")
        .join("unibo-api-contract.schema.json")
}

fn validate_api_contract_schema(payload: &Value) {
    let schema_raw =
        fs::read_to_string(api_contract_schema_path()).expect("read unibo api-contract schema");
    let schema_json: Value = serde_json::from_str(&schema_raw).expect("parse api-contract schema");
    if let Err(err) = validate_schema_node(payload, &schema_json, "$") {
        panic!("api-contract payload does not satisfy schema: {}", err);
    }
}

fn value_type_name(value: &Value) -> &'static str {
    match value {
        Value::Null => "null",
        Value::Bool(_) => "boolean",
        Value::Number(_) => "number",
        Value::String(_) => "string",
        Value::Array(_) => "array",
        Value::Object(_) => "object",
    }
}

fn validate_schema_node(value: &Value, schema: &Value, path: &str) -> Result<(), String> {
    let schema_obj = schema
        .as_object()
        .ok_or_else(|| format!("{} schema should be object", path))?;

    if let Some(expected_type) = schema_obj.get("type").and_then(Value::as_str) {
        validate_value_type(value, expected_type, path)?;
        match expected_type {
            "object" => validate_object_node(value, schema_obj, path)?,
            "array" => validate_array_node(value, schema_obj, path)?,
            "string" => validate_string_node(value, schema_obj, path)?,
            _ => {}
        }
        return Ok(());
    }

    if schema_obj.contains_key("required") || schema_obj.contains_key("properties") {
        validate_value_type(value, "object", path)?;
        validate_object_node(value, schema_obj, path)?;
    }
    if schema_obj.contains_key("items") || schema_obj.contains_key("minItems") {
        validate_value_type(value, "array", path)?;
        validate_array_node(value, schema_obj, path)?;
    }
    if schema_obj.contains_key("pattern") || schema_obj.contains_key("format") {
        validate_value_type(value, "string", path)?;
        validate_string_node(value, schema_obj, path)?;
    }
    Ok(())
}

fn validate_value_type(value: &Value, expected_type: &str, path: &str) -> Result<(), String> {
    let matched = match expected_type {
        "object" => value.is_object(),
        "array" => value.is_array(),
        "string" => value.is_string(),
        "number" => value.is_number(),
        "integer" => value.as_i64().is_some() || value.as_u64().is_some(),
        "boolean" => value.is_boolean(),
        "null" => value.is_null(),
        _ => true,
    };
    if matched {
        return Ok(());
    }
    Err(format!(
        "{} expected type `{}`, got `{}`",
        path,
        expected_type,
        value_type_name(value)
    ))
}

fn validate_object_node(
    value: &Value,
    schema_obj: &serde_json::Map<String, Value>,
    path: &str,
) -> Result<(), String> {
    let obj = value
        .as_object()
        .ok_or_else(|| format!("{} should be object", path))?;
    if let Some(required) = schema_obj.get("required").and_then(Value::as_array) {
        for field in required.iter().filter_map(Value::as_str) {
            if !obj.contains_key(field) {
                return Err(format!("{} missing required field `{}`", path, field));
            }
        }
    }

    let properties = schema_obj.get("properties").and_then(Value::as_object);
    let additional = schema_obj.get("additionalProperties");
    for (key, child) in obj {
        let child_path = format!("{}.{}", path, key);
        if let Some(prop_schema) = properties.and_then(|props| props.get(key)) {
            validate_schema_node(child, prop_schema, &child_path)?;
            continue;
        }
        if let Some(additional_schema) = additional {
            if let Some(false) = additional_schema.as_bool() {
                return Err(format!("{} is not allowed by schema", child_path));
            }
            if additional_schema.is_object() {
                validate_schema_node(child, additional_schema, &child_path)?;
            }
        }
    }
    Ok(())
}

fn validate_array_node(
    value: &Value,
    schema_obj: &serde_json::Map<String, Value>,
    path: &str,
) -> Result<(), String> {
    let arr = value
        .as_array()
        .ok_or_else(|| format!("{} should be array", path))?;
    if let Some(min_items) = schema_obj.get("minItems").and_then(Value::as_u64) {
        if (arr.len() as u64) < min_items {
            return Err(format!(
                "{} requires at least {} items, got {}",
                path,
                min_items,
                arr.len()
            ));
        }
    }
    if let Some(item_schema) = schema_obj.get("items") {
        for (idx, item) in arr.iter().enumerate() {
            validate_schema_node(item, item_schema, &format!("{}[{}]", path, idx))?;
        }
    }
    Ok(())
}

fn validate_string_node(
    value: &Value,
    schema_obj: &serde_json::Map<String, Value>,
    path: &str,
) -> Result<(), String> {
    let s = value
        .as_str()
        .ok_or_else(|| format!("{} should be string", path))?;
    if let Some(pattern) = schema_obj.get("pattern").and_then(Value::as_str) {
        let re = Regex::new(pattern)
            .map_err(|e| format!("{} has invalid pattern `{}`: {}", path, pattern, e))?;
        if !re.is_match(s) {
            return Err(format!(
                "{} value `{}` does not match pattern `{}`",
                path, s, pattern
            ));
        }
    }
    if let Some(format) = schema_obj.get("format").and_then(Value::as_str) {
        if format == "date-time" && chrono::DateTime::parse_from_rfc3339(s).is_err() {
            return Err(format!("{} value `{}` is not RFC3339 date-time", path, s));
        }
    }
    Ok(())
}

fn sample_seed_with_contracts() -> &'static str {
    r#"version: v1
modules:
  - module_id: billing
    precedence: 1
    path_rules:
      include: ["src/billing/**"]
relations_expected: []
boundaries:
  - module_id: billing
    contracts:
      - name: invoice
        kind: crud
        fields:
          amount: decimal
          buyer_id: uuid
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
        output:
          invoice_id: uuid
"#
}

fn sample_seed_without_contracts() -> &'static str {
    r#"version: v1
modules:
  - module_id: billing
    precedence: 1
    path_rules:
      include: ["src/billing/**"]
relations_expected: []
"#
}

fn collect_dir_snapshot(root: &Path) -> BTreeMap<String, String> {
    fn visit(base: &Path, dir: &Path, out: &mut BTreeMap<String, String>) {
        let mut entries: Vec<_> = fs::read_dir(dir)
            .expect("read_dir")
            .map(|e| e.expect("dir entry"))
            .collect();
        entries.sort_by_key(|e| e.path());
        for entry in entries {
            let path = entry.path();
            if path.is_dir() {
                visit(base, &path, out);
                continue;
            }
            let rel = path
                .strip_prefix(base)
                .expect("strip prefix")
                .to_string_lossy()
                .to_string();
            let content = fs::read_to_string(&path).expect("read file content");
            out.insert(rel, content);
        }
    }

    let mut snapshot = BTreeMap::new();
    if root.exists() {
        visit(root, root, &mut snapshot);
    }
    snapshot
}

#[test]
fn arch_generate_emit_code_keeps_existing_output() {
    let root = temp_dir("bcc_arch_generate_code");
    let seed = root.join("seed.yaml");
    let output = root.join("code-out");
    write(&seed, sample_seed_with_contracts());

    let status = run_arch_generate(&seed, "code", &output);
    assert!(status.success(), "emit code should succeed");
    assert!(output.join("generate-commands.sh").exists());
    assert!(output.join("billing.ex").exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_api_contract_only_outputs_contract_file() {
    let root = temp_dir("bcc_arch_generate_api_contract");
    let seed = root.join("seed.yaml");
    let output = root.join("api-contract-out");
    write(&seed, sample_seed_with_contracts());

    let status =
        run_arch_generate_with_options(&seed, "api-contract", &output, false, "error-on-conflict");
    assert!(status.success(), "emit api-contract should succeed");

    let contract_path = output.join("unibo-api-contract.json");
    assert!(
        contract_path.exists(),
        "unibo-api-contract.json should exist"
    );
    assert!(!output.join("generate-commands.sh").exists());
    assert!(!output.join("billing.ex").exists());
    assert!(!output.join("unibo-runtime-bridge.yaml").exists());

    let payload: Value =
        serde_json::from_str(&fs::read_to_string(&contract_path).expect("read contract file"))
            .expect("parse contract json");
    validate_api_contract_schema(&payload);
    assert_eq!(
        payload.get("bridgeVersion"),
        Some(&Value::String("1.0.0".to_string()))
    );
    assert_eq!(
        payload.get("targetRuntimeVersion"),
        Some(&Value::String("1.x".to_string()))
    );
    assert_eq!(
        payload.get("compatVersion"),
        Some(&Value::String("1".to_string()))
    );
    assert_eq!(
        payload.get("seedVersion"),
        Some(&Value::String("v1".to_string()))
    );
    assert!(
        payload
            .get("generatedAt")
            .and_then(|v| v.as_str())
            .map(|v| v.contains('T'))
            .unwrap_or(false),
        "generated_at should be RFC3339"
    );
    let contracts = payload
        .get("contracts")
        .and_then(|v| v.as_array())
        .expect("contracts array");
    assert_eq!(contracts.len(), 2);
    assert!(contracts
        .iter()
        .all(|item| item.get("moduleId") == Some(&Value::String("billing".to_string()))));
    assert!(contracts
        .iter()
        .all(|item| item.get("actions").and_then(Value::as_array).is_some()));

    let files = collect_dir_snapshot(&output);
    assert_eq!(
        files.len(),
        1,
        "api-contract output should contain only one file"
    );
    assert!(files.contains_key("unibo-api-contract.json"));
    assert!(!files.keys().any(|name| {
        name.contains("runtime") || name.contains("controller") || name.contains("resolver")
    }));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_api_contract_fails_without_contracts() {
    let root = temp_dir("bcc_arch_generate_api_contract_missing");
    let seed = root.join("seed.yaml");
    let output = root.join("api-contract-out");
    write(&seed, sample_seed_without_contracts());

    let status = run_arch_generate(&seed, "api-contract", &output);
    assert!(
        !status.success(),
        "emit api-contract should fail without contracts"
    );
    assert_eq!(status.code(), Some(1));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_all_is_equivalent_to_emit_code() {
    let root = temp_dir("bcc_arch_generate_all_equivalent");
    let seed = root.join("seed.yaml");
    let code_output = root.join("code-out");
    let all_output = root.join("all-out");
    write(&seed, sample_seed_with_contracts());

    let code_status = run_arch_generate(&seed, "code", &code_output);
    let all_status = run_arch_generate(&seed, "all", &all_output);
    assert!(code_status.success(), "emit code should succeed");
    assert!(all_status.success(), "emit all should succeed");

    let code_snapshot = collect_dir_snapshot(&code_output);
    let all_snapshot = collect_dir_snapshot(&all_output);
    assert_eq!(code_snapshot, all_snapshot);

    let _ = fs::remove_dir_all(&root);
}
