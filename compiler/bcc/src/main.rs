use bcc::*;
use clap::{ArgAction, Parser, Subcommand};

mod bdd_seed;
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
        /// Source file or directory path
        path: String,

        /// Output mode: ast (JSON), doc (markdown), yaml (draft)
        #[arg(long, default_value = "ast")]
        mode: String,

        /// Output file (default: stdout; required in --batch mode)
        #[arg(long)]
        output: Option<String>,

        /// Batch mode: recursively extract all files in directory, output AstSnapshot JSON
        #[arg(long)]
        batch: bool,

        /// Language filter for batch mode: typescript, elixir, rust, php
        #[arg(long)]
        lang: Option<String>,
    },

    /// Audit documentation coverage against source files
    Trace {
        #[command(subcommand)]
        action: TraceAction,
    },

    /// 从 git bugfix 历史提取测试规格书
    ///
    /// 四步流水线及产出：
    ///
    ///   collect(c)  扫描 git log → inventory.json（commit 列表、分级、标签）
    ///   context(x)  提取 diff + 函数 before/after → contexts/*.json
    ///   generate(g) 调用 LLM 生成测试规格书 → specs/*.json
    ///               需要安装 codex CLI（npm i -g @openai/codex）
    ///               未安装时降级：输出 prompts/*.prompt.txt 供手动喂给 LLM
    ///   organize(o) 按模块归类 → by_module/*.json + coverage.md
    ///
    /// 最终产出是结构化的测试规格书（JSON），作为 bddc autochain 的输入：
    ///   bcc bugfix = "这些 bug 要测什么"（知识提取）
    ///   bddc autochain = "怎么做测试"（指令设计 + DSL + 编译 + 运行）
    #[command(
        name = "bugfix",
        after_help = r#"示例:
  bcc bugfix /path/to/repo -o output/                    # 全量执行四步（当前分支 + 关键字）
  bcc bugfix /path/to/repo -o output/ -b main            # 指定扫描 main 分支
  bcc bugfix /path/to/repo -o output/ --path app/controllers/  # 按文件路径扫描
  bcc bugfix /path/to/repo -o output/ -s c               # 只扫描，输出 inventory.json
  bcc bugfix /path/to/repo -o output/ -s g --limit 20    # 前 20 个 commit 跑到生成
  bcc bugfix /path/to/repo -o output/ --lang elixir      # 扫描 Elixir 项目的 bugfix

产出目录结构:
  output/
  ├── inventory.json          # collect: commit 清单（hash/grade/module/tags）
  ├── contexts/               # context: 每个 commit 的 diff + 函数上下文
  │   └── <hash>.json
  ├── specs/                  # generate: 每个 commit 的测试规格书
  │   └── <hash>.json
  ├── prompts/                # generate 降级: 未安装 codex 时输出 prompt 文件
  │   └── <hash>.prompt.txt
  ├── by_module/              # organize: 按模块归类的规格书数组
  │   └── <module>.json
  └── coverage.md             # organize: 模块覆盖率报告

