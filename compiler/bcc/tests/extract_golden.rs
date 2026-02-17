use bcc::extract::{adapter, testing};
use serde::de::DeserializeOwned;
use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Debug, Deserialize)]
struct CaseConfig {
    cases: Vec<GoldenCase>,
}

#[derive(Debug, Clone, Deserialize)]
struct GoldenCase {
    name: String,
    lang: String,
    fixture: String,
    golden: String,
    #[serde(default)]
    tags: Vec<String>,
    #[serde(default)]
    cross_lang_group: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
struct CrossLangRules {
    #[serde(default = "default_compare_fields")]
    compare_fields: Vec<String>,
    #[serde(default)]
    call_root_separators: Vec<String>,
    #[serde(default)]
    import_kind_map: HashMap<String, String>,
    #[serde(default)]
    export_kind_map: HashMap<String, String>,
    #[serde(default)]
    whitelist: HashMap<String, Vec<String>>,
}

#[derive(Debug, Clone)]
struct ExecutedCase {
    case: GoldenCase,
    fixture_path: String,
    core: Value,
}

fn default_compare_fields() -> Vec<String> {
    vec![
        "calls".to_string(),
        "imports".to_string(),
        "exports".to_string(),
        "declarations".to_string(),
    ]
}

#[test]
fn extract_golden() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let update_golden = std::env::var("UPDATE_GOLDEN").ok().as_deref() == Some("1");

    let cases_file = manifest_dir.join("tests/extract_golden/cases.yaml");
    let rules_file = manifest_dir.join("tests/extract_golden/cross_lang_rules.yaml");

    let case_config: CaseConfig = load_yaml(&cases_file);
    assert!(
        !case_config.cases.is_empty(),
        "cases.yaml should contain at least one case"
    );
    let rules: CrossLangRules = load_yaml(&rules_file);

    let mut executed_cases = Vec::with_capacity(case_config.cases.len());
    for case in &case_config.cases {
        let executed = run_case(case, &manifest_dir, update_golden);
        executed_cases.push(executed);
    }

    assert_cross_lang_consistency(&executed_cases, &rules);
}

fn run_case(case: &GoldenCase, manifest_dir: &Path, update_golden: bool) -> ExecutedCase {
    let fixture_path = manifest_dir.join(&case.fixture);
    let golden_path = manifest_dir.join(&case.golden);

    assert!(
        fixture_path.exists(),
        "fixture does not exist, case={}, fixture={}",
        case.name,
        fixture_path.display()
    );

    let source = fs::read_to_string(&fixture_path).unwrap_or_else(|err| {
        panic!(
            "cannot read fixture, case={}, fixture={}, error={}",
            case.name,
            fixture_path.display(),
            err
        )
    });

    let lang_adapter = adapter::get_adapter(&case.lang).unwrap_or_else(|| {
        panic!(
            "unsupported language in case config, case={}, language={}",
            case.name, case.lang
        )
    });

    // 用相对路径写入 record.file_path，避免不同机器上的绝对路径噪音。
    let fixture_for_record = testing::normalize_test_path(fixture_path.to_string_lossy().as_ref());
    let record = lang_adapter.extract(&source, &fixture_for_record);
    let actual_core = testing::normalize_core_fields(&record);

    if update_golden {
        if let Some(parent) = golden_path.parent() {
            fs::create_dir_all(parent).unwrap_or_else(|err| {
                panic!(
                    "cannot create golden dir, case={}, dir={}, error={}",
                    case.name,
                    parent.display(),
                    err
                )
            });
        }
        fs::write(&golden_path, testing::stable_pretty_json(&actual_core)).unwrap_or_else(|err| {
            panic!(
                "cannot write golden, case={}, golden={}, error={}",
                case.name,
                golden_path.display(),
                err
            )
        });
    } else {
        let expected_raw = fs::read_to_string(&golden_path).unwrap_or_else(|err| {
            panic!(
                "cannot read golden, case={}, golden={}, error={}. If this is a new case, run with UPDATE_GOLDEN=1",
                case.name,
                golden_path.display(),
                err
            )
        });
        let expected_core: Value = serde_json::from_str(&expected_raw).unwrap_or_else(|err| {
            panic!(
                "invalid golden JSON, case={}, golden={}, error={}",
                case.name,
                golden_path.display(),
                err
            )
        });

        if expected_core != actual_core {
            let diff = testing::diff_core_fields(&expected_core, &actual_core, 32);
            panic!(
                "golden mismatch\ncase={}\nlanguage={}\ntags={}\nfixture={}\ngolden={}\n{}",
                case.name,
                case.lang,
                case.tags.join(","),
                fixture_path.display(),
                golden_path.display(),
                diff
            );
        }
    }

    ExecutedCase {
        case: case.clone(),
        fixture_path: testing::normalize_test_path(fixture_path.to_string_lossy().as_ref()),
        core: actual_core,
    }
}

