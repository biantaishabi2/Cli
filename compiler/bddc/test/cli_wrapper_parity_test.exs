defmodule BDDCompiler.CLIWrapperParityTest do
  use ExUnit.Case, async: false

  @fixture_project Path.expand("fixtures/mock_project", __DIR__)
  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  @wrapper_cases [
    {"annotations.check", "bdd.annotations.check", ["--include-test"]},
    {"registry.scaffold", "bdd.registry.scaffold", ["--module", "Fixture.Foo", "--functions", "bar/1"]},
    {"registry.upsert", "bdd.registry.upsert", ["--module", "Fixture.Foo", "--functions", "bar/1"]},
    {"instructions.docs", "bdd.instructions_docs", ["--version", "v1"]},
    {"contract.check", "bdd.contract.check", ["--in", "docs/bdd/contracts"]},
    {"fuzz", "bdd.fuzz", ["--seed", "42"]},
    {"mutation.report", "bdd.mutation.report", ["--in", "lib/shop/bdd"]},
    {"mutation.run", "bdd.mutation.run", ["--max-mutants", "10"]}
  ]

  @factory_cases [
    {"factories.scaffold", "bdd.factories.scaffold", ["--scope", "priv/bdd/factories_scope.exs"], "generated=2"},
    {"factories.upsert", "bdd.factories.upsert", ["--scope", "priv/bdd/factories_scope.exs"], "upserted=2"},
    {"factories.check", "bdd.factories.check", ["--paths", "tmp/factories_generated"], "ok paths="}
  ]

  test "wrapper 子命令与 mix 任务行为一致（参数透传/退出码/任务标识）" do
    Enum.each(@wrapper_cases, fn {wrapper_cmd, mix_task, passthrough} ->
      {wrapper_out, wrapper_status} =
        System.cmd(
          @escript,
          [wrapper_cmd, "--project-root", @fixture_project | passthrough],
          cd: Path.expand("..", __DIR__),
          stderr_to_stdout: true
        )

      {mix_out, mix_status} =
        System.cmd("mix", [mix_task | passthrough], cd: @fixture_project, stderr_to_stdout: true)

      assert wrapper_status == mix_status
      assert probe_signature(wrapper_out) == probe_signature(mix_out)
      assert probe_signature(wrapper_out) == {"TASK=#{mix_task}", "ARGS=#{Enum.join(passthrough, "|")}"}
    end)
  end

  test "factories 子命令与 mix 任务行为一致（退出码/关键输出/产物）" do
    File.rm_rf!(Path.join(@fixture_project, "tmp/factories_generated"))

    Enum.each(@factory_cases, fn {wrapper_cmd, mix_task, passthrough, marker} ->
      {wrapper_out, wrapper_status} =
        System.cmd(
          @escript,
          [wrapper_cmd, "--project-root", @fixture_project | passthrough],
          cd: Path.expand("..", __DIR__),
          stderr_to_stdout: true
        )

      {mix_out, mix_status} =
        System.cmd("mix", [mix_task | passthrough], cd: @fixture_project, stderr_to_stdout: true)

      assert wrapper_status == mix_status
      assert wrapper_out =~ marker
      assert mix_out =~ marker
    end)

    assert File.exists?(Path.join(@fixture_project, "tmp/factories_generated/order_factory.ex"))
    assert File.exists?(Path.join(@fixture_project, "tmp/factories_generated/item_factory.ex"))
  end

  defp probe_signature(output) when is_binary(output) do
    task = extract_line(output, ~r/^TASK=.*$/m)
    args = extract_line(output, ~r/^ARGS=.*$/m)
    {task, args}
  end

  defp extract_line(output, regex) do
    case Regex.run(regex, output) do
      [line | _] -> line
      _ -> nil
    end
  end
end
