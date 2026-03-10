defmodule BDDCompiler.CLIRuntimeCapsSyncTest do
  use ExUnit.Case, async: false

  @fixture_project Path.expand("fixtures/mock_project", __DIR__)
  @escript Path.expand("../bdd_compiler", __DIR__)

  setup_all do
    {out, 0} = System.cmd("mix", ["escript.build"], cd: Path.expand("..", __DIR__), stderr_to_stdout: true)
    _ = out
    :ok
  end

  test "runtime.caps.sync 默认按 runtime_module 自动发现源码并生成 caps 文件" do
    runtime_source = Path.join(@fixture_project, "test/support/bdd/instructions_auto_sync.ex")
    File.write!(runtime_source, runtime_source_fixture_content())
    on_exit(fn -> File.rm(runtime_source) end)

    out_file = Path.join(@fixture_project, "docs/bdd/_generated/runtime_caps_auto_sync.exs")
    File.rm(out_file)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-module",
          "Fixture.BDD.Instructions.AutoSync"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "generated=#{out_file}"
    assert out =~ "count=3"
    assert File.exists?(out_file)

    {caps, _binding} = Code.eval_file(out_file)
    assert caps == [:assert_done, :given_seed, :when_do]
  end

  test "runtime.caps.sync 支持 --runtime-source 与 --out 显式路径" do
    runtime_source = Path.join(@fixture_project, "tmp/runtime_source_for_caps_sync.ex")
    File.mkdir_p!(Path.dirname(runtime_source))
    File.write!(runtime_source, runtime_source_fixture_content())
    on_exit(fn -> File.rm(runtime_source) end)

    out_file = Path.join(@fixture_project, "tmp/runtime_caps_v1_from_source.exs")
    File.rm(out_file)

    out_meta_file = Path.join(@fixture_project, "tmp/runtime_caps_v1_from_source_meta.exs")
    File.rm(out_meta_file)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-module",
          "Fixture.BDD.Instructions.V1",
          "--runtime-source",
          "tmp/runtime_source_for_caps_sync.ex",
          "--out",
          "tmp/runtime_caps_v1_from_source.exs",
          "--out-meta",
          "tmp/runtime_caps_v1_from_source_meta.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "generated=#{out_file}"
    assert out =~ "sources="
    assert File.exists?(out_file)
    assert File.exists?(out_meta_file)

    {caps, _binding} = Code.eval_file(out_file)
    assert Enum.sort(caps) == [:assert_done, :given_seed, :when_do]

    {meta, _binding} = Code.eval_file(out_meta_file)
    assert Map.has_key?(meta, :given_seed)
  end

  test "runtime.caps.sync 在找不到 runtime 源文件时失败" do
    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-module",
          "Fixture.BDD.Instructions.Missing"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    normalized_out = normalize_cli_output(out)
    assert normalized_out =~ "未找到 runtime 源文件"
    assert normalized_out =~ "instructions*missing.ex"
  end

  test "runtime.caps.sync 在未提取到指令时失败" do
    runtime_source = Path.join(@fixture_project, "tmp/runtime_source_empty_caps.ex")
    File.mkdir_p!(Path.dirname(runtime_source))
    File.write!(runtime_source, runtime_source_without_steps_content())
    on_exit(fn -> File.rm(runtime_source) end)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-source",
          "tmp/runtime_source_empty_caps.ex",
          "--out",
          "tmp/runtime_caps_empty.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert normalize_cli_output(out) =~ "未提取到任何 runtime 指令"
  end

  test "runtime.caps.sync 在指令名非法时失败" do
    runtime_source = Path.join(@fixture_project, "tmp/runtime_source_invalid_name.ex")
    File.mkdir_p!(Path.dirname(runtime_source))
    File.write!(runtime_source, runtime_source_invalid_name_content())
    on_exit(fn -> File.rm(runtime_source) end)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-source",
          "tmp/runtime_source_invalid_name.ex",
          "--out",
          "tmp/runtime_caps_invalid_name.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert normalize_cli_output(out) =~ "runtime 指令名非法"
  end

  test "runtime.caps.sync 在同名跨 step 类型冲突时失败" do
    runtime_source = Path.join(@fixture_project, "tmp/runtime_source_conflict_kind.ex")
    File.mkdir_p!(Path.dirname(runtime_source))
    File.write!(runtime_source, runtime_source_conflict_kind_content())
    on_exit(fn -> File.rm(runtime_source) end)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-source",
          "tmp/runtime_source_conflict_kind.ex",
          "--out",
          "tmp/runtime_caps_conflict_kind.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 1
    assert normalize_cli_output(out) =~ "runtime 指令冲突"
  end

  test "runtime.caps.sync 支持多个 --runtime-source 合并提取" do
    s1 = Path.join(@fixture_project, "tmp/runtime_source_multi_1.ex")
    s2 = Path.join(@fixture_project, "tmp/runtime_source_multi_2.ex")
    File.mkdir_p!(Path.dirname(s1))
    File.write!(s1, runtime_source_multi_1())
    File.write!(s2, runtime_source_multi_2())
    on_exit(fn -> File.rm(s1) end)
    on_exit(fn -> File.rm(s2) end)

    out_file = Path.join(@fixture_project, "tmp/runtime_caps_multi.exs")
    File.rm(out_file)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-source",
          "tmp/runtime_source_multi_1.ex",
          "--runtime-source",
          "tmp/runtime_source_multi_2.ex",
          "--out",
          "tmp/runtime_caps_multi.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "runtime_caps_multi.exs"
    assert File.exists?(out_file)

    {caps, _binding} = Code.eval_file(out_file)
    assert Enum.sort(caps) == [:a_given, :b_when]
  end

  test "runtime.caps.sync 支持从 run!/5 形参中提取指令" do
    runtime_source = Path.join(@fixture_project, "tmp/runtime_source_fun_heads.ex")
    File.mkdir_p!(Path.dirname(runtime_source))
    File.write!(runtime_source, runtime_source_fun_heads())
    on_exit(fn -> File.rm(runtime_source) end)

    out_file = Path.join(@fixture_project, "tmp/runtime_caps_fun_heads.exs")
    File.rm(out_file)

    {out, status} =
      System.cmd(
        @escript,
        [
          "runtime.caps.sync",
          "--project-root",
          @fixture_project,
          "--runtime-source",
          "tmp/runtime_source_fun_heads.ex",
          "--out",
          "tmp/runtime_caps_fun_heads.exs"
        ],
        cd: Path.expand("..", __DIR__),
        stderr_to_stdout: true
      )

    assert status == 0
    assert out =~ "runtime_caps_fun_heads.exs"

    {caps, _binding} = Code.eval_file(out_file)
    assert Enum.sort(caps) == [:a_given, :b_when, :c_then]
  end

  defp runtime_source_fixture_content do
    """
    defmodule Fixture.BDD.RuntimeSourceFixture do
      def run!(_ctx, tuple) do
        case tuple do
          {:given, :given_seed} -> :ok
          {:when, :when_do} -> :ok
          {:then, :assert_done} -> :ok
        end
      end
    end
    """
  end

  defp runtime_source_without_steps_content do
    """
    defmodule Fixture.BDD.RuntimeSourceWithoutSteps do
      def run!(_ctx, _tuple), do: :ok
    end
    """
  end

  defp runtime_source_invalid_name_content do
    """
    defmodule Fixture.BDD.RuntimeSourceInvalidName do
      def run!(_ctx, tuple) do
        case tuple do
          {:given, :BadName} -> :ok
        end
      end
    end
    """
  end

  defp runtime_source_conflict_kind_content do
    """
    defmodule Fixture.BDD.RuntimeSourceConflictKind do
      def run!(_ctx, tuple) do
        case tuple do
          {:given, :same_name} -> :ok
          {:when, :same_name} -> :ok
        end
      end
    end
    """
  end

  defp runtime_source_multi_1 do
    """
    defmodule Fixture.BDD.RuntimeSourceMulti1 do
      def run!(_ctx, tuple) do
        case tuple do
          {:given, :a_given} -> :ok
        end
      end
    end
    """
  end

  defp runtime_source_multi_2 do
    """
    defmodule Fixture.BDD.RuntimeSourceMulti2 do
      def run!(_ctx, tuple) do
        case tuple do
          {:when, :b_when} -> :ok
        end
      end
    end
    """
  end

  defp runtime_source_fun_heads do
    """
    defmodule Fixture.BDD.RuntimeSourceFunHeads do
      def run!(_ctx, :given, :a_given, _args, _meta), do: :ok
      def run!(_ctx, :when, :b_when, _args, _meta), do: :ok
      def run!(_ctx, :then, :c_then, _args, _meta), do: :ok
    end
    """
  end

  defp normalize_cli_output(output) when is_binary(output) do
    Regex.replace(~r/\\x\{([0-9A-Fa-f]+)\}/, output, fn _, hex ->
      {codepoint, ""} = Integer.parse(hex, 16)
      <<codepoint::utf8>>
    end)
  end
end