下游使用（bddc autochain 消费规格书）:
  bddc domain.autowire --specs output/by_module/order.json"#
    )]
    Bugfix {
        /// Git 仓库路径
        repo: Option<String>,

        /// 输出目录（collect → inventory.json，context → contexts/，
        /// generate → specs/，organize → by_module/ + coverage.md）
        #[arg(short, long)]
        output: Option<String>,

        /// 执行到哪一步停止，不指定则全部执行
        ///   collect(c)  — git log 扫描分级
        ///   context(x)  — diff + 函数上下文
        ///   generate(g) — LLM 生成测试规格书 JSON
        ///   organize(o) — 按模块归类 + 覆盖率报告
        #[arg(short, long, value_name = "STEP")]
        step: Option<String>,

        /// 源码语言：php, elixir, typescript, rust [默认: php]
        #[arg(short, long, default_value = "php")]
        lang: String,

        /// 筛选级别（逗号分隔）：A(≤10行) B(10-50行) C(>50行)
        #[arg(long, default_value = "A,B,C")]
        grade: String,

        /// 扫描分支（默认当前分支，不再扫全部分支）
        #[arg(short, long)]
        branch: Option<String>,

        /// 按文件路径扫描（传了则用 git log <branch> -- <path>，不再按关键字匹配）
        #[arg(long, value_name = "PATH")]
        path: Option<String>,

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

        /// 手动指定关联的 GitHub Issue 编号（覆盖 commit message 中的 #N 自动提取）
        #[arg(long)]
        issue: Option<u64>,
    },

    /// 架构矩阵与门禁工具（matrix/validate/export-module-map/report/score）
    Arch {
        #[command(subcommand)]
        action: ArchAction,
    },

    /// BDD 场景种子生成（新项目主线）
    Bdd {
        #[command(subcommand)]
        action: BddAction,
    },

    /// 代码图谱索引工具（build/query/analyze）
    Graph {
        #[command(subcommand)]
        action: GraphAction,
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

#[derive(Subcommand)]
enum ArchAction {
    /// 生成 target/transition/gates 初稿
    Matrix {
        #[arg(long)]
        seed_file: String,
        #[arg(long)]
        ast_file: String,
        #[arg(long, default_value = "docs/backend-trace/trace2contract/seed")]
        out_dir: String,
        #[arg(long, default_value = "v3")]
        version: String,
        #[arg(long, default_value = "all")]
        emit: String,
        #[arg(long, default_value_t = false)]
        detect_injection: bool,
        #[arg(long)]
        injection_patterns: Option<String>,
        #[arg(long)]
        force: bool,
    },

    /// 回放实际依赖并做 gate 验证
    Validate {
        #[arg(long)]
        target: String,
        #[arg(long)]
        transition: String,
        #[arg(long)]
        gates: String,
        #[arg(long)]
        actual: String,
        #[arg(
            long,
            default_value = "docs/backend-trace/artifacts/trace2contract/versions/v3-draft"
        )]
        out_dir: String,
        #[arg(long, default_value = "both")]
        profile: String,
        #[arg(long, default_value_t = true, action = ArgAction::Set)]
        fail_on_gate: bool,
        #[arg(long, default_value_t = true, action = ArgAction::Set)]
        fail_on_forbidden: bool,
        /// 导出 bdd seed 可消费的 YAML source 文件到指定目录
        #[arg(long)]
        export_bdd_source: Option<String>,
    },

    /// 导出 bugfix 可消费的 module_map.json
    ExportModuleMap {
        #[arg(long)]
        module_map: String,
        #[arg(long)]
        module_registry: Option<String>,
        #[arg(
            long,
            default_value = "docs/backend-trace/artifacts/module_map.bugfix.json"
        )]
        out: String,
        #[arg(long, default_value = "file")]
        mapping_mode: String,
        #[arg(long, default_value_t = true)]
        include_module_names: bool,
    },

    /// 聚合架构债务报告
    Report {
        #[arg(long)]
        scenario_validation: String,
        #[arg(long)]
        gate_evaluation: String,
        #[arg(long)]
        summary: String,
        #[arg(long)]
        out: String,
        #[arg(long, default_value_t = 20)]
        top: usize,
        #[arg(long, default_value = "md")]
        format: String,
    },

    /// 架构健康度评分
    Score {
        #[command(subcommand)]
        action: arch::score::cli::ScoreAction,
    },
}

#[derive(Subcommand)]
enum BddAction {
    /// 生成/归并 BDD 场景种子
    Seed {
        #[arg(long)]
        source: String,
        #[arg(short, long)]
        output: String,
        #[arg(
            short,
            long,
            default_value = "organize",
            help = "context|generate|organize|check|fix"
        )]
        step: String,
        #[arg(long)]
        module: Option<String>,
        #[arg(long, default_value = "all")]
        edge_class: String,
        #[arg(long)]
        limit: Option<usize>,
        #[arg(long)]
        prompt_template: Option<String>,
        #[arg(long)]
        coverage_report: Option<String>,
        #[arg(long)]
        force: bool,
    },
}

#[derive(Subcommand)]
enum GraphAction {
    /// 从 extract 输出构建索引
    Build {
        /// 仓库ID (如 github.com/HKUDS/nanobot)
        #[arg(short, long)]
        repo: String,
        
        /// 仓库名称
        #[arg(short, long)]
        name: String,
        
        /// 仓库根路径
        #[arg(short, long)]
        path: String,
        
        /// extract 输出的 JSON 文件路径
        #[arg(short, long)]
        input: String,
        
        /// commit hash
        #[arg(short, long)]
        commit: String,
    },
    
