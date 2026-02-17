use serde_json::Value;
use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::thread;
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
fn arch_matrix_switch_on_appends_classification_reason_when_relations_expected_exists() {
    let root = temp_dir("bcc_arch_injection_relations_expected_reason");
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
    callee: PROVIDERS
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
            "--detect-injection",
        ])
        .status()
        .expect("run matrix on with relations_expected");
    assert!(status.success());

    let target = fs::read_to_string(out.join("v3.target-matrix.yaml")).expect("read target");
    assert!(target.contains("from relations_expected"));
    assert!(target.contains("call_type=external_registration"));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_matrix_injection_accuracy_on_gong_and_pi_mono_labeled_samples_is_above_ninety_percent() {
    #[derive(Clone, Copy)]
    struct LabeledEdge {
        project: &'static str,
        caller: &'static str,
        callee: &'static str,
        expected_call_type: &'static str,
    }

    let load_predictions = |report_path: &Path| -> HashMap<(String, String), String> {
        let payload: Value = serde_json::from_str(&fs::read_to_string(report_path).expect("read"))
            .expect("parse classification report");
        let rows = payload.as_array().expect("classification rows");
        let mut predicted = HashMap::new();
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
        predicted
    };

    let root = temp_dir("bcc_arch_injection_accuracy_real");
    let gong_root = root.join("gong");
    let pi_root = root.join("pi-mono");

    write(
        &gong_root.join("lib/gong/agent.ex"),
        "defmodule Gong.Agent do\n  use Jido.AI.ReActAgent, tools: [Gong.Tools.Read, Gong.Tools.Write]\n  alias Gong.Prompt.Builder\n  alias Gong.Data.Session\n\n  def run do\n    Builder.build()\n    Session.load()\n  end\nend\n",
    );
    write(
        &gong_root.join("lib/gong/prompt/builder.ex"),
        "defmodule Gong.Prompt.Builder do\n  def build, do: :ok\nend\n",
    );
    write(
        &gong_root.join("lib/gong/application.ex"),
        "defmodule Gong.Application do\n  alias Gong.Data.Session\n\n  def init do\n    ReqLLM.Providers.register(Gong.Provider.Anthropic)\n    Session.load()\n  end\nend\n",
    );
    write(
        &gong_root.join("lib/gong/tools/read.ex"),
        "defmodule Gong.Tools.Read do\n  alias Gong.Data.Session\n  def run, do: Session.load()\nend\n",
    );
    write(
        &gong_root.join("lib/gong/tools/write.ex"),
        "defmodule Gong.Tools.Write do\n  alias Gong.Data.Session\n  def run, do: Session.save()\nend\n",
    );
    write(
        &gong_root.join("lib/gong/provider/anthropic.ex"),
        "defmodule Gong.Provider.Anthropic do\n  alias Gong.Data.Model\n  def run, do: Model.resolve()\nend\n",
    );
    write(
        &gong_root.join("lib/gong/data/session.ex"),
        "defmodule Gong.Data.Session do\n  def load, do: :ok\n  def save, do: :ok\nend\n",
    );
    write(
        &gong_root.join("lib/gong/data/model.ex"),
        "defmodule Gong.Data.Model do\n  def resolve, do: :ok\nend\n",
    );

    let gong_ast = gong_root.join("ast.json");
    let gong_extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &gong_root.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &gong_ast.to_string_lossy(),
        ])
        .output()
        .expect("run gong extract");
    assert!(
        gong_extract.status.success(),
        "gong extract failed: stdout={}, stderr={}",
        String::from_utf8_lossy(&gong_extract.stdout),
        String::from_utf8_lossy(&gong_extract.stderr)
    );

    let gong_seed = gong_root.join("seed.yaml");
    write(
        &gong_seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: AGENT\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/agent.ex\"]\n  - module_id: PROMPT\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/prompt/**\"]\n  - module_id: TOOLS\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/tools/**\"]\n  - module_id: INFRA\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/application.ex\"]\n  - module_id: PROVIDERS\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/provider/**\"]\n  - module_id: DATA\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/data/**\"]\nrelations_expected: []\n",
    );

    let gong_out = gong_root.join("out");
    let gong_matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &gong_seed.to_string_lossy(),
            "--ast-file",
            &gong_ast.to_string_lossy(),
            "--out-dir",
            &gong_out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .output()
        .expect("run gong matrix");
    assert!(
        gong_matrix.status.success(),
        "gong matrix failed: stdout={}, stderr={}",
        String::from_utf8_lossy(&gong_matrix.stdout),
        String::from_utf8_lossy(&gong_matrix.stderr)
    );
    let gong_predicted = load_predictions(&gong_out.join("v3.relation-classification.json"));

    write(
        &pi_root.join("apps/api/src/app.module.ts"),
        "import { Module } from '@nestjs/common';\nimport { UserModule } from '../../user/src/user.module';\nimport { PrismaModule } from '../../prisma/src/prisma.module';\nimport { AuthModule } from '../../auth/src/auth.module';\n\n@Module({\n  imports: [UserModule, PrismaModule, AuthModule],\n})\nexport class AppModule {}\n",
    );
    write(
        &pi_root.join("apps/api/src/health.ts"),
        "import { Logger } from '../../common/src/logger';\n\nexport function health() {\n  return Logger.ok();\n}\n",
    );
    write(
        &pi_root.join("apps/user/src/user.module.ts"),
        "import { Module } from '@nestjs/common';\nimport { UserService } from './user.service';\n\n@Module({ providers: [UserService] })\nexport class UserModule {}\n",
    );
    write(
        &pi_root.join("apps/user/src/user.service.ts"),
        "import { Logger } from '../../common/src/logger';\n\nexport class UserService {\n  run() {\n    return Logger.ok();\n  }\n}\n",
    );
    write(
        &pi_root.join("apps/prisma/src/prisma.module.ts"),
        "import { Module } from '@nestjs/common';\nimport { PrismaService } from './prisma.service';\n\n@Module({ providers: [PrismaService] })\nexport class PrismaModule {}\n",
    );
    write(
        &pi_root.join("apps/prisma/src/prisma.service.ts"),
        "import { Logger } from '../../common/src/logger';\n\nexport class PrismaService {\n  ping() {\n    return Logger.ok();\n  }\n}\n",
    );
    write(
        &pi_root.join("apps/auth/src/auth.module.ts"),
        "import { Module } from '@nestjs/common';\nimport { AuthService } from './auth.service';\n\n@Module({ providers: [AuthService] })\nexport class AuthModule {}\n",
    );
    write(
        &pi_root.join("apps/auth/src/auth.service.ts"),
        "import { Logger } from '../../common/src/logger';\n\nexport class AuthService {\n  verify() {\n    return Logger.ok();\n  }\n}\n",
    );
    write(
        &pi_root.join("apps/common/src/logger.ts"),
        "export const Logger = {\n  ok() {\n    return 'ok';\n  },\n};\n",
    );

    let pi_ast = pi_root.join("ast.json");
    let pi_extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &pi_root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &pi_ast.to_string_lossy(),
        ])
        .output()
        .expect("run pi-mono extract");
    assert!(
        pi_extract.status.success(),
        "pi-mono extract failed: stdout={}, stderr={}",
        String::from_utf8_lossy(&pi_extract.stdout),
        String::from_utf8_lossy(&pi_extract.stderr)
    );

    let pi_seed = pi_root.join("seed.yaml");
    write(
        &pi_seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: API\n    precedence: 10\n    path_rules:\n      include: [\"apps/api/src/**\"]\n  - module_id: USER\n    precedence: 10\n    path_rules:\n      include: [\"apps/user/src/**\"]\n  - module_id: PRISMA\n    precedence: 10\n    path_rules:\n      include: [\"apps/prisma/src/**\"]\n  - module_id: AUTH\n    precedence: 10\n    path_rules:\n      include: [\"apps/auth/src/**\"]\n  - module_id: COMMON\n    precedence: 10\n    path_rules:\n      include: [\"apps/common/src/**\"]\nrelations_expected: []\n",
    );

    let pi_out = pi_root.join("out");
    let pi_matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &pi_seed.to_string_lossy(),
            "--ast-file",
            &pi_ast.to_string_lossy(),
            "--out-dir",
            &pi_out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .output()
        .expect("run pi-mono matrix");
    assert!(
        pi_matrix.status.success(),
        "pi-mono matrix failed: stdout={}, stderr={}",
        String::from_utf8_lossy(&pi_matrix.stdout),
        String::from_utf8_lossy(&pi_matrix.stderr)
    );
    let pi_predicted = load_predictions(&pi_out.join("v3.relation-classification.json"));

    let labeled_edges = vec![
        LabeledEdge {
            project: "gong",
            caller: "AGENT",
            callee: "TOOLS",
            expected_call_type: "framework_injection",
        },
        LabeledEdge {
            project: "gong",
            caller: "INFRA",
            callee: "PROVIDERS",
            expected_call_type: "external_registration",
        },
        LabeledEdge {
            project: "gong",
            caller: "AGENT",
            callee: "PROMPT",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "gong",
            caller: "AGENT",
            callee: "DATA",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "gong",
            caller: "INFRA",
            callee: "DATA",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "gong",
            caller: "TOOLS",
            callee: "DATA",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "gong",
            caller: "PROVIDERS",
            callee: "DATA",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "API",
            callee: "USER",
            expected_call_type: "framework_injection",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "API",
            callee: "PRISMA",
            expected_call_type: "framework_injection",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "API",
            callee: "AUTH",
            expected_call_type: "framework_injection",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "API",
            callee: "COMMON",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "USER",
            callee: "COMMON",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "PRISMA",
            callee: "COMMON",
            expected_call_type: "direct_call",
        },
        LabeledEdge {
            project: "pi-mono",
            caller: "AUTH",
            callee: "COMMON",
            expected_call_type: "direct_call",
        },
    ];

    let mut correct = 0usize;
    let mut project_summary: HashMap<String, (usize, usize)> = HashMap::new();
    let mut expected_by_type: HashMap<String, usize> = HashMap::new();
    let mut predicted_by_type: HashMap<String, usize> = HashMap::new();
    let mut mismatches = Vec::new();

    for sample in &labeled_edges {
        let predicted = match sample.project {
            "gong" => gong_predicted.get(&(sample.caller.to_string(), sample.callee.to_string())),
            "pi-mono" => pi_predicted.get(&(sample.caller.to_string(), sample.callee.to_string())),
            _ => None,
        }
        .map(|s| s.as_str())
        .unwrap_or("missing");

        let entry = project_summary
            .entry(sample.project.to_string())
            .or_insert((0usize, 0usize));
        entry.1 += 1;

        *expected_by_type
            .entry(sample.expected_call_type.to_string())
            .or_insert(0) += 1;
        *predicted_by_type.entry(predicted.to_string()).or_insert(0) += 1;

        if predicted == sample.expected_call_type {
            correct += 1;
            entry.0 += 1;
        } else {
            mismatches.push(format!(
                "{}:{}->{} expected={} got={}",
                sample.project, sample.caller, sample.callee, sample.expected_call_type, predicted
            ));
        }
    }

    let total = labeled_edges.len();
    let accuracy = correct as f64 / total as f64;
    assert!(
        accuracy > 0.90,
        "accuracy summary (Gong + PI-Mono labeled samples): {}/{} = {:.2}%, project_summary={:?}, expected_by_type={:?}, predicted_by_type={:?}, mismatches={:?}",
        correct,
        total,
        accuracy * 100.0,
        project_summary,
        expected_by_type,
        predicted_by_type,
        mismatches
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_matrix_detect_injection_is_stable_in_parallel_runs() {
    let mut handles = Vec::new();

    for i in 0..4 {
        handles.push(thread::spawn(move || {
            let root = temp_dir(&format!("bcc_arch_injection_parallel_{}", i));
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
                .expect("run matrix in parallel");
            assert!(status.success());

            let report_path = out.join("v3.relation-classification.json");
            assert!(report_path.exists());
            let payload: Value =
                serde_json::from_str(&fs::read_to_string(&report_path).expect("read report"))
                    .expect("parse report");
            let rows = payload.as_array().expect("classification rows");
            assert!(rows.iter().any(|row| {
                row.get("caller") == Some(&Value::String("INFRA".to_string()))
                    && row.get("callee") == Some(&Value::String("PROVIDERS".to_string()))
                    && row.get("call_type")
                        == Some(&Value::String("external_registration".to_string()))
            }));

            let _ = fs::remove_dir_all(&root);
        }));
    }

    for handle in handles {
        handle.join().expect("thread should not panic");
    }
}

#[test]
fn arch_matrix_handles_deep_relation_chain_with_injection_hints() {
    let depth = 128usize;
    let root = temp_dir("bcc_arch_injection_deep_chain");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let out = root.join("out");

    let mut seed_yaml = String::from("version: v3\nsource_of_truth: test\nmodules:\n");
    for i in 0..=depth {
        seed_yaml.push_str(&format!(
            "  - module_id: M{}\n    precedence: 10\n    path_rules:\n      include: [\"src/m{}/**\"]\n",
            i, i
        ));
    }
    seed_yaml.push_str("relations_expected: []\n");
    write(&seed, &seed_yaml);

    let mut records = Vec::new();
    for i in 0..=depth {
        if i < depth {
            records.push(format!(
                r#"{{
      "sourcePath": "src/m{0}/file.ts",
      "localDependencies": ["src/m{1}/file.ts"],
      "localCallTargets": [],
      "relationHints": [
        {{
          "target": "src/m{1}/file.ts",
          "call_type_hint": "framework_injection",
          "via": "@Module.imports",
          "confidence": 0.95,
          "detector": "typescript.nest.module",
          "reason": "deep chain"
        }}
      ]
    }}"#,
                i,
                i + 1
            ));
        } else {
            records.push(format!(
                r#"{{
      "sourcePath": "src/m{0}/file.ts",
      "localDependencies": [],
      "localCallTargets": []
    }}"#,
                i
            ));
        }
    }
    let ast_json = format!(
        "{{\n  \"source_count\": {},\n  \"records\": [\n{}\n  ]\n}}",
        depth + 1,
        records.join(",\n")
    );
    write(&ast, &ast_json);

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
        .expect("run matrix deep chain");
    assert!(status.success());

    let payload: Value = serde_json::from_str(
        &fs::read_to_string(out.join("v3.relation-classification.json")).expect("read report"),
    )
    .expect("parse report");
    let rows = payload.as_array().expect("classification rows");
    assert_eq!(rows.len(), depth);
    assert!(
        rows.iter()
            .all(|row| row.get("call_type")
                == Some(&Value::String("framework_injection".to_string())))
    );

    let _ = fs::remove_dir_all(&root);
}
