defmodule BDDCompiler.GoEmitterTest do
  use ExUnit.Case, async: true

  alias BDDCompiler.{DslParser, GoEmitter}

  # 辅助函数：解析 DSL 并生成 Go 代码
  defp emit(dsl, opts \\ []) do
    scenarios = DslParser.parse_string!(dsl)
    GoEmitter.emit_to_string("test.dsl", scenarios, opts)
  end

  test "简单场景（GIVEN/WHEN/THEN）生成有效 Go 代码" do
    code =
      emit("""
      [SCENARIO: S-1] TITLE: simple test
      GIVEN create_user user_id=uuid()
      WHEN do_action name="hello"
      THEN assert_result status="ok"
      """)

    # 包含 package 声明
    assert code =~ "package bdd_test"
    # 包含 import
    assert code =~ "\"github.com/cucumber/godog\""
    assert code =~ "\"testing\""
    # 包含 TestFeatures 函数
    assert code =~ "func TestFeatures(t *testing.T)"
    # 包含 init 函数
    assert code =~ "func initS1(sc *godog.ScenarioContext)"
    # 包含 ctx 初始化
    assert code =~ "ctx := make(map[string]interface{})"
    # 包含 sc.Given/When/Then 调用
    assert code =~ "sc.Given("
    assert code =~ "sc.When("
    assert code =~ "sc.Then("
    # 包含 runtime.RunStep 调用
    assert code =~ "runtime.RunStep(ctx,"
    # 参数正确
    assert code =~ "\"given\""
    assert code =~ "\"create_user\""
    assert code =~ "\"user_id\": uuid.New().String(),"
    # initializeScenario 函数调用 init 函数
    assert code =~ "func initializeScenario(sc *godog.ScenarioContext)"
    assert code =~ "initS1(sc)"
  end

  test "LET 绑定映射到 ctx 赋值" do
    code =
      emit("""
      [SCENARIO: S-LET] TITLE: let test
      LET my_var = uuid()
      LET count = 42
      GIVEN setup_env var=$my_var
      """)

    assert code =~ "ctx[\"my_var\"] = uuid.New().String()"
    assert code =~ "ctx[\"count\"] = 42"
    # 变量引用使用 ctx 访问
    assert code =~ "\"var\": ctx[\"my_var\"]"
  end

  test "表达式映射（uuid, now, dec）正确" do
    code =
      emit("""
      [SCENARIO: S-EXPR] TITLE: expr test
      LET id = uuid()
      LET ts = now()
      LET amount = dec("1.5")
      GIVEN noop
      """)

    assert code =~ "uuid.New().String()"
    assert code =~ "time.Now()"
    assert code =~ "decimal.NewFromString(\"1.5\")"
  end

  test "多个场景生成独立的 init 函数" do
    code =
      emit("""
      [SCENARIO: S-A] TITLE: first
      GIVEN step_a

      [SCENARIO: S-B] TITLE: second
      GIVEN step_b
      """)

    assert code =~ "func initSA(sc *godog.ScenarioContext)"
    assert code =~ "func initSB(sc *godog.ScenarioContext)"
    # initializeScenario 中调用了两个 init 函数
    assert code =~ "initSA(sc)"
    assert code =~ "initSB(sc)"
  end

  test "空场景仍生成有效 Go 代码" do
    code =
      emit("""
      [SCENARIO: S-EMPTY] TITLE: empty scenario
      """)

    # 仍然生成完整的文件结构
    assert code =~ "package bdd_test"
    assert code =~ "func TestFeatures(t *testing.T)"
    assert code =~ "func initSEmpty(sc *godog.ScenarioContext)"
    assert code =~ "func initializeScenario(sc *godog.ScenarioContext)"
  end

  test "module_prefix 选项影响 import 路径" do
    code =
      emit(
        """
        [SCENARIO: S-MP] TITLE: module prefix
        GIVEN noop
        """,
        module_prefix: "github.com/myorg/myapp"
      )

    assert code =~ "\"github.com/myorg/myapp/bdd/runtime\""
  end

  test "布尔和字符串表达式正确映射" do
    code =
      emit("""
      [SCENARIO: S-TYPES] TITLE: types test
      GIVEN check_flag active=true disabled=false label="test"
      """)

    assert code =~ "\"active\": true,"
    assert code =~ "\"disabled\": false,"
    assert code =~ "\"label\": \"test\","
  end

  test "无参数指令生成空 map" do
    code =
      emit("""
      [SCENARIO: S-NOARGS] TITLE: no args
      GIVEN noop
      """)

    assert code =~ "runtime.RunStep(ctx, \"given\", \"noop\", map[string]interface{}{})"
  end
end
