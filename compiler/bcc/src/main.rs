use clap::{Parser, Subcommand};

mod spec;
mod compile;
mod extract;
mod trace;
mod bugfix;

/// BCC — Backend Compiler for YAML-driven Elixir skeleton generation
#[derive(Parser)]
#[command(name = "bcc", version, about)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// Compile YAML contract to Elixir module skeleton
    Compile {
        /// YAML contract file path
        path: String,

        /// Only parse and validate, do not generate code
        #[arg(long)]
        dry_run: bool,

        /// Emit AST as JSON instead of Elixir source
        #[arg(long)]
        emit_ast: bool,

        /// Comma-separated list of passes to apply (e.g. envelope,error_code)
        #[arg(long, default_value = "envelope,error_code")]
        passes: String,

        /// Output directory
        #[arg(short, long)]
        output: Option<String>,

        /// Force overwrite existing output
        #[arg(long)]
        force: bool,

        /// Show verbose emit pipeline logs
        #[arg(long)]
        verbose: bool,
    },

    /// Extract structural info from source files into FileRecord JSON
    Extract {
        /// Source file path
        path: String,

        /// Output mode: ast (JSON), doc (markdown), yaml (draft)
        #[arg(long, default_value = "ast")]
        mode: String,

        /// Output file (default: stdout)
        #[arg(long)]
        output: Option<String>,
    },

    /// Audit documentation coverage against source files
    Trace {
        #[command(subcommand)]
        action: TraceAction,
    },

    /// 从 git bugfix 历史中提取 BDD 场景
    ///
    /// 四步流水线：collect → context → generate → organize
    ///   collect(c)  扫描 git log，按变更行数分级(A/B/C)，自动打标签
    ///   context(x)  提取每个 commit 的 diff 和修改函数的 before/after 代码
    ///   generate(g) 调用 codex exec 将上下文转为 bddc DSL 场景
    ///   organize(o) 按模块归类，标记疑似重复，生成覆盖率报告
    #[command(
        name = "bugfix",
        after_help = r#"示例:
  bcc bugfix /path/to/repo -o output/                    # 全量执行四步
  bcc bugfix /path/to/repo -o output/ -s c               # 只扫描，输出 inventory.json
  bcc bugfix /path/to/repo -o output/ -s x               # 扫描 + 上下文提取
  bcc bugfix /path/to/repo -o output/ -s g --limit 20    # 前 20 个 commit 跑到生成
  bcc bugfix /path/to/repo -o output/ --lang elixir      # 扫描 Elixir 项目的 bugfix"#
    )]
    Bugfix {
        /// Git 仓库路径
        repo: Option<String>,

        /// 输出目录（collect 输出 inventory.json，context 输出 contexts/，
        /// generate 输出 scenarios/，organize 输出 features/ + coverage.md）
        #[arg(short, long)]
        output: Option<String>,

        /// 执行到哪一步停止，不指定则全部执行
        ///   collect(c)  — git log 扫描分级
        ///   context(x)  — diff + 函数上下文
        ///   generate(g) — codex exec 生成 DSL
        ///   organize(o) — 归类 + 覆盖率报告
        #[arg(short, long, value_name = "STEP")]
        step: Option<String>,

        /// 源码语言：php, elixir, typescript [默认: php]
        #[arg(short, long, default_value = "php")]
        lang: String,

        /// 筛选级别（逗号分隔）：A(≤10行) B(10-50行) C(>50行)
        #[arg(long, default_value = "A,B")]
        grade: String,

        /// 扫描关键字（逗号分隔），匹配 commit message
        #[arg(long, default_value = "修复,fix,bug")]
        keywords: String,

        /// 模块映射 JSON（格式: {"mapping":{"path/prefix":"MODULE"}, "module_names":{"MODULE":"名称"}}）
        #[arg(long)]
        module_map: Option<String>,

        /// 自定义 prompt 模板文件路径（默认使用内置模板）
        #[arg(long)]
        prompt_template: Option<String>,

        /// 最多处理 N 个 commit
        #[arg(long)]
        limit: Option<usize>,

        /// 强制重做已有输出（默认跳过已存在的文件）
        #[arg(long)]
        force: bool,

        /// 覆盖率报告输出路径（默认: <output>/coverage.md）
        #[arg(long)]
        coverage_report: Option<String>,
    },
}

#[derive(Subcommand)]
enum TraceAction {
    /// Show coverage status
    Status {
        /// Source directory
        source_dir: String,
        /// Documentation directory
        docs_dir: String,
    },

    /// Generate audit report files
    Report {
        /// Source directory
        source_dir: String,
        /// Documentation directory
        docs_dir: String,
        /// Output directory for report
        #[arg(long)]
        output: String,
    },

    /// Create template drafts for missing docs
    Seed {
        /// Source directory
        source_dir: String,
        /// Documentation directory
        docs_dir: String,
        /// Actually write files
        #[arg(long)]
        write: bool,
        /// Max files to seed
        #[arg(long)]
        max: Option<usize>,
    },
}

fn main() {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::Compile { path, dry_run, emit_ast, passes, output, force, verbose }) => {
            compile::run(&path, dry_run, emit_ast, &passes, output.as_deref(), force, verbose);
        }
        Some(Commands::Extract { path, mode, output }) => {
            extract::run(&path, &mode, output.as_deref());
        }
        Some(Commands::Trace { action }) => match action {
            TraceAction::Status { source_dir, docs_dir } => {
                trace::status(&source_dir, &docs_dir);
            }
            TraceAction::Report { source_dir, docs_dir, output } => {
                trace::report(&source_dir, &docs_dir, &output);
            }
            TraceAction::Seed { source_dir, docs_dir, write, max } => {
                trace::seed(&source_dir, &docs_dir, write, max);
            }
        },
        Some(Commands::Bugfix {
            repo, output, step, lang, grade, keywords,
            module_map, prompt_template, limit, force, coverage_report,
        }) => {
            let repo = match repo {
                Some(r) if r != "help" => r,
                _ => {
                    // 没给 repo 或 repo="help"，打印帮助
                    eprintln!("从 git bugfix 历史中提取 BDD 场景\n");
                    eprintln!("用法: bcc bugfix <REPO> -o <OUTPUT> [OPTIONS]\n");
                    eprintln!("详细帮助: bcc bugfix --help");
                    std::process::exit(0);
                }
            };
            let output = match output {
                Some(o) => o,
                None => {
                    eprintln!("缺少 --output 参数\n");
                    eprintln!("用法: bcc bugfix <REPO> -o <OUTPUT> [OPTIONS]\n");
                    eprintln!("详细帮助: bcc bugfix --help");
                    std::process::exit(1);
                }
            };
            bugfix::run(
                &repo, &output,
                step.as_deref(),
                &lang,
                &grade, &keywords,
                module_map.as_deref(),
                prompt_template.as_deref(),
                limit, force,
                coverage_report.as_deref(),
            );
        }
        None => {
            eprintln!("Usage: bcc <COMMAND>\n\nFor more information, try '--help'.");
            std::process::exit(2);
        }
    }
}
