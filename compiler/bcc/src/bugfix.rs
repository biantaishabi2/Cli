use std::collections::HashMap;
use std::fs;
use std::process::Command;
use serde::{Deserialize, Serialize};

// ─── 数据结构 ───────────────────────────────────────────

#[derive(Debug, Clone, PartialEq)]
enum Step { Collect, Context, Generate, Organize }

#[derive(Debug, Serialize, Deserialize)]
struct Inventory {
    meta: InventoryMeta,
    commits: Vec<BugfixCommit>,
}

#[derive(Debug, Serialize, Deserialize)]
struct InventoryMeta {
    repo: String,
    scanned_at: String,
    total_commits: usize,
    bugfix_commits: usize,
    by_grade: HashMap<String, usize>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct BugfixCommit {
    hash: String,
    message: String,
    author: String,
    date: String,
    grade: String,
    module: String,
    tags: Vec<String>,
    changed_files: Vec<ChangedFile>,
    total_lines: usize,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct ChangedFile {
    path: String,
    add: usize,
    del: usize,
    kind: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct ModuleMap {
    mapping: HashMap<String, String>,
    #[serde(default)]
    module_names: HashMap<String, String>,
}

// ─── 自动标签规则 ───────────────────────────────────────

const TAG_RULES: &[(&[&str], &str)] = &[
    (&["xss", "漏洞", "注入", "安全", "越权", "任意访问"], "security"),
    (&["分页", "翻页", "page", "offset"], "pagination"),
    (&["导出", "导入", "下载", "export"], "export"),
    (&["时间", "日期", "时区", "utc", "timezone"], "datetime"),
    (&["空", "null", "为空", "类型", "type"], "null_safety"),
    (&["事务", "commit", "回滚", "rollback"], "transaction"),
    (&["并发", "锁", "重复", "concurrent"], "concurrency"),
];

// ─── 入口 ───────────────────────────────────────────────

pub fn run(
    repo: &str,
    output: &str,
    step: Option<&str>,
    lang: &str,
    branch: Option<&str>,
    path: Option<&str>,
    grade: &str,
    keywords: &str,
    module_map: Option<&str>,
    prompt_template: Option<&str>,
    limit: Option<usize>,
    force: bool,
    coverage_report: Option<&str>,
) {
    let target = match step {
        Some("collect") | Some("c") => Step::Collect,
        Some("context") | Some("x") => Step::Context,
        Some("generate") | Some("g") => Step::Generate,
        Some("organize") | Some("o") => Step::Organize,
        None => Step::Organize, // 默认全跑
        Some(s) => {
            eprintln!("unknown step: '{}'. Valid: collect(c), context(x), generate(g), organize(o)", s);
            std::process::exit(1);
        }
    };

    // 验证语言
    let extensions = lang_extensions(lang);
    if extensions.is_empty() {
        eprintln!("unsupported language: '{}'. Valid: php, elixir, typescript", lang);
        std::process::exit(1);
    }

    let grades: Vec<&str> = grade.split(',').map(|s| s.trim()).collect();

    // 确保输出目录存在
    fs::create_dir_all(output).ok();

    // collect
    collect(repo, output, branch, path, keywords, module_map, &grades, limit, force, lang);
    if target == Step::Collect { return; }

    // context
    context(output, repo, &grades, limit, force, lang);
    if target == Step::Context { return; }

    // generate
    generate(output, prompt_template, force, limit);
    if target == Step::Generate { return; }

    // organize
    organize(output, coverage_report);
}

/// 语言 → 文件扩展名列表
fn lang_extensions(lang: &str) -> Vec<&'static str> {
    match lang {
        "php" => vec![".php"],
        "elixir" => vec![".ex", ".exs"],
        "typescript" | "ts" => vec![".ts", ".tsx"],
        _ => vec![],
    }
}

/// 根据语言判断是否为后端业务文件
fn is_backend_file(path: &str, lang: &str) -> bool {
    let exts = lang_extensions(lang);
    let has_ext = exts.iter().any(|ext| path.ends_with(ext));
    if !has_ext { return false; }

    match lang {
        "php" => {
            let lower = path.to_lowercase();
            lower.contains("controller") || lower.contains("model")
                || lower.contains("trait") || lower.contains("service")
                || lower.contains("logic") || lower.contains("provider")
        }
        "elixir" => {
            // Elixir 项目：lib/ 下的业务代码（排除 test/、deps/、_build/）
            path.starts_with("lib/") || path.contains("/lib/")
        }
        "typescript" | "ts" => {
            let lower = path.to_lowercase();
            (lower.contains("src/") || lower.contains("lib/"))
                && !lower.contains("node_modules") && !lower.contains(".test.")
                && !lower.contains(".spec.")
        }
        _ => false,
    }
}

// ─── collect ────────────────────────────────────────────

fn collect(
    repo: &str,
    output: &str,
    branch: Option<&str>,
    path: Option<&str>,
    keywords: &str,
    module_map_path: Option<&str>,
    grades: &[&str],
    limit: Option<usize>,
    force: bool,
    lang: &str,
) {
    let inventory_path = format!("{}/inventory.json", output);
    if !force && std::path::Path::new(&inventory_path).exists() {
        eprintln!("[collect] skip: {} already exists (use --force to overwrite)", inventory_path);
        return;
    }

    // 加载 module_map
    let module_map = module_map_path.and_then(|p| {
        fs::read_to_string(p).ok().and_then(|s| serde_json::from_str::<ModuleMap>(&s).ok())
    });

    // 确定分支：指定 > 当前分支
    let branch_ref = branch.map(|b| b.to_string()).unwrap_or_else(|| {
        Command::new("git")
            .args(["-C", repo, "rev-parse", "--abbrev-ref", "HEAD"])
            .output()
            .ok()
            .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
            .unwrap_or_else(|| "HEAD".to_string())
    });

    let mut all_commits: HashMap<String, BugfixCommit> = HashMap::new();
    let fmt = "--format=__COMMIT__%H%n__MSG__%s%n__AUTHOR__%an%n__DATE__%ai";

    if let Some(file_path) = path {
        // 按文件路径扫描：git log <branch> --no-merges --numstat -- <path>
        eprintln!("[collect] scanning by path: {} (branch: {})", file_path, branch_ref);
        let git_output = Command::new("git")
            .args([
                "-C", repo,
                "log", &branch_ref, "--no-merges",
                "--numstat", fmt,
                "--", file_path,
            ])
            .output();

        match git_output {
            Ok(out) => {
                let stdout = String::from_utf8_lossy(&out.stdout);
                parse_git_log(&stdout, &module_map, &mut all_commits);
            }
            Err(e) => {
                eprintln!("[collect] git log failed for path '{}': {}", file_path, e);
            }
        }
    } else {
        // 按关键字扫描：git log <branch> --no-merges --grep=<kw>
        let kw_list: Vec<&str> = keywords.split(',').map(|s| s.trim()).collect();
        eprintln!("[collect] scanning by keywords: {:?} (branch: {})", kw_list, branch_ref);

        for kw in &kw_list {
            let git_output = Command::new("git")
                .args([
                    "-C", repo,
                    "log", &branch_ref, "--no-merges",
                    &format!("--grep={}", kw), "-i",
                    "--numstat", fmt,
                ])
                .output();

            match git_output {
                Ok(out) => {
                    let stdout = String::from_utf8_lossy(&out.stdout);
                    parse_git_log(&stdout, &module_map, &mut all_commits);
                }
                Err(e) => {
                    eprintln!("[collect] git log failed for keyword '{}': {}", kw, e);
                }
            }
        }
    }

    // 过滤 + 分级 + 打标签
    let mut commits: Vec<BugfixCommit> = all_commits.into_values()
        .filter(|c| c.changed_files.iter().any(|f| is_backend_file(&f.path, lang)))
        .collect();

    // 分级
    for commit in &mut commits {
        commit.grade = classify_grade(commit.total_lines);
        commit.tags = auto_tag(&commit.message);
    }

    // 按 grade 筛选
    commits.retain(|c| grades.contains(&c.grade.as_str()));

    // 排序（按日期降序）
    commits.sort_by(|a, b| b.date.cmp(&a.date));

    // 限制数量
    if let Some(max) = limit {
        commits.truncate(max);
    }

    // 统计
    let mut by_grade: HashMap<String, usize> = HashMap::new();
    for c in &commits {
        *by_grade.entry(c.grade.clone()).or_insert(0) += 1;
    }

    // 获取被扫描分支的总 commit 数
    let total_commits = Command::new("git")
        .args(["-C", repo, "rev-list", &branch_ref, "--count"])
        .output()
        .ok()
        .and_then(|o| String::from_utf8_lossy(&o.stdout).trim().parse::<usize>().ok())
        .unwrap_or(0);

    let inventory = Inventory {
        meta: InventoryMeta {
            repo: repo.to_string(),
            scanned_at: chrono_now(),
            total_commits,
            bugfix_commits: commits.len(),
            by_grade,
        },
        commits,
    };

    let json = serde_json::to_string_pretty(&inventory).expect("JSON serialize failed");
    fs::write(&inventory_path, &json).expect("failed to write inventory.json");
    eprintln!("[collect] wrote {} ({} commits)", inventory_path, inventory.meta.bugfix_commits);
}

/// 解析 git log --numstat 输出
fn parse_git_log(
    output: &str,
    module_map: &Option<ModuleMap>,
    commits: &mut HashMap<String, BugfixCommit>,
) {
    let mut current: Option<BugfixCommit> = None;

    for line in output.lines() {
        if let Some(hash) = line.strip_prefix("__COMMIT__") {
            // 保存上一个
            if let Some(c) = current.take() {
                if !commits.contains_key(&c.hash) {
                    commits.insert(c.hash.clone(), c);
                }
            }
            current = Some(BugfixCommit {
                hash: hash.to_string(),
                message: String::new(),
                author: String::new(),
                date: String::new(),
                grade: "C".into(),
                module: String::new(),
                tags: Vec::new(),
                changed_files: Vec::new(),
                total_lines: 0,
            });
        } else if let Some(msg) = line.strip_prefix("__MSG__") {
            if let Some(ref mut c) = current {
                c.message = msg.to_string();
            }
        } else if let Some(author) = line.strip_prefix("__AUTHOR__") {
            if let Some(ref mut c) = current {
                c.author = author.to_string();
            }
        } else if let Some(date) = line.strip_prefix("__DATE__") {
            if let Some(ref mut c) = current {
                // 只保留日期部分 "2025-12-22 10:30:00 +0800" → "2025-12-22"
                c.date = date.split_whitespace().next().unwrap_or(date).to_string();
            }
        } else if !line.trim().is_empty() && !line.starts_with("__") {
            // numstat 行: "10\t5\tpath/to/file.php"
            let parts: Vec<&str> = line.split('\t').collect();
            if parts.len() == 3 {
                let add = parts[0].parse::<usize>().unwrap_or(0);
                let del = parts[1].parse::<usize>().unwrap_or(0);
                let path = parts[2].to_string();
                let kind = classify_file_kind(&path);

                if let Some(ref mut c) = current {
                    c.total_lines += add + del;
                    // 用第一个 backend 文件确定 module
                    if c.module.is_empty() {
                        c.module = resolve_module(&path, module_map);
                    }
                    c.changed_files.push(ChangedFile { path, add, del, kind });
                }
            }
        }
    }
    // 最后一个
    if let Some(c) = current {
        if !commits.contains_key(&c.hash) {
            commits.insert(c.hash.clone(), c);
        }
    }
}


fn classify_file_kind(path: &str) -> String {
    let lower = path.to_lowercase();
    if lower.contains("controller") { "controller".into() }
    else if lower.contains("model") { "model".into() }
    else if lower.contains("trait") { "trait".into() }
    else if lower.contains("service") { "service".into() }
    else { "other".into() }
}

fn classify_grade(total_lines: usize) -> String {
    if total_lines <= 10 { "A".into() }
    else if total_lines <= 50 { "B".into() }
    else { "C".into() }
}

fn auto_tag(message: &str) -> Vec<String> {
    let lower = message.to_lowercase();
    let mut tags = Vec::new();
    for (keywords, tag) in TAG_RULES {
        if keywords.iter().any(|kw| lower.contains(kw)) {
            tags.push(tag.to_string());
        }
    }
    tags
}

fn resolve_module(path: &str, module_map: &Option<ModuleMap>) -> String {
    if let Some(ref mm) = module_map {
        // 尝试匹配 mapping 中的前缀
        for (prefix, module) in &mm.mapping {
            if path.contains(prefix) {
                return module.clone();
            }
        }
    }
    // 默认策略：从文件名推导模块名
    // 例如 OrderController.php → order, PayService.php → pay
    // lib/shop/order/cart.ex → order, src/order/service.ts → order
    module_from_filename(path)
}

/// 从文件路径推导模块名（无 module_map 时的智能默认）
fn module_from_filename(path: &str) -> String {
    // 取文件名（不含扩展名）
    let filename = path.rsplit('/').next().unwrap_or(path);
    let stem = filename.split('.').next().unwrap_or(filename);

    // 去掉常见后缀（Controller, Service, Trait, Model, Handler, Command, Query, Helper, Test）
    let suffixes = ["Controller", "Service", "Trait", "Model", "Handler",
                    "Command", "Query", "Helper", "Test", "Spec", "Factory",
                    "_controller", "_service", "_test", "_handler", "_model"];
    let mut name = stem.to_string();
    for suffix in &suffixes {
        if name.ends_with(suffix) && name.len() > suffix.len() {
            name = name[..name.len() - suffix.len()].to_string();
            break;
        }
    }

    // 转为 snake_case（驼峰 → 下划线小写）
    let snake = camel_to_snake(&name);

    // 如果结果太短或太通用，用路径中的上下文补充
    if snake.is_empty() || snake == "base" || snake == "index" || snake == "app" || snake == "main" {
        // 尝试从路径中取有意义的目录名
        // 例如 lib/shop/order/cart.ex → order, app/controllers/OrderController.php → order
        let parts: Vec<&str> = path.split('/').collect();
        // 跳过通用目录名
        let skip = ["lib", "app", "src", "controllers", "models", "services",
                     "traits", "handlers", "commands", "queries", "helpers",
                     "test", "tests", "spec", "shop", "web"];
        for part in parts.iter().rev().skip(1) {
            let lower = part.to_lowercase();
            if !skip.contains(&lower.as_str()) && !lower.is_empty() {
                return lower;
            }
        }
        return snake;
    }

    snake
}

/// 驼峰转下划线小写：OrderItem → order_item
fn camel_to_snake(s: &str) -> String {
    let mut result = String::new();
    for (i, c) in s.chars().enumerate() {
        if c.is_uppercase() && i > 0 {
            result.push('_');
        }
        result.push(c.to_ascii_lowercase());
    }
    result
}

fn chrono_now() -> String {
    // 简单实现：调用 date 命令
    Command::new("date")
        .args(["+%Y-%m-%dT%H:%M:%S%z"])
        .output()
        .ok()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_else(|| "unknown".to_string())
}

// ─── context ────────────────────────────────────────────

fn context(output: &str, repo: &str, _grades: &[&str], _limit: Option<usize>, force: bool, lang: &str) {
    let inventory_path = format!("{}/inventory.json", output);
    let inventory_str = match fs::read_to_string(&inventory_path) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("[context] cannot read {}: {}", inventory_path, e);
            std::process::exit(1);
        }
    };
    let inventory: Inventory = serde_json::from_str(&inventory_str)
        .expect("failed to parse inventory.json");

