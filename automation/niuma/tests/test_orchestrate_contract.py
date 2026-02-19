import json
import pathlib
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
SCHEMA_PATH = REPO_ROOT / "automation/niuma/contracts/orchestrate_inputs.schema.json"
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"
ENTRYPOINT_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate.yml"


def evaluate_entrypoint_trigger(
    event_name: str,
    label_name: str = "",
    action: str = "",
    event_source: str = "",
) -> bool:
    if event_name == "issues":
        return label_name in {"bot:queued", "bot:pr-reviewable"}
    if event_name == "repository_dispatch":
        return action == "niuma.task.completed" and event_source == "close-after-integration-merge"
    if event_name == "schedule":
        return True
    return False


def evaluate_reusable_trigger_gate(
    event_name: str,
    label_whitelist: str = "bot:queued,bot:pr-reviewable",
    label_name: str = "",
    enable_dispatch_wakeup: bool = True,
    action: str = "",
    event_source: str = "",
) -> tuple[bool, str]:
    if event_name == "issues":
        labels = [item.strip() for item in label_whitelist.split(",")]
        if label_name in labels:
            return True, "accepted"
        return False, "label_not_whitelisted"
    if event_name == "repository_dispatch":
        if not enable_dispatch_wakeup:
            return False, "dispatch_disabled"
        if action != "niuma.task.completed":
            return False, "dispatch_type_mismatch"
        if event_source != "close-after-integration-merge":
            return False, "dispatch_source_mismatch"
        return True, "accepted"
    return True, "accepted"


class TestOrchestrateContract(unittest.TestCase):
    def test_schema_defaults_and_required(self) -> None:
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        self.assertEqual(schema["required"], ["repo"])
        props = schema["properties"]
        self.assertEqual(props["repo_dir"]["default"], ".")
        self.assertTrue(props["build_niuma"]["default"])
        self.assertEqual(props["label_whitelist"]["default"], "bot:queued,bot:pr-reviewable")
        self.assertTrue(props["enable_dispatch_wakeup"]["default"])
        self.assertEqual(props["event_id"]["default"], "")
        self.assertEqual(props["dedup_window_hours"]["default"], 24)
        self.assertEqual(props["dedup_window_hours"]["minimum"], 1)
        self.assertEqual(props["dedup_window_hours"]["multipleOf"], 1)
        self.assertEqual(props["concurrency_key"]["default"], "niuma-orchestrate-${repo}")

    def test_reusable_declares_minimal_permissions(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("permissions:", content)
        self.assertIn("issues: write", content)
        self.assertIn("contents: read", content)
        self.assertNotIn("actions: write", content)

    def test_entrypoint_is_thin_wrapper(self) -> None:
        content = ENTRYPOINT_PATH.read_text(encoding="utf-8")
        self.assertIn("types: [labeled]", content)
        self.assertIn("types: [niuma.task.completed]", content)
        self.assertIn("uses: ./.github/workflows/niuma-orchestrate-reusable.yml", content)
        self.assertIn("label_whitelist: \"bot:queued,bot:pr-reviewable\"", content)
        self.assertNotIn("control close-merged", content)

    def test_trigger_matrix_for_issues_labels(self) -> None:
        for label_name in ("bot:queued", "bot:pr-reviewable"):
            self.assertTrue(evaluate_entrypoint_trigger(event_name="issues", label_name=label_name))
            should_run, reason = evaluate_reusable_trigger_gate(event_name="issues", label_name=label_name)
            self.assertTrue(should_run)
            self.assertEqual(reason, "accepted")

        self.assertFalse(evaluate_entrypoint_trigger(event_name="issues", label_name="bot:unknown"))
        should_run, reason = evaluate_reusable_trigger_gate(event_name="issues", label_name="bot:unknown")
        self.assertFalse(should_run)
        self.assertEqual(reason, "label_not_whitelisted")

    def test_trigger_matrix_for_repository_dispatch_and_schedule(self) -> None:
        self.assertTrue(
            evaluate_entrypoint_trigger(
                event_name="repository_dispatch",
                action="niuma.task.completed",
                event_source="close-after-integration-merge",
            )
        )
        should_run, reason = evaluate_reusable_trigger_gate(
            event_name="repository_dispatch",
            action="niuma.task.completed",
            event_source="close-after-integration-merge",
        )
        self.assertTrue(should_run)
        self.assertEqual(reason, "accepted")

        should_run, reason = evaluate_reusable_trigger_gate(
            event_name="repository_dispatch",
            enable_dispatch_wakeup=False,
            action="niuma.task.completed",
            event_source="close-after-integration-merge",
        )
        self.assertFalse(should_run)
        self.assertEqual(reason, "dispatch_disabled")

        self.assertTrue(evaluate_entrypoint_trigger(event_name="schedule"))
        should_run, reason = evaluate_reusable_trigger_gate(event_name="schedule")
        self.assertTrue(should_run)
        self.assertEqual(reason, "accepted")


if __name__ == "__main__":
    unittest.main()
