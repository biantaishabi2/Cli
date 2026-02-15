// BDD 集成测试：bugfix 流水线设计缺陷修复验证
// 对应 https://github.com/biantaishabi2/Cli/issues/1

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

// ─── helpers ──────────────────────────────────────────────

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

/// 创建含多种 grade 的测试 git 仓库
/// 返回 (repo_path, a_grade_count, b_grade_count)
fn setup_multi_grade_repo(root: &Path) -> (PathBuf, usize, usize) {
    let repo = root.join("repo");
    let src = repo.join("src");
    fs::create_dir_all(&src).expect("create repo dirs");

    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(&repo)
            .args(args)
            .output()
            .expect("run git")
    };

    // 初始化（文件路径须含 controller/model/service 等关键词，否则 is_backend_file 过滤）
    let ctrl = src.join("app/controllers");
    let svc = src.join("app/services");
    fs::create_dir_all(&ctrl).unwrap();
    fs::create_dir_all(&svc).unwrap();
    write(&ctrl.join("MainController.php"), "<?php\nclass MainController {}\n");
    git(&["init"]);
    git(&["config", "user.email", "test@bdd.com"]);
    git(&["config", "user.name", "bdd-test"]);
    git(&["add", "."]);
    git(&["commit", "-m", "init: project setup"]);

    // 4 个 A 级 bugfix（每个新增 ~4 行，total_lines ≤ 10）
    for i in 0..4 {
        let file = ctrl.join(format!("FixAController{}.php", i));
        write(
            &file,
            &format!("<?php\nclass FixAController{} {{\n    public function handle() {{ return true; }}\n}}\n", i),
        );
        git(&["add", "."]);
        git(&["commit", "-m", &format!("fix: A级修复 {}", i)]);
    }

    // 3 个 B 级 bugfix（每个新增 ~17 行，10 < total_lines ≤ 50）
    for i in 0..3 {
        let file = svc.join(format!("FixBService{}.php", i));
        let mut content = format!("<?php\nclass FixBService{} {{\n", i);
        for j in 0..14 {
            content.push_str(&format!(
                "    public function method_{}() {{ return {}; }}\n",
                j, j
            ));
        }
        content.push_str("}\n");
        write(&file, &content);
        git(&["add", "."]);
        git(&["commit", "-m", &format!("bug: B级修复 {}", i)]);
    }

    (repo, 4, 3)
}

/// 创建测试用 git 仓库，包含 2 个 bugfix commit（A 级）
fn setup_bugfix_repo(root: &Path) -> PathBuf {
    let repo = root.join("repo");
    let ctrl = repo.join("app/controllers");
    fs::create_dir_all(&ctrl).expect("create repo dirs");

    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(&repo)
            .args(args)
            .output()
            .expect("run git")
    };

    // 初始化 repo
    write(
        &ctrl.join("OrderController.php"),
        "<?php\nclass OrderController {\n    public function show() { return 1; }\n}\n",
    );
    git(&["init"]);
    git(&["config", "user.email", "test@bdd.com"]);
    git(&["config", "user.name", "bdd-test"]);
    git(&["add", "."]);
    git(&["commit", "-m", "init: add order controller"]);

    // bugfix commit 1（A 级，< 10 行）
    write(
        &ctrl.join("OrderController.php"),
        "<?php\nclass OrderController {\n    public function show() {\n        if (!$id) return null;\n        return 1;\n    }\n}\n",
    );
    git(&["add", "."]);
    git(&["commit", "-m", "fix: 空值校验"]);

    // bugfix commit 2（A 级）
    write(
        &ctrl.join("PayController.php"),
        "<?php\nclass PayController {\n    public function pay() { return ok(); }\n}\n",
    );
    git(&["add", "."]);
    git(&["commit", "-m", "init: add pay"]);

    write(
        &ctrl.join("PayController.php"),
        "<?php\nclass PayController {\n    public function pay() {\n        if (!$amt) throw new E('bad');\n        return ok();\n    }\n}\n",
    );
    git(&["add", "."]);
    git(&["commit", "-m", "bug: 金额校验缺失"]);

    repo
}

