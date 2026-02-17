use serde::{Deserialize, Serialize};
use std::cmp::Ordering;

/// 注入提示最小置信度，低于阈值统一降级为 direct_call。
pub const MIN_HINT_CONFIDENCE: f64 = 0.75;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CallType {
    DirectCall,
    FrameworkInjection,
    ExternalRegistration,
}

impl Default for CallType {
    fn default() -> Self {
        Self::DirectCall
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationHint {
    pub target: String,
    #[serde(default)]
    pub call_type_hint: CallType,
    #[serde(default)]
    pub via: String,
    #[serde(default)]
    pub confidence: f64,
    #[serde(default)]
    pub detector: String,
    #[serde(default)]
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationClassification {
    pub caller: String,
    pub callee: String,
    pub call_type: CallType,
    #[serde(default)]
    pub via: String,
    pub confidence: f64,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub detector: String,
    #[serde(default)]
    pub reason: String,
}

fn normalize_confidence(value: f64) -> f64 {
    if !value.is_finite() {
        return 0.0;
    }
    value.clamp(0.0, 1.0)
}

fn call_type_priority(call_type: CallType) -> i32 {
    match call_type {
        CallType::ExternalRegistration => 3,
        CallType::FrameworkInjection => 2,
        CallType::DirectCall => 1,
    }
}

fn call_type_label(call_type: CallType) -> &'static str {
    match call_type {
        CallType::DirectCall => "direct_call",
        CallType::FrameworkInjection => "framework_injection",
        CallType::ExternalRegistration => "external_registration",
    }
}

fn compare_hint(a: &RelationHint, b: &RelationHint) -> Ordering {
    call_type_priority(a.call_type_hint)
        .cmp(&call_type_priority(b.call_type_hint))
        .then_with(|| {
            normalize_confidence(a.confidence)
                .partial_cmp(&normalize_confidence(b.confidence))
                .unwrap_or(Ordering::Equal)
        })
        .then_with(|| a.via.len().cmp(&b.via.len()))
        .then_with(|| a.detector.len().cmp(&b.detector.len()))
}

fn direct_classification(
    caller: &str,
    callee: &str,
    source: &str,
    reason: &str,
) -> RelationClassification {
    RelationClassification {
        caller: caller.to_string(),
        callee: callee.to_string(),
        call_type: CallType::DirectCall,
        via: String::new(),
        confidence: 1.0,
        source: source.to_string(),
        detector: String::new(),
        reason: reason.to_string(),
    }
}

/// 基于候选 hint 选择边级别分类。
pub fn classify_edge(
    caller: &str,
    callee: &str,
    hints: &[RelationHint],
    detect_injection: bool,
) -> RelationClassification {
    if !detect_injection {
        return direct_classification(caller, callee, "fallback", "detect_injection disabled");
    }

    let Some(best) = hints.iter().max_by(|a, b| compare_hint(a, b)) else {
        return direct_classification(caller, callee, "fallback", "no relation hint");
    };

    let confidence = normalize_confidence(best.confidence);
    if best.call_type_hint != CallType::DirectCall && confidence < MIN_HINT_CONFIDENCE {
        return direct_classification(
            caller,
            callee,
            "fallback",
            &format!(
                "hint confidence {:.2} below threshold {:.2}",
                confidence, MIN_HINT_CONFIDENCE
            ),
        );
    }

    RelationClassification {
        caller: caller.to_string(),
        callee: callee.to_string(),
        call_type: best.call_type_hint,
        via: best.via.clone(),
        confidence,
        source: "ast_hint".to_string(),
        detector: best.detector.clone(),
        reason: best.reason.clone(),
    }
}

/// 给 matrix 中的 reason 增补分类证据，仅在非 direct_call 时追加。
pub fn append_classification_reason(
    base_reason: &str,
    classification: &RelationClassification,
    detect_injection: bool,
) -> String {
    if !detect_injection || classification.call_type == CallType::DirectCall {
        return base_reason.to_string();
    }

    let via = if classification.via.is_empty() {
        "unknown"
    } else {
        classification.via.as_str()
    };
    let detector = if classification.detector.is_empty() {
        "unknown"
    } else {
        classification.detector.as_str()
    };

    let evidence = format!(
        "call_type={}, via={}, confidence={:.2}, detector={}",
        call_type_label(classification.call_type),
        via,
        classification.confidence,
        detector
    );

    if base_reason.trim().is_empty() {
        evidence
    } else {
        format!("{}; {}", base_reason, evidence)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn conflict_uses_priority_then_confidence() {
        let hints = vec![
            RelationHint {
                target: "a".to_string(),
                call_type_hint: CallType::FrameworkInjection,
                via: "@Module.imports".to_string(),
                confidence: 0.98,
                detector: "ts.nest".to_string(),
                reason: "framework".to_string(),
            },
            RelationHint {
                target: "a".to_string(),
                call_type_hint: CallType::ExternalRegistration,
                via: "ReqLLM.Providers.register".to_string(),
                confidence: 0.80,
                detector: "elixir.register".to_string(),
                reason: "register".to_string(),
            },
        ];

        let row = classify_edge("INFRA", "PROVIDERS", &hints, true);
        assert_eq!(row.call_type, CallType::ExternalRegistration);
        assert_eq!(row.source, "ast_hint");
    }

    #[test]
    fn low_confidence_hint_downgrades_to_direct() {
        let hints = vec![RelationHint {
            target: "a".to_string(),
            call_type_hint: CallType::FrameworkInjection,
            via: "use Jido.Agent".to_string(),
            confidence: 0.30,
            detector: "elixir.use".to_string(),
            reason: "weak".to_string(),
        }];

        let row = classify_edge("A", "B", &hints, true);
        assert_eq!(row.call_type, CallType::DirectCall);
        assert_eq!(row.source, "fallback");
    }

    #[test]
    fn disabled_switch_is_fully_backward_compatible() {
        let hints = vec![RelationHint {
            target: "a".to_string(),
            call_type_hint: CallType::ExternalRegistration,
            via: "x".to_string(),
            confidence: 1.0,
            detector: "x".to_string(),
            reason: "x".to_string(),
        }];

        let row = classify_edge("A", "B", &hints, false);
        assert_eq!(row.call_type, CallType::DirectCall);
    }
}