fn assert_cross_lang_consistency(executed_cases: &[ExecutedCase], rules: &CrossLangRules) {
    let mut groups: BTreeMap<String, Vec<&ExecutedCase>> = BTreeMap::new();
    for case in executed_cases {
        let Some(group) = case
            .case
            .cross_lang_group
            .as_ref()
            .map(|item| item.trim())
            .filter(|item| !item.is_empty())
        else {
            continue;
        };
        groups.entry(group.to_string()).or_default().push(case);
    }

    for (group_name, members) in groups {
        if members.len() <= 1 {
            continue;
        }

        let projected: Vec<(&ExecutedCase, HashMap<String, Value>)> = members
            .iter()
            .map(|case| (*case, cross_lang_fields(&case.core, rules)))
            .collect();

        for field in &rules.compare_fields {
            for i in 0..projected.len() {
                let (left_case, left_fields) = &projected[i];
                if is_field_whitelisted(field, &left_case.case, rules) {
                    continue;
                }

                for j in (i + 1)..projected.len() {
                    let (right_case, right_fields) = &projected[j];
                    if is_field_whitelisted(field, &right_case.case, rules) {
                        continue;
                    }

                    let left = left_fields.get(field).cloned().unwrap_or(Value::Null);
                    let right = right_fields.get(field).cloned().unwrap_or(Value::Null);
                    if left == right {
                        continue;
                    }

                    panic!(
                        "cross-lang mismatch\ngroup={}\nfield={}\nleft_case={}\nleft_fixture={}\nright_case={}\nright_fixture={}\nleft={}\nright={}",
                        group_name,
                        field,
                        left_case.case.name,
                        left_case.fixture_path,
                        right_case.case.name,
                        right_case.fixture_path,
                        serde_json::to_string_pretty(&left).unwrap_or_else(|_| left.to_string()),
                        serde_json::to_string_pretty(&right).unwrap_or_else(|_| right.to_string()),
                    );
                }
            }
        }
    }
}

fn cross_lang_fields(core: &Value, rules: &CrossLangRules) -> HashMap<String, Value> {
    let mut fields = HashMap::new();

    let raw_calls: Vec<String> = core
        .get("calls")
        .and_then(Value::as_array)
        .map(|items| {
            items
                .iter()
                .filter_map(|item| item.get("callee").and_then(Value::as_str))
                .map(normalize_spaces)
                .collect()
        })
        .unwrap_or_default();
    let mut qualified_method_suffixes = BTreeSet::new();
    for call in &raw_calls {
        if let Some(suffix) = extract_call_suffix(call, rules) {
            qualified_method_suffixes.insert(suffix);
        }
    }
    let calls: BTreeSet<String> = raw_calls
        .into_iter()
        .filter(|call| {
            if has_call_separator(call, rules) {
                return true;
            }
            !qualified_method_suffixes.contains(call)
        })
        .map(|call| canonicalize_call(&call, rules))
        .collect();
    fields.insert(
        "calls".to_string(),
        json!(calls.into_iter().collect::<Vec<_>>()),
    );

    let imports: BTreeSet<String> = core
        .get("imports")
        .and_then(Value::as_array)
        .map(|items| {
            items
                .iter()
                .filter_map(|item| {
                    let kind = item.get("kind").and_then(Value::as_str)?;
                    let specifier = item.get("specifier").and_then(Value::as_str)?;
                    let mapped_kind = rules
                        .import_kind_map
                        .get(kind)
                        .cloned()
                        .unwrap_or_else(|| kind.to_string());
                    Some(format!("{}:{}", mapped_kind, normalize_spaces(specifier)))
                })
                .collect()
        })
        .unwrap_or_default();
    fields.insert(
        "imports".to_string(),
        json!(imports.into_iter().collect::<Vec<_>>()),
    );

    let exports: BTreeSet<String> = core
        .get("exports")
        .and_then(Value::as_array)
        .map(|items| {
            items
                .iter()
                .filter_map(|item| {
                    let name = item.get("name").and_then(Value::as_str)?;
                    let kind = item.get("kind").and_then(Value::as_str)?;
                    let mapped_kind = rules
                        .export_kind_map
                        .get(kind)
                        .cloned()
                        .unwrap_or_else(|| kind.to_string());
                    Some(format!("{}:{}", mapped_kind, normalize_spaces(name)))
                })
                .collect()
        })
        .unwrap_or_default();
    fields.insert(
        "exports".to_string(),
        json!(exports.into_iter().collect::<Vec<_>>()),
    );

    let declarations = core.get("declarations").cloned().unwrap_or(Value::Null);
    fields.insert("declarations".to_string(), declarations);

    fields
}

fn canonicalize_call(callee: &str, rules: &CrossLangRules) -> String {
    let mut value = normalize_spaces(callee);
    for separator in &rules.call_root_separators {
        if let Some((prefix, _)) = value.split_once(separator) {
            value = prefix.to_string();
            break;
        }
    }
    value
}

fn extract_call_suffix(callee: &str, rules: &CrossLangRules) -> Option<String> {
    for separator in &rules.call_root_separators {
        if let Some((_, suffix)) = callee.split_once(separator) {
            return Some(suffix.to_string());
        }
    }
    None
}

fn has_call_separator(callee: &str, rules: &CrossLangRules) -> bool {
    rules
        .call_root_separators
        .iter()
        .any(|separator| callee.contains(separator))
}

fn is_field_whitelisted(field: &str, case: &GoldenCase, rules: &CrossLangRules) -> bool {
    let Some(items) = rules.whitelist.get(field) else {
        return false;
    };
    items
        .iter()
        .any(|item| item == &case.lang || item == &case.name)
}

fn normalize_spaces(input: &str) -> String {
    input.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn load_yaml<T>(path: &Path) -> T
where
    T: DeserializeOwned,
{
    let content = fs::read_to_string(path)
        .unwrap_or_else(|err| panic!("cannot read YAML, path={}, error={}", path.display(), err));
    serde_yaml::from_str(&content)
        .unwrap_or_else(|err| panic!("invalid YAML, path={}, error={}", path.display(), err))
}
