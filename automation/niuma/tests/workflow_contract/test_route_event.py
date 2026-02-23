import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[4]


class RouteEventWorkflowContractTest(unittest.TestCase):
    def test_orchestrate_reusable_calls_control_route_event(self):
        content = (ROOT / ".github/workflows/niuma-orchestrate-reusable.yml").read_text()
        self.assertIn("control route-event", content)
        self.assertIn("steps.route_event.outputs.decision == 'run'", content)
        self.assertNotIn("label_whitelist", content)
        self.assertNotIn("dispatch_source_mismatch", content)

    def test_iterate_workflow_no_shell_regex_parser(self):
        content = (ROOT / ".github/workflows/niuma-iterate.yml").read_text()
        self.assertNotIn("grep -Eo 'Closes", content)
        self.assertIn("call-iterate-from-human", content)

    def test_entry_workflows_route_via_control(self):
        for path in [
            ROOT / ".github/workflows/niuma-plan.yml",
            ROOT / ".github/workflows/niuma-implement.yml",
            ROOT / ".github/workflows/niuma-review.yml",
        ]:
            content = path.read_text()
            self.assertIn("control route-event", content)
            self.assertIn("needs.route-event.outputs.decision == 'run'", content)


if __name__ == "__main__":
    unittest.main()
