defmodule BDDCompiler.ValidatorTest do
  use ExUnit.Case, async: true

  alias BDDCompiler.{DslParser, InstructionSet, Validator}

  test "validate! fails on unknown instruction" do
    scenarios =
      DslParser.parse_string!("""
      [SCENARIO: S-1] TITLE: hello TAGS: integration bdd_v1
      GIVEN unknown_g
      WHEN do_something
      THEN assert_noop
      """)

    iset = %InstructionSet{
      v1: %{
        assert_noop: %{name: :assert_noop, kind: :then, args: %{}, outputs: %{}, rules: [], scopes: [:integration]}
      }
    }

    assert_raise BDDCompiler.CompileError, ~r/未知指令/, fn ->
      Validator.validate!("in-memory.dsl", scenarios, iset)
    end
  end

  test "validate! fails when now() is used before clock_freeze" do
    scenarios =
      DslParser.parse_string!("""
      [SCENARIO: S-1] TITLE: hello TAGS: integration bdd_v1
      LET t = now()
      WHEN do_something
      THEN assert_noop
      """)

    iset = %InstructionSet{
      v1: %{
        do_something: %{name: :do_something, kind: :when, args: %{}, outputs: %{}, rules: [], scopes: [:integration]},
        assert_noop: %{name: :assert_noop, kind: :then, args: %{}, outputs: %{}, rules: [], scopes: [:integration]}
      }
    }

    assert_raise BDDCompiler.CompileError, ~r/clock_freeze/, fn ->
      Validator.validate!("in-memory.dsl", scenarios, iset)
    end
  end
end
