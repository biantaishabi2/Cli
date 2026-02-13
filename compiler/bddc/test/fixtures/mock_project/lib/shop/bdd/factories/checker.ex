defmodule Shop.BDD.Factories.Checker do
  @moduledoc false

  def check_paths!(paths) when is_list(paths) do
    if Enum.any?(paths, &(&1 == "__FAIL__")) do
      raise "invalid path marker"
    end

    :ok
  end
end
