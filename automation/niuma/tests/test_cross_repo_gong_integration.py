import pathlib
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
GONG_ORCHESTRATE_PATH = REPO_ROOT / "gong/.github/workflows/niuma-orchestrate.yml"
GONG_DISPATCH_PATH = REPO_ROOT / "gong/.github/workflows/niuma-dispatch-completed.yml"


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


def evaluate_dispatch_failure_degrade(dispatch_exit_code: int) -> tuple[bool, bool]:
    dispatch_step_failed = dispatch_exit_code != 0
    workflow_continues = True
    warning_emitted = dispatch_step_failed
    return workflow_continues, warning_emitted


def resolve_niuma_binary(build_niuma: bool, preinstalled_path: str) -> str:
    if build_niuma:
        return "$GITHUB_WORKSPACE/.tmp/niuma"
    if not preinstalled_path:
        raise FileNotFoundError("build_niuma=false 但未找到预装 niuma")
    return preinstalled_path


class TestCrossRepoGongIntegration(unittest.TestCase):
    def test_gong_consumes_cli_reusable_with_sha_pin(self) -> None:
        content = GONG_ORCHESTRATE_PATH.read_text(encoding="utf-8")
        self.assertRegex(
            content,
            r"uses:\s*biantaishabi2/Cli/\.github/workflows/niuma-orchestrate-reusable\.yml@[0-9a-f]{40}",
        )
        self.assertIn("build_niuma: false", content)
        self.assertIn("label_whitelist: \"bot:queued,bot:pr-reviewable\"", content)
        self.assertIn("types: [niuma.task.completed]", content)

    def test_cross_repo_issues_labels_have_consistent_trigger_behavior(self) -> None:
        self.assertTrue(evaluate_entrypoint_trigger(event_name="issues", label_name="bot:queued"))
        self.assertTrue(evaluate_entrypoint_trigger(event_name="issues", label_name="bot:pr-reviewable"))
        self.assertFalse(evaluate_entrypoint_trigger(event_name="issues", label_name="bot:other"))

    def test_completed_dispatch_wakeup_is_accepted_without_waiting_schedule(self) -> None:
        self.assertTrue(
            evaluate_entrypoint_trigger(
                event_name="repository_dispatch",
                action="niuma.task.completed",
                event_source="close-after-integration-merge",
            )
        )

    def test_dispatch_failure_degrades_to_warning_and_non_blocking(self) -> None:
        workflow_continues, warning_emitted = evaluate_dispatch_failure_degrade(dispatch_exit_code=1)
        self.assertTrue(workflow_continues)
        self.assertTrue(warning_emitted)
        content = GONG_DISPATCH_PATH.read_text(encoding="utf-8")
        self.assertIn("continue-on-error: true", content)
        self.assertIn("Warn Dispatch Failure", content)

    def test_build_niuma_false_uses_preinstalled_binary(self) -> None:
        resolved = resolve_niuma_binary(build_niuma=False, preinstalled_path="/usr/local/bin/niuma")
        self.assertEqual(resolved, "/usr/local/bin/niuma")
        with self.assertRaises(FileNotFoundError):
            resolve_niuma_binary(build_niuma=False, preinstalled_path="")

    def test_gong_dispatch_payload_matches_contract(self) -> None:
        content = GONG_DISPATCH_PATH.read_text(encoding="utf-8")
        self.assertIn("event_type: \"niuma.task.completed\"", content)
        self.assertIn("source_issue", content)
        self.assertIn("source_issues", content)
        self.assertIn("trigger_pr", content)
        self.assertIn("event_source: \"close-after-integration-merge\"", content)
        self.assertIn("event_id", content)
        self.assertRegex(content, r"event_id=\"pr-\$\{PR_NUMBER\}-run-\$\{GITHUB_RUN_ID\}-\$\{GITHUB_RUN_ATTEMPT\}\"")


if __name__ == "__main__":
    unittest.main()