/// 运行 bcc bugfix 命令，返回 Output（强制 prompt-only 模式）
fn bcc_bugfix(args: &[&str]) -> std::process::Output {
    Command::new(env!("CARGO_BIN_EXE_bcc"))
        .arg("bugfix")
        .args(args)
        .env("BCC_FORCE_PROMPT_MODE", "1")
        .output()
        .expect("run bcc bugfix")
}

/// 统计目录下 .json 文件数量
fn count_json_files(dir: &Path) -> usize {
    fs::read_dir(dir)
        .map(|entries| {
            entries
                .filter_map(|e| e.ok())
                .filter(|e| e.path().extension().map_or(false, |ext| ext == "json"))
                .count()
        })
        .unwrap_or(0)
}

/// 统计目录下 .prompt.txt 文件数量
fn count_prompt_files(dir: &Path) -> usize {
    fs::read_dir(dir)
        .map(|entries| {
            entries
                .filter_map(|e| e.ok())
                .filter(|e| {
                    e.path()
                        .to_string_lossy()
                        .ends_with(".prompt.txt")
                })
                .count()
        })
        .unwrap_or(0)
}

// ─── 场景 1：generate 只处理 inventory 中的 commit ─────────

#[test]
fn generate_ignores_stale_context_files() {
    let root = temp_dir("bdd_s1_stale");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // Step 1: collect + context
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c.status.success(), "collect failed");

    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success(), "context failed");

    // Step 2: 注入一个 stale context 文件（hash 不在 inventory 中）
    let stale_path = out.join("contexts/deadbeef_stale.json");
    write(
        &stale_path,
        r#"{"hash":"deadbeef_stale","message":"stale","grade":"C","module":"ghost","tags":[],"diffs":[]}"#,
    );
    assert!(stale_path.exists(), "stale file should exist before generate");

    // Step 3: generate（降级模式，无 codex → 写 prompts/）
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--force",
    ]);
    assert!(g.status.success(), "generate failed");

    // Step 4: 验证 prompts/ 中不包含 stale 文件
    let prompts = out.join("prompts");
    assert!(
        !prompts.join("deadbeef_stale.prompt.txt").exists(),
        "generate should NOT process stale context file outside inventory"
    );

    // 验证 prompts/ 中的文件数 == inventory 中的 commit 数
    let inv: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    )
    .unwrap();
    let inv_count = inv["commits"].as_array().unwrap().len();
    let prompt_count = count_prompt_files(&prompts);
    assert_eq!(
        prompt_count, inv_count,
        "prompts count ({}) should match inventory commits ({})",
        prompt_count, inv_count,
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 2：generate 尊重 grade 过滤 ─────────────────────

#[test]
fn generate_respects_grade_filter() {
    let root = temp_dir("bdd_s2_grade");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // collect 只保留 A 级（repo 中的 commit 都是 A 级）
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A", "--force",
    ]);
    assert!(c.status.success());

    // context
    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success());

    // 注入一个 C 级 context（不在 inventory 中，因为 collect 只保留了 A 级）
    write(
        &out.join("contexts/fake_c_grade.json"),
        r#"{"hash":"fake_c_grade","message":"big change","grade":"C","module":"test","tags":[],"diffs":[]}"#,
    );

    // generate
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--force",
    ]);
    assert!(g.status.success());

    // C 级的不应被处理
    assert!(
        !out.join("prompts/fake_c_grade.prompt.txt").exists(),
        "generate should NOT process C-grade context not in inventory"
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 3：-s o 只执行 organize，不触发 generate ─────────

#[test]
fn step_organize_does_not_trigger_generate() {
    let root = temp_dir("bdd_s3_step_o");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // 先跑 collect + context 准备数据
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c.status.success());
    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success());

    // 手动创建一个 spec 让 organize 有东西处理
    let specs_dir = out.join("specs");
    fs::create_dir_all(&specs_dir).unwrap();
    write(
        &specs_dir.join("dummy.json"),
        r#"{"source_commit":"dummy","module":"test","bug_summary":"test","test_type":"regression"}"#,
    );

    // 确保 prompts/ 不存在
    let prompts = out.join("prompts");
    if prompts.exists() {
        fs::remove_dir_all(&prompts).ok();
    }

    // 只执行 organize
    let o = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "o",
    ]);
    assert!(o.status.success(), "organize should succeed");

    // prompts/ 不应有新文件（generate 未执行）
    let prompt_count = count_prompt_files(&prompts);
    assert_eq!(prompt_count, 0, "organize should NOT trigger generate (found {} prompt files)", prompt_count);

    // by_module/ 应该被创建
    assert!(out.join("by_module").exists(), "organize should create by_module/");

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 4：-s g 只执行 generate，不重跑 collect ──────────

