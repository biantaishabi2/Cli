import json
import os
import pathlib
import subprocess
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
SCHEMA_PATH = REPO_ROOT / "automation/niuma/contracts/orchestrate_inputs.schema.json"
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"
ENTRYPOINT_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate.yml"


def extract_step_run_script(workflow_path: pathlib.Path, step_name: str) -> str:
    lines = workflow_path.read_text(encoding="utf-8").splitlines()
    step_index = None
    step_indent = 0
    for index, line in enumerate(lines):
        stripped = line.strip()
        if stripped == f"- name: {step_name}":
            step_index = index
            step_indent = len(line) - len(line.lstrip(" "))
            break
    if step_index is None:
        raise AssertionError(f"未找到 step: {step_name}")

    run_index = None
    run_indent = 0
    for index in range(step_index + 1, len(lines)):
        line = lines[index]
        indent = len(line) - len(line.lstrip(" "))
        if line.lstrip().startswith("- name:") and indent <= step_indent:
            break
        if line.strip() == "run: |":
            run_index = index
            run_indent = indent
            break
    if run_index is None:
        raise AssertionError(f"step {step_name} 缺少 run: |")

    block_lines: list[str] = []
    for index in range(run_index + 1, len(lines)):
        line = lines[index]
        if line.strip() == "":
            block_lines.append("")
            continue
        indent = len(line) - len(line.lstrip(" "))
        if indent <= run_indent:
            break
        block_lines.append(line)

    min_indent = min(
        (len(line) - len(line.lstrip(" ")) for line in block_lines if line.strip()),
        default=0,
    )
    return "\n".join(line[min_indent:] if line else "" for line in block_lines)


def extract_job_if_expression(workflow_path: pathlib.Path, job_name: str) -> str:
    lines = workflow_path.read_text(encoding="utf-8").splitlines()
    job_index = None
    job_indent = 0
    for index, line in enumerate(lines):
        stripped = line.strip()
        if stripped == f"{job_name}:":
            job_index = index
            job_indent = len(line) - len(line.lstrip(" "))
            break
    if job_index is None:
        raise AssertionError(f"未找到 job: {job_name}")

    for index in range(job_index + 1, len(lines)):
        line = lines[index]
        indent = len(line) - len(line.lstrip(" "))
        if indent <= job_indent and line.strip():
            break
        if line.strip().startswith("if: >"):
            parts: list[str] = []
            for sub_index in range(index + 1, len(lines)):
                sub_line = lines[sub_index]
                sub_indent = len(sub_line) - len(sub_line.lstrip(" "))
                if sub_indent <= indent and sub_line.strip():
                    break
                text = sub_line.strip()
                if text:
                    parts.append(text)
            return " ".join(parts)
        if line.strip().startswith("if:"):
            return line.split("if:", 1)[1].strip()
    raise AssertionError(f"job {job_name} 缺少 if 条件")


def evaluate_entrypoint_if(
    expression: str,
    *,
    event_name: str,
    label_name: str = "",
    action: str = "",
    event_source: str = "",
) -> bool:
    compiled = " ".join(expression.split())
    compiled = compiled.replace("&&", " and ").replace("||", " or ")
    replacements = {
        "github.event.client_payload.event_source": repr(event_source),
        "github.event.label.name": repr(label_name),
        "github.event.action": repr(action),
        "github.event_name": repr(event_name),
    }
    for key, value in replacements.items():
        compiled = compiled.replace(key, value)
    if "github." in compiled:
        raise AssertionError(f"存在未替换上下文: {compiled}")
    return bool(eval(compiled, {"__builtins__": {}}, {}))


def run_step_script(script: str, env: dict[str, str]) -> tuple[subprocess.CompletedProcess[str], dict[str, str]]:
    github_output = pathlib.Path(os.environ.get("TMPDIR", "/tmp")) / f"niuma-test-output-{os.getpid()}-{os.urandom(4).hex()}"
    github_output.write_text("", encoding="utf-8")
    run_env = os.environ.copy()
    run_env.update(env)
    run_env["GITHUB_OUTPUT"] = str(github_output)

    completed = subprocess.run(
        ["bash", "-c", f"set -euo pipefail\n{script}"],
        check=False,
        capture_output=True,
        text=True,
        env=run_env,
    )

    outputs: dict[str, str] = {}
    if github_output.exists():
        for line in github_output.read_text(encoding="utf-8").splitlines():
            if "=" in line:
                key, value = line.split("=", 1)
                outputs[key] = value
        github_output.unlink(missing_ok=True)

    return completed, outputs


