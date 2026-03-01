use crate::spec::ModuleSpec;
use std::fs;

/// 解析 YAML 文件为 ModuleSpec
pub fn parse_yaml(path: &str) -> Result<ModuleSpec, String> {
    let content = fs::read_to_string(path).map_err(|e| format!("cannot read '{}': {}", path, e))?;
    let spec: ModuleSpec = serde_yaml::from_str(&content).map_err(|e| {
        // serde_yaml 错误信息包含行号
        format!("{}", e)
    })?;
    Ok(spec)
}

/// 校验 ModuleSpec，返回 (errors, warnings)
pub fn validate(spec: &ModuleSpec, path: &str) -> (Vec<String>, Vec<String>) {
    let mut errors = Vec::new();
    let mut warnings = Vec::new();

    // 必须项
    if spec.module.name.is_empty() {
        errors.push(format!("[ERROR] {}:module.name: must not be empty", path));
    }

    for (i, port) in spec.ports.iter().enumerate() {
        if port.name.is_empty() {
            errors.push(format!(
                "[ERROR] {}:ports[{}].name: must not be empty",
                path, i
            ));
        }
        if port.kind.is_none() {
            errors.push(format!(
                "[ERROR] {}:ports[{}].kind: missing required field `kind`",
                path, i
            ));
        }

        // 推荐项
        if port.errors.is_empty() {
            warnings.push(format!(
                "[WARNING] {}:ports[{}].errors: no error codes defined",
                path, i
            ));
        }
        if port.input.is_none() {
            warnings.push(format!(
                "[WARNING] {}:ports[{}].input: no input type defined",
                path, i
            ));
        }
        if port.output.is_none() {
            warnings.push(format!(
                "[WARNING] {}:ports[{}].output: no output type defined",
                path, i
            ));
        }
    }

    // relation callee 存在性（跨模块引用，PoC 阶段只做 warning）
    for rel in &spec.relations {
        // 当前只有单文件，无法验证 callee 是否真实存在，标记 warning
        warnings.push(format!(
            "[WARNING] {}:relations.callee: '{}' reference cannot be verified in single-file mode",
            path, rel.callee
        ));
    }

    (errors, warnings)
}

/// compile 命令入口
pub fn run(
    path: &str,
    dry_run: bool,
    emit_ast: bool,
    passes: &str,
    output: Option<&str>,
    force: bool,
    verbose: bool,
) {
    // Step 1: 解析 YAML
    let spec = match parse_yaml(path) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("{}", e);
            std::process::exit(1);
        }
    };

    // Step 2: 校验
    let (errors, warnings) = validate(&spec, path);
    for w in &warnings {
        eprintln!("{}", w);
    }
    for e in &errors {
        eprintln!("{}", e);
    }
    if !errors.is_empty() {
        std::process::exit(1);
    }

    // dry-run 模式：只输出解析结果
    if dry_run {
        println!("module: {}", spec.module.name);
        println!("ports: {}", spec.ports.len());
        for port in &spec.ports {
            println!(
                "  - {} ({})",
                port.name,
                port.kind.as_deref().unwrap_or("unknown")
            );
        }
        println!("relations: {}", spec.relations.len());
        return;
    }

    // Step 3: 构建 AST
    let ast = crate::compile::ast::build_skeleton(&spec);

    // Step 4: Pass Pipeline
    let ast = crate::compile::passes::apply_passes(ast, &spec, passes);

    // emit-ast 模式：输出 JSON
    if emit_ast {
        let json = serde_json::to_string_pretty(&ast).expect("AST serialization failed");
        println!("{}", json);
        return;
    }

    // Step 5: 序列化为 .ex
    let output_dir = match output {
        Some(d) => d,
        None => {
            eprintln!("[ERROR] --output is required when not using --dry-run or --emit-ast");
            std::process::exit(1);
        }
    };

    crate::compile::emit::emit_to_dir(&ast, &spec, output_dir, force, verbose);
}

pub mod ast;
pub mod emit;
pub mod passes;