#[test]
fn step_generate_does_not_rerun_collect() {
    let root = temp_dir("bdd_s4_step_g");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // 先 collect
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c.status.success());

    // context
    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success());

    // 记录 inventory.json 的 mtime
    let inv_path = out.join("inventory.json");
    let mtime_before = fs::metadata(&inv_path)
        .unwrap()
        .modified()
        .unwrap();

    // 等 1 秒确保 mtime 差异可检测
    std::thread::sleep(std::time::Duration::from_secs(1));

    // 只执行 generate
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--force",
    ]);
    assert!(g.status.success(), "generate should succeed");

    // inventory.json 的 mtime 不应变化
    let mtime_after = fs::metadata(&inv_path)
        .unwrap()
        .modified()
        .unwrap();
    assert_eq!(
        mtime_before, mtime_after,
        "inventory.json mtime should not change when running -s g alone"
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 5：generate 连续失败 3 次后熔断 ──────────────────

#[test]
fn generate_circuit_breaker_after_consecutive_failures() {
    let root = temp_dir("bdd_s5_breaker");
    let out = root.join("out");
    let contexts = out.join("contexts");
    fs::create_dir_all(&contexts).unwrap();

    // 创建 mock codex 脚本：--version 成功，exec 始终失败
    let mock_dir = root.join("mock_bin");
    fs::create_dir_all(&mock_dir).unwrap();
    let mock_codex = mock_dir.join("codex");
    write(
        &mock_codex,
        "#!/bin/bash\nif [ \"$1\" = \"--version\" ]; then echo 'codex 0.0.1-mock'; exit 0; fi\nexit 1\n",
    );
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&mock_codex, fs::Permissions::from_mode(0o755)).unwrap();
    }

    // 创建 5 个 context 和对应 inventory
    let mut commits = Vec::new();
    for i in 0..5 {
        let hash = format!("breaker{:03}", i);
        write(
            &contexts.join(format!("{}.json", hash)),
            &format!(r#"{{"hash":"{}","message":"fix {}","grade":"A","module":"test","tags":[],"diffs":[]}}"#, hash, i),
        );
        commits.push(serde_json::json!({
            "hash": hash,
            "message": format!("fix {}", i),
            "author": "test",
            "date": "2025-01-01",
            "grade": "A",
            "module": "test",
            "tags": [],
            "changed_files": [],
            "total_lines": 5,
        }));
    }
    let inventory = serde_json::json!({
        "meta": {
            "repo": "/tmp/test",
            "scanned_at": "2025-01-01T00:00:00+0000",
            "total_commits": 5,
            "bugfix_commits": 5,
            "by_grade": {"A": 5},
        },
        "commits": commits,
    });
    write(
        &out.join("inventory.json"),
        &serde_json::to_string_pretty(&inventory).unwrap(),
    );

    // 将 mock_bin 加入 PATH 前面
    let original_path = std::env::var("PATH").unwrap_or_default();
    let new_path = format!("{}:{}", mock_dir.to_str().unwrap(), original_path);

    // 不设置 BCC_FORCE_PROMPT_MODE，让 mock codex 被发现
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .arg("bugfix")
        .args([
            root.join("dummy_repo").to_str().unwrap(), // repo 参数（generate 不使用）
            "-o", out.to_str().unwrap(),
            "--lang", "php",
            "-s", "g",
            "--force",
        ])
        .env("PATH", &new_path)
        .env_remove("BCC_FORCE_PROMPT_MODE")
        .output()
        .expect("run bcc bugfix generate with mock codex");

    let stderr = String::from_utf8_lossy(&output.stderr);

    // 验证 stderr 包含熔断日志
    assert!(
        stderr.contains("circuit breaker"),
        "stderr should contain circuit breaker message, got:\n{}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 6：context 移除死参数后仍正常 ────────────────────

#[test]
fn context_works_after_dead_param_removal() {
    // 编译通过即验证（context 签名已删除 _grades / _limit）
    // 这里做一次端到端调用确认
    let root = temp_dir("bdd_s6_dead_param");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c.status.success());

    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success(), "context should work without dead params");

    assert!(out.join("contexts").exists(), "contexts/ should be created");
    assert!(
        count_json_files(&out.join("contexts")) > 0,
        "should have at least one context file"
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 7：collect --force 清理旧 context 文件 ───────────

#[test]
fn collect_force_cleans_stale_contexts() {
    let root = temp_dir("bdd_s7_cleanup");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // 第一次 collect
    let c1 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c1.status.success());

    // 注入 stale 文件到 contexts/ 和 specs/
    let contexts = out.join("contexts");
    let specs = out.join("specs");
    fs::create_dir_all(&contexts).unwrap();
    fs::create_dir_all(&specs).unwrap();

    let stale_ctx = contexts.join("stale_hash_999.json");
    let stale_spec = specs.join("stale_hash_999.json");
    write(&stale_ctx, r#"{"stale": true}"#);
    write(&stale_spec, r#"{"stale": true}"#);
    assert!(stale_ctx.exists());
    assert!(stale_spec.exists());

    // 再次 collect --force → 应清理 stale 文件
    let c2 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--force",
    ]);
    assert!(c2.status.success());

    assert!(
        !stale_ctx.exists(),
        "stale context file should be cleaned after collect --force"
    );
    assert!(
        !stale_spec.exists(),
        "stale spec file should be cleaned after collect --force"
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景 8：全量运行清理旧产出 ────────────────────────────

#[test]
fn full_run_cleans_stale_artifacts() {
    let root = temp_dir("bdd_s8_full_clean");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // 注入 stale 文件（在第一次运行之前）
    let contexts = out.join("contexts");
    let specs = out.join("specs");
    fs::create_dir_all(&contexts).unwrap();
    fs::create_dir_all(&specs).unwrap();

    let stale_ctx = contexts.join("old_dead_hash.json");
    let stale_spec = specs.join("old_dead_hash.json");
    write(&stale_ctx, r#"{"stale": true}"#);
    write(&stale_spec, r#"{"stale": true}"#);

    // 全量运行（不传 -s）
    let full = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "--force",
    ]);
    assert!(full.status.success(), "full run should succeed");

    // stale 文件应被清理
    assert!(
        !stale_ctx.exists(),
        "stale context should be cleaned during full run"
    );
    assert!(
        !stale_spec.exists(),
        "stale spec should be cleaned during full run"
    );

    // 正常产出应存在
    assert!(out.join("inventory.json").exists());
    assert!(out.join("contexts").exists());
    assert!(count_json_files(&out.join("contexts")) > 0);

    let _ = fs::remove_dir_all(&root);
}

// ═══════════════════════════════════════════════════════════
// 补充场景：覆盖 --limit 端到端 / grade 两轮 / 前置校验 / CLI limit
// ═══════════════════════════════════════════════════════════

// ─── 补充 1：--limit 端到端，两轮 collect 不同 limit ──────────

#[test]
fn limit_end_to_end_two_collect_rounds() {
    let root = temp_dir("bdd_sup1_limit_e2e");
    let (repo, a_count, b_count) = setup_multi_grade_repo(&root);
    let total = a_count + b_count; // 7
    let out = root.join("out");

    // 第一轮：全量 collect + context（7 个 bugfix commit）
    let c1 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A,B", "--force",
    ]);
    assert!(c1.status.success());

    let inv1: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let inv1_count = inv1["commits"].as_array().unwrap().len();
    assert_eq!(inv1_count, total, "first collect should have {} commits", total);

    let x1 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x1.status.success());
    let ctx_full = count_json_files(&out.join("contexts"));
    assert_eq!(ctx_full, total, "first context should produce {} files", total);

    // 第二轮：collect --limit 3 重写 inventory
    let c2 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A,B", "--limit", "3", "--force",
    ]);
    assert!(c2.status.success());

    let inv2: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let inv2_count = inv2["commits"].as_array().unwrap().len();
    assert_eq!(inv2_count, 3, "second collect with --limit 3 should have 3 commits");

    // collect --force 后清理了旧 context，contexts/ 应只剩 3 个
    let ctx_after = count_json_files(&out.join("contexts"));
    assert_eq!(ctx_after, 3, "cleanup after collect should leave 3 context files, got {}", ctx_after);

    // 注入一个 stale context（模拟清理失败或手动残留）
    write(
        &out.join("contexts/stale_leftover.json"),
        r#"{"hash":"stale_leftover","message":"stale","grade":"A","module":"x","tags":[],"diffs":[]}"#,
    );

    // generate — 应只处理 inventory 中的 3 个，忽略 stale
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--force",
    ]);
    assert!(g.status.success());

    let prompt_count = count_prompt_files(&out.join("prompts"));
    assert_eq!(
        prompt_count, 3,
        "generate should process exactly 3 (inventory), not {} (dir has stale)",
        prompt_count,
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 补充 2：两轮 collect 不同 grade ──────────────────────────

#[test]
fn grade_filter_two_collect_rounds() {
    let root = temp_dir("bdd_sup2_grade_e2e");
    let (repo, a_count, b_count) = setup_multi_grade_repo(&root);
    let total = a_count + b_count;
    let out = root.join("out");

    // 第一轮：collect --grade A,B（全部）
    let c1 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A,B", "--force",
    ]);
    assert!(c1.status.success());

    let x1 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x1.status.success());
    assert_eq!(count_json_files(&out.join("contexts")), total);

    // 第二轮：collect --grade A（只保留 A 级）
    let c2 = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A", "--force",
    ]);
    assert!(c2.status.success());

    let inv2: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let inv2_count = inv2["commits"].as_array().unwrap().len();
    assert_eq!(inv2_count, a_count, "second collect --grade A should have {} A-grade commits", a_count);

    // B 级的 context 应被清理
    let ctx_after = count_json_files(&out.join("contexts"));
    assert_eq!(ctx_after, a_count, "cleanup should remove B-grade contexts, leaving {}", a_count);

    // generate
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--force",
    ]);
    assert!(g.status.success());

    let prompt_count = count_prompt_files(&out.join("prompts"));
    assert_eq!(
        prompt_count, a_count,
        "generate should process {} A-grade commits only",
        a_count,
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 补充 3：-s x 缺少 inventory.json → 报错退出 ─────────────

#[test]
fn step_context_fails_without_inventory() {
    let root = temp_dir("bdd_sup3_no_inv_x");
    let out = root.join("out");
    fs::create_dir_all(&out).unwrap();

    let result = bcc_bugfix(&[
        "/nonexistent/repo", "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x",
    ]);

    assert!(
        !result.status.success(),
        "should fail without inventory.json, exit code: {:?}",
        result.status.code(),
    );
    let stderr = String::from_utf8_lossy(&result.stderr);
    assert!(
        stderr.contains("inventory.json not found"),
        "should report missing inventory.json, got:\n{}",
        stderr,
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 补充 4：-s g 缺少 inventory.json → 报错退出 ─────────────

#[test]
fn step_generate_fails_without_inventory() {
    let root = temp_dir("bdd_sup4_no_inv_g");
    let out = root.join("out");
    fs::create_dir_all(&out).unwrap();

    let result = bcc_bugfix(&[
        "/nonexistent/repo", "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g",
    ]);

    assert!(
        !result.status.success(),
        "should fail without inventory.json, exit code: {:?}",
        result.status.code(),
    );
    let stderr = String::from_utf8_lossy(&result.stderr);
    assert!(
        stderr.contains("inventory.json not found"),
        "should report missing inventory.json, got:\n{}",
        stderr,
    );

    let _ = fs::remove_dir_all(&root);
}

// ─── 补充 5：generate --limit N 通过 CLI 限制处理数量 ─────────

#[test]
fn generate_limit_via_cli() {
    let root = temp_dir("bdd_sup5_gen_limit");
    let (repo, _, _) = setup_multi_grade_repo(&root);
    let out = root.join("out");

    // collect + context（全量）
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "c", "--grade", "A,B", "--force",
    ]);
    assert!(c.status.success());

    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "x", "--force",
    ]);
    assert!(x.status.success());

    let inv: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let total = inv["commits"].as_array().unwrap().len();
    assert!(total >= 5, "need at least 5 commits for limit test, got {}", total);

    // generate --limit 2（只处理前 2 个）
    let g = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "php", "-s", "g", "--limit", "2", "--force",
    ]);
    assert!(g.status.success());

    let prompt_count = count_prompt_files(&out.join("prompts"));
    assert_eq!(
        prompt_count, 2,
        "generate with --limit 2 should produce exactly 2 prompts, got {}",
        prompt_count,
    );

    let _ = fs::remove_dir_all(&root);
}

