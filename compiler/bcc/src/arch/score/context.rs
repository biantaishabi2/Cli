//! 评分上下文
//!
//! 提供评分所需的所有输入数据

use serde::Deserialize;
use std::collections::{BTreeMap, HashMap};
use std::path::Path;

/// 评分上下文
#[derive(Debug, Clone, Default)]
pub struct ScoringContext {
    /// 验证场景结果
    pub scenario: ScenarioData,
    /// 结构统计
    pub structure: StructureData,
    /// 门禁检查结果
    pub gate_checks: Vec<GateCheckData>,
    /// 模块信息
    pub modules: Vec<ModuleInfo>,
    /// 原始数据（用于扩展）
    pub raw_data: RawValidationData,
}

/// 场景数据
#[derive(Debug, Clone, Default)]
pub struct ScenarioData {
    pub name: String,
    pub allow_count: i64,
    pub forbid_count: i64,
    pub matched_edges_count: i64,
    pub matched_total_edges: i64,
    pub unexpected_edges_count: i64,
    pub unexpected_total_edges: i64,
    pub forbidden_edges_count: i64,
    pub forbidden_total_edges: i64,
    pub missing_edges_count: i64,
}

/// 结构数据
#[derive(Debug, Clone, Default)]
pub struct StructureData {
    pub modules_count: i64,
    pub directed_edges_possible: i64,
    pub directed_edges_actual: i64,
    pub directed_density_pct: f64,
    pub total_module_edge_weight: i64,
    pub bidirectional_pair_count: i64,
    pub bidirectional_pairs_top: Vec<(String, i64, i64, i64)>,
}

/// 门禁检查数据
#[derive(Debug, Clone)]
pub struct GateCheckData {
    pub profile: String,
    pub metric: String,
    pub actual: f64,
    pub limit: f64,
    pub passed: bool,
}

/// 模块信息
#[derive(Debug, Clone)]
pub struct ModuleInfo {
    pub id: String,
    pub name: String,
    pub layer: Option<String>,
    pub tags: Vec<String>,
    pub dependencies: Vec<String>,
}

/// 原始验证数据
#[derive(Debug, Clone, Default)]
pub struct RawValidationData {
    pub summary: Option<serde_json::Value>,
    pub scenario_rows: Vec<HashMap<String, String>>,
    pub gate_rows: Vec<HashMap<String, String>>,
}

// 输入数据结构定义
#[derive(Debug, Deserialize)]
struct SummaryJson {
    scenarios: BTreeMap<String, ScenarioSummary>,
}

#[derive(Debug, Deserialize)]
struct ScenarioSummary {
    unexpected_edges_count: i64,
    forbidden_edges_count: i64,
    gate_pass: bool,
}

impl ScoringContext {
    /// 从输入目录构建上下文
    pub fn from_input_dir(input_dir: &str) -> Result<Self, String> {
        let path = Path::new(input_dir);

        // 读取 summary.json
        let summary_path = path.join("summary.json");
        let summary_data = if summary_path.exists() {
            let content = std::fs::read_to_string(&summary_path)
                .map_err(|e| format!("read summary.json failed: {}", e))?;
            Some(
                serde_json::from_str(&content)
                    .map_err(|e| format!("parse summary.json failed: {}", e))?,
            )
        } else {
            None
        };

        // 读取 scenario-validation.tsv
        let scenario_path = path.join("scenario-validation.tsv");
        let scenario_rows = if scenario_path.exists() {
            parse_tsv(&std::fs::read_to_string(&scenario_path).unwrap_or_default())
        } else {
            Vec::new()
        };

        // 读取 gate-evaluation.tsv
        let gate_path = path.join("gate-evaluation.tsv");
        let gate_rows = if gate_path.exists() {
            parse_tsv(&std::fs::read_to_string(&gate_path).unwrap_or_default())
        } else {
            Vec::new()
        };

        // 构建场景数据（优先使用 target 场景）
        let scenario = Self::build_scenario_data(&scenario_rows, &summary_data)?;

        // 构建结构数据
        let structure = Self::build_structure_data(&scenario_rows, &gate_rows)?;

        // 构建门禁检查数据
        let gate_checks = Self::build_gate_checks(&gate_rows)?;

        // 构建模块信息（从数据中推断）
        let modules = Self::build_module_info(&scenario_rows)?;

        Ok(Self {
            scenario,
            structure,
            gate_checks,
            modules,
            raw_data: RawValidationData {
                summary: summary_data,
                scenario_rows,
                gate_rows,
            },
        })
    }

