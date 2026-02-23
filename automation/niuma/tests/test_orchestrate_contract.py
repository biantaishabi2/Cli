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
        cls.reusable_content = REUSABLE_PATH.read_text(encoding="utf-8")
        cls.entrypoint_content = ENTRYPOINT_PATH.read_text(encoding="utf-8")

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
        self.assertNotIn("label_whitelist:", content)
        self.assertNotIn("control close-merged", content)

    def test_entrypoint_is_routed_by_reusable_without_job_if(self) -> None:
        content = self.entrypoint_content
        self.assertIn("orchestrate:\n    uses: ./.github/workflows/niuma-orchestrate-reusable.yml", content)
        self.assertNotIn("orchestrate:\n    if:", content)

    def test_reusable_routes_event_in_control(self) -> None:
        content = self.reusable_content
        self.assertIn("- name: Route Event in Control", content)
        self.assertIn("control route-event", content)
        self.assertIn("decision=", content)
        self.assertIn("reason=", content)
        self.assertIn("action=", content)
        self.assertIn("steps.route_event.outputs.decision == 'run'", content)
        self.assertIn("steps.route_event.outputs.action == 'orchestrate'", content)


if __name__ == "__main__":
    unittest.main()