    let contexts_dir = format!("{}/contexts", output);
    fs::create_dir_all(&contexts_dir).ok();

    let mut processed = 0;
    let mut skipped = 0;

    for commit in &inventory.commits {
        let out_path = format!("{}/{}.json", contexts_dir, commit.hash);
        if !force && std::path::Path::new(&out_path).exists() {
            eprintln!("[context] skip: {}.json already exists", commit.hash);
            skipped += 1;
            continue;
        }

        // git diff HASH^..HASH
        let diff_output = Command::new("git")
            .args(["-C", repo, "diff", &format!("{}^..{}", commit.hash, commit.hash)])
            .output();

        let raw_diff = match diff_output {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(e) => {
                eprintln!("[context] git diff failed for {}: {}", commit.hash, e);
                continue;
            }
        };

        // 对每个源码文件提取函数上下文
        let exts = lang_extensions(lang);
        let mut diffs = Vec::new();
        for file in &commit.changed_files {
            if !exts.iter().any(|ext| file.path.ends_with(ext)) { continue; }

            // 获取修复后文件
            let after_content = git_show(repo, &commit.hash, &file.path);
            // 获取修复前文件
            let before_content = git_show(repo, &format!("{}^", commit.hash), &file.path);

            // 解析 diff hunks
            let hunks = parse_diff_hunks(&raw_diff, &file.path, &before_content, &after_content, lang);

            diffs.push(serde_json::json!({
                "file": file.path,
                "raw_diff": extract_file_diff(&raw_diff, &file.path),
                "hunks": hunks,
            }));
        }

        let context_json = serde_json::json!({
            "hash": commit.hash,
            "message": commit.message,
            "grade": commit.grade,
            "module": commit.module,
            "tags": commit.tags,
            "diffs": diffs,
        });

        let json_str = serde_json::to_string_pretty(&context_json).unwrap();
        fs::write(&out_path, &json_str).expect("failed to write context json");
        processed += 1;
    }

