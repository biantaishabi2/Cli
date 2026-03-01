use serde::Serialize;
use std::collections::BTreeMap;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UniboContractProducer {
    pub name: String,
    pub version: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UniboActionContract {
    pub action_key: String,
    pub action: String,
    pub graphql_kind: String,
    pub input: BTreeMap<String, String>,
    pub output: BTreeMap<String, String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub errors: Vec<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UniboBoundaryContract {
    pub contract_key: String,
    pub module_id: String,
    pub name: String,
    pub source_kind: String,
    pub graphql_kind: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub fields: BTreeMap<String, String>,
    pub actions: Vec<UniboActionContract>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UniboApiContractDocument {
    pub bridge_version: String,
    pub target_runtime_version: String,
    pub compat_version: String,
    pub producer: UniboContractProducer,
    pub seed_version: String,
    pub generated_at: String,
    pub contracts: Vec<UniboBoundaryContract>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RuntimeBindingConfig {
    pub package: String,
    pub mode: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ContractSourceConfig {
    pub path: String,
    pub format: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UniboRuntimeBridgeConfig {
    pub bridge_version: String,
    pub target_runtime_version: String,
    pub compat_version: String,
    pub runtime: RuntimeBindingConfig,
    pub contract_source: ContractSourceConfig,
}
