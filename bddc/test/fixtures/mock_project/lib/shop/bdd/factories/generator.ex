defmodule Shop.BDD.Factories.Generator do
  @moduledoc false

  def generate_files!(schemas) when is_list(schemas) do
    Enum.map(schemas, fn schema ->
      file_name = Map.fetch!(schema, :file_name)
      module_name = Map.get(schema, :module, "Unknown")

      %{
        path: file_name,
        content: "# generated for #{module_name}\n"
      }
    end)
  end
end