    fn build_scenario_data(
        rows: &[HashMap<String, String>],
        _summary: &Option<serde_json::Value>,
    ) -> Result<ScenarioData, String> {
        // 优先查找 target 场景，如果没有则使用第一个
        let row = rows
            .iter()
            .find(|r| r.get("name").map(|n| n.contains("target")).unwrap_or(false))
            .or(rows.first())
            .cloned()
            .unwrap_or_default();

        Ok(ScenarioData {
            name: row.get("name").cloned().unwrap_or_else(|| "default".into()),
            allow_count: parse_i64(row.get("allow_count")),
            forbid_count: parse_i64(row.get("forbid_count")),
            matched_edges_count: parse_i64(row.get("matched_edges_count")),
            matched_total_edges: parse_i64(row.get("matched_total_edges")),
            unexpected_edges_count: parse_i64(row.get("unexpected_edges_count")),
            unexpected_total_edges: parse_i64(row.get("unexpected_total_edges")),
            forbidden_edges_count: parse_i64(row.get("forbidden_edges_count")),
            forbidden_total_edges: parse_i64(row.get("forbidden_total_edges")),
            missing_edges_count: parse_i64(row.get("missing_edges_count")),
        })
    }

    fn build_structure_data(
        scenario_rows: &[HashMap<String, String>],
        gate_rows: &[HashMap<String, String>],
    ) -> Result<StructureData, String> {
        // 从 gate_rows 中提取结构相关指标
        let mut modules_count = 0i64;
        let mut directed_density_pct = 0.0;
        let mut bidirectional_pair_count = 0i64;

        for row in gate_rows {
            if let Some(metric) = row.get("metric") {
                match metric.as_str() {
                    "directed_density_pct" => {
                        if let Ok(v) = row.get("actual").unwrap_or(&"0".to_string()).parse::<f64>()
                        {
                            directed_density_pct = v;
                        }
                    }
                    "bidirectional_pair_count" => {
                        bidirectional_pair_count = parse_i64(row.get("actual"));
                    }
                    _ => {}
                }
            }
        }

        // 估算模块数
        if let Some(row) = scenario_rows.first() {
            let allow = parse_i64(row.get("allow_count"));
            let forbid = parse_i64(row.get("forbid_count"));
            // 粗略估算：假设平均每个模块有 5 个依赖
            modules_count = ((allow + forbid) as f64 / 5.0).sqrt() as i64;
        }

        let directed_edges_possible = if modules_count > 1 {
            modules_count * (modules_count - 1)
        } else {
            0
        };

        let directed_edges_actual =
            (directed_edges_possible as f64 * directed_density_pct / 100.0) as i64;

        Ok(StructureData {
            modules_count,
            directed_edges_possible,
            directed_edges_actual,
            directed_density_pct,
            total_module_edge_weight: 0,
            bidirectional_pair_count,
            bidirectional_pairs_top: Vec::new(),
        })
    }

    fn build_gate_checks(rows: &[HashMap<String, String>]) -> Result<Vec<GateCheckData>, String> {
        let mut checks = Vec::new();

        for row in rows {
            let actual_str = row.get("actual").cloned().unwrap_or_default();
            let limit_str = row.get("limit").cloned().unwrap_or_default();

            // 尝试解析为数字
            let actual = actual_str.parse::<f64>().unwrap_or(0.0);
            let limit = limit_str.parse::<f64>().unwrap_or(0.0);

            checks.push(GateCheckData {
                profile: row.get("profile").cloned().unwrap_or_default(),
                metric: row.get("metric").cloned().unwrap_or_default(),
                actual,
                limit,
                passed: row.get("pass").map(|p| p == "pass").unwrap_or(false),
            });
        }

        Ok(checks)
    }

    fn build_module_info(
        _scenario_rows: &[HashMap<String, String>],
    ) -> Result<Vec<ModuleInfo>, String> {
        // TODO: 从 module_registry 或其他来源加载模块信息
        Ok(Vec::new())
    }

    /// 获取特定指标的门禁检查
    pub fn get_gate_check(&self, metric: &str) -> Option<&GateCheckData> {
        self.gate_checks.iter().find(|g| g.metric == metric)
    }

