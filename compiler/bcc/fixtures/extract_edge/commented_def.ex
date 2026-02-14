defmodule CommentedDef do
  # def fake_function(x), do: x
  @doc "defines something"
  def real_function(x), do: x
end
