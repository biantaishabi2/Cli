defmodule SampleService do
  @moduledoc """
  会话上下文管理服务
  """

  alias CompactionService

  @spec load_context(String.t()) :: {:ok, map()} | {:error, term()}
  def load_context(session_id) do
    {:ok, %{session_id: session_id}}
  end

  @spec persist_turn(String.t(), map()) :: {:ok, boolean()} | {:error, term()}
  def persist_turn(session_id, turn) do
    CompactionService.save(session_id, turn)
    {:ok, true}
  end

  defp do_internal_work(data) do
    # 私有函数，不应被 extract 提取
    data
  end
end