    eprintln!("[context] processed: {}, skipped: {}", processed, skipped);
}

fn git_show(repo: &str, rev: &str, path: &str) -> String {
    Command::new("git")
        .args(["-C", repo, "show", &format!("{}:{}", rev, path)])
        .output()
        .ok()
        .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
        .unwrap_or_default()
}

/// 从完整 diff 中提取单个文件的 diff 部分
fn extract_file_diff(full_diff: &str, file_path: &str) -> String {
    let mut result = String::new();
    let mut in_file = false;

    for line in full_diff.lines() {
        if line.starts_with("diff --git ") {
            in_file = line.contains(file_path);
        }
        if in_file {
            result.push_str(line);
            result.push('\n');
        }
    }
    result
}

/// 从 file diff 中提取改动行号（after 侧）
fn parse_changed_line_numbers(file_diff: &str) -> Vec<usize> {
    let mut changed = Vec::new();
    let mut current_line: usize = 0;

    for line in file_diff.lines() {
        if line.starts_with("@@ ") {
            // 解析 @@ -a,b +c,d @@ 中的 +c
            if let Some(plus_part) = line.split('+').nth(1) {
                let num_str = plus_part.split(|c: char| !c.is_ascii_digit()).next().unwrap_or("0");
                current_line = num_str.parse::<usize>().unwrap_or(0);
            }
        } else if line.starts_with('+') && !line.starts_with("+++") {
            // 新增行
            changed.push(current_line);
            current_line += 1;
        } else if line.starts_with('-') && !line.starts_with("---") {
            // 删除行不计入 after 行号
        } else {
            // 上下文行
            current_line += 1;
        }
    }
    changed
}

/// 解析 diff hunks，定位改动行所在的函数
fn parse_diff_hunks(
    full_diff: &str,
    file_path: &str,
    before_content: &str,
    after_content: &str,
    lang: &str,
) -> Vec<serde_json::Value> {
    let mut hunks = Vec::new();

    if after_content.is_empty() { return hunks; }

    // 提取本文件的 diff 并解析改动行号
    let file_diff = extract_file_diff(full_diff, file_path);
    let all_changed_lines = parse_changed_line_numbers(&file_diff);

    // 根据语言调度到对应的 extract 模块获取函数列表
    let after_record = match lang {
        "php" => crate::extract::php::extract(after_content, file_path),
        "elixir" => crate::extract::elixir::extract(after_content, file_path),
        "typescript" | "ts" => {
            let ts_lang = if file_path.ends_with(".tsx") { "tsx" } else { "typescript" };
            crate::extract::typescript::extract(after_content, file_path, ts_lang)
        }
        _ => return hunks,
    };

    // 对每个 export 函数，提取函数体并比较 before/after
    for export in &after_record.exports {
        let before_func = extract_function_body(before_content, &export.name);
        let after_func = extract_function_body(after_content, &export.name);

        if before_func != after_func && !after_func.is_empty() {
            // 计算函数范围内的改动行号
            let func_start = export.line;
            let func_end = func_start + after_func.lines().count();
            let func_changed: Vec<usize> = all_changed_lines.iter()
                .filter(|&&ln| ln >= func_start && ln < func_end)
                .copied()
                .collect();

            hunks.push(serde_json::json!({
                "function_name": export.name,
                "function_line": export.line,
                "changed_lines": func_changed,
                "before_function": before_func,
                "after_function": after_func,
            }));
        }
    }

    hunks
}

// ─── generate ──────────────────────────────────────────

fn generate(output: &str, prompt_template: Option<&str>, force: bool, limit: Option<usize>) {
    let contexts_dir = format!("{}/contexts", output);
    let specs_dir = format!("{}/specs", output);
    fs::create_dir_all(&specs_dir).ok();

    // 加载 prompt 模板
    let template = load_prompt_template(prompt_template);

    // 检查 codex 是否可用
    let codex_available = Command::new("codex")
        .args(["--version"])
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false);

    if !codex_available {
        eprintln!("[generate] codex not found, falling back to prompt-only mode");
    }

    // 扫描 contexts/ 下所有 JSON
    let entries = match fs::read_dir(&contexts_dir) {
        Ok(entries) => entries,
        Err(e) => {
            eprintln!("[generate] cannot read {}: {}", contexts_dir, e);
            std::process::exit(1);
        }
    };

    let mut processed = 0;
    let mut skipped = 0;
    let mut failed = 0;

    for entry in entries {
        let entry = match entry {
            Ok(e) => e,
            Err(_) => continue,
        };

        let path = entry.path();
        if path.extension().map_or(true, |ext| ext != "json") {
            continue;
        }

        let stem = path.file_stem().unwrap().to_string_lossy().to_string();
        let out_path = format!("{}/{}.json", specs_dir, stem);

        if !force && std::path::Path::new(&out_path).exists() {
            skipped += 1;
            continue;
        }

        // limit 检查（只计数实际处理的，不计 skip 的）
        if let Some(max) = limit {
            if processed + failed >= max {
                eprintln!("[generate] reached limit: {}", max);
                break;
            }
        }

        // 读取 context JSON
        let context_str = match fs::read_to_string(&path) {
            Ok(s) => s,
            Err(e) => {
                eprintln!("[generate] cannot read {:?}: {}", path, e);
                failed += 1;
                continue;
            }
        };

        // 构建 prompt
        let prompt = template.replace("{context_json}", &context_str);

        if codex_available {
            // 通过 stdin 传入 prompt（避免大 JSON 超 ARG_MAX）
            let tmp_out = format!("{}/{}.tmp", specs_dir, stem);

            // 查找 output-schema 文件
            let schema_path = find_schema_file();
            let mut args = vec![
                "exec".to_string(),
                "--full-auto".to_string(),
                "--ephemeral".to_string(),
                "-o".to_string(), tmp_out.clone(),
            ];
            if let Some(ref sp) = schema_path {
                args.push("--output-schema".to_string());
                args.push(sp.clone());
            }

            let result = Command::new("codex")
                .args(&args)
                .stdin(std::process::Stdio::piped())
                .stdout(std::process::Stdio::piped())
                .stderr(std::process::Stdio::piped())
                .spawn()
                .and_then(|mut child| {
                    if let Some(mut stdin) = child.stdin.take() {
                        use std::io::Write;
                        stdin.write_all(prompt.as_bytes()).ok();
                    }
                    child.wait_with_output()
                });

            match result {
                Ok(o) if o.status.success() => {
                    // 读取 codex 输出，提取 JSON
                    let content = fs::read_to_string(&tmp_out).unwrap_or_default();
                    let raw = if content.is_empty() {
                        String::from_utf8_lossy(&o.stdout).to_string()
                    } else {
                        content
                    };

                    let json_str = extract_json_block(&raw);
                    if json_str.trim().is_empty() {
                        eprintln!("[generate] empty output for {}", stem);
                        failed += 1;
                    } else {
                        // 校验 JSON 格式
                        match serde_json::from_str::<serde_json::Value>(&json_str) {
                            Ok(val) => {
                                let pretty = serde_json::to_string_pretty(&val).unwrap();
                                fs::write(&out_path, &pretty).ok();
                                processed += 1;
                            }
                            Err(e) => {
                                eprintln!("[generate] invalid JSON for {}: {}", stem, e);
                                // 保存原始输出供调试
                                let raw_path = format!("{}/{}.raw.txt", specs_dir, stem);
                                fs::write(&raw_path, &json_str).ok();
                                failed += 1;
                            }
                        }
                    }
                    fs::remove_file(&tmp_out).ok();
                }
                Ok(o) => {
                    let stderr = String::from_utf8_lossy(&o.stderr);
                    eprintln!("[generate] codex failed for {}: {}", stem, stderr.trim());
                    fs::remove_file(&tmp_out).ok();
                    failed += 1;
                }
                Err(e) => {
                    eprintln!("[generate] codex exec error for {}: {}", stem, e);
                    failed += 1;
                }
            }
        } else {
            // 降级模式：输出 prompt 文件供手动处理
            let prompts_dir = format!("{}/prompts", output);
            fs::create_dir_all(&prompts_dir).ok();
            let prompt_path = format!("{}/{}.prompt.txt", prompts_dir, stem);
            fs::write(&prompt_path, &prompt).ok();
            processed += 1;
        }
    }

    if codex_available {
        eprintln!("[generate] specs: {}, skipped: {}, failed: {}", processed, skipped, failed);
    } else {
        eprintln!("[generate] prompts written: {}, skipped: {} (feed to LLM manually)", processed, skipped);
    }
}

