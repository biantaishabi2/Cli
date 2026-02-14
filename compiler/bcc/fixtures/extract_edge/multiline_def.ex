defmodule MultilineDef do
  @spec handle_call({:get, term()}, term(), map()) :: {:reply, term(), map()}
  def handle_call({:get, key},
                   _from,
                   %{data: data} = state) do
    {:reply, Map.get(data, key), state}
  end
end
