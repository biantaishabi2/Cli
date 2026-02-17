defmodule DemoService do
  def run do
    Helper.exec()
    Helper.exec()
    Alpha.start()
    Alpha.start()
    :ok
  end
end