/// 加载 prompt 模板
fn load_prompt_template(custom_path: Option<&str>) -> String {
    // 优先使用自定义模板
    if let Some(path) = custom_path {
        if let Ok(content) = fs::read_to_string(path) {
            return content;
        }
        eprintln!("[generate] warning: cannot read template '{}', using default", path);
    }

    // 尝试加载内置模板（bcc 二进制所在目录/prompts/bugfix_generate.txt）
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let builtin = dir.join("../../compiler/bcc/prompts/bugfix_generate.txt");
            if let Ok(content) = fs::read_to_string(&builtin) {
                return content;
            }
        }
    }

    // 最终降级：内嵌默认模板
    DEFAULT_PROMPT_TEMPLATE.to_string()
}

/// 查找 output-schema JSON 文件（与 prompt 模板同目录）
fn find_schema_file() -> Option<String> {
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let path = dir.join("../../compiler/bcc/prompts/bugfix_schema.json");
            if path.exists() {
                return Some(path.to_string_lossy().to_string());
            }
        }
    }
    None
}

const DEFAULT_PROMPT_TEMPLATE: &str = r#"你是后端测试分析专家。分析 git bugfix 记录，输出测试规格书 JSON。

## 完整示例

输入是一个"$config_id 从 999 改为 1001"的 bugfix，正确输出如下：

{"source_commit":"abc123","bug_summary":"配置ID错误导致查询命中错误数据","module":"activity","action":"showAction","fix_summary":"将 $config_id 从 999 修正为 1001","test_type":"regression","boundary":"http","test_spec":{"preconditions":[{"what":"用户已登录","involves":"session"},{"what":"数据库存在 config_id=1001 的记录","involves":"db"}],"trigger":{"type":"controller_action","target":"ActivityController","method":"showAction"},"args":{"config_id":{"type":"int","required":true,"allowed":null},"user_id":{"type":"uuid","required":true,"allowed":null}},"outputs":{"result_count":"int","title":"string"},"assertions":[{"what":"查询使用 config_id=1001","type":"db_state","expected":"config_id=1001"}]},"wrong_behavior":"使用 config_id=999 查询","correct_behavior":"使用 config_id=1001 查询","data_dependencies":["db"]}

注意示例中的关键结构：
- 顶层有 boundary 字段
- test_spec 中有 args（每个参数有 type/required/allowed）和 outputs
- preconditions 只有 what 和 involves 两个字段
- trigger 只有 type、target、method 三个字段

## 字段规格

必填顶层字段：source_commit, bug_summary, module, action, fix_summary, test_type, boundary, test_spec, wrong_behavior, correct_behavior, data_dependencies

test_spec 必填子字段：preconditions, trigger, args, outputs, assertions

args 格式：{"参数名": {"type": "string|int|uuid|bool|decimal|datetime|date|array|json", "required": true|false, "allowed": null|[...]}}
outputs 格式：{"变量名": "类型"}
preconditions 格式：[{"what": "描述", "involves": "db|redis|session|config|file"}]
trigger 格式：{"type": "controller_action|...", "target": "类名", "method": "方法名"}

boundary 取值：controller_action→http, service_call→service, event_handler→event, cron_job→cron

如果不是 bugfix：{"skip": true, "reason": "feature|refactor|config", "summary": "简述"}

## 参数提取

从 PHP 代码推导 args：$request->get()、$request->getPost()、函数参数
type：用于 DB 查询→uuid，算术→int，日期→datetime/date，json_decode→json，其他→string
required：有 if(!$param) throw→true，有默认值/??→false
allowed：switch/case 或 in_array→列举，无约束→null

从 PHP 代码推导 outputs：_jsonformat() 数据结构、$this->view->setVar()、return 值

## Bugfix 记录

{context_json}

## 提醒

输出纯 JSON（不要 markdown 包裹）。必须包含 boundary、args、outputs。preconditions 和 trigger 中不要加 key_params 或任何多余字段。参照上面的示例格式。"#;

/// 从 LLM 输出中提取 JSON（去掉 markdown 包裹和非 JSON 文本）
fn extract_json_block(text: &str) -> String {
    let trimmed = text.trim();

    // 如果整体就是合法 JSON，直接返回
    if (trimmed.starts_with('{') && trimmed.ends_with('}'))
        || (trimmed.starts_with('[') && trimmed.ends_with(']'))
    {
        return trimmed.to_string();
    }

    // 尝试从 markdown 代码块中提取
    let mut code_block = String::new();
    let mut in_block = false;
    for line in text.lines() {
        if line.trim().starts_with("```") {
            if in_block {
                break; // 结束第一个代码块
            }
            in_block = true;
            continue;
        }
        if in_block {
            code_block.push_str(line);
            code_block.push('\n');
        }
    }
    if !code_block.trim().is_empty() {
        return code_block.trim().to_string();
    }

    // 最后尝试：找第一个 { 到最后一个 } 之间的内容
    if let (Some(start), Some(end)) = (trimmed.find('{'), trimmed.rfind('}')) {
        if start < end {
            return trimmed[start..=end].to_string();
        }
    }

    trimmed.to_string()
}

// ─── organize ──────────────────────────────────────────

fn organize(output: &str, coverage_report: Option<&str>) {
    let specs_dir = format!("{}/specs", output);
    let by_module_dir = format!("{}/by_module", output);
    let inventory_path = format!("{}/inventory.json", output);
    fs::create_dir_all(&by_module_dir).ok();

    // 读取 inventory 用于模块信息兜底
    let inventory: Option<Inventory> = fs::read_to_string(&inventory_path)
        .ok()
        .and_then(|s| serde_json::from_str(&s).ok());

    let mut commit_map: HashMap<String, &BugfixCommit> = HashMap::new();
    if let Some(ref inv) = inventory {
        for c in &inv.commits {
            commit_map.insert(c.hash.clone(), c);
            if c.hash.len() >= 6 {
                commit_map.insert(c.hash[..6].to_string(), c);
            }
        }
    }

    // 扫描 specs/*.json
    let entries = match fs::read_dir(&specs_dir) {
        Ok(e) => e,
        Err(e) => {
            eprintln!("[organize] cannot read {}: {}", specs_dir, e);
            std::process::exit(1);
        }
    };

    // 按模块分组
    let mut by_module: HashMap<String, Vec<serde_json::Value>> = HashMap::new();
    let mut total_specs = 0;
    let mut skipped = 0;

    for entry in entries {
        let entry = match entry { Ok(e) => e, Err(_) => continue };
        let path = entry.path();
        if path.extension().map_or(true, |ext| ext != "json") { continue; }

        let stem = path.file_stem().unwrap().to_string_lossy().to_string();
        let content = match fs::read_to_string(&path) {
            Ok(s) => s,
            Err(_) => continue,
        };

        let spec: serde_json::Value = match serde_json::from_str(&content) {
            Ok(v) => v,
            Err(_) => continue,
        };

        // 跳过非 bugfix（skip: true）
        if spec.get("skip").and_then(|v| v.as_bool()).unwrap_or(false) {
            skipped += 1;
            continue;
        }

        // 取 module：优先用 spec 里的，否则从 inventory 兜底
        let module = spec.get("module")
            .and_then(|v| v.as_str())
            .map(|s| s.to_string())
            .unwrap_or_else(|| {
                commit_map.get(&stem)
                    .map(|c| c.module.clone())
                    .unwrap_or_else(|| "unknown".to_string())
            });

        total_specs += 1;
        by_module.entry(module).or_default().push(spec);
    }

    // 写出按模块合并的 JSON 数组
    let mut module_stats: Vec<(String, usize)> = Vec::new();

    for (module, specs) in &by_module {
        let module_json = serde_json::json!({
            "module": module,
            "generated_at": chrono_now(),
            "spec_count": specs.len(),
            "specs": specs,
        });

        // module 名可能含 /（如 sysadmin/property），替换为 _ 作为文件名
        let safe_name = module.replace('/', "_");
        let module_path = format!("{}/{}.json", by_module_dir, safe_name);
        let pretty = serde_json::to_string_pretty(&module_json).unwrap();
        fs::write(&module_path, &pretty).ok();

        module_stats.push((module.clone(), specs.len()));
    }

    module_stats.sort_by(|a, b| b.1.cmp(&a.1));

    // 生成覆盖率报告
    let report_path = coverage_report
        .map(|s| s.to_string())
        .unwrap_or_else(|| format!("{}/coverage.md", output));

    let mut report = String::new();
    report.push_str("# Bugfix 测试规格书覆盖率\n\n");
    report.push_str(&format!("生成时间：{}\n\n", chrono_now()));
    report.push_str("| 模块 | 规格书数 |\n");
    report.push_str("|------|--------|\n");

    for (module, count) in &module_stats {
        report.push_str(&format!("| {} | {} |\n", module, count));
    }

    report.push_str(&format!("| **合计** | **{}** |\n", total_specs));
    if skipped > 0 {
        report.push_str(&format!("\n跳过非 bugfix：{}\n", skipped));
    }

    report.push_str(&format!("\n## 下游使用\n\n"));
    report.push_str("```bash\n");
    report.push_str("# 将规格书喂给 bddc autochain 生成可执行测试\n");
    report.push_str(&format!("bddc domain.autowire --specs {}/by_module/<module>.json\n", output));
    report.push_str("```\n");

    fs::write(&report_path, &report).ok();

    eprintln!("[organize] {} modules, {} specs (skipped {} non-bugfix) → {}", module_stats.len(), total_specs, skipped, by_module_dir);
    eprintln!("[organize] coverage report: {}", report_path);
}

