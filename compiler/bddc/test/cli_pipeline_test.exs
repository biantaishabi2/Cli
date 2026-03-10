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

  defp write_nested_dsl_fixture!(root) do
    nested_dir = Path.join(root, "docs/bdd/features")
    File.mkdir_p!(nested_dir)

    File.write!(
      Path.join(nested_dir, "nested.dsl"),
      """
      [SCENARIO: FX-NESTED-001] TITLE: nested dsl TAGS: smoke
      GIVEN given_seed id="seed-nested"
      WHEN when_do id=$id
      THEN assert_done id=$id
      """
    )
  end

  defp write_duplicate_scenario_fixture!(root) do
    features_dir = Path.join(root, "docs/bdd/features")
    scenarios_dir = Path.join(root, "docs/bdd/scenarios")
    File.mkdir_p!(features_dir)
    File.mkdir_p!(scenarios_dir)

    source = File.read!(Path.join(root, "docs/bdd/simple.dsl"))
    File.write!(Path.join(features_dir, "simple.dsl"), source)
    File.write!(Path.join(scenarios_dir, "simple.dsl"), source)
  end

  test "缺少指令来源时 compile 失败" do
    {out, status} =
      System.cmd(@escript, ["compile", "--in", "docs/bdd", "--out", "/tmp/bdd_compiler_missing_src"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert out =~ "--instructions"
    assert out =~ "registry_module"
  end

  test "可通过 --registry-module 自动装载指令并 compile 成功，并默认推导 runtime module" do
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
          "--in",
          "docs/bdd",
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ out_dir
    assert File.exists?(Path.join(out_dir, "simple_generated_test.exs"))
    assert File.read!(Path.join(out_dir, "simple_generated_test.exs")) =~
             "Fixture.BDD.Instructions.V1.run_step!"
  end

  test "显式 --runtime-module 优先于 registry 推导" do
    out_dir = tmp_dir!("compile_runtime_override")

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
          "Custom.Runtime.V1",
          "--in",
          "docs/bdd",
          "--out",
          out_dir
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ out_dir
    assert File.read!(Path.join(out_dir, "simple_generated_test.exs")) =~ "Custom.Runtime.V1.run_step!"
  end

  test "compile 在 features 布局下优先扫描 docs/bdd/features/**/*.dsl 并忽略 scenarios 重复" do
    root = tmp_dir!("nested_compile_project")
    File.cp_r!(@fixture_project, root)
    write_nested_dsl_fixture!(root)
    write_duplicate_scenario_fixture!(root)
    out_dir = Path.join(root, "tmp_out")

    {out, status} =
      System.cmd(
        @escript,
        [
          "compile",
          "--project-root",
          root,
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
    assert out =~ out_dir
    assert File.exists?(Path.join(out_dir, "nested_generated_test.exs"))
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

  test "check 默认会推导 runtime module 并运行生成测试（可用 --skip-bdd-test 跳过）" do
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
end
