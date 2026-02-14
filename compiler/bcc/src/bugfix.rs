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
    generate(output, prompt_template, force);
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

    // 获取总 commit 数
    let total_commits = Command::new("git")
        .args(["-C", repo, "rev-list", "--all", "--count"])
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

fn generate(output: &str, prompt_template: Option<&str>, force: bool) {
    let contexts_dir = format!("{}/contexts", output);
    let scenarios_dir = format!("{}/scenarios", output);
    fs::create_dir_all(&scenarios_dir).ok();

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
        let out_path = format!("{}/{}.dsl", scenarios_dir, stem);

        if !force && std::path::Path::new(&out_path).exists() {
            skipped += 1;
            continue;
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
            // 调用 codex exec
            let tmp_out = format!("{}/{}.tmp", scenarios_dir, stem);
            let result = Command::new("codex")
                .args([
                    "exec",
                    "--full-auto",
                    "--ephemeral",
                    "-o", &tmp_out,
                    &prompt,
                ])
                .output();

            match result {
                Ok(o) if o.status.success() => {
                    // 读取 codex 输出
                    let content = fs::read_to_string(&tmp_out).unwrap_or_default();
                    if content.is_empty() {
                        // codex 可能直接输出到 stdout
                        let stdout = String::from_utf8_lossy(&o.stdout).to_string();
                        if !stdout.trim().is_empty() {
                            fs::write(&out_path, extract_dsl_block(&stdout)).ok();
                        } else {
                            eprintln!("[generate] empty output for {}", stem);
                            failed += 1;
                            continue;
                        }
                    } else {
                        fs::write(&out_path, extract_dsl_block(&content)).ok();
                    }
                    // 清理临时文件
                    fs::remove_file(&tmp_out).ok();
                    processed += 1;
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
        eprintln!("[generate] processed: {}, skipped: {}, failed: {}", processed, skipped, failed);
    } else {
        eprintln!("[generate] prompts written: {}, skipped: {} (use codex manually)", processed, skipped);
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

const DEFAULT_PROMPT_TEMPLATE: &str = r#"你是BDD测试专家。请将以下PHP bugfix记录转为bddc DSL场景。

## bddc DSL 语法

```
# Source: {hash}
# Bug: {一句话根因}
[SCENARIO: BDD-{MODULE}-BUGFIX-{HASH_SHORT}] TITLE: {标题} TAGS: regression {tags}
GIVEN {前置条件指令} {参数}
WHEN {触发操作指令} {参数}
THEN {断言指令} {参数}
```

## 规则

1. diff中被替换的旧代码 = bug的根因，新代码 = 正确行为
2. GIVEN 描述触发bug的数据条件（使用 $变量名 作为参数）
3. WHEN 描述用户/系统操作
4. THEN 描述修复后的期望行为
5. 每个场景头部加注释：# Source: {hash} 和 # Bug: {一句话根因}
6. SCENARIO ID 格式：BDD-{模块}-BUGFIX-{commit前6位}
7. 只输出DSL文本，不要包裹在markdown代码块中

## Bugfix记录

{context_json}"#;

/// 从 codex 输出中提取 DSL 块（去掉 markdown 代码块包裹）
fn extract_dsl_block(text: &str) -> String {
    let mut lines = Vec::new();
    let mut in_code_block = false;

    for line in text.lines() {
        if line.trim().starts_with("```") {
            if in_code_block {
                in_code_block = false;
                continue;
            }
            in_code_block = true;
            continue;
        }

        // DSL 特征行：注释/场景声明/步骤关键字
        let is_dsl_line = line.contains("[SCENARIO:")
            || line.starts_with("# Source:") || line.starts_with("# Bug:")
            || line.starts_with("# WARNING:")
            || line.starts_with("GIVEN ") || line.starts_with("WHEN ") || line.starts_with("THEN ");

        if in_code_block || is_dsl_line {
            lines.push(line);
        }
    }

    if lines.is_empty() {
        text.to_string()
    } else {
        lines.join("\n")
    }
}

// ─── organize ──────────────────────────────────────────

fn organize(output: &str, coverage_report: Option<&str>) {
    let scenarios_dir = format!("{}/scenarios", output);
    let features_dir = format!("{}/features", output);
    let inventory_path = format!("{}/inventory.json", output);
    fs::create_dir_all(&features_dir).ok();

    // 读取 inventory 用于模块信息
    let inventory: Option<Inventory> = fs::read_to_string(&inventory_path)
        .ok()
        .and_then(|s| serde_json::from_str(&s).ok());

    // 构建 hash → commit 查找表
    let mut commit_map: HashMap<String, &BugfixCommit> = HashMap::new();
    if let Some(ref inv) = inventory {
        for c in &inv.commits {
            commit_map.insert(c.hash.clone(), c);
            // 也用短 hash 建索引
            if c.hash.len() >= 6 {
                commit_map.insert(c.hash[..6].to_string(), c);
            }
        }
    }

    // 扫描所有 .dsl 文件
    let entries = match fs::read_dir(&scenarios_dir) {
        Ok(e) => e,
        Err(e) => {
            eprintln!("[organize] cannot read {}: {}", scenarios_dir, e);
            std::process::exit(1);
        }
    };

    // 按模块分组
    let mut by_module: HashMap<String, Vec<(String, String)>> = HashMap::new(); // module → [(filename, content)]
    let mut total_scenarios = 0;

    for entry in entries {
        let entry = match entry { Ok(e) => e, Err(_) => continue };
        let path = entry.path();
        if path.extension().map_or(true, |ext| ext != "dsl") { continue; }

        let filename = path.file_name().unwrap().to_string_lossy().to_string();
        let stem = path.file_stem().unwrap().to_string_lossy().to_string();
        let content = match fs::read_to_string(&path) {
            Ok(s) => s,
            Err(_) => continue,
        };

        // 从 commit_map 查找模块
        let module = commit_map.get(&stem)
            .map(|c| c.module.clone())
            .unwrap_or_else(|| "unknown".to_string());

        // 统计场景数
        let scenario_count = content.matches("[SCENARIO:").count();
        total_scenarios += scenario_count;

        by_module.entry(module).or_default().push((filename, content));
    }

    // 写出按模块合并的 feature 文件
    let mut module_stats: Vec<(String, usize, usize)> = Vec::new(); // (module, file_count, scenario_count)

    for (module, files) in &by_module {
        let mut merged = String::new();
        merged.push_str(&format!("# Module: {}\n", module));
        merged.push_str(&format!("# Generated by: bcc bugfix --step organize\n"));
        merged.push_str(&format!("# Files: {}\n\n", files.len()));

        let mut scenario_count = 0;
        let mut seen_scenarios: Vec<(String, Vec<String>)> = Vec::new(); // (function_name, tags)

        for (filename, content) in files {
            merged.push_str(&format!("# --- {} ---\n", filename));
            merged.push_str(content);
            if !content.ends_with('\n') {
                merged.push('\n');
            }
            merged.push('\n');

            // 提取场景信息用于重复检测
            for line in content.lines() {
                if line.contains("[SCENARIO:") {
                    scenario_count += 1;
                    // 提取 TITLE 后的函数名线索
                    let func_hint = line.split("TITLE:").nth(1)
                        .unwrap_or("")
                        .trim()
                        .split_whitespace()
                        .take(3)
                        .collect::<Vec<_>>()
                        .join(" ");
                    let tags: Vec<String> = line.split("TAGS:")
                        .nth(1)
                        .unwrap_or("")
                        .trim()
                        .split_whitespace()
                        .map(|s| s.to_string())
                        .collect();
                    seen_scenarios.push((func_hint, tags));
                }
            }
        }

        // 检测疑似重复（标题前3词相同 + tags 相同）
        let mut dup_warnings = Vec::new();
        for i in 0..seen_scenarios.len() {
            for j in (i+1)..seen_scenarios.len() {
                if seen_scenarios[i].0 == seen_scenarios[j].0
                    && seen_scenarios[i].1 == seen_scenarios[j].1
                    && !seen_scenarios[i].0.is_empty()
                {
                    dup_warnings.push(format!(
                        "# WARNING: 疑似重复 — '{}' tags={:?}",
                        seen_scenarios[i].0, seen_scenarios[i].1
                    ));
                }
            }
        }

        if !dup_warnings.is_empty() {
            merged.push_str("\n# === 重复检测 ===\n");
            for w in &dup_warnings {
                merged.push_str(w);
                merged.push('\n');
            }
        }

        let feature_path = format!("{}/{}.dsl", features_dir, module);
        fs::write(&feature_path, &merged).ok();

        module_stats.push((module.clone(), files.len(), scenario_count));
    }

    // 排序
    module_stats.sort_by(|a, b| b.2.cmp(&a.2));

    // 生成覆盖率报告
    let report_path = coverage_report
        .map(|s| s.to_string())
        .unwrap_or_else(|| format!("{}/coverage.md", output));

    let mut report = String::new();
    report.push_str("# BDD Bugfix 场景覆盖率\n\n");
    report.push_str(&format!("生成时间：{}\n\n", chrono_now()));
    report.push_str("| 模块 | commit 数 | 场景数 |\n");
    report.push_str("|------|----------|--------|\n");

    for (module, file_count, scenario_count) in &module_stats {
        report.push_str(&format!("| {} | {} | {} |\n", module, file_count, scenario_count));
    }

    let total_files: usize = module_stats.iter().map(|s| s.1).sum();
    report.push_str(&format!("| **合计** | **{}** | **{}** |\n", total_files, total_scenarios));

    fs::write(&report_path, &report).ok();

    eprintln!("[organize] {} modules, {} scenarios → {}", module_stats.len(), total_scenarios, features_dir);
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

    // ─── extract_dsl_block ─────────────────────────────

    #[test]
    fn dsl_block_plain_text() {
        let input = r#"# Source: abc123
# Bug: 空值未处理
[SCENARIO: BDD-H-BUGFIX-abc123] TITLE: 测试 TAGS: regression
GIVEN some_setup
WHEN some_action
THEN some_assertion"#;
        let result = extract_dsl_block(input);
        assert!(result.contains("[SCENARIO:"));
        assert!(result.contains("GIVEN"));
    }

    #[test]
    fn dsl_block_markdown_wrapped() {
        let input = "Here is the scenario:\n```dsl\n# Source: abc\n[SCENARIO: BDD-H-001] TITLE: test TAGS: t\nGIVEN x\nWHEN y\nTHEN z\n```\nDone.";
        let result = extract_dsl_block(input);
        assert!(result.contains("[SCENARIO:"));
        assert!(!result.contains("```"));
        assert!(!result.contains("Done."));
    }

    #[test]
    fn dsl_block_empty_returns_original() {
        let input = "no dsl content here at all";
        let result = extract_dsl_block(input);
        assert_eq!(result, input);
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
}