/// 从 PHP 源码中提取指定函数名的函数体（简单实现：按大括号匹配）
fn extract_function_body(content: &str, func_name: &str) -> String {
    let pattern = format!("function {}", func_name);
    let lines: Vec<&str> = content.lines().collect();

    for (i, line) in lines.iter().enumerate() {
        if line.contains(&pattern) {
            // 找到函数开头，按大括号平衡提取
            let mut depth = 0;
            let mut started = false;
            let mut body_lines = Vec::new();

            for j in i..lines.len() {
                body_lines.push(lines[j]);
                for ch in lines[j].chars() {
                    if ch == '{' { depth += 1; started = true; }
                    if ch == '}' { depth -= 1; }
                }
                if started && depth == 0 {
                    return body_lines.join("\n");
                }
            }
        }
    }
    String::new()
}

#[cfg(test)]
mod tests {
    use super::*;

    // ─── classify_grade ────────────────────────────────

    #[test]
    fn grade_a_boundary() {
        assert_eq!(classify_grade(0), "A");
        assert_eq!(classify_grade(1), "A");
        assert_eq!(classify_grade(10), "A");
    }

    #[test]
    fn grade_b_boundary() {
        assert_eq!(classify_grade(11), "B");
        assert_eq!(classify_grade(50), "B");
    }

    #[test]
    fn grade_c_boundary() {
        assert_eq!(classify_grade(51), "C");
        assert_eq!(classify_grade(1000), "C");
    }

    // ─── auto_tag ──────────────────────────────────────

    #[test]
    fn tag_security() {
        assert!(auto_tag("修复xss漏洞").contains(&"security".to_string()));
        assert!(auto_tag("fix越权访问").contains(&"security".to_string()));
    }

    #[test]
    fn tag_datetime() {
        assert!(auto_tag("修复时区问题").contains(&"datetime".to_string()));
        assert!(auto_tag("fix UTC时间").contains(&"datetime".to_string()));
    }

    #[test]
    fn tag_null_safety() {
        assert!(auto_tag("空值判断fix").contains(&"null_safety".to_string()));
        assert!(auto_tag("null pointer fix").contains(&"null_safety".to_string()));
    }

    #[test]
    fn tag_transaction() {
        assert!(auto_tag("修复事务回滚").contains(&"transaction".to_string()));
    }

    #[test]
    fn tag_multiple() {
        let tags = auto_tag("修复xss漏洞导致空值");
        assert!(tags.contains(&"security".to_string()));
        assert!(tags.contains(&"null_safety".to_string()));
    }

    #[test]
    fn tag_no_match() {
        assert!(auto_tag("重构代码").is_empty());
        assert!(auto_tag("add new feature").is_empty());
    }

    // ─── lang_extensions ───────────────────────────────

    #[test]
    fn lang_extensions_php() {
        assert_eq!(lang_extensions("php"), vec![".php"]);
    }

    #[test]
    fn lang_extensions_elixir() {
        assert_eq!(lang_extensions("elixir"), vec![".ex", ".exs"]);
    }

    #[test]
    fn lang_extensions_typescript() {
        assert_eq!(lang_extensions("typescript"), vec![".ts", ".tsx"]);
        assert_eq!(lang_extensions("ts"), vec![".ts", ".tsx"]);
    }

    #[test]
    fn lang_extensions_unknown() {
        assert!(lang_extensions("python").is_empty());
        assert!(lang_extensions("").is_empty());
    }

    // ─── is_backend_file ───────────────────────────────

    #[test]
    fn backend_file_php() {
        assert!(is_backend_file("app/controllers/FooController.php", "php"));
        assert!(is_backend_file("app/models/User.php", "php"));
        assert!(is_backend_file("app/traits/AuthTrait.php", "php"));
        assert!(is_backend_file("app/services/PayService.php", "php"));
        // 非业务目录的 PHP 不算
        assert!(!is_backend_file("config/app.php", "php"));
        assert!(!is_backend_file("routes/web.php", "php"));
        // 非 PHP 文件不算
        assert!(!is_backend_file("app/controllers/foo.js", "php"));
    }

    #[test]
    fn backend_file_elixir() {
        assert!(is_backend_file("lib/shop/order/cart.ex", "elixir"));
        assert!(is_backend_file("lib/shop_web/live/page_live.ex", "elixir"));
        // test/ 不算
        assert!(!is_backend_file("test/shop/order_test.exs", "elixir"));
        // 非 Elixir 文件
        assert!(!is_backend_file("lib/shop/readme.md", "elixir"));
    }

    #[test]
    fn backend_file_typescript() {
        assert!(is_backend_file("src/components/App.ts", "typescript"));
        assert!(is_backend_file("src/pages/Home.tsx", "typescript"));
        // test 文件不算
        assert!(!is_backend_file("src/components/App.test.ts", "typescript"));
        assert!(!is_backend_file("src/components/App.spec.tsx", "typescript"));
        // node_modules 不算
        assert!(!is_backend_file("node_modules/foo/src/index.ts", "typescript"));
    }

    // ─── classify_file_kind ────────────────────────────

    #[test]
    fn file_kind_controller() {
        assert_eq!(classify_file_kind("app/controllers/FooController.php"), "controller");
    }

    #[test]
    fn file_kind_model() {
        assert_eq!(classify_file_kind("app/models/User.php"), "model");
    }

    #[test]
    fn file_kind_trait() {
        assert_eq!(classify_file_kind("app/traits/AuthTrait.php"), "trait");
    }

    #[test]
    fn file_kind_service() {
        assert_eq!(classify_file_kind("app/services/PayService.php"), "service");
    }

    #[test]
    fn file_kind_other() {
        assert_eq!(classify_file_kind("app/helpers/utils.php"), "other");
    }

    // ─── extract_function_body ─────────────────────────

    #[test]
    fn extract_simple_function() {
        let php = r#"<?php
class Foo {
    public function bar() {
        return 1;
    }
    public function baz() {
        return 2;
    }
}"#;
        let body = extract_function_body(php, "bar");
        assert!(body.contains("function bar()"));
        assert!(body.contains("return 1;"));
        assert!(!body.contains("return 2;"));
    }

    #[test]
    fn extract_nested_braces() {
        let php = r#"<?php
class Foo {
    public function complex() {
        if (true) {
            foreach ($items as $item) {
                echo $item;
            }
        }
        return true;
    }
}"#;
        let body = extract_function_body(php, "complex");
        assert!(body.contains("function complex()"));
        assert!(body.contains("foreach"));
        assert!(body.contains("return true;"));
    }

    #[test]
    fn extract_nonexistent_function() {
        let php = "<?php\nclass Foo { public function bar() { return 1; } }";
        let body = extract_function_body(php, "nonexistent");
        assert!(body.is_empty());
    }

    // ─── parse_git_log ─────────────────────────────────

    #[test]
    fn parse_single_commit() {
        let output = "__COMMIT__abc123def456\n__MSG__修复空值bug\n__AUTHOR__test\n__DATE__2025-12-22 10:30:00 +0800\n3\t1\tapp/controllers/FooController.php\n";
        let mut commits = HashMap::new();
        parse_git_log(output, &None, &mut commits);
        assert_eq!(commits.len(), 1);
        let c = commits.get("abc123def456").unwrap();
        assert_eq!(c.message, "修复空值bug");
        assert_eq!(c.author, "test");
        assert_eq!(c.date, "2025-12-22");
        assert_eq!(c.total_lines, 4); // 3 + 1
        assert_eq!(c.changed_files.len(), 1);
        assert_eq!(c.changed_files[0].add, 3);
        assert_eq!(c.changed_files[0].del, 1);
    }

    #[test]
    fn parse_dedup_commits() {
        let output = "__COMMIT__aaa\n__MSG__fix1\n__AUTHOR__a\n__DATE__2025-01-01\n1\t1\tapp/controllers/A.php\n__COMMIT__aaa\n__MSG__fix1\n__AUTHOR__a\n__DATE__2025-01-01\n1\t1\tapp/controllers/A.php\n";
        let mut commits = HashMap::new();
        parse_git_log(output, &None, &mut commits);
        assert_eq!(commits.len(), 1);
    }

