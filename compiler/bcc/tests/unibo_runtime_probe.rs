use serde::Deserialize;
use serde_json::Value;
use std::path::Path;
use std::process::Command;

#[derive(Debug, Clone)]
pub struct RuntimeProbe {
    command: String,
}

#[derive(Debug)]
pub enum RuntimeProbeAvailability {
    Available(RuntimeProbe),
    Missing { message: String, strict: bool },
}

#[derive(Debug, Deserialize)]
struct RuntimeProbeResult {
    ok: bool,
    #[serde(default)]
    data: Option<Value>,
    #[serde(default)]
    error: Option<String>,
}

impl RuntimeProbe {
    pub fn resolve() -> RuntimeProbeAvailability {
        let strict = matches!(
            super::gate_common::GateConfig::from_env().mode,
            super::gate_common::GateMode::Strict
        );
        match std::env::var("UNIBO_RUNTIME_PROBE_CMD") {
            Ok(raw) if !raw.trim().is_empty() => {
                RuntimeProbeAvailability::Available(Self { command: raw })
            }
            _ => RuntimeProbeAvailability::Missing {
                strict,
                message:
                    "UNIBO_RUNTIME_PROBE_CMD 未配置，无法执行真实 UniBO runtime query/mutation 校验"
                        .to_string(),
            },
        }
    }

    pub fn execute_action(
        &self,
        contract_path: &Path,
        operation: &str,
        action_key: &str,
        input: &Value,
    ) -> Result<Value, String> {
        let input_raw = serde_json::to_string(input)
            .map_err(|err| format!("serialize runtime input failed: {}", err))?;
        let output = Command::new("bash")
            .arg("-lc")
            .arg(&self.command)
            .env("UNIBO_RUNTIME_CONTRACT_PATH", contract_path.as_os_str())
            .env("UNIBO_RUNTIME_OPERATION", operation)
            .env("UNIBO_RUNTIME_ACTION", action_key)
            .env("UNIBO_RUNTIME_INPUT_JSON", input_raw)
            .output()
            .map_err(|err| format!("spawn runtime probe failed: {}", err))?;

        if !output.status.success() {
            return Err(format!(
                "runtime probe exit_code={:?} stderr={}",
                output.status.code(),
                String::from_utf8_lossy(&output.stderr).trim()
            ));
        }

        let stdout = String::from_utf8(output.stdout)
            .map_err(|err| format!("runtime probe stdout is not utf-8: {}", err))?;
        let parsed: RuntimeProbeResult = serde_json::from_str(stdout.trim()).map_err(|err| {
            format!(
                "parse runtime probe output failed: {} | raw={}",
                err, stdout
            )
        })?;
        if parsed.ok {
            parsed
                .data
                .ok_or_else(|| "runtime probe succeeded but data is empty".to_string())
        } else {
            Err(parsed
                .error
                .unwrap_or_else(|| "runtime probe failed without error message".to_string()))
        }
    }
}
