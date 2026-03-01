//! 架构验证模块

use crate::graph::error::Result;
use crate::graph::store::CodeGraphStore;
use crate::graph::types::*;
use std::collections::HashMap;
use std::path::Path;

/// 架构验证器
pub struct ArchValidator {
    target_matrix: TargetMatrix,
}

impl ArchValidator {
    /// 从 YAML 文件加载目标架构
    pub fn from_yaml<P: AsRef<Path>>(path: P) -> Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let target_matrix: TargetMatrix = serde_yaml::from_str(&content).map_err(|e| {
            crate::graph::error::GraphError::InvalidArgs(format!("YAML parse error: {}", e))
        })?;
        Ok(Self { target_matrix })
    }

    /// 验证仓库架构
    pub fn validate(&self, store: &dyn CodeGraphStore) -> Result<ArchValidationResult> {
        let mut violations = Vec::new();
        let mut checked_deps = 0;

        // 获取所有函数
        let all_functions = self.get_all_functions(store);
        let total_functions = all_functions.len();

        // 为每个函数确定其层
        let func_layers: HashMap<String, String> = all_functions
            .iter()
            .map(|f| (f.id.clone(), self.determine_layer(&f.file_path)))
            .collect();

        // 检查每个函数的调用关系
        for func in &all_functions {
            let source_layer = match func_layers.get(&func.id) {
                Some(layer) if !layer.is_empty() => layer.clone(),
                _ => continue, // 跳过未知层的函数
            };

            // 获取被调用者（直接调用）
            let callees = store.find_callees(&func.id, 1)?;
            for callee in callees {
                checked_deps += 1;

                let target_layer = match func_layers.get(&callee.id) {
                    Some(layer) if !layer.is_empty() => layer.clone(),
                    _ => continue,
                };

                // 检查是否违反架构规则
                if let Some(violation) =
                    self.check_violation(&func, &callee, &source_layer, &target_layer)
                {
                    violations.push(violation);
                }
            }
        }

        let violation_count = violations.len();
        Ok(ArchValidationResult {
            passed: violations.is_empty(),
            violations,
            stats: ValidationStats {
                total_functions,
                checked_deps,
                violation_count,
            },
        })
    }

    /// 根据文件路径确定层
    fn determine_layer(&self, file_path: &str) -> String {
        // 先检查目标矩阵中的模式
        for layer_def in &self.target_matrix.layers {
            for pattern in &layer_def.patterns {
                if self.matches_pattern(file_path, pattern) {
                    return layer_def.name.clone();
                }
            }
        }

        // 默认启发式规则
        let lower_path = file_path.to_lowercase();
        if lower_path.contains("controller") || lower_path.contains("api") {
            "api".to_string()
        } else if lower_path.contains("service") || lower_path.contains("biz") {
            "service".to_string()
        } else if lower_path.contains("dao")
            || lower_path.contains("repository")
            || lower_path.contains("model")
        {
            "dao".to_string()
        } else if lower_path.contains("util") || lower_path.contains("helper") {
            "util".to_string()
        } else {
            String::new() // 未知层
        }
    }

    /// 检查文件路径是否匹配模式
    fn matches_pattern(&self, file_path: &str, pattern: &str) -> bool {
        // 简单的通配符匹配
        let pattern = pattern.replace("*", ".*");
        let regex =
            regex::Regex::new(&pattern).unwrap_or_else(|_| regex::Regex::new(".*").unwrap());
        regex.is_match(file_path)
    }

    /// 检查是否违反架构规则
    fn check_violation(
        &self,
        source: &FunctionRecord,
        target: &FunctionRecord,
        source_layer: &str,
        target_layer: &str,
    ) -> Option<ArchViolation> {
        // 同层调用总是允许的
        if source_layer == target_layer {
            return None;
        }

        // 检查是否有明确的允许规则
        let is_allowed = self
            .target_matrix
            .allowed_deps
            .iter()
            .any(|rule| rule.from == source_layer && rule.to == target_layer);

        if is_allowed {
            return None;
        }

        // 检测跳过层（如 api -> dao）
        if source_layer == "api" && target_layer == "dao" {
            return Some(ArchViolation {
                violation_type: ViolationType::SkipLayer,
                source_func: source.clone(),
                target_func: target.clone(),
                source_layer: source_layer.to_string(),
                target_layer: target_layer.to_string(),
                message: format!(
                    "{} layer function '{}' directly calls {} layer function '{}' (skips service layer)",
                    source_layer, source.name, target_layer, target.name
                ),
            });
        }

        // 检测反向依赖（如 dao -> service）
        if self.is_reverse_dep(source_layer, target_layer) {
            return Some(ArchViolation {
                violation_type: ViolationType::ReverseDep,
                source_func: source.clone(),
                target_func: target.clone(),
                source_layer: source_layer.to_string(),
                target_layer: target_layer.to_string(),
                message: format!(
                    "{} layer function '{}' should not depend on {} layer function '{}'",
                    source_layer, source.name, target_layer, target.name
                ),
            });
        }

        None
    }

    /// 检查是否是反向依赖
    fn is_reverse_dep(&self, source: &str, target: &str) -> bool {
        let layer_order = ["api", "service", "dao", "util"];

        let source_idx = layer_order.iter().position(|&l| l == source);
        let target_idx = layer_order.iter().position(|&l| l == target);

        match (source_idx, target_idx) {
            (Some(s), Some(t)) => s > t && target != "util",
            _ => false,
        }
    }

    /// 获取所有函数
    fn get_all_functions(&self, store: &dyn CodeGraphStore) -> Vec<FunctionRecord> {
        store.list_functions()
    }

    /// 验证指定函数的架构合规性
    pub fn validate_function(
        &self,
        store: &dyn CodeGraphStore,
        function_id: &str,
    ) -> Result<ArchValidationResult> {
        let mut violations = Vec::new();

        let source_func = match store.get_function(function_id) {
            Some(f) => f,
            None => {
                return Ok(ArchValidationResult {
                    passed: false,
                    violations: vec![],
                    stats: ValidationStats {
                        total_functions: 0,
                        checked_deps: 0,
                        violation_count: 0,
                    },
                });
            }
        };

        let source_layer = self.determine_layer(&source_func.file_path);
        if source_layer.is_empty() {
            return Ok(ArchValidationResult {
                passed: false,
                violations: vec![],
                stats: ValidationStats {
                    total_functions: 1,
                    checked_deps: 0,
                    violation_count: 0,
                },
            });
        }

        // 检查所有调用
        let callees = store.find_callees(function_id, 1)?;
        let mut checked_deps = 0;

        for target_func in callees {
            checked_deps += 1;
            let target_layer = self.determine_layer(&target_func.file_path);

            if let Some(violation) =
                self.check_violation(&source_func, &target_func, &source_layer, &target_layer)
            {
                violations.push(violation);
            }
        }

        let violation_count = violations.len();

        Ok(ArchValidationResult {
            passed: violations.is_empty(),
            violations,
            stats: ValidationStats {
                total_functions: 1,
                checked_deps,
                violation_count,
            },
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_layer_detection() {
        let matrix = TargetMatrix {
            layers: vec![
                LayerDef {
                    name: "api".to_string(),
                    patterns: vec!["*/Controllers/*".to_string()],
                },
                LayerDef {
                    name: "service".to_string(),
                    patterns: vec!["*/Services/*".to_string()],
                },
                LayerDef {
                    name: "dao".to_string(),
                    patterns: vec!["*/Models/*".to_string()],
                },
            ],
            allowed_deps: vec![
                DepRule {
                    from: "api".to_string(),
                    to: "service".to_string(),
                },
                DepRule {
                    from: "service".to_string(),
                    to: "dao".to_string(),
                },
                DepRule {
                    from: "api".to_string(),
                    to: "dao".to_string(),
                }, // 允许直接访问（某些框架）
            ],
        };

        let validator = ArchValidator {
            target_matrix: matrix,
        };

        assert_eq!(
            validator.determine_layer("app/Controllers/UserController.php"),
            "api"
        );
        assert_eq!(
            validator.determine_layer("app/Services/UserService.php"),
            "service"
        );
        assert_eq!(validator.determine_layer("app/Models/User.php"), "dao");
    }
}
