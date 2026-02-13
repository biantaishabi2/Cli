defmodule BDDCompiler.CLIInitAndConfigTest do
  use ExUnit.Case, async: false

  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  test "init 生成最小脚手架 + runtime.caps.sync + check 可跑通（纯文件项目，无 mix）" do
    root = Path.join(System.tmp_dir!(), "bddc_init_" <> Integer.to_string(System.unique_integer([:positive])))
    File.mkdir_p!(root)

    {out_init, status_init} =
      System.cmd(
        @escript,
        ["init", "--project-root", root, "--namespace", "Demo", "--force"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_init == 0
    assert out_init =~ "[init] done"
    assert File.exists?(Path.join(root, ".bddc.toml"))
    assert File.exists?(Path.join(root, "priv/bdd/instructions_v1.exs"))
    assert File.exists?(Path.join(root, "test/support/bdd/instructions_v1.ex"))
    assert File.exists?(Path.join(root, "docs/bdd/hello.dsl"))

    {out_caps, status_caps} =
      System.cmd(
        @escript,
        ["runtime.caps.sync", "--project-root", root],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_caps == 0
    assert out_caps =~ "runtime_caps_v1.exs"

    {out_check, status_check} =
      System.cmd(
        @escript,
        ["check", "--project-root", root, "--no-fail-on-warn"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_check == 0
    assert out_check =~ "BDD lint:"
    assert File.exists?(Path.join(root, "test/bdd_generated/hello_generated_test.exs"))
  end

  test ".bddc.toml 支持覆盖默认 in/out/instructions 等参数（命令行可 override）" do
    root = Path.join(System.tmp_dir!(), "bddc_cfg_" <> Integer.to_string(System.unique_integer([:positive])))
    File.mkdir_p!(Path.join(root, "docs/bdd"))
    File.mkdir_p!(Path.join(root, "docs/bdd/_generated"))
    File.mkdir_p!(Path.join(root, "priv/bdd"))

    File.write!(Path.join(root, "docs/bdd/a.dsl"), """
    [SCENARIO: CFG-001] TITLE: cfg TAGS: integration
    GIVEN x
    WHEN z
    THEN y
    """)

    File.write!(Path.join(root, "priv/bdd/instructions_v1.exs"), """
    %{
      x: %{kind: :given, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: nil},
      z: %{kind: :when, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: nil},
      y: %{kind: :then, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: :weak}
    }
    """)

    File.write!(Path.join(root, "docs/bdd/_generated/runtime_caps_v1.exs"), "[:x, :z, :y]\n")

    File.write!(Path.join(root, ".bddc.toml"), """
    [global]
    instructions = [\"priv/bdd/instructions_v1.exs\"]
    in = \"docs/bdd\"
    out = \"test/bdd_generated\"\n
    runtime_module = \"Demo.BDD.Instructions.V1\"
    runtime_caps_file = \"docs/bdd/_generated/runtime_caps_v1.exs\"
    test_case = \"ExUnit.Case\"
    module_prefix = \"Demo.BDD.Generated\"
    """)

    out_dir = Path.join(root, "tmp/out_override")
    File.rm_rf!(out_dir)

    {out, status} =
      System.cmd(
        @escript,
        ["check", "--project-root", root, "--out", "tmp/out_override", "--no-fail-on-warn"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    _ = out
    assert File.exists?(Path.join(out_dir, "a_generated_test.exs"))
  end
end
