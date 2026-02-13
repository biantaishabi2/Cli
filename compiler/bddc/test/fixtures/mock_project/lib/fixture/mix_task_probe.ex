defmodule Fixture.MixTaskProbe do
  @moduledoc false

  def emit(task, args) when is_binary(task) and is_list(args) do
    IO.puts("TASK=#{task}")
    IO.puts("ARGS=#{Enum.join(args, "|")}")
  end
end

