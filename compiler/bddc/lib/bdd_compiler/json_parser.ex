defmodule BDDCompiler.JsonParser do
  @moduledoc """
  零依赖的简易 JSON 解析器。

  仅支持 instruction_manifest.json 需要的 JSON 子集：
  object, array, string, number (integer), boolean, null。
  不支持浮点数、unicode 转义等复杂特性。
  """

  @spec parse!(String.t()) :: term()
  def parse!(input) when is_binary(input) do
    {value, rest} = parse_value(String.trim(input))
    rest = String.trim(rest)

    if rest != "" do
      raise ArgumentError, "JSON 解析错误：多余内容 #{String.slice(rest, 0, 50)}"
    end

    value
  end

  # --- 内部解析函数 ---

  defp parse_value(<<"{", rest::binary>>), do: parse_object(String.trim(rest), %{})
  defp parse_value(<<"[", rest::binary>>), do: parse_array(String.trim(rest), [])
  defp parse_value(<<"\"", rest::binary>>), do: parse_string(rest, [])
  defp parse_value(<<"true", rest::binary>>), do: {true, rest}
  defp parse_value(<<"false", rest::binary>>), do: {false, rest}
  defp parse_value(<<"null", rest::binary>>), do: {nil, rest}

  defp parse_value(<<c, _::binary>> = input) when c in ?0..?9 or c == ?- do
    parse_number(input, [])
  end

  defp parse_value(input) do
    raise ArgumentError, "JSON 解析错误：非法字符 #{String.slice(input, 0, 20)}"
  end

  # 解析对象
  defp parse_object(<<"}", rest::binary>>, acc), do: {acc, rest}

  defp parse_object(<<"\"", rest::binary>>, acc) do
    {key, rest} = parse_string(rest, [])
    rest = rest |> String.trim() |> expect_char(?:)
    {value, rest} = rest |> String.trim() |> parse_value()
    acc = Map.put(acc, key, value)
    rest = String.trim(rest)

    case rest do
      <<"}", rest::binary>> -> {acc, rest}
      <<",", rest::binary>> -> parse_object(String.trim(rest), acc)
      _ -> raise ArgumentError, "JSON 解析错误：对象中期望 , 或 }"
    end
  end

  defp parse_object(input, _acc) do
    raise ArgumentError, "JSON 解析错误：对象中期望字符串键，得到 #{String.slice(input, 0, 20)}"
  end

  # 解析数组
  defp parse_array(<<"]", rest::binary>>, acc), do: {Enum.reverse(acc), rest}

  defp parse_array(input, acc) do
    {value, rest} = parse_value(input)
    acc = [value | acc]
    rest = String.trim(rest)

    case rest do
      <<"]", rest::binary>> -> {Enum.reverse(acc), rest}
      <<",", rest::binary>> -> parse_array(String.trim(rest), acc)
      _ -> raise ArgumentError, "JSON 解析错误：数组中期望 , 或 ]"
    end
  end

  # 解析字符串（处理基本转义）
  defp parse_string(<<"\\\"", rest::binary>>, acc), do: parse_string(rest, [?" | acc])
  defp parse_string(<<"\\\\", rest::binary>>, acc), do: parse_string(rest, [?\\ | acc])
  defp parse_string(<<"\\/", rest::binary>>, acc), do: parse_string(rest, [?/ | acc])
  defp parse_string(<<"\\n", rest::binary>>, acc), do: parse_string(rest, [?\n | acc])
  defp parse_string(<<"\\r", rest::binary>>, acc), do: parse_string(rest, [?\r | acc])
  defp parse_string(<<"\\t", rest::binary>>, acc), do: parse_string(rest, [?\t | acc])
  defp parse_string(<<"\\b", rest::binary>>, acc), do: parse_string(rest, [?\b | acc])
  defp parse_string(<<"\\f", rest::binary>>, acc), do: parse_string(rest, [?\f | acc])

  defp parse_string(<<"\"", rest::binary>>, acc) do
    {acc |> Enum.reverse() |> IO.chardata_to_string(), rest}
  end

  defp parse_string(<<c::utf8, rest::binary>>, acc) do
    parse_string(rest, [c | acc])
  end

  defp parse_string("", _acc) do
    raise ArgumentError, "JSON 解析错误：字符串未闭合"
  end

  # 解析数字（仅整数）
  defp parse_number(<<c, rest::binary>>, acc) when c in ?0..?9 or c == ?- do
    parse_number(rest, [c | acc])
  end

  defp parse_number(rest, acc) do
    num = acc |> Enum.reverse() |> IO.chardata_to_string() |> String.to_integer()
    {num, rest}
  end

  defp expect_char(<<c, rest::binary>>, c), do: rest

  defp expect_char(input, c) do
    raise ArgumentError, "JSON 解析错误：期望 '#{<<c>>}'，得到 #{String.slice(input, 0, 10)}"
  end
end
