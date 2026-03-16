defmodule BDDCompiler.CLIInitAndConfigTest do
  use ExUnit.Case, async: false

  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} =
      System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)

    _ = out
    :ok
  end

  test "init 生成最小脚手架 + runtime.caps.sync + check 可跑通（纯文件项目，无 mix）" do
    root =
      Path.join(
        System.tmp_dir!(),
        "bddc_init_" <> Integer.to_string(System.unique_integer([:positive]))
      )

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
    assert File.exists?(Path.join(root, "test/support/bdd/common_instructions.ex"))
    assert File.exists?(Path.join(root, "test/support/bdd/bddc_runtime.ex"))
    assert File.exists?(Path.join(root, "test/support/bdd/instructions_v1.ex"))
    assert File.exists?(Path.join(root, "docs/bdd/hello.dsl"))

    instructions_spec = File.read!(Path.join(root, "priv/bdd/instructions_v1.exs"))
    common_instructions = File.read!(Path.join(root, "test/support/bdd/common_instructions.ex"))
    runtime_macro = File.read!(Path.join(root, "test/support/bdd/bddc_runtime.ex"))
    runtime_dispatcher = File.read!(Path.join(root, "test/support/bdd/instructions_v1.ex"))

    Enum.each(
      ["given_seed_context", "when_execute_seed_contract", "then_seed_contract_should_hold"],
      fn instruction ->
        assert instructions_spec =~ instruction
        assert common_instructions =~ instruction
        assert runtime_macro =~ instruction
        assert runtime_dispatcher =~ instruction
      end
    )

    {out_caps, status_caps} =
      System.cmd(
        @escript,
        ["runtime.caps.sync", "--project-root", root],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status_caps == 0
    assert out_caps =~ "runtime_caps_v1.exs"

    caps_file = Path.join(root, "docs/bdd/_generated/runtime_caps_v1.exs")
    {caps, _binding} = Code.eval_file(caps_file)

    Enum.each(
      [:given_seed_context, :when_execute_seed_contract, :then_seed_contract_should_hold],
      fn cap -> assert cap in caps end
    )

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

  test "init 默认模板支持最小 seed DSL smoke" do
    root =
      Path.join(
        System.tmp_dir!(),
        "bddc_init_seed_" <> Integer.to_string(System.unique_integer([:positive]))
      )

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

    File.write!(Path.join(root, "docs/bdd/seed.dsl"), """
    [SCENARIO: SEED-001] TITLE: seed smoke TAGS: integration seed
    GIVEN given_seed_context id="seed-1" module="TRAVEL_ORDER"
    WHEN when_execute_seed_contract module="TRAVEL_ORDER"
    THEN then_seed_contract_should_hold module="TRAVEL_ORDER"
    """)

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
    refute out_check =~ "未知指令：:given_seed_context"
    assert File.exists?(Path.join(root, "test/bdd_generated/seed_generated_test.exs"))
  end

  test ".bddc.toml 支持覆盖默认 in/out/instructions 等参数（命令行可 override）" do
    root =
      Path.join(
        System.tmp_dir!(),
        "bddc_cfg_" <> Integer.to_string(System.unique_integer([:positive]))
      )

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

  test "显式 --registry-module 会覆盖 .bddc.toml 中陈旧的 instructions" do
    root =
      Path.join(
        System.tmp_dir!(),
        "bddc_cfg_registry_" <> Integer.to_string(System.unique_integer([:positive]))
      )

    File.mkdir_p!(Path.join(root, "docs/bdd"))
    File.mkdir_p!(Path.join(root, "docs/bdd/_generated"))
    File.mkdir_p!(Path.join(root, "priv/bdd"))
    File.mkdir_p!(Path.join(root, "lib/fixture/bdd"))

    File.write!(Path.join(root, "mix.exs"), """
    defmodule Fixture.MixProject do
      use Mix.Project

      def project do
        [app: :fixture, version: "0.1.0", elixir: "~> 1.15", deps: []]
      end

      def application, do: [extra_applications: [:logger]]
    end
    """)

    File.write!(Path.join(root, "docs/bdd/a.dsl"), """
    [SCENARIO: CFG-REG-001] TITLE: cfg registry TAGS: integration
    GIVEN given_seed_context id="seed-1" module="TRAVEL_ORDER"
    WHEN when_do
    THEN assert_done
    """)

    File.write!(Path.join(root, "priv/bdd/instructions_v1.exs"), """
    %{
      when_do: %{kind: :when, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: nil},
      assert_done: %{kind: :then, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: :weak}
    }
    """)

    File.write!(
      Path.join(root, "docs/bdd/_generated/runtime_caps_v1.exs"),
      "[:given_seed_context, :when_do, :assert_done]\n"
    )

    File.write!(Path.join(root, ".bddc.toml"), """
    [global]
    instructions = [\"priv/bdd/instructions_v1.exs\"]
    in = \"docs/bdd\"
    out = \"test/bdd_generated\"
    runtime_module = \"Fixture.BDD.Instructions.V1\"
    runtime_caps_file = \"docs/bdd/_generated/runtime_caps_v1.exs\"
    test_case = \"ExUnit.Case\"
    module_prefix = \"Fixture.BDD.Generated\"
    registry_module = \"Fixture.BDD.InstructionRegistry\"
    """)

    File.write!(Path.join(root, "lib/fixture/bdd/instruction_registry.ex"), """
    defmodule Fixture.BDD.InstructionRegistry do
      def all(_version \\\\ :v1) do
        [
          %{name: :given_seed_context, kind: :given, args: %{id: %{type: :string, required?: true, allowed: nil}, module: %{type: :string, required?: true, allowed: nil}}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: nil},
          %{name: :when_do, kind: :when, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: nil},
          %{name: :assert_done, kind: :then, args: %{}, outputs: %{}, rules: [], scopes: [:integration, :e2e], boundary: :service, async?: false, eventually?: false, assert_class: :weak}
        ]
      end
    end
    """)

    out_dir = Path.join(root, "test/bdd_generated")

    {out, status} =
      System.cmd(
        @escript,
        [
          "check",
          "--project-root",
          root,
          "--registry-module",
          "Fixture.BDD.InstructionRegistry",
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--runtime-caps-file",
          "docs/bdd/_generated/runtime_caps_v1.exs",
          "--skip-annotations-check",
          "--skip-bdd-test",
          "--no-fail-on-warn"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    refute out =~ "未知指令：:given_seed_context"
    assert File.exists?(Path.join(out_dir, "a_generated_test.exs"))
  end
end
