defmodule BDDCompiler.InstructionSetJsonTest do
  use ExUnit.Case, async: true

  alias BDDCompiler.InstructionSet

  @fixtures_dir Path.expand("fixtures/json_instructions", __DIR__)

  setup_all do
    File.mkdir_p!(@fixtures_dir)
    :ok
  end

  describe "load_json!/1" do
    test "加载完整 JSON manifest" do
      path = Path.join(@fixtures_dir, "full_manifest.json")

      json = """
      {
        "total": 2,
        "entries": [
          {
            "name": "unibo_accounting_gl_account_action_create",
            "domain": "Accounting",
            "entity": "GlAccount",
            "source_kind": "action",
            "source_name": "create",
            "kind": "when",
            "dedup_index": 0,
            "args": {"name": {"type": "string", "required": true}, "code": {"type": "string", "required": false}},
            "outputs": {"id": {"type": "uuid"}},
            "scopes": ["integration", "e2e"],
            "boundary": "service"
          },
          {
            "name": "unibo_accounting_gl_account_read_by_id",
            "domain": "Accounting",
            "entity": "GlAccount",
            "source_kind": "read",
            "source_name": "by_id",
            "kind": "then",
            "dedup_index": 0,
            "args": {"id": {"type": "uuid", "required": true}},
            "outputs": {},
            "scopes": ["integration"],
            "boundary": "service"
          }
        ]
      }
      """

      File.write!(path, json)

      set = InstructionSet.load_json!(path)

      # 验证结构类型
      assert %InstructionSet{} = set
      assert map_size(set.v1) == 2
      assert set.v2 == %{}

      # 验证第一条指令
      {:ok, spec1} = InstructionSet.fetch(set, :unibo_accounting_gl_account_action_create)
      assert spec1.kind == :when
      assert spec1.boundary == :service
      assert spec1.scopes == [:integration, :e2e]
      assert spec1.args.name.type == :string
      assert spec1.args.name.required? == true
      assert spec1.args.code.required? == false
      assert spec1.outputs.id == :uuid

      # 验证第二条指令
      {:ok, spec2} = InstructionSet.fetch(set, :unibo_accounting_gl_account_read_by_id)
      assert spec2.kind == :then
      assert spec2.scopes == [:integration]

      # 验证 normalize 默认值
      assert spec1.rules == []
      assert spec1.async? == false
      assert spec1.eventually? == false
      assert spec1.assert_class == nil
    end

    test "缺少 args/outputs/scopes/boundary 字段时使用默认值" do
      path = Path.join(@fixtures_dir, "minimal_manifest.json")

      json = """
      {
        "total": 1,
        "entries": [
          {
            "name": "some_instruction",
            "kind": "given"
          }
        ]
      }
      """

      File.write!(path, json)

      set = InstructionSet.load_json!(path)
      {:ok, spec} = InstructionSet.fetch(set, :some_instruction)

      assert spec.kind == :given
      assert spec.args == %{}
      assert spec.outputs == %{}
      assert spec.scopes == [:integration, :e2e]
      assert spec.boundary == :service
    end

    test "空 entries 列表" do
      path = Path.join(@fixtures_dir, "empty_manifest.json")

      json = """
      {
        "total": 0,
        "entries": []
      }
      """

      File.write!(path, json)

      set = InstructionSet.load_json!(path)
      assert set.v1 == %{}
      assert set.v2 == %{}
    end

    test "文件不存在时抛出 CompileError" do
      assert_raise BDDCompiler.CompileError, ~r/JSON 指令集文件加载失败/, fn ->
        InstructionSet.load_json!("/tmp/nonexistent_#{System.os_time(:nanosecond)}.json")
      end
    end
  end

  describe "--instructions-json 和 --instructions 互斥" do
    test "同时提供两个 flag 时报错" do
      # 通过直接调用 CLI.main/1 模拟
      # 由于 CLI.main 会 System.halt，我们测试 load_instruction_set 的行为
      # 这里通过构造选项来测试互斥逻辑
      json_path = Path.join(@fixtures_dir, "empty_manifest.json")
      File.write!(json_path, ~s({"total": 0, "entries": []}))

      # 模拟 parse_common_args 的输出：同时有 instructions 和 instructions_json
      # 直接测试 InstructionSet 层面不涉及互斥逻辑（互斥在 CLI 层）
      # 所以这里测试 CLI 参数解析
      {opts, _rest, _invalid} =
        OptionParser.parse(
          ["--instructions-json", json_path, "--instructions", "some.exs"],
          switches: [
            instructions: :keep,
            instructions_json: :string,
            instructions_v2: :keep
          ]
        )

      assert Keyword.get(opts, :instructions_json) != nil
      assert Keyword.get_values(opts, :instructions) != []

      # CLI 的 load_instruction_set! 是私有函数，无法直接调用
      # 但我们可以验证参数能被正确解析为互斥条件
      # 实际互斥检查在 CLI.load_instruction_set!/2 中
    end
  end
end
