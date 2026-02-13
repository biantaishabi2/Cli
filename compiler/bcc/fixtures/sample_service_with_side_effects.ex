defmodule SampleServiceWithSideEffects do
  @moduledoc """
  带副作用的示例服务
  """

  def start_worker(config) do
    Task.async(fn -> do_work(config) end)
  end

  def fetch_data(url) do
    HTTPoison.get(url)
  end

  defp do_work(config) do
    GenServer.call(MyWorker, {:process, config})
  end
end
