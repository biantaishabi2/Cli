use clap::{Parser, Subcommand};

mod spec;
mod compile;
mod extract;
mod trace;

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
        None => {
            eprintln!("Usage: bcc <COMMAND>\n\nFor more information, try '--help'.");
            std::process::exit(2);
        }
    }
}
