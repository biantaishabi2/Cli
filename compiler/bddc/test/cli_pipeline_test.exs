defmodule BDDCompiler.CLIPipelineTest do
  use ExUnit.Case, async: false

  @fixture_project Path.expand("fixtures/mock_project", __DIR__)
  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  defp tmp_dir!(name) do
    dir = Path.join(System.tmp_dir!(), "bdd_compiler_" <> name <> "_" <> Integer.to_string(System.unique_integer([:positive])))
    File.mkdir_p!(dir)
    dir
  end

  test "缺少指令来源时 compile 失败" do
    {out, status} =
      System.cmd(@escript, ["compile", "--in", "docs/bdd", "--out", "/tmp/bdd_compiler_missing_src"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert out =~ "必须提供指令来源"
  end

  test "可通过 --registry-module 自动装载指令并 compile 成功" do
    out_dir = "/tmp/bdd_compiler_fixture_out"

    {out, status} =
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
          "--in",
          "docs/bdd",
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "BDD 编译完成"
    assert File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
  end

  test "check 在 runtime 不实现 capabilities/0 时失败" do
    {out, status} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.NoCaps",
          "--in",
          "docs/bdd",
          "--out",
          "/tmp/bdd_compiler_fixture_out_nocaps",
          "--fail-on-warn",
          "false"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert out =~ "runtime module must implement capabilities/0"
  end

  test "lint 支持 fail-tags/fail-globs 精准阻断" do
    in_dir = tmp_dir!("lint_in")

    File.write!(
      Path.join(in_dir, "warn.dsl"),
      """
      [SCENARIO: FX-WARN-001] TITLE: lint warning TAGS: integration,strict_evidence
      GIVEN given_seed id="seed-1"
      WHEN when_do id=$id
      THEN unknown_assert id=$id
      """
    )

    # 1) 只 fail-on-warn：任何 warning 都阻断
    {out1, status1} =
      System.cmd(
        @escript,
        [
          "lint",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--in",
          in_dir,
          "--fail-on-warn",
          "true"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status1 == 2
    assert out1 =~ "BDD lint:"

    # 2) 指定 fail-tags=strict_interaction：warning tags 不命中则不阻断
    {_out2, status2} =
      System.cmd(
        @escript,
        [
          "lint",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--in",
          in_dir,
          "--fail-on-warn",
          "true",
          "--fail-tags",
          "strict_interaction"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status2 == 0

    # 3) 指定 fail-globs：命中 file 则阻断
    {_out3, status3} =
      System.cmd(
        @escript,
        [
          "lint",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--in",
          in_dir,
          "--fail-on-warn",
          "true",
          "--fail-globs",
          Path.join(in_dir, "**")
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status3 == 2
  end

  test "check 默认会运行生成测试（可用 --skip-bdd-test 跳过）" do
    out_dir = tmp_dir!("check_out")

    {out, status} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--skip-annotations-check",
          "--no-fail-on-warn"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
    assert out =~ "Running ExUnit"

    {_out2, status2} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir,
          "--skip-annotations-check",
          "--skip-bdd-test",
          "--no-fail-on-warn"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status2 == 0
  end

  test "compile/check 支持递归扫描 features 子目录下的 DSL" do
    in_dir = tmp_dir!("nested_bdd")
    features_dir = Path.join(in_dir, "features")
    File.mkdir_p!(features_dir)

    File.write!(
      Path.join(features_dir, "nested.dsl"),
      """
      [SCENARIO: FX-NESTED-001] TITLE: nested dsl TAGS: integration
      GIVEN given_seed id="seed-1"
      WHEN when_do id=$id
      THEN assert_done id=$id
      """
    )

    out_dir = tmp_dir!("nested_bdd_out")

    {out_compile, status_compile} =
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
          "--in",
          in_dir,
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_compile == 0
    assert out_compile =~ "BDD 编译完成"
    assert File.exists?(Path.join(out_dir, "nested_generated_test.exs"))

    {out_check, status_check} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          in_dir,
          "--out",
          out_dir,
          "--skip-annotations-check",
          "--skip-bdd-test",
          "--no-fail-on-warn"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_check == 0
    assert out_check =~ "compiled_out="
    assert out_check =~ "count=1"
  end

  test "check 在 organize 输出同时包含 features/scenarios 时优先消费 features" do
    in_dir = tmp_dir!("organized_bdd")
    features_dir = Path.join(in_dir, "features")
    scenarios_dir = Path.join(in_dir, "scenarios")
    File.mkdir_p!(features_dir)
    File.mkdir_p!(scenarios_dir)

    feature_dsl = """
    [SCENARIO: FX-ORG-001] TITLE: organized feature TAGS: integration
    GIVEN given_seed id="seed-1"
    WHEN when_do id=$id
    THEN assert_done id=$id
    """

    File.write!(Path.join(features_dir, "organized.dsl"), feature_dsl)
    File.write!(Path.join(scenarios_dir, "organized.dsl"), feature_dsl)

    out_dir = tmp_dir!("organized_bdd_out")

    {out, status} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          @fixture_project,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--in",
          in_dir,
          "--out",
          out_dir,
          "--skip-annotations-check",
          "--skip-bdd-test",
          "--no-fail-on-warn"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "compiled_out="
    assert out =~ "count=1"
  end

  test "显式 --registry-module 优先于 .bddc.toml 中的 instructions" do
    config_path = Path.join(@fixture_project, ".bddc.toml")
    stale_path = Path.join(@fixture_project, "tmp/stale_instructions.exs")

    original_config =
      if File.exists?(config_path) do
        File.read!(config_path)
      else
        nil
      end

    on_exit(fn ->
      if original_config do
        File.write!(config_path, original_config)
      else
        File.rm(config_path)
      end

      File.rm(stale_path)
    end)

    File.mkdir_p!(Path.dirname(stale_path))
    File.write!(stale_path, "%{noop: %{kind: :when, args: %{}, outputs: %{}, rules: [], scopes: [:integration], boundary: :test_runtime, async?: false, eventually?: false, assert_class: nil}}\n")
    File.write!(config_path, "[global]\ninstructions = [\"tmp/stale_instructions.exs\"]\n")

    out_dir = tmp_dir!("registry_priority_out")

    {out, status} =
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
          "--in",
          "docs/bdd",
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "BDD 编译完成"
    assert File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
  end
end
