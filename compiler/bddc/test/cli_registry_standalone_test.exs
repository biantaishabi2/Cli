defmodule BDDCompiler.CLIRegistryStandaloneTest do
  use ExUnit.Case, async: false

  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  test "registry.scaffold/upsert standalone 可从源码注解生成 spec 并写入 GENERATED 区域" do
    root = Path.join(System.tmp_dir!(), "bddc_reg_" <> Integer.to_string(System.unique_integer([:positive])))
    File.mkdir_p!(root)

    {out_init, status_init} =
      System.cmd(
        @escript,
        ["init", "--project-root", root, "--namespace", "RegDemo", "--force"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_init == 0
    assert out_init =~ "[init] done"

    File.mkdir_p!(Path.join(root, "lib"))

    File.write!(Path.join(root, "lib/demo.ex"), """
    defmodule RegDemo.Sample do
      @bdd_instruction %{
        name: :given_seed,
        kind: :given,
        args: %{id: %{type: :string, required?: true, allowed: nil}},
        outputs: %{id: :string},
        rules: [],
        scopes: [:integration, :e2e],
        boundary: :service,
        async?: false,
        eventually?: false,
        assert_class: nil
      }
    end
    """)

    scaffold = "priv/bdd/_generated/instructions_v1_scaffold.exs"

    {out_scaffold, status_scaffold} =
      System.cmd(
        @escript,
        ["registry.scaffold", "--project-root", root, "--standalone", "--src", "lib", "--out", scaffold],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_scaffold == 0
    assert out_scaffold =~ "mode=standalone"

    {out_upsert, status_upsert} =
      System.cmd(
        @escript,
        ["registry.upsert", "--project-root", root, "--standalone", "--scaffold", scaffold, "--target", "priv/bdd/instructions_v1.exs"],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_upsert == 0
    assert out_upsert =~ "mode=standalone"

    target = File.read!(Path.join(root, "priv/bdd/instructions_v1.exs"))
    assert target =~ "given_seed:"

    # Ensure compiler can load merged instructions and compile a DSL that references it.
    File.write!(Path.join(root, "docs/bdd/reg.dsl"), """
    [SCENARIO: REG-001] TITLE: reg TAGS: integration
    GIVEN given_seed id=\"seed-1\"
    WHEN noop
    THEN assert_noop
    """)

    {out_compile, status_compile} =
      System.cmd(
        @escript,
        ["compile", "--project-root", root],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_compile == 0
    assert out_compile =~ "BDD 编译完成"
    assert File.exists?(Path.join(root, "test/bdd_generated/reg_generated_test.exs"))
  end
end
