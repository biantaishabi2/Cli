use super::{elixir, php, rust, typescript, FileRecord};

/// 语言适配器：封装不同语言提取器的统一调用方式。
pub trait LanguageAdapter {
    fn language(&self) -> &'static str;
    fn extract(&self, content: &str, path: &str) -> FileRecord;
}

pub struct ElixirAdapter;
pub struct TypeScriptAdapter;
pub struct TsxAdapter;
pub struct PhpAdapter;
pub struct RustAdapter;

impl LanguageAdapter for ElixirAdapter {
    fn language(&self) -> &'static str {
        "elixir"
    }

    fn extract(&self, content: &str, path: &str) -> FileRecord {
        elixir::extract(content, path)
    }
}

impl LanguageAdapter for TypeScriptAdapter {
    fn language(&self) -> &'static str {
        "typescript"
    }

    fn extract(&self, content: &str, path: &str) -> FileRecord {
        typescript::extract(content, path, "typescript")
    }
}

impl LanguageAdapter for TsxAdapter {
    fn language(&self) -> &'static str {
        "tsx"
    }

    fn extract(&self, content: &str, path: &str) -> FileRecord {
        typescript::extract(content, path, "tsx")
    }
}

impl LanguageAdapter for PhpAdapter {
    fn language(&self) -> &'static str {
        "php"
    }

    fn extract(&self, content: &str, path: &str) -> FileRecord {
        php::extract(content, path)
    }
}

impl LanguageAdapter for RustAdapter {
    fn language(&self) -> &'static str {
        "rust"
    }

    fn extract(&self, content: &str, path: &str) -> FileRecord {
        rust::extract(content, path)
    }
}

static ELIXIR_ADAPTER: ElixirAdapter = ElixirAdapter;
static TS_ADAPTER: TypeScriptAdapter = TypeScriptAdapter;
static TSX_ADAPTER: TsxAdapter = TsxAdapter;
static PHP_ADAPTER: PhpAdapter = PhpAdapter;
static RUST_ADAPTER: RustAdapter = RustAdapter;

/// 根据语言名获取适配器。
pub fn get_adapter(lang: &str) -> Option<&'static dyn LanguageAdapter> {
    match lang {
        "elixir" => Some(&ELIXIR_ADAPTER),
        "typescript" => Some(&TS_ADAPTER),
        "tsx" => Some(&TSX_ADAPTER),
        "php" => Some(&PHP_ADAPTER),
        "rust" => Some(&RUST_ADAPTER),
        _ => None,
    }
}

/// 根据文件后缀检测语言。
pub fn detect_language(path: &str) -> String {
    if path.ends_with(".ex") || path.ends_with(".exs") {
        "elixir".into()
    } else if path.ends_with(".tsx") {
        "tsx".into()
    } else if path.ends_with(".ts") {
        "typescript".into()
    } else if path.ends_with(".go") {
        "go".into()
    } else if path.ends_with(".rs") {
        "rust".into()
    } else if path.ends_with(".php") {
        "php".into()
    } else {
        "unknown".into()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detect_language_by_extension() {
        assert_eq!(detect_language("a.ex"), "elixir");
        assert_eq!(detect_language("a.tsx"), "tsx");
        assert_eq!(detect_language("a.ts"), "typescript");
        assert_eq!(detect_language("a.rs"), "rust");
        assert_eq!(detect_language("a.php"), "php");
        assert_eq!(detect_language("a.unknown"), "unknown");
    }

    #[test]
    fn adapter_registry_contains_supported_languages() {
        assert_eq!(get_adapter("elixir").map(|a| a.language()), Some("elixir"));
        assert_eq!(
            get_adapter("typescript").map(|a| a.language()),
            Some("typescript")
        );
        assert_eq!(get_adapter("tsx").map(|a| a.language()), Some("tsx"));
        assert_eq!(get_adapter("rust").map(|a| a.language()), Some("rust"));
        assert_eq!(get_adapter("php").map(|a| a.language()), Some("php"));
        assert!(get_adapter("go").is_none());
    }
}