// ═══════════════════════════════════════════════════════════
// Rust 语言支持测试
// ═══════════════════════════════════════════════════════════

// ─── Rust extract 冒烟测试 ───────────────────────────────

#[test]
fn rust_extract_smoke() {
    let root = temp_dir("bdd_rust_extract");
    let rs_file = root.join("sample.rs");
    write(&rs_file, r#"//! 示例模块

use std::collections::HashMap;
use serde::Serialize;

pub struct Config {
    pub name: String,
}

pub enum Status {
    Active,
    Inactive,
}

pub trait Handler {
    fn handle(&self);
}

impl Handler for Config {
    fn handle(&self) {
        println!("{}", self.name);
    }
}

pub fn run(config: &Config) -> Result<(), String> {
    let map = HashMap::new();
    println!("running {}", config.name);
    Ok(())
}

fn private_helper() {
    std::fs::read_to_string("file.txt").ok();
}
"#);

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args(["extract", rs_file.to_str().unwrap(), "--mode", "ast"])
        .output()
        .expect("run bcc extract");
    assert!(output.status.success(), "extract should succeed");

    let json: serde_json::Value = serde_json::from_slice(&output.stdout)
        .expect("output should be valid JSON");

    // 验证语言
    assert_eq!(json["language"], "rust");

    // 验证 exports 包含 pub 声明
    let exports = json["exports"].as_array().unwrap();
    let export_names: Vec<&str> = exports.iter()
        .filter_map(|e| e["name"].as_str())
        .collect();
    assert!(export_names.contains(&"Config"), "should export Config struct");
    assert!(export_names.contains(&"Status"), "should export Status enum");
    assert!(export_names.contains(&"Handler"), "should export Handler trait");
    assert!(export_names.contains(&"run"), "should export run function");

    // 验证 imports
    let imports = json["imports"].as_array().unwrap();
    assert!(imports.len() >= 2, "should have at least 2 imports");

    // 验证 calls 包含 println! 宏
    let calls = json["calls"].as_array().unwrap();
    let call_names: Vec<&str> = calls.iter()
        .filter_map(|c| c["callee"].as_str())
        .collect();
    assert!(call_names.contains(&"println!"), "should detect println! macro");

    // 验证副作用
    let se = &json["side_effects"];
    assert_eq!(se["hasFileIo"], true, "should detect std::fs usage");

    let _ = fs::remove_dir_all(&root);
}

// ─── Rust bugfix collect + context 端到端 ────────────────

/// 创建 Rust 测试用 git 仓库，包含 2 个 bugfix commit
fn setup_rust_bugfix_repo(root: &Path) -> PathBuf {
    let repo = root.join("repo");
    let src = repo.join("src");
    fs::create_dir_all(&src).expect("create repo dirs");

    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(&repo)
            .args(args)
            .output()
            .expect("run git")
    };

    // 初始化 repo
    write(&src.join("main.rs"), r#"pub fn run() {
    println!("hello");
}
"#);
    git(&["init"]);
    git(&["config", "user.email", "test@bdd.com"]);
    git(&["config", "user.name", "bdd-test"]);
    git(&["add", "."]);
    git(&["commit", "-m", "init: rust project"]);

    // bugfix commit 1
    write(&src.join("main.rs"), r#"pub fn run() {
    if std::env::args().len() < 2 {
        eprintln!("missing argument");
        return;
    }
    println!("hello");
}
"#);
    git(&["add", "."]);
    git(&["commit", "-m", "fix: handle empty input"]);

    // bugfix commit 2
    write(&src.join("lib.rs"), r#"pub fn parse(input: &str) -> Result<i32, String> {
    input.parse::<i32>().map_err(|e| e.to_string())
}
"#);
    git(&["add", "."]);
    git(&["commit", "-m", "init: add parser"]);

    write(&src.join("lib.rs"), r#"pub fn parse(input: &str) -> Result<i32, String> {
    let trimmed = input.trim();
    if trimmed.is_empty() {
        return Err("empty input".into());
    }
    trimmed.parse::<i32>().map_err(|e| e.to_string())
}
"#);
    git(&["add", "."]);
    git(&["commit", "-m", "bug: parse 空字符串 panic"]);

    repo
}

