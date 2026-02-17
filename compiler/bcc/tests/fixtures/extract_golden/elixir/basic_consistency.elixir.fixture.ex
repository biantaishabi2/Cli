defmodule DemoService do
  alias App.Helper

  @spec run(String.t()) :: :ok
  def run(input) do
    Helper.exec(input)
    :ok
  end
end
