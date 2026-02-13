defmodule Shop.BDD.Factories.Scope do
  @moduledoc false

  defstruct default_out_dir: "tmp/factories_generated", schemas: []

  def load!(nil), do: load!("priv/bdd/factories_scope.exs")

  def load!(path) when is_binary(path) do
    {value, _binding} = Code.eval_file(path)

    case value do
      %__MODULE__{} = s -> s
      %{default_out_dir: out, schemas: schemas} -> %__MODULE__{default_out_dir: out, schemas: schemas}
      _ -> raise "invalid factories scope: #{inspect(value)}"
    end
  end
end