    #[test]
    fn parse_multiple_files() {
        let output = "__COMMIT__bbb\n__MSG__fix\n__AUTHOR__b\n__DATE__2025-01-01\n5\t2\tapp/controllers/A.php\n3\t0\tapp/models/B.php\n";
        let mut commits = HashMap::new();
        parse_git_log(output, &None, &mut commits);
        let c = commits.get("bbb").unwrap();
        assert_eq!(c.changed_files.len(), 2);
        assert_eq!(c.total_lines, 10); // 5+2+3+0
    }

    // ─── parse_changed_line_numbers ───────────────────

    #[test]
    fn changed_lines_simple_add() {
        let diff = "@@ -10,3 +10,5 @@ function foo()\n context\n+added line 1\n+added line 2\n context\n";
        let lines = parse_changed_line_numbers(diff);
        assert_eq!(lines, vec![11, 12]); // 行 11 和 12 是新增的
    }

    #[test]
    fn changed_lines_mixed() {
        let diff = "@@ -5,4 +5,4 @@ function bar()\n context\n-old line\n+new line\n context\n";
        let lines = parse_changed_line_numbers(diff);
        assert_eq!(lines, vec![6]); // 行 6 是修改后的新行
    }

    #[test]
    fn changed_lines_multiple_hunks() {
        let diff = "@@ -1,3 +1,4 @@\n context\n+line2\n context\n@@ -10,3 +11,4 @@\n context\n+line12\n context\n";
        let lines = parse_changed_line_numbers(diff);
        assert_eq!(lines, vec![2, 12]);
    }

    #[test]
    fn changed_lines_empty_diff() {
        assert!(parse_changed_line_numbers("").is_empty());
    }

    // ─── module_from_filename ─────────────────────────

    #[test]
    fn module_php_controller() {
        // OrderController.php → order
        assert_eq!(module_from_filename("app/controllers/OrderController.php"), "order");
    }

    #[test]
    fn module_php_service() {
        // PayService.php → pay
        assert_eq!(module_from_filename("app/services/PayService.php"), "pay");
    }

    #[test]
    fn module_php_model() {
        // UserModel.php → user
        assert_eq!(module_from_filename("app/models/UserModel.php"), "user");
    }

    #[test]
    fn module_php_trait() {
        // AuthTrait.php → auth
        assert_eq!(module_from_filename("app/traits/AuthTrait.php"), "auth");
    }

    #[test]
    fn module_elixir_context() {
        // lib/shop/order/cart.ex → order（cart 不是通用名，但 order 是更有意义的上下文）
        assert_eq!(module_from_filename("lib/shop/order/cart.ex"), "cart");
    }

    #[test]
    fn module_elixir_generic_name() {
        // lib/shop/order/index.ex → order（index 是通用名，回退到目录）
        assert_eq!(module_from_filename("lib/shop/order/index.ex"), "order");
    }

    #[test]
    fn module_camel_to_snake() {
        assert_eq!(camel_to_snake("OrderItem"), "order_item");
        assert_eq!(camel_to_snake("HTTPClient"), "h_t_t_p_client");
        assert_eq!(camel_to_snake("foo"), "foo");
        assert_eq!(camel_to_snake("A"), "a");
    }

    #[test]
    fn module_complex_php_name() {
        // MembershipCardController.php → membership_card
        assert_eq!(module_from_filename("app/controllers/MembershipCardController.php"), "membership_card");
    }

    #[test]
    fn resolve_module_with_map() {
        let mm = ModuleMap {
            mapping: [("app/controllers/Order".to_string(), "A-order".to_string())].into(),
            module_names: [("A-order".to_string(), "订单模块".to_string())].into(),
        };
        assert_eq!(resolve_module("app/controllers/OrderController.php", &Some(mm)), "A-order");
    }

    #[test]
    fn resolve_module_without_map() {
        // 无 module_map 时走 module_from_filename
        assert_eq!(resolve_module("app/controllers/OrderController.php", &None), "order");
    }

    // ─── generate (集成测试) ─────────────────────────

    /// 创建临时 contexts/ 目录并写入 N 个假 context JSON
    fn setup_generate_test(n: usize) -> tempfile::TempDir {
        let dir = tempfile::tempdir().expect("create tempdir");
        let contexts = dir.path().join("contexts");
        fs::create_dir_all(&contexts).unwrap();
        for i in 0..n {
            let json = serde_json::json!({
                "hash": format!("abc{:03}", i),
                "message": format!("fix bug {}", i),
                "grade": "A",
                "module": "test",
                "tags": ["regression"],
                "diffs": [],
            });
            fs::write(
                contexts.join(format!("abc{:03}.json", i)),
                serde_json::to_string(&json).unwrap(),
            ).unwrap();
        }
        dir
    }

    /// 统计 generate 产出文件数（codex 模式写 specs/*.json，降级模式写 prompts/*.prompt.txt）
    fn count_generate_output(dir: &std::path::Path) -> usize {
        let spec_count = fs::read_dir(dir.join("specs"))
            .map(|e| e.filter_map(|f| f.ok())
                .filter(|f| {
                    let p = f.path();
                    p.extension().map_or(false, |ext| ext == "json")
                        && !p.to_string_lossy().ends_with(".raw.txt")
                })
                .count())
            .unwrap_or(0);
        let prompt_count = fs::read_dir(dir.join("prompts"))
            .map(|e| e.filter_map(|f| f.ok())
                .filter(|f| f.path().to_string_lossy().ends_with(".prompt.txt"))
                .count())
            .unwrap_or(0);
        spec_count + prompt_count
    }

    #[test]
    fn generate_limit_respected() {
        // 准备 5 个 context，limit=2，应只处理 2 个
        let dir = setup_generate_test(5);
        let output = dir.path().to_str().unwrap();
        generate(output, None, false, Some(2));

        let count = count_generate_output(dir.path());
        assert_eq!(count, 2, "should only process 2 out of 5");
    }

    #[test]
    fn generate_no_limit_processes_all() {
        // 准备 3 个 context，无 limit，应全部处理
        let dir = setup_generate_test(3);
        let output = dir.path().to_str().unwrap();
        generate(output, None, false, None);

        let count = count_generate_output(dir.path());
        assert_eq!(count, 3, "should process all 3");
    }

    #[test]
    fn generate_skip_existing() {
        // 准备 3 个 context，预先创建 1 个已有 spec，应跳过
        let dir = setup_generate_test(3);
        let output = dir.path().to_str().unwrap();
        let specs = dir.path().join("specs");
        fs::create_dir_all(&specs).unwrap();
        fs::write(specs.join("abc000.json"), r#"{"existing": true}"#).unwrap();

        generate(output, None, false, None);

        let count = count_generate_output(dir.path());
        // abc000 被 skip（已有 1 个 .json），另外 2 个被处理
        // 总产出 = 1 (预存) + 2 (新生成) = 3
        assert_eq!(count, 3, "should have 1 existing + 2 newly generated");
    }

    // ─── extract_json_block ───────────────────────────

    #[test]
    fn json_block_plain() {
        let input = r#"{"source_commit": "abc", "bug_summary": "test"}"#;
        let result = extract_json_block(input);
        assert!(result.contains("source_commit"));
    }

    #[test]
    fn json_block_markdown_wrapped() {
        let input = "Here is the result:\n```json\n{\"skip\": true, \"reason\": \"feature\"}\n```\nDone.";
        let result = extract_json_block(input);
        assert!(result.contains("skip"));
        assert!(!result.contains("```"));
        assert!(!result.contains("Done"));
    }

    #[test]
    fn json_block_with_surrounding_text() {
        let input = "Analysis complete. Output:\n{\"module\": \"order\", \"action\": \"create\"}\nEnd of output.";
        let result = extract_json_block(input);
        assert!(result.starts_with('{'));
        assert!(result.ends_with('}'));
        assert!(result.contains("order"));
    }

    // ─── organize (集成测试) ──────────────────────────

    /// 构造 specs/ 目录写入假 spec JSON，返回 tempdir
    fn setup_organize_test(specs: &[(&str, serde_json::Value)]) -> tempfile::TempDir {
        let dir = tempfile::tempdir().expect("create tempdir");
        let specs_dir = dir.path().join("specs");
        fs::create_dir_all(&specs_dir).unwrap();
        for (name, json) in specs {
            fs::write(
                specs_dir.join(format!("{}.json", name)),
                serde_json::to_string(json).unwrap(),
            ).unwrap();
        }
        dir
    }

    #[test]
    fn organize_groups_by_module() {
        let dir = setup_organize_test(&[
            ("aaa111", serde_json::json!({
                "source_commit": "aaa111", "module": "order",
                "bug_summary": "空值", "test_type": "null_safety"
            })),
            ("bbb222", serde_json::json!({
                "source_commit": "bbb222", "module": "order",
                "bug_summary": "重复", "test_type": "regression"
            })),
            ("ccc333", serde_json::json!({
                "source_commit": "ccc333", "module": "payment",
                "bug_summary": "金额", "test_type": "boundary"
            })),
        ]);
        let output = dir.path().to_str().unwrap();
        organize(output, None);

        // by_module/ 应有 2 个文件
        let by_module = dir.path().join("by_module");
        assert!(by_module.join("order.json").exists(), "order.json should exist");
        assert!(by_module.join("payment.json").exists(), "payment.json should exist");

        // order 模块应有 2 个 spec
        let order: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(by_module.join("order.json")).unwrap()
        ).unwrap();
        assert_eq!(order["spec_count"], 2);
        assert_eq!(order["specs"].as_array().unwrap().len(), 2);

        // payment 模块应有 1 个 spec
        let payment: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(by_module.join("payment.json")).unwrap()
        ).unwrap();
        assert_eq!(payment["spec_count"], 1);

