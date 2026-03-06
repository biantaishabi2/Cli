defmodule BDDCompiler.CLIGoIntegrationTest do
  use ExUnit.Case, async: false

  alias BDDCompiler.CLI

  # 端到端 Go 编译集成测试

  defp tmp_dir!(label) do
    dir =
      Path.join(
        System.tmp_dir!(),
        "bdd_go_integ_#{label}_#{System.unique_integer([:positive])}"
      )

    File.rm_rf!(dir)
    File.mkdir_p!(dir)
    dir
  end

  # 创建临时 DSL 文件和 instructions JSON
  defp setup_project!(base) do
    dsl_dir = Path.join(base, "docs/bdd")
    File.mkdir_p!(dsl_dir)

    # 简单的 DSL 文件
    File.write!(Path.join(dsl_dir, "hello.dsl"), """
    [SCENARIO: GO-INT-001] TITLE: go integration test
    GIVEN setup_env name="test"
    WHEN run_action id="1"
    THEN verify_result status="ok"
    """)

    # 最小化的 instructions JSON manifest
    json_path = Path.join(base, "instructions.json")

    File.write!(json_path, """
    {
      "total": 3,
      "entries": [
        {
          "name": "setup_env",
          "domain": "Test",
          "entity": "Env",
          "source_kind": "action",
          "source_name": "setup",
          "kind": "given",
          "dedup_index": 0,
          "args": {"name": {"type": "string", "required": true}},
          "outputs": {},
          "scopes": ["integration"],
          "boundary": "service"
        },
        {
          "name": "run_action",
          "domain": "Test",
          "entity": "Action",
          "source_kind": "action",
          "source_name": "run",
          "kind": "when",
          "dedup_index": 0,
          "args": {"id": {"type": "string", "required": true}},
          "outputs": {},
          "scopes": ["integration"],
          "boundary": "service"
        },
        {
          "name": "verify_result",
          "domain": "Test",
          "entity": "Result",
          "source_kind": "read",
          "source_name": "verify",
          "kind": "then",
          "dedup_index": 0,
          "args": {"status": {"type": "string", "required": true}},
          "outputs": {},
          "scopes": ["integration"],
          "boundary": "service"
        }
      ]
    }
    """)

    {dsl_dir, json_path}
  end

  describe "--target go 端到端编译" do
    test "生成 Go 测试文件，包含正确的 package 和 import" do
      base = tmp_dir!("go_e2e")
      {dsl_dir, json_path} = setup_project!(base)
      out_dir = Path.join(base, "out")

      # 通过 CLI.main 调用完整编译流程
      CLI.main([
        "compile",
        "--target", "go",
        "--go-module", "github.com/test/app",
        "--instructions-json", json_path,
        "--in", dsl_dir,
        "--out", out_dir
      ])

      # 验证输出文件是 _generated_test.go（而非 .exs）
      go_files = Path.join(out_dir, "*_generated_test.go") |> Path.wildcard()
      assert length(go_files) == 1, "应生成恰好 1 个 Go 测试文件，实际: #{inspect(go_files)}"

      content = File.read!(List.first(go_files))

      # 验证 Go package 声明
      assert content =~ "package bdd_test"

      # 验证 import 包含指定的 go_module 路径
      assert content =~ "github.com/test/app/bdd/runtime"

      # 验证不存在 .exs 文件
      exs_files = Path.join(out_dir, "*_generated_test.exs") |> Path.wildcard()
      assert exs_files == [], "Go target 不应生成 .exs 文件"
    end

    test "默认 target（无 --target 标志）仍输出 .exs" do
      base = tmp_dir!("default_target")
      {dsl_dir, json_path} = setup_project!(base)
      out_dir = Path.join(base, "out")

      # 不指定 --target，应默认 elixir
      CLI.main([
        "compile",
        "--instructions-json", json_path,
        "--in", dsl_dir,
        "--out", out_dir
      ])

      exs_files = Path.join(out_dir, "*_generated_test.exs") |> Path.wildcard()
      assert length(exs_files) == 1, "默认应生成 .exs 文件"

      go_files = Path.join(out_dir, "*_generated_test.go") |> Path.wildcard()
      assert go_files == [], "默认不应生成 Go 文件"
    end
  end

  describe ".bddc.toml 配置支持" do
    test "从 .bddc.toml 读取 target 和 go_module 作为默认值" do
      base = tmp_dir!("toml_defaults")
      {dsl_dir, json_path} = setup_project!(base)
      out_dir = Path.join(base, "out")

      # 写入 .bddc.toml 到项目根目录
      File.write!(Path.join(base, ".bddc.toml"), """
      [global]
      target = "go"
      go_module = "github.com/toml/example"
      """)

      # 不在 CLI 指定 --target 和 --go-module，依赖 .bddc.toml
      CLI.main([
        "compile",
        "--project-root", base,
        "--instructions-json", json_path,
        "--in", dsl_dir,
        "--out", out_dir
      ])

      go_files = Path.join(out_dir, "*_generated_test.go") |> Path.wildcard()
      assert length(go_files) == 1, ".bddc.toml 中 target=go 应生成 Go 文件"

      content = File.read!(List.first(go_files))
      assert content =~ "github.com/toml/example/bdd/runtime"
    end

    test "CLI 标志覆盖 .bddc.toml 中的配置" do
      base = tmp_dir!("toml_override")
      {dsl_dir, json_path} = setup_project!(base)
      out_dir = Path.join(base, "out")

      # .bddc.toml 指定 go target
      File.write!(Path.join(base, ".bddc.toml"), """
      [global]
      target = "go"
      go_module = "github.com/toml/default"
      """)

      # CLI 用 --go-module 覆盖
      CLI.main([
        "compile",
        "--project-root", base,
        "--target", "go",
        "--go-module", "github.com/cli/override",
        "--instructions-json", json_path,
        "--in", dsl_dir,
        "--out", out_dir
      ])

      go_files = Path.join(out_dir, "*_generated_test.go") |> Path.wildcard()
      assert length(go_files) == 1

      content = File.read!(List.first(go_files))
      # CLI 标志应覆盖 toml 值
      assert content =~ "github.com/cli/override/bdd/runtime"
      refute content =~ "github.com/toml/default"
    end
  end
end
