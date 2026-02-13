defmodule Fixture.BDD.InstructionRegistry do
  @spec all(:v1 | :v2) :: [map()]
  def all(_version \\ :v1) do
    [
      %{
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
      },
      %{
        name: :when_do,
        kind: :when,
        args: %{id: %{type: :string, required?: true, allowed: nil}},
        outputs: %{},
        rules: [],
        scopes: [:integration, :e2e],
        boundary: :service,
        async?: false,
        eventually?: false,
        assert_class: nil
      },
      %{
        name: :assert_done,
        kind: :then,
        args: %{id: %{type: :string, required?: true, allowed: nil}},
        outputs: %{},
        rules: [],
        scopes: [:integration, :e2e],
        boundary: :service,
        async?: false,
        eventually?: false,
        assert_class: :A
      }
    ]
  end
end

