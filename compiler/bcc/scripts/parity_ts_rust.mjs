#!/usr/bin/env node

import { readFileSync, writeFileSync, mkdirSync, existsSync, rmSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

function parseArgs(argv) {
  const out = {
    projectRoot: "",
    bccBin: "bcc",
    out: "",
    maxFiles: 0,
    strict: false,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === "--project-root") out.projectRoot = argv[++i] ?? "";
    else if (a === "--bcc-bin") out.bccBin = argv[++i] ?? "bcc";
    else if (a === "--out") out.out = argv[++i] ?? "";
    else if (a === "--max-files") out.maxFiles = Number(argv[++i] ?? "0");
    else if (a === "--strict") out.strict = true;
  }
  if (!out.projectRoot) {
    throw new Error("missing --project-root");
  }
  if (!out.out) {
    out.out = path.join(out.projectRoot, "docs/backend-trace/artifacts/ts-rust-parity.json");
  }
  return out;
}

function run(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, {
    cwd: opts.cwd,
    encoding: "utf8",
    maxBuffer: 1024 * 1024 * 50,
  });
  if (r.error) {
    return { ok: false, code: 1, stdout: "", stderr: String(r.error) };
  }
  return { ok: r.status === 0, code: r.status ?? 1, stdout: r.stdout ?? "", stderr: r.stderr ?? "" };
}

function writeJson(outPath, payload) {
  mkdirSync(path.dirname(outPath), { recursive: true });
  writeFileSync(outPath, `${JSON.stringify(payload, null, 2)}\n`, "utf8");
}

function sum(arr, key) {
  return arr.reduce((acc, x) => acc + (Number(x?.[key] ?? 0) || 0), 0);
}

function countTruthy(arr, key) {
  return arr.reduce((acc, x) => acc + (x?.[key] ? 1 : 0), 0);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const projectRoot = path.resolve(args.projectRoot);
  const outPath = path.resolve(args.out);
  const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
  const tsAudit = path.join(scriptsDir, "audit-backend-ast.ts");

  // Ensure TypeScript dependency exists (required by audit-backend-ast.ts).
  const tsCheck = run("node", ["-e", "require('typescript')"], { cwd: scriptsDir });
  if (!tsCheck.ok) {
    const payload = {
      status: "skipped",
      reason: "typescript dependency missing in compiler/bcc/scripts",
      hint: "run: npm --prefix compiler/bcc/scripts install",
    };
    writeJson(outPath, payload);
    process.exit(args.strict ? 2 : 0);
  }

  // Clean previous AST artifact for deterministic run.
  const astJson = path.join(projectRoot, "docs/backend-trace/artifacts/backend-docs-ast.json");
  if (existsSync(astJson)) {
    rmSync(astJson);
  }

  const tsxBin = path.join(scriptsDir, "node_modules", ".bin", process.platform === "win32" ? "tsx.cmd" : "tsx");
  const tsRun = existsSync(tsxBin)
    ? run(tsxBin, [tsAudit, "snapshot", "--write"], { cwd: projectRoot })
    : run("node", ["--experimental-strip-types", tsAudit, "snapshot", "--write"], { cwd: projectRoot });
  if (!tsRun.ok || !existsSync(astJson)) {
    const payload = {
      status: "failed",
      step: "ts_snapshot",
      code: tsRun.code,
      stderr: tsRun.stderr.slice(0, 4000),
    };
    writeJson(outPath, payload);
    process.exit(2);
  }

  const tsSummary = JSON.parse(readFileSync(astJson, "utf8"));
  const records = Array.isArray(tsSummary.records) ? tsSummary.records : [];
  const sample = args.maxFiles > 0 ? records.slice(0, args.maxFiles) : records;

  let rustSourceCount = 0;
  let rustImportEdges = 0;
  let rustCallEdges = 0;
  let rustAsyncCount = 0;
  let rustHttpCount = 0;
  let rustFailures = 0;

  for (const rec of sample) {
    const rel = String(rec.sourcePath ?? "");
    if (!rel) continue;
    const abs = path.join(projectRoot, rel);
    const r = run(args.bccBin, ["extract", abs, "--mode", "ast"]);
    if (!r.ok) {
      rustFailures += 1;
      continue;
    }
    let parsed;
    try {
      parsed = JSON.parse(r.stdout);
    } catch {
      rustFailures += 1;
      continue;
    }
    rustSourceCount += 1;
    const imports = Array.isArray(parsed.imports) ? parsed.imports : [];
    const calls = Array.isArray(parsed.calls) ? parsed.calls : [];
    rustImportEdges += imports.filter((im) => String(im.specifier ?? "").startsWith(".")).length;
    rustCallEdges += calls.length;
    if (parsed?.side_effects?.hasAsync) rustAsyncCount += 1;
    if (parsed?.side_effects?.hasHttp) rustHttpCount += 1;
  }

  const tsImportEdges = sample.reduce(
    (acc, rec) => acc + (Array.isArray(rec.localDependencies) ? rec.localDependencies.length : 0),
    0,
  );
  const tsCallEdges = sample.reduce(
    (acc, rec) => acc + (Array.isArray(rec.localCallTargets) ? rec.localCallTargets.length : 0),
    0,
  );
  const tsAsyncCount = countTruthy(sample.map((r) => r.sideEffects || {}), "hasAsync");
  const tsHttpCount = countTruthy(sample.map((r) => r.sideEffects || {}), "hasNetworkCall");

  const parity = {
    source_count_match: sample.length === rustSourceCount,
    import_edges_match: tsImportEdges === rustImportEdges,
    call_edges_match: tsCallEdges === rustCallEdges,
    async_count_match: tsAsyncCount === rustAsyncCount,
    network_count_match: tsHttpCount === rustHttpCount,
  };
  const requiredParity = {
    source_count_match: parity.source_count_match,
    import_edges_match: parity.import_edges_match,
    async_count_match: parity.async_count_match,
    network_count_match: parity.network_count_match,
  };
  const pass = Object.values(requiredParity).every(Boolean) && rustFailures === 0;

  const payload = {
    status: pass ? "pass" : "fail",
    scope: {
      project_root: projectRoot,
      sampled_files: sample.length,
    },
    ts_metrics: {
      source_count: sample.length,
      import_edges: tsImportEdges,
      call_edges: tsCallEdges,
      async_count: tsAsyncCount,
      network_count: tsHttpCount,
      loc_lines_sum: sum(sample, "locLines"),
    },
    rust_metrics: {
      source_count: rustSourceCount,
      import_edges: rustImportEdges,
      call_edges: rustCallEdges,
      async_count: rustAsyncCount,
      network_count: rustHttpCount,
      extract_failures: rustFailures,
    },
    parity,
    required_parity: requiredParity,
  };
  writeJson(outPath, payload);

  if (!pass && args.strict) {
    process.exit(2);
  }
}

main();
