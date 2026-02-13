defmodule SampleServiceWithSpec do
  @moduledoc """
  带完整 @spec 的示例服务
  """

  @spec greet(String.t()) :: String.t()
  def greet(name) do
    "Hello, #{name}"
  end
end
