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

  test "workflow single-step seed contract remains weak" do
    root = temp_dir("bddc_linter_workflow_weak")

    write(
      Path.join(root, "scenarios/shipment_status.dsl"),
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
    assert Enum.any?(
             warnings,
             &(&1.rule == :weak_assertion and
                 &1.scenario_id ==
                   "BDD-DELIVERY_SHIPMENT_STATUS-SEED-shipment_status_record_flow")
           )
  end
end