    /// 获取特定配置文件的检查
    pub fn get_gate_check_for_profile(
        &self,
        profile: &str,
        metric: &str,
    ) -> Option<&GateCheckData> {
        self.gate_checks
            .iter()
            .find(|g| g.profile == profile && g.metric == metric)
    }

    /// 计算总依赖数
    pub fn total_dependencies(&self) -> i64 {
        self.scenario.matched_edges_count
            + self.scenario.unexpected_edges_count
            + self.scenario.forbidden_edges_count
    }

    /// 计算模块对数
    pub fn total_module_pairs(&self) -> i64 {
        let n = self.structure.modules_count;
        if n > 1 {
            n * (n - 1) / 2
        } else {
            0
        }
    }

    /// 分析分层违规（简化版）
    pub fn analyze_layer_violations(&self) -> Vec<LayerViolation> {
        // TODO: 实现完整的分层违规检测
        Vec::new()
    }

    /// 获取 BDD 覆盖率（占位）
    pub fn bdd_coverage(&self) -> f64 {
        // TODO: 从实际数据源获取
        0.0
    }

    /// 获取契约测试覆盖率（占位）
    pub fn contract_coverage(&self) -> f64 {
        // TODO: 从实际数据源获取
        0.0
    }
}

/// 分层违规
#[derive(Debug, Clone)]
pub struct LayerViolation {
    pub from: String,
    pub to: String,
    pub from_layer: String,
    pub to_layer: String,
    pub location: String,
}

/// 解析 TSV 文件
fn parse_tsv(content: &str) -> Vec<HashMap<String, String>> {
    let lines: Vec<&str> = content.lines().filter(|l| !l.trim().is_empty()).collect();
    if lines.is_empty() {
        return Vec::new();
    }

    let headers: Vec<String> = lines[0].split('\t').map(|s| s.to_string()).collect();
    let mut out = Vec::new();

    for line in lines.iter().skip(1) {
        let cols: Vec<&str> = line.split('\t').collect();
        let mut row = HashMap::new();
        for (i, h) in headers.iter().enumerate() {
            row.insert(h.clone(), cols.get(i).unwrap_or(&"").to_string());
        }
        out.push(row);
    }

    out
}

/// 解析 i64
fn parse_i64(v: Option<&String>) -> i64 {
    v.and_then(|s| s.parse().ok()).unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::TempDir;

    #[test]
    fn test_parse_tsv() {
        let tsv = "name\tvalue\na\t1\nb\t2";
        let rows = parse_tsv(tsv);
        assert_eq!(rows.len(), 2);
        assert_eq!(rows[0].get("name"), Some(&"a".to_string()));
        assert_eq!(rows[0].get("value"), Some(&"1".to_string()));
    }

    #[test]
    fn test_from_input_dir() {
        let temp = TempDir::new().unwrap();

        // 创建 summary.json
        let summary = r#"{
            "generated_at": "2026-02-16T15:30:00Z",
            "scenarios": {
                "v3_target": {
                    "unexpected_edges_count": 10,
                    "forbidden_edges_count": 5,
                    "gate_pass": false
                }
            }
        }"#;
        std::fs::write(temp.path().join("summary.json"), summary).unwrap();

        // 创建 scenario-validation.tsv
        let scenario = "name\tallow_count\tforbid_count\tmatched_edges_count\tunexpected_edges_count\tforbidden_edges_count\tmissing_edges_count\nv3_target\t27\t6\t27\t10\t5\t0";
        std::fs::write(temp.path().join("scenario-validation.tsv"), scenario).unwrap();

        // 创建 gate-evaluation.tsv
        let gate = "profile\tmetric\tactual\tlimit\tpass\ntarget\tdirected_density_pct\t75\t40\tfail\ntarget\tbidirectional_pair_count\t20\t5\tfail";
        std::fs::write(temp.path().join("gate-evaluation.tsv"), gate).unwrap();

        let ctx = ScoringContext::from_input_dir(temp.path().to_str().unwrap()).unwrap();

        assert_eq!(ctx.scenario.unexpected_edges_count, 10);
        assert_eq!(ctx.scenario.forbidden_edges_count, 5);
        assert_eq!(ctx.structure.directed_density_pct, 75.0);
        assert_eq!(ctx.structure.bidirectional_pair_count, 20);
        assert_eq!(ctx.gate_checks.len(), 2);
    }
}
