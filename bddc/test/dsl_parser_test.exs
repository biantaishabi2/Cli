defmodule BDDCompiler.DslParserTest do
  use ExUnit.Case, async: true

  alias BDDCompiler.DslParser

  test "parse_string! parses scenario header, tags and steps" do
    scenarios =
      DslParser.parse_string!("""
      [SCENARIO: S-1] TITLE: hello TAGS: integration bdd_v1
      GIVEN create_user user_id=uuid()
      WHEN do_something
      THEN assert_noop
      """)

    assert [%{id: "S-1", title: "hello", tags: [:integration, :bdd_v1], steps: steps}] = scenarios
    assert {:given, :create_user, %{user_id: {:call, :uuid, []}}, _meta} = Enum.at(steps, 0)
    assert {:when, :do_something, %{}, _meta} = Enum.at(steps, 1)
    assert {:then, :assert_noop, %{}, _meta} = Enum.at(steps, 2)
  end
end

