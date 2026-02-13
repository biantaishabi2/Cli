use crate::compile::ast::QuotedAST;
use crate::spec::ModuleSpec;
use std::fs;
use std::path::Path;
use std::process::Command;

/// 将 AST 序列化为 Elixir 源码并写入文件
pub fn emit_to_dir(ast: &[QuotedAST], spec: &ModuleSpec, output_dir: &str, force: bool) {
    let module_name_snake = to_snake_case(&spec.module.name);
    let target_dir = Path::new(output_dir).join(&module_name_snake);

    // 安全检查：目录已存在时拒绝覆盖
    if target_dir.exists() && !force {
        eprintln!(
            "[ERROR] output directory '{}' already exists. Use --force to overwrite.",
            target_dir.display()
        );
        std::process::exit(1);
    }

    fs::create_dir_all(&target_dir).expect("failed to create output directory");

    let output_file = target_dir.join(format!("{}.ex", module_name_snake));

    // 先尝试用 Elixir 序列化
    let source = match emit_via_elixir(ast) {
        Ok(s) => s,
        Err(_) => {
            // Elixir 不可用时，用 Rust 侧简易序列化
            emit_simple(ast, spec)
        }
    };

    fs::write(&output_file, &source).expect("failed to write .ex file");
    println!("Generated: {}", output_file.display());
}

/// 通过 Elixir helper 序列化 AST
fn emit_via_elixir(ast: &[QuotedAST]) -> Result<String, String> {
    let json = serde_json::to_string(ast).map_err(|e| e.to_string())?;

    // 查找 bcc_emit.exs 脚本
    let script_path = find_emit_script()?;

    let output = Command::new("elixir")
        .arg(&script_path)
        .arg("--stdin")
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .and_then(|mut child| {
            use std::io::Write;
            if let Some(ref mut stdin) = child.stdin {
                stdin.write_all(json.as_bytes())?;
            }
            child.wait_with_output()
        })
        .map_err(|e| format!("elixir not available: {}", e))?;

    if output.status.success() {
        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    } else {
        Err(String::from_utf8_lossy(&output.stderr).to_string())
    }
}

fn find_emit_script() -> Result<String, String> {
    // 按优先级查找脚本位置
    let candidates = [
        // 相对于 binary 的位置
        "scripts/bcc_emit.exs",
        // 开发环境
        "compiler/bcc/scripts/bcc_emit.exs",
    ];

    // 获取 binary 所在目录
    if let Ok(exe_path) = std::env::current_exe() {
        if let Some(exe_dir) = exe_path.parent() {
            let script = exe_dir.join("scripts/bcc_emit.exs");
            if script.exists() {
                return Ok(script.to_string_lossy().to_string());
            }
        }
    }

    for c in &candidates {
        if Path::new(c).exists() {
            return Ok(c.to_string());
        }
    }

    Err("bcc_emit.exs not found".into())
}

/// Rust 侧简易序列化（不依赖 Elixir）
fn emit_simple(_ast: &[QuotedAST], spec: &ModuleSpec) -> String {
    let mut out = String::new();

    out.push_str(&format!("defmodule {} do\n", spec.module.name));

    if let Some(ref resp) = spec.module.responsibility {
        out.push_str(&format!("  @moduledoc \"\"\"\n  {}\n  \"\"\"\n\n", resp));
    }

    for port in &spec.ports {
        let kind = port.kind.as_deref().unwrap_or("command");
        let param_names: Vec<String> = port.input.as_ref()
            .map(|m| m.keys().cloned().collect())
            .unwrap_or_default();

        // @spec
        let spec_params: Vec<String> = port.input.as_ref()
            .map(|m| m.values().map(|t| yaml_type_to_elixir_simple(t)).collect())
            .unwrap_or_default();
        let return_type = match kind {
            "query" => "{:ok, map()} | {:error, term()}".to_string(),
            _ => "{:ok, map()} | {:error, term()}".to_string(),
        };
        out.push_str(&format!("  @spec {}({}) :: {}\n", port.name, spec_params.join(", "), return_type));

        // def
        out.push_str(&format!("  def {}({}) do\n", port.name, param_names.join(", ")));
        out.push_str("    {:ok, nil}\n");
        out.push_str("  end\n\n");
    }

    // 去掉最后一个 port 后面的多余空行
    let out = out.trim_end().to_string();
    format!("{}\nend\n", out)
}

fn yaml_type_to_elixir_simple(t: &str) -> String {
    match t {
        "uuid" | "string" => "String.t()".into(),
        "integer" => "integer()".into(),
        "boolean" => "boolean()".into(),
        "map" => "map()".into(),
        "list" => "list()".into(),
        other => format!("{}()", other),
    }
}

fn to_snake_case(s: &str) -> String {
    let mut result = String::new();
    for (i, c) in s.chars().enumerate() {
        if c.is_uppercase() {
            if i > 0 {
                result.push('_');
            }
            result.push(c.to_lowercase().next().unwrap());
        } else {
            result.push(c);
        }
    }
    result
}
