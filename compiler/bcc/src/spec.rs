use serde::Deserialize;
use std::collections::HashMap;

/// 模块契约顶层结构（从 YAML 反序列化）
#[derive(Debug, Deserialize)]
pub struct ModuleSpec {
    pub module: ModuleInfo,
    #[serde(default)]
    pub ports: Vec<PortSpec>,
    #[serde(default)]
    pub relations: Vec<RelationSpec>,
}

#[derive(Debug, Deserialize)]
pub struct ModuleInfo {
    pub name: String,
    #[serde(default)]
    pub responsibility: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct PortSpec {
    pub name: String,
    pub kind: Option<String>,
    #[serde(default)]
    pub input: Option<HashMap<String, String>>,
    #[serde(default)]
    pub output: Option<HashMap<String, String>>,
    #[serde(default)]
    pub errors: Vec<String>,
}

#[derive(Debug, Deserialize)]
pub struct RelationSpec {
    pub callee: String,
    // TODO: 实现调用模式校验（sync/async/parallel）
    // See: https://github.com/biantaishabi2/Cli/issues/TODO
    #[serde(default)]
    pub mode: Option<String>,
}
