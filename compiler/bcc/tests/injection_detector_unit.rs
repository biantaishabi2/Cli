use bcc::arch::injection::{append_classification_reason, classify_edge, CallType, RelationHint};
use bcc::extract::{elixir, typescript};
use std::thread;

#[test]
fn classifier_prefers_external_registration_when_conflict() {
    let hints = vec![
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::FrameworkInjection,
            via: "use Jido.Agent".to_string(),
            confidence: 0.99,
            detector: "elixir.use_macro".to_string(),
            reason: "framework".to_string(),
        },
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::ExternalRegistration,
            via: "ReqLLM.Providers.register".to_string(),
            confidence: 0.80,
            detector: "elixir.external_register".to_string(),
            reason: "register".to_string(),
        },
    ];

    let row = classify_edge("INFRA", "PROVIDERS", &hints, true);
    assert_eq!(row.call_type, CallType::ExternalRegistration);
    assert_eq!(row.source, "ast_hint");
}

#[test]
fn classifier_uses_high_confidence_candidate_before_priority() {
    let hints = vec![
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::FrameworkInjection,
            via: "@Module.imports".to_string(),
            confidence: 0.95,
            detector: "typescript.nest.module".to_string(),
            reason: "framework".to_string(),
        },
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::ExternalRegistration,
            via: "ReqLLM.Providers.register".to_string(),
            confidence: 0.40,
            detector: "elixir.external_register".to_string(),
            reason: "weak register".to_string(),
        },
    ];

    let row = classify_edge("INFRA", "PROVIDERS", &hints, true);
    assert_eq!(row.call_type, CallType::FrameworkInjection);
    assert_eq!(row.source, "ast_hint");
}

#[test]
fn classifier_downgrades_to_direct_call_on_low_confidence() {
    let hints = vec![RelationHint {
        target: "src/provider.ex".to_string(),
        call_type_hint: CallType::FrameworkInjection,
        via: "@Module.imports".to_string(),
        confidence: 0.30,
        detector: "typescript.nest.module".to_string(),
        reason: "weak signal".to_string(),
    }];

    let row = classify_edge("API", "USER", &hints, true);
    assert_eq!(row.call_type, CallType::DirectCall);
    assert_eq!(row.source, "fallback");
}

#[test]
fn classifier_is_backward_compatible_when_switch_off() {
    let hints = vec![RelationHint {
        target: "src/provider.ex".to_string(),
        call_type_hint: CallType::ExternalRegistration,
        via: "ReqLLM.Providers.register".to_string(),
        confidence: 1.0,
        detector: "elixir.external_register".to_string(),
        reason: "register".to_string(),
    }];

    let row = classify_edge("INFRA", "PROVIDERS", &hints, false);
    assert_eq!(row.call_type, CallType::DirectCall);
}

#[test]
fn reason_includes_classification_evidence() {
    let row = classify_edge(
        "INFRA",
        "PROVIDERS",
        &[RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::ExternalRegistration,
            via: "ReqLLM.Providers.register".to_string(),
            confidence: 0.98,
            detector: "elixir.external_register".to_string(),
            reason: "register".to_string(),
        }],
        true,
    );

    let reason = append_classification_reason("derived from actual", &row, true);
    assert!(reason.contains("call_type=external_registration"));
    assert!(reason.contains("via=ReqLLM.Providers.register"));
}

#[test]
fn elixir_detector_extracts_use_and_register_hints() {
    let source = r#"
defmodule Gong.Agent do
  use Jido.AI.ReActAgent, tools: [Gong.Tools.Read]

  def init do
    ReqLLM.Providers.register(Gong.Provider.Anthropic)
  end
end
"#;

    let record = elixir::extract(source, "lib/gong/agent.ex");
    assert!(record
        .relation_hints
        .iter()
        .any(|h| h.target == "Gong.Tools.Read" && h.call_type_hint == CallType::FrameworkInjection));
    assert!(record.relation_hints.iter().any(|h| {
        h.target == "Gong.Provider.Anthropic"
            && h.call_type_hint == CallType::ExternalRegistration
            && h.via.contains("ReqLLM.Providers.register")
    }));
}

#[test]
fn elixir_detector_ignores_register_in_comments_and_strings() {
    let source = r#"
defmodule Gong.Agent do
  # ReqLLM.Providers.register(Gong.Provider.Anthropic)
  @doc "ReqLLM.Providers.register(Gong.Provider.Anthropic)"
  def init, do: :ok
end
"#;

    let record = elixir::extract(source, "lib/gong/agent.ex");
    assert!(record
        .relation_hints
        .iter()
        .all(|h| h.call_type_hint != CallType::ExternalRegistration));
}

#[test]
fn elixir_detector_downgrades_same_namespace_register_conflict() {
    let source = r#"
defmodule Gong.Agent do
  alias Gong.ReqLLM

  def init do
    ReqLLM.Providers.register(Gong.Provider.Anthropic)
  end
end
"#;

    let record = elixir::extract(source, "lib/gong/agent.ex");
    assert!(record
        .relation_hints
        .iter()
        .all(|h| h.call_type_hint != CallType::ExternalRegistration));
}