        // coverage.md 应存在且包含模块统计
        let coverage = fs::read_to_string(dir.path().join("coverage.md")).unwrap();
        assert!(coverage.contains("order"));
        assert!(coverage.contains("payment"));
        assert!(coverage.contains("**3**")); // 合计 3 个 spec
    }

    #[test]
    fn organize_skips_non_bugfix() {
        let dir = setup_organize_test(&[
            ("aaa111", serde_json::json!({
                "source_commit": "aaa111", "module": "order",
                "bug_summary": "空值", "test_type": "null_safety"
            })),
            ("bbb222", serde_json::json!({
                "skip": true, "reason": "feature", "summary": "新增功能"
            })),
            ("ccc333", serde_json::json!({
                "skip": true, "reason": "refactor", "summary": "重构"
            })),
        ]);
        let output = dir.path().to_str().unwrap();
        organize(output, None);

        // 只有 order 模块 1 个 spec
        let by_module = dir.path().join("by_module");
        assert!(by_module.join("order.json").exists());
        let order: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(by_module.join("order.json")).unwrap()
        ).unwrap();
        assert_eq!(order["spec_count"], 1);

        // coverage 应显示跳过数
        let coverage = fs::read_to_string(dir.path().join("coverage.md")).unwrap();
        assert!(coverage.contains("跳过非 bugfix：2"));
    }

    #[test]
    fn organize_module_with_slash_in_name() {
        // module 含 /（如 sysadmin/property）应替换为 _ 作为文件名
        let dir = setup_organize_test(&[
            ("aaa111", serde_json::json!({
                "source_commit": "aaa111", "module": "sysadmin/property",
                "bug_summary": "权限bug", "test_type": "regression"
            })),
            ("bbb222", serde_json::json!({
                "source_commit": "bbb222", "module": "outterajax/oaactive",
                "bug_summary": "空值", "test_type": "null_safety"
            })),
        ]);
        let output = dir.path().to_str().unwrap();
        organize(output, None);

        let by_module = dir.path().join("by_module");
        // / 被替换为 _
        assert!(by_module.join("sysadmin_property.json").exists(),
            "sysadmin/property should become sysadmin_property.json");
        assert!(by_module.join("outterajax_oaactive.json").exists(),
            "outterajax/oaactive should become outterajax_oaactive.json");

        // 不应创建子目录
        assert!(!by_module.join("sysadmin").exists(),
            "should not create sysadmin/ subdirectory");
    }

    #[test]
    fn organize_custom_coverage_report_path() {
        let dir = setup_organize_test(&[
            ("aaa111", serde_json::json!({
                "source_commit": "aaa111", "module": "order",
                "bug_summary": "test", "test_type": "regression"
            })),
        ]);
        let output = dir.path().to_str().unwrap();
        let custom_path = dir.path().join("custom_report.md");
        organize(output, Some(custom_path.to_str().unwrap()));

        assert!(custom_path.exists(), "custom coverage report should exist");
        assert!(!dir.path().join("coverage.md").exists(),
            "default coverage.md should not be created");
    }

    // ─── extract_file_diff ──────────────────────────────

    #[test]
    fn extract_file_diff_single_file() {
        let diff = "\
diff --git a/app/controllers/OrderController.php b/app/controllers/OrderController.php
index abc..def 100644
--- a/app/controllers/OrderController.php
+++ b/app/controllers/OrderController.php
@@ -10,3 +10,4 @@
 context line
-old line
+new line
+added line
";
        let result = extract_file_diff(diff, "OrderController.php");
        assert!(result.contains("diff --git"));
        assert!(result.contains("+new line"));
        assert!(result.contains("+added line"));
    }

    #[test]
    fn extract_file_diff_from_multi_file_diff() {
        let diff = "\
diff --git a/foo.php b/foo.php
--- a/foo.php
+++ b/foo.php
@@ -1,2 +1,2 @@
-old foo
+new foo
diff --git a/bar.php b/bar.php
--- a/bar.php
+++ b/bar.php
@@ -1,2 +1,2 @@
-old bar
+new bar
diff --git a/baz.php b/baz.php
--- a/baz.php
+++ b/baz.php
@@ -1,2 +1,2 @@
-old baz
+new baz
";
        // 只提取 bar.php 的部分
        let result = extract_file_diff(diff, "bar.php");
        assert!(result.contains("+new bar"));
        assert!(!result.contains("foo"), "should not contain foo.php content");
        assert!(!result.contains("baz"), "should not contain baz.php content");
    }

    #[test]
    fn extract_file_diff_not_found() {
        let diff = "diff --git a/foo.php b/foo.php\n+something\n";
        let result = extract_file_diff(diff, "nonexistent.php");
        assert!(result.is_empty());
    }

    // ─── parse_diff_hunks ────────────────────────────────

    #[test]
    fn parse_diff_hunks_php_function_changed() {
        let before = r#"<?php
class OrderController {
    public function create($data) {
        return $data;
    }
    public function delete($id) {
        DB::delete($id);
    }
}"#;
        let after = r#"<?php
class OrderController {
    public function create($data) {
        if (empty($data)) { return null; }
        return $data;
    }
    public function delete($id) {
        DB::delete($id);
    }
}"#;
        // 构造与 after 对应的 diff
        let diff = "\
diff --git a/OrderController.php b/OrderController.php
--- a/OrderController.php
+++ b/OrderController.php
@@ -2,4 +2,5 @@
 class OrderController {
     public function create($data) {
+        if (empty($data)) { return null; }
         return $data;
     }
";
        let hunks = parse_diff_hunks(diff, "OrderController.php", before, after, "php");
        // create 被改了，应出现在 hunks 中
        assert!(!hunks.is_empty(), "should have at least one hunk");
        let func_names: Vec<&str> = hunks.iter()
            .filter_map(|h| h["function_name"].as_str())
            .collect();
        assert!(func_names.contains(&"create"), "should contain changed function 'create'");
        // delete 未改，不应出现
        assert!(!func_names.contains(&"delete"), "should not contain unchanged function 'delete'");
    }

    #[test]
    fn parse_diff_hunks_empty_after() {
        // after 为空时应返回空
        let hunks = parse_diff_hunks("diff...", "foo.php", "before", "", "php");
        assert!(hunks.is_empty());
    }

    // ─── load_prompt_template ────────────────────────────

    #[test]
    fn load_prompt_template_custom() {
        let dir = tempfile::tempdir().expect("create tempdir");
        let tpl_path = dir.path().join("my_template.txt");
        fs::write(&tpl_path, "custom prompt: {context_json}").unwrap();

        let result = load_prompt_template(Some(tpl_path.to_str().unwrap()));
        assert_eq!(result, "custom prompt: {context_json}");
    }

    #[test]
    fn load_prompt_template_fallback_default() {
        // 传一个不存在的路径，应降级到内置默认模板
        let result = load_prompt_template(Some("/nonexistent/path/template.txt"));
        assert!(result.contains("测试规格书"), "should contain default template content");
        assert!(result.contains("{context_json}"), "should contain placeholder");
    }

    #[test]
    fn load_prompt_template_none() {
        // 不传自定义路径，使用默认模板
        let result = load_prompt_template(None);
        assert!(result.contains("测试规格书"));
    }

    // ─── collect + context (git 集成测试) ────────────────

    /// 创建一个临时 git 仓库，提交若干 PHP bugfix commit
    fn setup_git_repo() -> tempfile::TempDir {
        let dir = tempfile::tempdir().expect("create tempdir");
        let repo = dir.path().to_str().unwrap();

        // git init
        Command::new("git").args(["-C", repo, "init"]).output().unwrap();
        Command::new("git").args(["-C", repo, "config", "user.email", "test@test.com"]).output().unwrap();
        Command::new("git").args(["-C", repo, "config", "user.name", "Test"]).output().unwrap();

        // commit 1: 初始文件
        let ctrl_dir = dir.path().join("app/controllers");
        fs::create_dir_all(&ctrl_dir).unwrap();
        fs::write(ctrl_dir.join("OrderController.php"), r#"<?php
class OrderController {
    public function create($data) {
        return save($data);
    }
}
"#).unwrap();
        Command::new("git").args(["-C", repo, "add", "."]).output().unwrap();
        Command::new("git").args(["-C", repo, "commit", "-m", "init: add OrderController"]).output().unwrap();

        // commit 2: bugfix（关键字匹配 + 文件路径匹配）
        fs::write(ctrl_dir.join("OrderController.php"), r#"<?php
class OrderController {
    public function create($data) {
        if (empty($data)) { return null; }
        return save($data);
    }
}
"#).unwrap();
        Command::new("git").args(["-C", repo, "add", "."]).output().unwrap();
        Command::new("git").args(["-C", repo, "commit", "-m", "fix: 修复空值导致报错"]).output().unwrap();

        // commit 3: 另一个 bugfix
        fs::write(ctrl_dir.join("PayController.php"), r#"<?php
class PayController {
    public function refund($id) {
        return do_refund($id);
    }
}
"#).unwrap();
        Command::new("git").args(["-C", repo, "add", "."]).output().unwrap();
        Command::new("git").args(["-C", repo, "commit", "-m", "init: add PayController"]).output().unwrap();

        fs::write(ctrl_dir.join("PayController.php"), r#"<?php
class PayController {
    public function refund($id) {
        if ($id <= 0) { throw new Exception("invalid id"); }
        return do_refund($id);
    }
}
"#).unwrap();
        Command::new("git").args(["-C", repo, "add", "."]).output().unwrap();
        Command::new("git").args(["-C", repo, "commit", "-m", "bug: 退款ID校验缺失"]).output().unwrap();

        // commit 4: 非 bugfix（feature）
        fs::write(ctrl_dir.join("UserController.php"), r#"<?php
class UserController {
    public function list() { return []; }
}
"#).unwrap();
        Command::new("git").args(["-C", repo, "add", "."]).output().unwrap();
        Command::new("git").args(["-C", repo, "commit", "-m", "feat: 新增用户列表"]).output().unwrap();

        dir
    }

    #[test]
    fn collect_by_keyword() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");

        let inv_path = output_dir.path().join("inventory.json");
        assert!(inv_path.exists(), "inventory.json should be created");

        let inv: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(&inv_path).unwrap()
        ).unwrap();

        // 应该扫到 2 个 bugfix commit（"fix: 修复空值" + "bug: 退款ID"）
        let commits = inv["commits"].as_array().unwrap();
        assert_eq!(commits.len(), 2, "should find 2 bugfix commits by keyword");

        // 每个 commit 应有 grade 和 module
        for c in commits {
            assert!(c["grade"].as_str().is_some());
            assert!(c["module"].as_str().is_some());
            assert!(!c["changed_files"].as_array().unwrap().is_empty());
        }
    }

    #[test]
    fn collect_by_path() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        // 按路径扫描 app/controllers/，应扫到所有涉及该路径的 commit
        collect(repo, output, None, Some("app/controllers/"), "fix,bug", None, &["A", "B"], None, false, "php");

        let inv_path = output_dir.path().join("inventory.json");
        assert!(inv_path.exists());

        let inv: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(&inv_path).unwrap()
        ).unwrap();

        // 按路径扫到的应比按关键字多（init commit 也会被扫到，只要改了后端文件）
        let commits = inv["commits"].as_array().unwrap();
        assert!(commits.len() >= 2, "path scan should find at least 2 commits, got {}", commits.len());
    }

    #[test]
    fn collect_with_limit() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        collect(repo, output, None, Some("app/controllers/"), "fix,bug", None, &["A", "B"], Some(1), false, "php");

        let inv: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(output_dir.path().join("inventory.json")).unwrap()
        ).unwrap();

        assert_eq!(inv["commits"].as_array().unwrap().len(), 1, "limit=1 should keep only 1 commit");
    }

    #[test]
    fn collect_skip_existing() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        // 第一次 collect
        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");
        let content1 = fs::read_to_string(output_dir.path().join("inventory.json")).unwrap();

        // 第二次 collect（不带 force），应跳过
        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");
        let content2 = fs::read_to_string(output_dir.path().join("inventory.json")).unwrap();
        assert_eq!(content1, content2, "without force, inventory should not be overwritten");
    }

    #[test]
    fn context_extracts_diffs() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        // 先 collect
        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");

        // 再 context
        context(output, repo, &["A", "B"], None, false, "php");

        let contexts_dir = output_dir.path().join("contexts");
        assert!(contexts_dir.exists(), "contexts/ should be created");

        let context_files: Vec<_> = fs::read_dir(&contexts_dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "json"))
            .collect();

        assert_eq!(context_files.len(), 2, "should have 2 context files (one per bugfix commit)");

        // 验证 context 内容结构
        for entry in &context_files {
            let content: serde_json::Value = serde_json::from_str(
                &fs::read_to_string(entry.path()).unwrap()
            ).unwrap();
            assert!(content["hash"].as_str().is_some(), "should have hash");
            assert!(content["message"].as_str().is_some(), "should have message");
            assert!(content["diffs"].as_array().is_some(), "should have diffs array");

            // diffs 中应有函数级 hunks
            let diffs = content["diffs"].as_array().unwrap();
            assert!(!diffs.is_empty(), "should have at least one file diff");
            for d in diffs {
                assert!(d["file"].as_str().is_some());
                assert!(d["raw_diff"].as_str().is_some());
                assert!(d["hunks"].as_array().is_some());
            }
        }
    }

    #[test]
    fn context_skip_existing() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");
        context(output, repo, &["A", "B"], None, false, "php");

        // 读取 inventory 获取 commit hash
        let inv: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(output_dir.path().join("inventory.json")).unwrap()
        ).unwrap();
        let first_hash = inv["commits"][0]["hash"].as_str().unwrap();
        let ctx_path = output_dir.path().join(format!("contexts/{}.json", first_hash));
        let content1 = fs::read_to_string(&ctx_path).unwrap();

        // 再跑一次 context（不带 force），文件不应被覆盖
        context(output, repo, &["A", "B"], None, false, "php");
        let content2 = fs::read_to_string(&ctx_path).unwrap();
        assert_eq!(content1, content2, "without force, context file should not be overwritten");
    }

    // ─── 端到端：collect → context → organize ────────────

    #[test]
    fn end_to_end_collect_context_organize() {
        let repo_dir = setup_git_repo();
        let output_dir = tempfile::tempdir().expect("create output dir");
        let repo = repo_dir.path().to_str().unwrap();
        let output = output_dir.path().to_str().unwrap();
        fs::create_dir_all(output).ok();

        // Step 1: collect
        collect(repo, output, None, None, "fix,bug", None, &["A", "B"], None, false, "php");
        assert!(output_dir.path().join("inventory.json").exists());

        // Step 2: context
        context(output, repo, &["A", "B"], None, false, "php");
        assert!(output_dir.path().join("contexts").exists());

        // Step 3: 模拟 generate 产出（直接写 specs，不调 codex）
        let specs_dir = output_dir.path().join("specs");
        fs::create_dir_all(&specs_dir).unwrap();

        let inv: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(output_dir.path().join("inventory.json")).unwrap()
        ).unwrap();

        for commit in inv["commits"].as_array().unwrap() {
            let hash = commit["hash"].as_str().unwrap();
            let spec = serde_json::json!({
                "source_commit": hash,
                "module": commit["module"],
                "bug_summary": "test bug",
                "test_type": "regression",
                "test_spec": {
                    "preconditions": [],
                    "trigger": {"type": "controller_action", "target": "Test", "method": "test"},
                    "assertions": [{"what": "test", "type": "return_value", "expected": "ok"}]
                },
                "wrong_behavior": "before",
                "correct_behavior": "after",
            });
            fs::write(specs_dir.join(format!("{}.json", hash)), serde_json::to_string(&spec).unwrap()).unwrap();
        }

        // Step 4: organize
        organize(output, None);

        // 验证 by_module 产出
        let by_module = output_dir.path().join("by_module");
        assert!(by_module.exists(), "by_module/ should exist");
        let module_files: Vec<_> = fs::read_dir(&by_module).unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "json"))
            .collect();
        assert!(!module_files.is_empty(), "should have at least one module file");

        // 验证 coverage.md
        let coverage = fs::read_to_string(output_dir.path().join("coverage.md")).unwrap();
        assert!(coverage.contains("覆盖率"));
        assert!(coverage.contains("bddc"));
    }
}
