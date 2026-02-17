use serde_json::Value;
use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
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

fn write(path: &Path, content: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).expect("create parent");
    }
    fs::write(path, content).expect("write file");
}

#[test]
fn arch_matrix_switch_off_is_backward_compatible() {
    let root = temp_dir("bcc_arch_injection_off");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let out = root.join("out");

    write(
        &seed,
        r#"version: v3
source_of_truth: test
modules:
  - module_id: INFRA
    precedence: 10
    path_rules:
      include: ["src/infra/**"]
  - module_id: PROVIDERS
    precedence: 10
    path_rules:
      include: ["src/providers/**"]
relations_expected:
  - caller: INFRA
    callee: INFRA
    allowed: true
"#,
    );
    write(
        &ast,
        r#"{
  "source_count": 2,
  "records": [
    {
      "sourcePath": "src/infra/req_llm.ex",
      "localDependencies": ["src/providers/anthropic.ex"],
      "localCallTargets": [],
      "relationHints": [
        {
          "target": "src/providers/anthropic.ex",
          "call_type_hint": "external_registration",
          "via": "ReqLLM.Providers.register",
          "confidence": 0.99,
          "detector": "elixir.external_register",
          "reason": "register"
        }
      ]
    },
    {
      "sourcePath": "src/providers/anthropic.ex",
      "localDependencies": [],
      "localCallTargets": []
    }
  ]
}"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
        ])
        .status()
        .expect("run matrix off");
    assert!(status.success());

    assert!(out.join("v3.target-matrix.yaml").exists());
    assert!(out.join("v3.transition-matrix.yaml").exists());
    assert!(out.join("v3.gates.yaml").exists());
    assert!(!out.join("v3.relation-classification.json").exists());

    let transition = fs::read_to_string(out.join("v3.transition-matrix.yaml")).expect("read");
    assert!(!transition.contains("call_type="));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_matrix_switch_on_emits_relation_classification_report() {
    let root = temp_dir("bcc_arch_injection_on");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let out = root.join("out");

    write(
        &seed,
        r#"version: v3
source_of_truth: test
modules:
  - module_id: INFRA
    precedence: 10
    path_rules:
      include: ["src/infra/**"]
  - module_id: PROVIDERS
    precedence: 10
    path_rules:
      include: ["src/providers/**"]
relations_expected: []
"#,
    );
    write(
        &ast,
        r#"{
  "source_count": 2,
  "records": [
    {
      "sourcePath": "src/infra/req_llm.ex",
      "localDependencies": ["src/providers/anthropic.ex"],
      "localCallTargets": [],
      "relationHints": [
        {
          "target": "src/providers/anthropic.ex",
          "call_type_hint": "external_registration",
          "via": "ReqLLM.Providers.register",
          "confidence": 0.99,
          "detector": "elixir.external_register",
          "reason": "register"
        }
      ]
    },
    {
      "sourcePath": "src/providers/anthropic.ex",
      "localDependencies": [],
      "localCallTargets": []
    }
  ]
}"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .status()
        .expect("run matrix on");
    assert!(status.success());

    let report_path = out.join("v3.relation-classification.json");
    assert!(report_path.exists());

    let payload: Value = serde_json::from_str(&fs::read_to_string(&report_path).expect("read"))
        .expect("parse classification report");
    let rows = payload.as_array().expect("classification rows");
    let row = rows
        .iter()
        .find(|item| {
            item.get("caller") == Some(&Value::String("INFRA".to_string()))
                && item.get("callee") == Some(&Value::String("PROVIDERS".to_string()))
        })
        .expect("INFRA -> PROVIDERS row");

    assert_eq!(
        row.get("call_type"),
        Some(&Value::String("external_registration".to_string()))
    );
    assert_eq!(
        row.get("via"),
        Some(&Value::String("ReqLLM.Providers.register".to_string()))
    );

    let target = fs::read_to_string(out.join("v3.target-matrix.yaml")).expect("read target");
    assert!(target.contains("call_type=external_registration"));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_matrix_injection_accuracy_is_above_ninety_percent() {
    let root = temp_dir("bcc_arch_injection_accuracy");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let out = root.join("out");

    write(
        &seed,
        r#"version: v3
source_of_truth: test
modules:
  - module_id: API
    precedence: 10
    path_rules:
      include: ["src/api/**"]
  - module_id: INFRA
    precedence: 10
    path_rules:
      include: ["src/infra/**"]
  - module_id: USER
    precedence: 10
    path_rules:
      include: ["src/user/**"]
  - module_id: PRISMA
    precedence: 10
    path_rules:
      include: ["src/prisma/**"]
  - module_id: PROVIDERS
    precedence: 10
    path_rules:
      include: ["src/providers/**"]
relations_expected: []
"#,
    );
    write(
        &ast,
        r#"{
  "source_count": 6,
  "records": [
    {
      "sourcePath": "src/api/app.module.ts",
      "localDependencies": ["src/user/user.module.ts", "src/prisma/prisma.module.ts"],
      "localCallTargets": [],
      "relationHints": [
        {
          "target": "src/user/user.module.ts",
          "call_type_hint": "framework_injection",
          "via": "@Module.imports",
          "confidence": 0.95,
          "detector": "typescript.nest.module",
          "reason": "module imports"
        },
        {
          "target": "src/prisma/prisma.module.ts",
          "call_type_hint": "framework_injection",
          "via": "@Module.imports",
          "confidence": 0.95,
          "detector": "typescript.nest.module",
          "reason": "module imports"
        }
      ]
    },
    {
      "sourcePath": "src/api/health.ts",
      "localDependencies": ["src/infra/boot.ex"],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/infra/boot.ex",
      "localDependencies": ["src/providers/anthropic.ex"],
      "localCallTargets": [],
      "relationHints": [
        {
          "target": "src/providers/anthropic.ex",
          "call_type_hint": "external_registration",
          "via": "ReqLLM.Providers.register",
          "confidence": 0.99,
          "detector": "elixir.external_register",
          "reason": "register"
        }
      ]
    },
    {
      "sourcePath": "src/user/user.module.ts",
      "localDependencies": [],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/prisma/prisma.module.ts",
      "localDependencies": [],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/providers/anthropic.ex",
      "localDependencies": [],
      "localCallTargets": []
    }
  ]
}"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .status()
        .expect("run matrix for accuracy");
    assert!(status.success());

    let report_path = out.join("v3.relation-classification.json");
    let payload: Value = serde_json::from_str(&fs::read_to_string(&report_path).expect("read"))
        .expect("parse classification report");
    let rows = payload.as_array().expect("classification rows");

    let mut predicted: HashMap<(String, String), String> = HashMap::new();
    for row in rows {
        let Some(caller) = row.get("caller").and_then(|v| v.as_str()) else {
            continue;
        };
        let Some(callee) = row.get("callee").and_then(|v| v.as_str()) else {
            continue;
        };
        let Some(call_type) = row.get("call_type").and_then(|v| v.as_str()) else {
            continue;
        };
        predicted.insert(
            (caller.to_string(), callee.to_string()),
            call_type.to_string(),
        );
    }

    let expected = vec![
        (("API", "USER"), "framework_injection"),
        (("API", "PRISMA"), "framework_injection"),
        (("INFRA", "PROVIDERS"), "external_registration"),
        (("API", "INFRA"), "direct_call"),
    ];

    let mut correct = 0usize;
    for (edge, want) in &expected {
        let got = predicted
            .get(&(edge.0.to_string(), edge.1.to_string()))
            .map(|v| v.as_str())
            .unwrap_or("missing");
        if got == *want {
            correct += 1;
        }
    }
    let total = expected.len();
    let accuracy = correct as f64 / total as f64;

    let mut call_type_counts: HashMap<String, usize> = HashMap::new();
    for value in predicted.values() {
        *call_type_counts.entry(value.clone()).or_insert(0) += 1;
    }
    assert!(
        accuracy > 0.90,
        "accuracy summary: {}/{} = {:.2}%, by_type={:?}",
        correct,
        total,
        accuracy * 100.0,
        call_type_counts
    );

    let _ = fs::remove_dir_all(&root);
}