#[test]
fn typescript_detector_extracts_nest_hints() {
    let source = r#"
import { Module, Injectable } from '@nestjs/common';
import { UserModule } from '@app/user';
import { PrismaModule } from '@app/prisma';
import { UserService } from '@app/user';

@Module({
  imports: [UserModule, PrismaModule],
})
export class AppModule {}

@Injectable()
export class AppService {
  constructor(private readonly userService: UserService) {}
}
"#;

    let record = typescript::extract(source, "apps/api/src/app.module.ts", "typescript");
    assert!(record
        .relation_hints
        .iter()
        .any(|h| h.target == "@app/user" && h.via.contains("@Module.imports")));
    assert!(record
        .relation_hints
        .iter()
        .any(|h| h.target == "@app/prisma" && h.via.contains("@Module.imports")));
    assert!(record
        .relation_hints
        .iter()
        .any(|h| h.target == "@app/user" && h.via.contains("@Injectable constructor")));
}

#[test]
fn typescript_detector_downgrades_ambiguous_same_name_imports() {
    let source = r#"
import { Module } from '@nestjs/common';
import { UserModule } from './user.module';
import { UserModule as UserModule } from '../legacy/user.module';

@Module({
  imports: [UserModule],
})
export class AppModule {}
"#;

    let record = typescript::extract(source, "apps/api/src/app.module.ts", "typescript");
    assert!(!record
        .relation_hints
        .iter()
        .any(|h| h.via.contains("@Module.imports")));
}

#[test]
fn typescript_detector_prefers_unique_internal_source_on_name_conflict() {
    let source = r#"
import { Module } from '@nestjs/common';
import { UserModule } from '@vendor/user';
import { UserModule as UserModule } from '@app/user';

@Module({
  imports: [UserModule],
})
export class AppModule {}
"#;

    let record = typescript::extract(source, "apps/api/src/app.module.ts", "typescript");
    assert!(record
        .relation_hints
        .iter()
        .any(|h| h.via.contains("@Module.imports") && h.target == "@app/user"));
    assert!(!record
        .relation_hints
        .iter()
        .any(|h| h.via.contains("@Module.imports") && h.target == "@vendor/user"));
}

#[test]
fn typescript_detector_ignores_commented_or_stringified_decorators() {
    let source = r#"
import { Module, Injectable } from '@nestjs/common';
import { UserModule } from '@app/user';
import { UserService } from '@app/user';

// @Module({ imports: [UserModule] })
const fakeModule = "@Module({ imports: [UserModule] })";
const fakeInjectable = "@Injectable() constructor(userService: UserService) {}";

export class AppModule {}
"#;

    let record = typescript::extract(source, "apps/api/src/app.module.ts", "typescript");
    assert!(record.relation_hints.is_empty());
}

#[test]
fn classifier_is_thread_safe_under_parallel_load() {
    let hints = vec![
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::FrameworkInjection,
            via: "@Module.imports".to_string(),
            confidence: 0.95,
            detector: "typescript.nest.module".to_string(),
            reason: "framework".to_string(),
        },
        RelationHint {
            target: "src/provider.ex".to_string(),
            call_type_hint: CallType::ExternalRegistration,
            via: "ReqLLM.Providers.register".to_string(),
            confidence: 0.97,
            detector: "elixir.external_register".to_string(),
            reason: "register".to_string(),
        },
    ];

    let mut handles = Vec::new();
    for _ in 0..8 {
        let thread_hints = hints.clone();
        handles.push(thread::spawn(move || {
            for _ in 0..200 {
                let row = classify_edge("INFRA", "PROVIDERS", &thread_hints, true);
                assert_eq!(row.call_type, CallType::ExternalRegistration);
                assert_eq!(row.source, "ast_hint");
            }
        }));
    }

    for handle in handles {
        handle.join().expect("thread should not panic");
    }
}

#[test]
fn typescript_detector_handles_deep_module_imports_without_recursion_break() {
    let depth = 256usize;
    let mut source = String::from("import { Module } from '@nestjs/common';\n");
    for i in 0..depth {
        source.push_str(&format!("import {{ M{} }} from '@app/m{}';\n", i, i));
    }
    source.push_str("\n@Module({\n  imports: [");
    for i in 0..depth {
        if i > 0 {
            source.push_str(", ");
        }
        source.push_str(&format!("M{}", i));
    }
    source.push_str("],\n})\nexport class AppModule {}\n");

    let record = typescript::extract(&source, "apps/api/src/app.module.ts", "typescript");
    let module_import_hints = record
        .relation_hints
        .iter()
        .filter(|h| h.via.contains("@Module.imports"))
        .count();
    assert_eq!(module_import_hints, depth);
}
