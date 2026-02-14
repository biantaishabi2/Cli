defmodule MacrosModule do
  defmacro my_macro(arg) do
    quote do: unquote(arg)
  end

  defguard is_valid(x) when is_integer(x) and x > 0

  def real_func(x) do
    x + 1
  end
end
