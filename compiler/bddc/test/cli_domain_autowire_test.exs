defmodule BDDCompiler.CLIDomainAutowireTest do
  use ExUnit.Case, async: false

  @fixture_project Path.expand("fixtures/mock_project", __DIR__)
  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    ensure_factory_dirs!()
    :ok
  end

  test "domain.autowire 串联执行成功（fail-on-warn 显式关闭可覆盖 strict）" do
    out_dir = "/tmp/bdd_compiler_autowire_out"
    File.rm_rf!(out_dir)

    {out, status} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--strict",
          "true",
          "--fail-on-warn",
          "false"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "[autowire] step=annotations.check"
    assert out =~ "TASK=bdd.annotations.check"
    assert out =~ "[autowire] step=registry.scaffold"
    assert out =~ "TASK=bdd.registry.scaffold"
    assert out =~ "[autowire] step=registry.upsert"
    assert out =~ "TASK=bdd.registry.upsert"
    assert out =~ "[autowire] step=instructions.docs"
    assert out =~ "TASK=bdd.instructions_docs"
    assert out =~ "[autowire] step=check"
    assert out =~ "BDD lint:"
    assert out =~ "[autowire] step=factories.check"
    assert out =~ "ok paths=[\"test/support/bdd/factories_generated\"]"
    assert out =~ "[autowire] acceptance-report-template"
    assert File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
  end

  test "domain.autowire 默认 strict=true，遇 warning 会阻断" do
    {out, status} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          "/tmp/bdd_compiler_autowire_out_strict_default"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 2
    assert out =~ "BDD lint: 1 warnings"
  end

  test "domain.autowire dry-run 不执行任何子步骤" do
    out_dir = "/tmp/bdd_compiler_autowire_out_dry_run"
    File.rm_rf!(out_dir)

    {out, status} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--dry-run"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "[dry-run] command=domain.autowire"
    assert out =~ "[dry-run] step=annotations.check"
    assert out =~ "[dry-run] step=registry.scaffold"
    assert out =~ "[dry-run] step=check"
    assert out =~ "[dry-run] step=factories.check"
    refute out =~ "TASK=bdd.annotations.check"
    refute File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
  end

  test "domain.autowire 支持 sync-runtime-caps 并在 check 前生成离线 caps 文件" do
    out_dir = "/tmp/bdd_compiler_autowire_out_sync_caps"
    File.rm_rf!(out_dir)

    caps_file = Path.join(@fixture_project, "docs/bdd/_generated/runtime_caps_v1.exs")
    File.rm(caps_file)

    {out, status} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--runtime-caps-file",
          "docs/bdd/_generated/runtime_caps_v1.exs",
          "--sync-runtime-caps",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--strict",
          "true",
          "--fail-on-warn",
          "false"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "[autowire] step=runtime.caps.sync"
    assert File.exists?(caps_file)
  end

  test "domain.autowire 支持 strict-factories 与 skip-factories-check" do
    out_dir = "/tmp/bdd_compiler_autowire_out_factories_mode"
    File.rm_rf!(out_dir)

    {out_strict, status_strict} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--strict",
          "true",
          "--fail-on-warn",
          "false",
          "--strict-factories",
          "true"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_strict == 0
    assert out_strict =~ "ok paths=[\"test/support/bdd/factories_generated\", \"test/support/bdd/semantic_givens\"]"

    {out_skip, status_skip} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--module",
          "Fixture.Foo",
          "--functions",
          "bar/1",
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--strict",
          "true",
          "--fail-on-warn",
          "false",
          "--skip-factories-check"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_skip == 0
    refute out_skip =~ "[autowire] step=factories.check"
  end

  test "domain.autowire 未跳过 registry 时缺少 module/functions 会失败" do
    {out, status} =
      System.cmd(
        @escript,
        [
          "domain.autowire",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          "/tmp/bdd_compiler_autowire_out_missing"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert normalize_cli_output(out) =~ "参数缺失: --module"
  end

  defp ensure_factory_dirs! do
    generated_dir = Path.join(@fixture_project, "test/support/bdd/factories_generated")
    semantic_dir = Path.join(@fixture_project, "test/support/bdd/semantic_givens")

    File.mkdir_p!(generated_dir)
    File.mkdir_p!(semantic_dir)

    File.write!(Path.join(generated_dir, "order_factory.ex"), "defmodule Fixture.OrderFactory do\nend\n")
    File.write!(Path.join(semantic_dir, "order_semantic_given.ex"), "defmodule Fixture.OrderSemanticGiven do\nend\n")
  end

  defp normalize_cli_output(output) when is_binary(output) do
    Regex.replace(~r/\\x\{([0-9A-Fa-f]+)\}/, output, fn _, hex ->
      {codepoint, ""} = Integer.parse(hex, 16)
      <<codepoint::utf8>>
    end)
  end
end