#[test]
fn rust_bugfix_collect_and_context() {
    let root = temp_dir("bdd_rust_bugfix");
    let repo = setup_rust_bugfix_repo(&root);
    let out = root.join("out");

    // collect
    let c = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "rust", "-s", "c", "--force",
    ]);
    assert!(c.status.success(), "collect failed: {}", String::from_utf8_lossy(&c.stderr));

    // 验证 inventory.json
    let inv: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let commits = inv["commits"].as_array().unwrap();
    assert!(commits.len() >= 1, "should have at least 1 bugfix commit, got {}", commits.len());

    // context
    let x = bcc_bugfix(&[
        repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
        "--lang", "rust", "-s", "x", "--force",
    ]);
    assert!(x.status.success(), "context failed: {}", String::from_utf8_lossy(&x.stderr));

    // 验证 contexts/ 有文件
    let ctx_count = count_json_files(&out.join("contexts"));
    assert!(ctx_count >= 1, "should have at least 1 context file, got {}", ctx_count);

    let _ = fs::remove_dir_all(&root);
}

// ═══════════════════════════════════════════════════════════
// GitHub Issue 关联测试
// ═══════════════════════════════════════════════════════════

// ─── 场景：collect 在 gh 不可用时静默跳过 issue 拉取 ──────

