defmodule BDDCompiler.CLIHelpTest do
  use ExUnit.Case, async: true

  import ExUnit.CaptureIO

  test "--help prints global help with descriptions" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["--help"])
      end)

    assert out =~ "bdd_compiler (escript)"
    assert out =~ "编译器本体命令"
    assert out =~ "项目命令"
    assert out =~ "bdd_compiler <command> --help"
    assert out =~ "factories.scaffold"
    assert out =~ "factories.check"
    assert out =~ "runtime.caps.sync"
    assert out =~ "domain.autowire"
    assert out =~ "init"
    assert out =~ "contract.check"
    assert out =~ "mutation.run"
  end

  test "compile --help prints command-specific help" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["compile", "--help"])
      end)

    assert out =~ "bdd_compiler compile"
    assert out =~ "作用:"
    assert out =~ "--instructions"
    assert out =~ "--registry-module"
    refute out =~ "未知命令"
  end

  test "check --help mentions runtime coverage" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["check", "--help"])
      end)

    assert out =~ "runtime 覆盖校验"
    assert out =~ "--registry-module"
  end

  test "mix-wrapper subcommands --help are supported" do
    for cmd <- [
          "annotations.check",
          "instructions.docs",
          "contract.check",
          "fuzz",
          "mutation.report",
          "mutation.run"
        ] do
      out =
        capture_io(fn ->
          BDDCompiler.CLI.main([cmd, "--help"])
        end)

      assert out =~ "bdd_compiler #{cmd}"
      assert out =~ "等价执行：mix"
    end
  end

  test "registry.* --help mentions fallback to standalone" do
    for cmd <- ["registry.scaffold", "registry.upsert"] do
      out =
        capture_io(fn ->
          BDDCompiler.CLI.main([cmd, "--help"])
        end)

      assert out =~ "bdd_compiler #{cmd}"
      assert out =~ "fallback"
      assert out =~ "--standalone"
      assert out =~ "等价执行：mix"
    end
  end

  test "domain.autowire --help is supported" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["domain.autowire", "--help"])
      end)

    assert out =~ "bdd_compiler domain.autowire"
    assert out =~ "--strict"
    assert out =~ "--strict-factories"
    assert out =~ "--skip-factories-check"
    assert out =~ "--sync-runtime-caps"
    assert out =~ "--runtime-caps-file"
    assert out =~ "--dry-run"
    assert out =~ "--module"
    assert out =~ "--functions"
  end

  test "factories subcommands --help are supported" do
    for cmd <- ["factories.scaffold", "factories.upsert", "factories.check"] do
      out =
        capture_io(fn ->
          BDDCompiler.CLI.main([cmd, "--help"])
        end)

      assert out =~ "bdd_compiler #{cmd}"
      assert out =~ "CLI 内置实现"
      assert out =~ "mix bdd.factories"
    end
  end

  test "runtime.caps.sync --help is supported" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["runtime.caps.sync", "--help"])
      end)

    assert out =~ "bdd_compiler runtime.caps.sync"
    assert out =~ "--runtime-module"
    assert out =~ "--runtime-source"
    assert out =~ "--out"
    assert out =~ "--out-meta"
  end

  test "help <command> works" do
    out =
      capture_io(fn ->
        BDDCompiler.CLI.main(["help", "lint"])
      end)

    assert out =~ "bdd_compiler lint"
    assert out =~ "静态检查"
  end
end
