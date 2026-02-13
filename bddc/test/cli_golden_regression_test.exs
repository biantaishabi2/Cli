defmodule BDDCompiler.CLIGoldenRegressionTest do
  use ExUnit.Case, async: false

  @fixture_project Path.expand("fixtures/mock_project", __DIR__)
  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  test "固定 DSL 样本编译结果保持关键结构稳定（金标准回归）" do
    out_dir = "/tmp/bdd_compiler_golden_regression"
    File.rm_rf!(out_dir)

    {_out, 0} =
      System.cmd(
        @escript,
        [
          "compile",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--test-case",
          "ExUnit.Case",
          "--module-prefix",
          "Fixture.BDD.Generated",
          "--in",
          "docs/bdd",
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    generated = Path.join(out_dir, "simple_generated_test.exs")
    assert File.exists?(generated)

    content = File.read!(generated)
    assert content =~ "defmodule Fixture.BDD.Generated.SimpleTest do"
    assert content =~ "use ExUnit.Case, async: false"
    assert content =~ "test \"[FX-001] fixture simple flow\" do"
    assert content =~ "Fixture.BDD.Instructions.V1.run_step!(ctx, :given, :given_seed"
    assert content =~ "Fixture.BDD.Instructions.V1.run_step!(ctx, :when, :when_do"
    assert content =~ "Fixture.BDD.Instructions.V1.run_step!(ctx, :then, :assert_done"
    assert content =~ "Fixture.BDD.Instructions.V1.get!(ctx, :id"

    md_generated =
      out_dir
      |> Path.join("md_*_block_*_generated_test.exs")
      |> Path.wildcard()

    assert length(md_generated) == 1
    md_content = File.read!(List.first(md_generated))
    assert md_content =~ "FX-MD-001"
    assert md_content =~ "fixture md flow"
  end
end