#[test]
fn collect_skips_issue_when_gh_unavailable() {
    let root = temp_dir("bdd_issue_no_gh");
    let repo = root.join("repo");
    let ctrl = repo.join("app/controllers");
    fs::create_dir_all(&ctrl).expect("create repo dirs");

    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(&repo)
            .args(args)
            .output()
            .expect("run git")
    };

    // 创建含 #N 引用的 commit
    write(&ctrl.join("OrderController.php"), "<?php\nclass OrderController {}\n");
    git(&["init"]);
    git(&["config", "user.email", "test@bdd.com"]);
    git(&["config", "user.name", "bdd-test"]);
    git(&["add", "."]);
    git(&["commit", "-m", "init"]);

    write(
        &ctrl.join("OrderController.php"),
        "<?php\nclass OrderController {\n    public function show() { return 1; }\n}\n",
    );
    git(&["add", "."]);
    git(&["commit", "-m", "fix: handle empty order (#99)"]);

    let out = root.join("out");

    // 用最小 PATH 确保 gh 不可用
    let result = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .arg("bugfix")
        .args([
            repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
            "--lang", "php", "-s", "c", "--force",
        ])
        .env("BCC_FORCE_PROMPT_MODE", "1")
        .env("PATH", "/usr/bin:/bin")
        .output()
        .expect("run bcc bugfix");

    assert!(result.status.success(), "collect should succeed without gh");

    // inventory.json 应存在且 commit 中无 issue 字段
    let inv: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(out.join("inventory.json")).unwrap(),
    ).unwrap();
    let commits = inv["commits"].as_array().unwrap();
    assert!(!commits.is_empty(), "should have commits");
    // 无 gh 时 issue 应为 null/不存在
    for c in commits {
        assert!(c.get("issue").is_none() || c["issue"].is_null(),
            "issue should not be present without gh, got: {}", c);
    }

    let _ = fs::remove_dir_all(&root);
}

// ─── 场景：--issue 参数被 CLI 接受 ──────────────────────

#[test]
fn cli_accepts_issue_flag() {
    let root = temp_dir("bdd_issue_cli_flag");
    let repo = setup_bugfix_repo(&root);
    let out = root.join("out");

    // 传 --issue 99（不存在的 issue，但 gh 不可用时应静默跳过）
    let result = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .arg("bugfix")
        .args([
            repo.to_str().unwrap(), "-o", out.to_str().unwrap(),
            "--lang", "php", "-s", "c", "--force", "--issue", "99",
        ])
        .env("BCC_FORCE_PROMPT_MODE", "1")
        .env("PATH", "/usr/bin:/bin")
        .output()
        .expect("run bcc bugfix with --issue");

    assert!(result.status.success(), "collect with --issue should succeed, stderr: {}",
        String::from_utf8_lossy(&result.stderr));
    assert!(out.join("inventory.json").exists(), "inventory.json should be created");

    let _ = fs::remove_dir_all(&root);
}
