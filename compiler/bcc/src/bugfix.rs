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

    let grades: Vec<&str> = grade.split(',').map(|s| s.trim()).collect();

    // 确保输出目录存在
    fs::create_dir_all(output).ok();

    // collect
    collect(repo, output, keywords, module_map, &grades, limit, force);
    if target == Step::Collect { return; }

    // context
    context(output, repo, &grades, limit, force);
    if target == Step::Context { return; }

    // generate
    generate(output, prompt_template, force);
    if target == Step::Generate { return; }

    // organize
    organize(output, coverage_report);
}

// ─── collect ────────────────────────────────────────────

fn collect(
    repo: &str,
    output: &str,
    keywords: &str,
    module_map_path: Option<&str>,
    grades: &[&str],
    limit: Option<usize>,
    force: bool,
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

    // 扫描 git log
    let kw_list: Vec<&str> = keywords.split(',').map(|s| s.trim()).collect();
    let mut all_commits: HashMap<String, BugfixCommit> = HashMap::new();

    for kw in &kw_list {
        let git_output = Command::new("git")
            .args([
                "-C", repo,
                "log", "--all", "--no-merges",
                &format!("--grep={}", kw), "-i",
                "--numstat",
                "--format=__COMMIT__%H%n__MSG__%s%n__AUTHOR__%an%n__DATE__%ai",
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

    // 过滤 + 分级 + 打标签
    let mut commits: Vec<BugfixCommit> = all_commits.into_values()
        .filter(|c| c.changed_files.iter().any(|f| is_backend_file(&f.path)))
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

fn is_backend_file(path: &str) -> bool {
    path.ends_with(".php")
        && (path.contains("controller") || path.contains("Controller")
            || path.contains("model") || path.contains("Model")
            || path.contains("trait") || path.contains("Trait")
            || path.contains("service") || path.contains("Service"))
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
    // 默认用路径的第一级目录
    path.split('/').nth(1).unwrap_or("unknown").to_string()
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

fn context(output: &str, repo: &str, _grades: &[&str], _limit: Option<usize>, force: bool) {
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

        // 对每个 PHP 文件提取函数上下文
        let mut diffs = Vec::new();
        for file in &commit.changed_files {
            if !file.path.ends_with(".php") { continue; }

            // 获取修复后文件
            let after_content = git_show(repo, &commit.hash, &file.path);
            // 获取修复前文件
            let before_content = git_show(repo, &format!("{}^", commit.hash), &file.path);

            // 解析 diff hunks
            let hunks = parse_diff_hunks(&raw_diff, &file.path, &before_content, &after_content);

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

/// 解析 diff hunks，定位改动行所在的函数
fn parse_diff_hunks(
    _full_diff: &str,
    file_path: &str,
    before_content: &str,
    after_content: &str,
) -> Vec<serde_json::Value> {
    // 简化实现：用 tree-sitter PHP 提取函数列表，匹配改动行
    // 完整实现需要解析 @@ hunk headers
    let mut hunks = Vec::new();

    if !file_path.ends_with(".php") || after_content.is_empty() {
        return hunks;
    }

    // 用 extract::php 获取函数列表
    let after_record = crate::extract::php::extract(after_content, file_path);

    // 对每个 export 函数，提取函数体
    for export in &after_record.exports {
        // 在 before_content 中查找同名函数
        let before_func = extract_function_body(before_content, &export.name);
        let after_func = extract_function_body(after_content, &export.name);

        // 如果 before/after 不同，说明这个函数被修改了
        if before_func != after_func && !after_func.is_empty() {
            hunks.push(serde_json::json!({
                "function_name": export.name,
                "function_line": export.line,
                "changed_lines": [],
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
    let mut found_dsl = false;

    for line in text.lines() {
        if line.trim().starts_with("```") {
            if in_code_block {
                in_code_block = false;
                continue;
            }
            in_code_block = true;
            continue;
        }

        // 检测 DSL 特征行
        if line.contains("[SCENARIO:") || line.starts_with("# Source:") || line.starts_with("# Bug:")
            || line.starts_with("GIVEN ") || line.starts_with("WHEN ") || line.starts_with("THEN ")
        {
            found_dsl = true;
        }

        if in_code_block || found_dsl || line.starts_with('#') {
            lines.push(line);
        }
    }

    if lines.is_empty() {
        // 没找到 DSL 特征，返回原文
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