class TestOrchestrateContract(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        gate_script = extract_step_run_script(REUSABLE_PATH, "Enforce Trigger Gate")
        cls.reusable_trigger_gate_script = (
            gate_script.replace("${{ inputs.label_whitelist }}", "${INPUT_LABEL_WHITELIST}")
            .replace("${{ github.event.label.name }}", "${EVENT_LABEL_NAME}")
            .replace("${{ inputs.enable_dispatch_wakeup }}", "${INPUT_ENABLE_DISPATCH_WAKEUP}")
            .replace("${{ github.event.action }}", "${EVENT_ACTION}")
            .replace("${{ github.event.client_payload.event_source }}", "${EVENT_SOURCE}")
        )
        cls.entrypoint_if_expr = extract_job_if_expression(ENTRYPOINT_PATH, "orchestrate")

    def test_schema_defaults_and_required(self) -> None:
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        self.assertEqual(schema["required"], ["repo"])
        props = schema["properties"]
        self.assertEqual(props["repo_dir"]["default"], ".")
        self.assertNotIn("build_niuma", props)
        self.assertEqual(props["label_whitelist"]["default"], "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged")
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
        self.assertIn("label_whitelist: \"bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged\"", content)
        self.assertNotIn("control close-merged", content)

    def test_entrypoint_trigger_matrix_uses_real_if_expression(self) -> None:
        cases = [
            {
                "name": "issues_queued",
                "event_name": "issues",
                "label_name": "bot:queued",
                "expected": True,
            },
            {
                "name": "issues_pr_reviewable",
                "event_name": "issues",
                "label_name": "bot:pr-reviewable",
                "expected": True,
            },
            {
                "name": "issues_premerged",
                "event_name": "issues",
                "label_name": "bot:premerged",
                "expected": True,
            },
            {
                "name": "issues_other",
                "event_name": "issues",
                "label_name": "bot:other",
                "expected": False,
            },
            {
                "name": "dispatch_completed",
                "event_name": "repository_dispatch",
                "action": "niuma.task.completed",
                "event_source": "close-after-integration-merge",
                "expected": True,
            },
            {
                "name": "dispatch_wrong_source",
                "event_name": "repository_dispatch",
                "action": "niuma.task.completed",
                "event_source": "other-source",
                "expected": False,
            },
        ]

        for case in cases:
            with self.subTest(case=case["name"]):
                actual = evaluate_entrypoint_if(
                    self.entrypoint_if_expr,
                    event_name=case["event_name"],
                    label_name=case.get("label_name", ""),
                    action=case.get("action", ""),
                    event_source=case.get("event_source", ""),
                )
                self.assertEqual(actual, case["expected"])

    def test_reusable_trigger_gate_behavior_from_real_script(self) -> None:
        cases = [
            {
                "name": "issues_queued",
                "env": {
                    "GITHUB_EVENT_NAME": "issues",
                    "INPUT_LABEL_WHITELIST": "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged",
                    "EVENT_LABEL_NAME": "bot:queued",
                    "INPUT_ENABLE_DISPATCH_WAKEUP": "true",
                    "EVENT_ACTION": "",
                    "EVENT_SOURCE": "",
                },
                "expected_should_run": "true",
                "expected_reason": "accepted",
            },
            {
                "name": "issues_not_whitelisted",
                "env": {
                    "GITHUB_EVENT_NAME": "issues",
                    "INPUT_LABEL_WHITELIST": "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged",
                    "EVENT_LABEL_NAME": "bot:blocked",
                    "INPUT_ENABLE_DISPATCH_WAKEUP": "true",
                    "EVENT_ACTION": "",
                    "EVENT_SOURCE": "",
                },
                "expected_should_run": "false",
                "expected_reason": "label_not_whitelisted",
            },
            {
                "name": "dispatch_accepted",
                "env": {
                    "GITHUB_EVENT_NAME": "repository_dispatch",
                    "INPUT_LABEL_WHITELIST": "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged",
                    "EVENT_LABEL_NAME": "",
                    "INPUT_ENABLE_DISPATCH_WAKEUP": "true",
                    "EVENT_ACTION": "niuma.task.completed",
                    "EVENT_SOURCE": "close-after-integration-merge",
                },
                "expected_should_run": "true",
                "expected_reason": "accepted",
            },
            {
                "name": "dispatch_disabled",
                "env": {
                    "GITHUB_EVENT_NAME": "repository_dispatch",
                    "INPUT_LABEL_WHITELIST": "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged",
                    "EVENT_LABEL_NAME": "",
                    "INPUT_ENABLE_DISPATCH_WAKEUP": "false",
                    "EVENT_ACTION": "niuma.task.completed",
                    "EVENT_SOURCE": "close-after-integration-merge",
                },
                "expected_should_run": "false",
                "expected_reason": "dispatch_disabled",
            },
            {
                "name": "schedule_accepted",
                "env": {
                    "GITHUB_EVENT_NAME": "schedule",
                    "INPUT_LABEL_WHITELIST": "bot:orchestrate,bot:queued,bot:pr-reviewable,bot:premerged",
                    "EVENT_LABEL_NAME": "",
                    "INPUT_ENABLE_DISPATCH_WAKEUP": "true",
                    "EVENT_ACTION": "",
                    "EVENT_SOURCE": "",
                },
                "expected_should_run": "true",
                "expected_reason": "accepted",
            },
        ]

        for case in cases:
            with self.subTest(case=case["name"]):
                completed, outputs = run_step_script(self.reusable_trigger_gate_script, case["env"])
                self.assertEqual(completed.returncode, 0, msg=completed.stderr + completed.stdout)
                self.assertEqual(outputs.get("should_run"), case["expected_should_run"])
                self.assertEqual(outputs.get("reason"), case["expected_reason"])


if __name__ == "__main__":
    unittest.main()