    /// 查询函数信息
    Query {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
        
        /// 查询目标ID
        #[arg(short, long)]
        id: String,
        
        /// 查询类型
        #[arg(short, long, default_value = "id")]
        by: String,
        
        /// 查询深度（用于 callers/callees）
        #[arg(short, long, default_value = "3")]
        depth: usize,
    },
    
    /// 分析影响面
    Analyze {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
        
        /// 函数ID
        #[arg(short, long)]
        id: String,
    },
    
    /// 列出所有索引的仓库
    List,
    
    /// 删除仓库索引
    Delete {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
    },
    
    /// 图搜索（多关系融合查询）
    Search {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
        
        /// 函数ID
        #[arg(short, long)]
        id: String,
        
        /// 搜索深度
        #[arg(short, long, default_value = "2")]
        depth: usize,
        
        /// 包含的关系类型（逗号分隔: callers,callees,siblings,same-file,same-module）
        #[arg(short, long, default_value = "callers,callees")]
        include: String,
    },
    
    /// 架构验证
    ValidateArch {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
        
        /// 目标架构 YAML 文件路径
        #[arg(short, long)]
        target: String,
        
        /// 输出 JSON 文件路径
        #[arg(short, long)]
        output: Option<String>,
    },
    
    /// 模块依赖查询
    Module {
        /// 仓库ID
        #[arg(short, long)]
        repo: String,
        
        /// 模块ID（文件路径）
        #[arg(short, long)]
        id: String,
        
        /// 查询类型
        #[arg(short, long, default_value = "id")]
        by: String,
        
        /// 查询深度
        #[arg(short, long, default_value = "3")]
        depth: usize,
    },
}

