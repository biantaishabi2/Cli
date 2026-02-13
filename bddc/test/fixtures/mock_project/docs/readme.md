## Fixture Docs

下面是一个嵌入在 Markdown fenced code block 的 BDD DSL，用于验证：
- `compile_dir!` 会从 `docs/**/*.md` 提取 fenced block 并生成测试

```bdd
[SCENARIO: FX-MD-001] TITLE: fixture md flow TAGS: integration
GIVEN given_seed id="seed-md-1"
WHEN when_do id=$id
THEN assert_done id=$id
```

