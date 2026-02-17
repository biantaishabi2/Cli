//! CLI 命令定义和处理

use clap::Subcommand;

#[derive(Subcommand)]
pub enum ScoreAction {
    /// 计算架构健康度评分
    Score {
        /// 验证结果目录（包含 summary.json, scenario-validation.tsv, gate-evaluation.tsv）
        #[arg(long)]
        input: String,

        /// 评分配置文件路径
        #[arg(long)]
        config: Option<String>,

        /// 评分模式: strict | lenient | warning
        #[arg(long, default_value = "strict")]
        mode: String,

        /// 输出格式: text | json | markdown
        #[arg(long, default_value = "text")]
        format: String,

        /// 输出文件路径（默认 stdout）
        #[arg(short, long)]
        output: Option<String>,

        /// 失败阈值
        #[arg(long, default_value_t = 60.0)]
        threshold: f64,

        /// 详细输出
        #[arg(short, long)]
        verbose: bool,
    },

    /// 生成默认配置文件
    InitConfig {
        /// 输出文件路径
        #[arg(short, long, default_value = "arch-score.yaml")]
        output: String,

        /// 配置模板: default | strict | lenient | minimal
        #[arg(long, default_value = "default")]
        template: String,
    },

    /// 对比多个版本的评分
    Compare {
        /// 多个输入目录
        #[arg(long, num_args = 2..)]
        inputs: Vec<String>,

        /// 版本标签
        #[arg(long, num_args = 2..)]
        labels: Vec<String>,

        /// 输出格式
        #[arg(long, default_value = "markdown")]
        format: String,

        /// 输出文件路径
        #[arg(short, long)]
        output: Option<String>,
    },
}

impl ScoreAction {
    pub fn execute(self) {
        match self {
            ScoreAction::Score {
                input,
                config,
                mode,
                format,
                output,
                threshold,
                verbose,
            } => {
                super::score(
                    &input,
                    config.as_deref(),
                    &mode,
                    &format,
                    output.as_deref(),
                    threshold,
                    verbose,
                );
            }
            ScoreAction::InitConfig { output, template } => {
                super::init_config(&output, &template);
            }
            ScoreAction::Compare {
                inputs,
                labels,
                format,
                output,
            } => {
                super::compare(&inputs, &labels, &format, output.as_deref());
            }
        }
    }
}