fn main() {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::Compile {
            path,
            dry_run,
            emit_ast,
            passes,
            output,
            force,
            verbose,
        }) => {
            compile::run(
                &path,
                dry_run,
                emit_ast,
                &passes,
                output.as_deref(),
                force,
                verbose,
            );
        }
        Some(Commands::Extract {
            path,
            mode,
            output,
            batch,
            lang,
        }) => {
            if batch {
                let lang = lang.unwrap_or_else(|| {
                    eprintln!("--lang is required in --batch mode");
                    std::process::exit(1);
                });
                let lang = extract::normalize_batch_lang(&lang).unwrap_or_else(|err| {
                    eprintln!("{}", err);
                    std::process::exit(1);
                });
                let output = output.unwrap_or_else(|| {
                    eprintln!("--output is required in --batch mode");
                    std::process::exit(1);
                });
                extract::run_batch(&path, &lang, &output);
            } else {
                extract::run(&path, &mode, output.as_deref());
            }
        }
        Some(Commands::Trace { action }) => match action {
            TraceAction::Status {
                source_dir,
                docs_dir,
            } => {
                trace::status(&source_dir, &docs_dir);
            }
            TraceAction::Report {
                source_dir,
                docs_dir,
                output,
            } => {
                trace::report(&source_dir, &docs_dir, &output);
            }
            TraceAction::Seed {
                source_dir,
                docs_dir,
                write,
                max,
            } => {
                trace::seed(&source_dir, &docs_dir, write, max);
            }
        },
        Some(Commands::Bugfix {
            repo,
            output,
            step,
            lang,
            branch,
            path,
            grade,
            keywords,
            module_map,
            prompt_template,
            limit,
            force,
            coverage_report,
            issue,
        }) => {
            let repo = match repo {
                Some(r) if r != "help" => r,
                _ => {
                    // 没给 repo 或 repo="help"，打印帮助
                    eprintln!("从 git bugfix 历史提取测试规格书\n");
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
                &repo,
                &output,
                step.as_deref(),
                &lang,
                branch.as_deref(),
                path.as_deref(),
                &grade,
                &keywords,
                module_map.as_deref(),
                prompt_template.as_deref(),
                limit,
                force,
                coverage_report.as_deref(),
                issue,
            );
        }
        Some(Commands::Arch { action }) => match action {
            ArchAction::Matrix {
                seed_file,
                ast_file,
                out_dir,
                version,
                emit,
                detect_injection,
                injection_patterns,
                force,
            } => {
                arch::matrix(
                    &seed_file,
                    &ast_file,
                    &out_dir,
                    &version,
                    &emit,
                    detect_injection,
                    injection_patterns.as_deref(),
                    force,
                );
            }
            ArchAction::Validate {
                target,
                transition,
                gates,
                actual,
                out_dir,
                profile,
                fail_on_gate,
                fail_on_forbidden,
                export_bdd_source,
            } => {
                arch::validate(
                    &target,
                    &transition,
                    &gates,
                    &actual,
                    &out_dir,
                    &profile,
                    fail_on_gate,
                    fail_on_forbidden,
                    export_bdd_source.as_deref(),
                );
            }
            ArchAction::ExportModuleMap {
                module_map,
                module_registry,
                out,
                mapping_mode,
                include_module_names,
            } => {
                arch::export_module_map(
                    &module_map,
                    module_registry.as_deref(),
                    &out,
                    &mapping_mode,
                    include_module_names,
                );
            }
            ArchAction::Report {
                scenario_validation,
                gate_evaluation,
                summary,
                out,
                top,
                format,
            } => {
                arch::report(
                    &scenario_validation,
                    &gate_evaluation,
                    &summary,
                    &out,
                    top,
                    &format,
                );
            }
            ArchAction::Score { action } => {
                action.execute();
            }
        },
        Some(Commands::Bdd { action }) => match action {
            BddAction::Seed {
                source,
                output,
                step,
                module,
                edge_class,
                limit,
                prompt_template,
                coverage_report,
                force,
            } => {
                let step_norm = step.to_ascii_lowercase();
                if !["context", "generate", "organize", "check", "fix"]
                    .contains(&step_norm.as_str())
                {
                    eprintln!(
                        "invalid --step '{}': expected context|generate|organize|check|fix",
                        step
                    );
                    std::process::exit(1);
                }
                bdd_seed::run(
                    &source,
                    &output,
                    &step_norm,
                    module.as_deref(),
                    &edge_class,
                    limit,
                    prompt_template.as_deref(),
                    coverage_report.as_deref(),
                    force,
                );
            }
        },
        Some(Commands::Graph { action }) => match action {
            GraphAction::Build { repo, name, path, input, commit } => {
                if let Err(e) = graph::cli::build_index(&repo, &name, &path, &input, &commit) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::Query { repo, id, by, depth } => {
                let query_type = match by.as_str() {
                    "id" => graph::cli::QueryType::ById,
                    "name" => graph::cli::QueryType::ByName,
                    "module" => graph::cli::QueryType::ByModule,
                    "callers" => graph::cli::QueryType::Callers { depth },
                    "callees" => graph::cli::QueryType::Callees { depth },
                    _ => {
                        eprintln!("Invalid query type: {}", by);
                        std::process::exit(10);
                    }
                };
                if let Err(e) = graph::cli::query_function(&repo, &id, query_type) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::Analyze { repo, id } => {
                let query_type = graph::cli::QueryType::Impact;
                if let Err(e) = graph::cli::query_function(&repo, &id, query_type) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::List => {
                if let Err(e) = graph::cli::list_repos() {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::Delete { repo } => {
                if let Err(e) = graph::cli::delete_repo(&repo) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::Search { repo, id, depth, include } => {
                if let Err(e) = graph::cli::search_graph(&repo, &id, depth, &include) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::ValidateArch { repo, target, output } => {
                if let Err(e) = graph::cli::validate_arch(&repo, &target, output.as_deref()) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
            GraphAction::Module { repo, id, by, depth } => {
                let query_type = match by.as_str() {
                    "id" => graph::cli::ModuleQueryType::ById,
                    "deps" => graph::cli::ModuleQueryType::Deps { depth },
                    "dependents" => graph::cli::ModuleQueryType::Dependents { depth },
                    "circular" => graph::cli::ModuleQueryType::Circular,
                    _ => {
                        eprintln!("Invalid module query type: {}", by);
                        std::process::exit(10);
                    }
                };
                if let Err(e) = graph::cli::query_module(&repo, &id, query_type) {
                    eprintln!("[graph-index] Error: {}", e);
                    std::process::exit(e.exit_code());
                }
            }
        },
        None => {
            eprintln!("Usage: bcc <COMMAND>\n\nFor more information, try '--help'.");
            std::process::exit(2);
        }
    }
}
