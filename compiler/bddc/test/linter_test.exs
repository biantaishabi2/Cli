defmodule BDDCompiler.LinterTest do
  use ExUnit.Case, async: true

  alias BDDCompiler.{InstructionSet, Linter}

  defp temp_dir(prefix) do
    root = System.tmp_dir!()
    path = Path.join(root, "#{prefix}_#{System.unique_integer([:positive])}")
    File.rm_rf!(path)
    File.mkdir_p!(path)
    path
  end

  defp write(path, content) do
    path |> Path.dirname() |> File.mkdir_p!()
    File.write!(path, content)
  end

  defp instruction_set do
    %InstructionSet{
      v1: %{
        given_seed_context: %{
          name: :given_seed_context,
          kind: :given,
          args: %{},
          outputs: %{},
          rules: [],
          scopes: [:integration, :e2e],
          boundary: :service,
          async?: false,
          eventually?: false,
          assert_class: nil
        },
        when_execute_seed_contract: %{
          name: :when_execute_seed_contract,
          kind: :when,
          args: %{},
          outputs: %{},
          rules: [],
          scopes: [:integration, :e2e],
          boundary: :service,
          async?: false,
          eventually?: false,
          assert_class: nil
        },
        then_seed_contract_should_hold: %{
          name: :then_seed_contract_should_hold,
          kind: :then,
          args: %{},
          outputs: %{},
          rules: [],
          scopes: [:integration, :e2e],
          boundary: :service,
          async?: false,
          eventually?: false,
          assert_class: nil
        }
      },
      v2: %{}
    }
  end

  test "structured seed contract suppresses weak assertion warning" do
    root = temp_dir("bddc_linter_seed_contract")

    write(
      Path.join(root, "scenarios/travel_order.dsl"),
      """
      [SCENARIO: BDD-TRAVEL_TRAVEL_ORDER-SEED-travel_order_submit] TITLE: submit TAGS: seed action_contract
      GIVEN given_seed_context id="travel_order_submit" module="TRAVEL_TRAVEL_ORDER"
      WHEN when_execute_seed_contract module="TRAVEL_TRAVEL_ORDER"
      THEN then_seed_contract_should_hold module="TRAVEL_TRAVEL_ORDER"
      """
    )

    write(
      Path.join(root, "contexts/travel_order_submit.json"),
      ~s|{
        "id": "travel_order_submit",
        "module": "TRAVEL_TRAVEL_ORDER",
        "edge_class": "action_contract",
        "source_file": "travel/action_travel_order_submit.yaml",
        "source_summary": "GIVEN ...",
        "expected_facts": ["record_persisted", "state_change"],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())
    refute Enum.any?(warnings, &(&1.rule == :weak_assertion))
  end

  test "workflow single-step seed contract is exempt from weak assertion metric" do
    root = temp_dir("bddc_linter_workflow_weak")
    features_dir = Path.join(root, "features")

    write(
      Path.join(features_dir, "shipment_status.dsl"),
      """
      [SCENARIO: BDD-DELIVERY_SHIPMENT_STATUS-SEED-shipment_status_record_flow] TITLE: workflow TAGS: seed workflow_contract
      GIVEN given_seed_context id="shipment_status_record_flow" module="DELIVERY_SHIPMENT_STATUS"
      WHEN when_execute_seed_contract module="DELIVERY_SHIPMENT_STATUS"
      THEN then_seed_contract_should_hold module="DELIVERY_SHIPMENT_STATUS"
      """
    )

    write(
      Path.join(root, "contexts/shipment_status_record_flow.json"),
      ~s|{
        "id": "shipment_status_record_flow",
        "module": "DELIVERY_SHIPMENT_STATUS",
        "edge_class": "workflow_contract",
        "source_file": "delivery/workflow_shipment_status_record_flow.yaml",
        "source_summary": "GIVEN ...",
        "expected_facts": ["workflow_checked", "steps_verified"],
        "action_path": ["create"],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())

    refute Enum.any?(
             warnings,
             &(&1.rule == :weak_assertion and
                 &1.scenario_id ==
                   "BDD-DELIVERY_SHIPMENT_STATUS-SEED-shipment_status_record_flow")
           )
  end

  test "lint_paths also consumes structured seed contract from sibling contexts" do
    root = temp_dir("bddc_linter_paths_seed_contract")
    features_dir = Path.join(root, "features")
    contexts_dir = Path.join(root, "contexts")

    write(
      Path.join(features_dir, "travel_order.dsl"),
      """
      [SCENARIO: BDD-TRAVEL_TRAVEL_ORDER-SEED-travel_order_submit] TITLE: submit TAGS: seed action_contract
      GIVEN given_seed_context id="travel_order_submit" module="TRAVEL_TRAVEL_ORDER"
      WHEN when_execute_seed_contract module="TRAVEL_TRAVEL_ORDER"
      THEN then_seed_contract_should_hold module="TRAVEL_TRAVEL_ORDER"
      """
    )

    write(
      Path.join(contexts_dir, "travel_order_submit.json"),
      ~s|{
        "id": "travel_order_submit",
        "module": "TRAVEL_TRAVEL_ORDER",
        "edge_class": "action_contract",
        "source_file": "travel/action_travel_order_submit.yaml",
        "source_summary": "GIVEN ...",
        "expected_facts": ["record_persisted"],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings =
      Linter.lint_paths([Path.join(features_dir, "travel_order.dsl")], instruction_set())

    refute Enum.any?(warnings, &(&1.rule == :weak_assertion))
  end

  test "determination_contract seed 场景触发 determination_degraded_pass 告警" do
    root = temp_dir("bddc_linter_determination_degraded")
    features_dir = Path.join(root, "features")
    contexts_dir = Path.join(root, "contexts")

    write(
      Path.join(features_dir, "order_determination.dsl"),
      """
      [SCENARIO: BDD-ORDER_DETERMINATION-SEED-order_det_check] TITLE: check TAGS: seed determination_contract
      GIVEN given_seed_context id="order_det_check" module="ORDER_DETERMINATION"
      WHEN when_execute_seed_contract module="ORDER_DETERMINATION"
      THEN then_seed_contract_should_hold module="ORDER_DETERMINATION"
      """
    )

    write(
      Path.join(contexts_dir, "order_det_check.json"),
      ~s|{
        "id": "order_det_check",
        "module": "ORDER_DETERMINATION",
        "edge_class": "determination_contract",
        "expected_facts": ["rule_matched"],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())

    assert Enum.any?(
             warnings,
             &(&1.rule == :determination_degraded_pass and
                 &1.scenario_id == "BDD-ORDER_DETERMINATION-SEED-order_det_check")
           )
  end

  test "reactor_contract seed 场景触发 reactor_degraded_pass 告警" do
    root = temp_dir("bddc_linter_reactor_degraded")
    features_dir = Path.join(root, "features")
    contexts_dir = Path.join(root, "contexts")

    write(
      Path.join(features_dir, "payment_reactor.dsl"),
      """
      [SCENARIO: BDD-PAYMENT_REACTOR-SEED-payment_react_handle] TITLE: handle TAGS: seed reactor_contract
      GIVEN given_seed_context id="payment_react_handle" module="PAYMENT_REACTOR"
      WHEN when_execute_seed_contract module="PAYMENT_REACTOR"
      THEN then_seed_contract_should_hold module="PAYMENT_REACTOR"
      """
    )

    write(
      Path.join(contexts_dir, "payment_react_handle.json"),
      ~s|{
        "id": "payment_react_handle",
        "module": "PAYMENT_REACTOR",
        "edge_class": "reactor_contract",
        "expected_facts": ["event_handled"],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())

    assert Enum.any?(
             warnings,
             &(&1.rule == :reactor_degraded_pass and
                 &1.scenario_id == "BDD-PAYMENT_REACTOR-SEED-payment_react_handle")
           )
  end

  test "action_contract 场景不触发 degraded 告警" do
    root = temp_dir("bddc_linter_no_degraded_for_action")
    features_dir = Path.join(root, "features")
    contexts_dir = Path.join(root, "contexts")

    write(
      Path.join(features_dir, "order_submit.dsl"),
      """
      [SCENARIO: BDD-ORDER_ORDER-SEED-order_submit] TITLE: submit TAGS: seed action_contract
      GIVEN given_seed_context id="order_submit" module="ORDER_ORDER"
      WHEN when_execute_seed_contract module="ORDER_ORDER"
      THEN then_seed_contract_should_hold module="ORDER_ORDER"
      """
    )

    write(
      Path.join(contexts_dir, "order_submit.json"),
      ~s|{
        "id": "order_submit",
        "module": "ORDER_ORDER",
        "edge_class": "action_contract",
        "expected_facts": ["record_persisted"],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())

    refute Enum.any?(
             warnings,
             &(&1.rule in [:determination_degraded_pass, :reactor_degraded_pass])
           )
  end

  test "determination_contract 无 then_seed_contract_should_hold 时不触发 degraded 告警" do
    root = temp_dir("bddc_linter_no_seed_then_no_degraded")
    features_dir = Path.join(root, "features")
    contexts_dir = Path.join(root, "contexts")

    write(
      Path.join(features_dir, "det_no_hold.dsl"),
      """
      [SCENARIO: BDD-ORDER_DETERMINATION-MANUAL-order_det_manual] TITLE: manual TAGS: integration determination_contract
      GIVEN given_seed_context id="order_det_manual" module="ORDER_DETERMINATION"
      WHEN when_execute_seed_contract module="ORDER_DETERMINATION"
      THEN assert_noop
      """
    )

    write(
      Path.join(contexts_dir, "order_det_manual.json"),
      ~s|{
        "id": "order_det_manual",
        "module": "ORDER_DETERMINATION",
        "edge_class": "determination_contract",
        "expected_facts": [],
        "action_path": [],
        "branch_hints": [],
        "declared_error_codes": [],
        "business_post_conditions": []
      }|
    )

    warnings = Linter.lint_dir(root, instruction_set())

    refute Enum.any?(
             warnings,
             &(&1.rule in [:determination_degraded_pass, :reactor_degraded_pass])
           )
  end

  test "stale context falls back to priv/bdd/sources structured fields" do
    root = temp_dir("bddc_linter_seed_source_fallback")
    features_dir = Path.join(root, "docs/bdd/features")
    contexts_dir = Path.join(root, "docs/bdd/contexts")
    sources_dir = Path.join(root, "priv/bdd/sources/travel")

    write(
      Path.join(features_dir, "travel_order.dsl"),
      """
      [SCENARIO: BDD-TRAVEL_TRAVEL_ORDER-SEED-travel_order_submit] TITLE: submit TAGS: seed action_contract
      GIVEN given_seed_context id="travel_order_submit" module="TRAVEL_TRAVEL_ORDER"
      WHEN when_execute_seed_contract module="TRAVEL_TRAVEL_ORDER"
      THEN then_seed_contract_should_hold module="TRAVEL_TRAVEL_ORDER"
      """
    )

    write(
      Path.join(contexts_dir, "travel_order_submit.json"),
      ~s|{
        "id": "travel_order_submit",
        "module": "TRAVEL_TRAVEL_ORDER",
        "edge_class": "action_contract",
        "source_file": "travel/action_travel_order_submit.yaml",
        "source_summary": "GIVEN ..."
      }|
    )

    write(
      Path.join(sources_dir, "action_travel_order_submit.yaml"),
      """
      module: TRAVEL_TRAVEL_ORDER
      edge_class: action_contract
      source_summary: "GIVEN 订单可提交 / WHEN 执行 submit / THEN 应落库"
      expected_facts:
        - record_persisted
        - state_change
      """
    )

    warnings = Linter.lint_dir(Path.join(root, "docs/bdd"), instruction_set())
    refute Enum.any?(warnings, &(&1.rule == :weak_assertion))
  end
end
