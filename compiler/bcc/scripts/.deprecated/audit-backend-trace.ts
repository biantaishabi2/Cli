#!/usr/bin/env node

import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";

type Command = "status" | "report" | "seed";

type Options = {
  command: Command;
  writeArtifacts: boolean;
  seedLimit: number;
  writeSeed: boolean;
  only: "all" | "missing";
  sort: "path" | "module";
};

type FileRecord = {
  sourcePath: string;
  docPath: string;
  hasDoc: boolean;
  hasIntentSection: boolean;
  hasBehaviorSection: boolean;
  hasConstraintSection: boolean;
  hasIOSection: boolean;
  hasCallChainSection: boolean;
  hasStateSection: boolean;
  missingSections: string[];
};

type AuditResult = {
  rootDir: string;
  scope: {
    totalSourceFiles: number;
    totalDocFiles: number;
    docsInScope: number;
    missingDocFiles: number;
    extraDocFiles: number;
  };
  missingSections: {
    intent: number;
    behavior: number;
    constraint: number;
  };
  records: FileRecord[];
  missingDocList: string[];
  extraDocList: string[];
};

const DEFAULT_LIMIT = 200;
const SOURCE_EXT = ".ts";

function parseOptions(argv: string[]): Options {
  const opts: Options = {
    command: "status",
    writeArtifacts: false,
    seedLimit: DEFAULT_LIMIT,
    writeSeed: false,
    only: "all",
    sort: "module",
  };

  const positional = new Set(["status", "report", "seed"]);

  for (const arg of argv) {
    if (arg === "--write" || arg === "--write-artifacts") {
      opts.writeArtifacts = true;
      continue;
    }

    if (arg === "--write-seed") {
      opts.writeSeed = true;
      continue;
    }

    if (arg.startsWith("--max=")) {
      const n = Number(arg.slice("--max=".length));
      if (!Number.isNaN(n) && n >= 0) {
        opts.seedLimit = n;
      }
      continue;
    }

    if (arg.startsWith("--seed=")) {
      const n = Number(arg.slice("--seed=".length));
      if (!Number.isNaN(n) && n >= 0) {
        opts.seedLimit = n;
      }
      continue;
    }

    if (arg === "--only=missing") {
      opts.only = "missing";
      continue;
    }

    if (arg === "--sort=path") {
      opts.sort = "path";
      continue;
    }

    if (arg === "--sort=module") {
      opts.sort = "module";
      continue;
    }

    if (positional.has(arg)) {
      opts.command = arg as Command;
    }
  }

  return opts;
}

function isBackendSource(file: string): boolean {
  if (!file.endsWith(SOURCE_EXT)) return false;
  if (file.endsWith(".test.ts") || file.endsWith(".spec.ts")) return false;
  if (file.includes(`${path.sep}__tests__${path.sep}`)) return false;
  return true;
}

async function collectSourceFiles(root: string): Promise<string[]> {
  const result: string[] = [];
  const stack = [root];

  while (stack.length > 0) {
    const dir = stack.pop()!;
    const items = await readdir(dir, { withFileTypes: true });

    for (const item of items) {
      const full = path.join(dir, item.name);
      if (item.isDirectory()) {
        stack.push(full);
        continue;
      }

      const rel = path.relative(process.cwd(), full).replaceAll(path.sep, "/");
      if (isBackendSource(rel)) {
        result.push(rel);
      }
    }
  }

  result.sort();
  return result;
}

function normalizeHeading(heading: string): string {
  return heading
    .replace(/[\s:：#\-]/g, "")
    .replace(/[（）()]/g, "")
    .toLowerCase();
}

function hasSection(content: string, candidates: string[]): boolean {
  const headings: string[] = [];
  const re = /^##\s+(.*?)\s*$/gm;
  let match: RegExpExecArray | null;

  while ((match = re.exec(content)) !== null) {
    headings.push(normalizeHeading(match[1] ?? ""));
  }

  const normalized = candidates.map((candidate) => normalizeHeading(candidate));
  return headings.some((heading) => normalized.includes(heading));
}

async function parseDocHeadings(docPath: string): Promise<Pick<
  FileRecord,
  "hasIntentSection" | "hasBehaviorSection" | "hasConstraintSection" | "hasIOSection" | "hasCallChainSection" | "hasStateSection"
>> {
  const body = await readFile(docPath, "utf8");

  const hasIntentSection = hasSection(body, ["职责", "意图", "行为意图", "行为目标"]);
  const hasBehaviorSection = hasSection(body, ["行为", "行为流程", "行为说明", "行为模式"]);
  const hasConstraintSection = hasSection(body, ["约束", "限制", "边界", "约束条件", "约束边界"]);
  const hasIOSection = hasSection(body, ["输入输出", "输入与输出", "输入输出信息", "输入/输出"]);
  const hasCallChainSection = hasSection(body, ["调用链", "调用链位置", "调用关系", "路由链路", "依赖链路"]);
  const hasStateSection = hasSection(body, ["状态", "副作用", "状态与副作用", "副作用与持久化"]);

  return {
    hasIntentSection,
    hasBehaviorSection,
    hasConstraintSection,
    hasIOSection,
    hasCallChainSection,
    hasStateSection,
  };
}

function missingSectionNames(
  record: Pick<
    FileRecord,
    "hasIntentSection" | "hasBehaviorSection" | "hasConstraintSection" | "hasIOSection" | "hasCallChainSection" | "hasStateSection"
  >
): string[] {
  const entries: Array<[string, boolean]> = [
    ["行为意图", record.hasIntentSection],
    ["行为", record.hasBehaviorSection],
    ["约束", record.hasConstraintSection],
    ["输入输出", record.hasIOSection],
    ["调用链", record.hasCallChainSection],
    ["状态与副作用", record.hasStateSection],
  ];
  return entries.filter(([, exists]) => !exists).map(([name]) => name);
}

async function collectDocs(docsRoot: string): Promise<string[]> {
  const files: string[] = [];
  const stack = [docsRoot];

  while (stack.length > 0) {
    const dir = stack.pop()!;
    const entries = await readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        stack.push(full);
        continue;
      }

      if (entry.name.endsWith(".md")) {
        files.push(full);
      }
    }
  }

  return files;
}

function toSourcePathFromDoc(docPath: string, docsRoot: string): string {
  return path
    .relative(path.join(docsRoot, "files"), docPath)
    .replaceAll(path.sep, "/")
    .replace(/\.md$/i, "");
}

function docPathForSource(source: string, docsRoot: string): string {
  return path.join(docsRoot, "files", `${source}.md`);
}

function groupByModule(files: string[]): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const file of files) {
    const rel = file.replace(/^src\//, "");
    const root = rel.includes("/") ? rel.split("/")[0] : "(root)";
    const bucket = map.get(root) ?? [];
    bucket.push(file);
    map.set(root, bucket);
  }

  return new Map([...map.entries()].sort((a, b) => a[0].localeCompare(b[0])));
}

async function runAudit(): Promise<AuditResult> {
  const rootDir = process.cwd();
  const srcRoot = path.join(rootDir, "src");
  const docsRoot = path.join(rootDir, "docs", "backend-trace");
  const docsFilesRoot = path.join(docsRoot, "files");

  const sourceFiles = await collectSourceFiles(srcRoot);
  const docFiles = await collectDocs(docsFilesRoot);

  const sourceSet = new Set(sourceFiles);

  const records: FileRecord[] = [];
  const missingDocList: string[] = [];

  for (const source of sourceFiles) {
    const docPath = docPathForSource(source, docsRoot);
    const hasDoc = existsSync(docPath);
    let parsed = {
      hasIntentSection: false,
      hasBehaviorSection: false,
      hasConstraintSection: false,
      hasIOSection: false,
      hasCallChainSection: false,
      hasStateSection: false,
    };

    if (hasDoc) {
      parsed = await parseDocHeadings(docPath);
    } else {
      missingDocList.push(source);
    }
    const missingSections = hasDoc ? missingSectionNames(parsed) : ["缺失文档"];

    records.push({
      sourcePath: source,
      docPath,
      hasDoc,
      ...parsed,
      missingSections,
    });
  }

  const extraDocList: string[] = [];
  for (const docFile of docFiles) {
    const rel = toSourcePathFromDoc(docFile, docsRoot);
    if (rel.endsWith(".md")) continue;

    if (!sourceSet.has(rel)) {
      if (rel.startsWith("src/")) {
        extraDocList.push(rel);
      } else {
        extraDocList.push(`src/${rel}`);
      }
    }
  }

  const documented = records.filter((r) => r.hasDoc);
  const missingSections = {
    intent: documented.filter((r) => !r.hasIntentSection).length,
    behavior: documented.filter((r) => !r.hasBehaviorSection).length,
    constraint: documented.filter((r) => !r.hasConstraintSection).length,
  };

  return {
    rootDir,
    scope: {
      totalSourceFiles: sourceFiles.length,
      totalDocFiles: docFiles.length,
      docsInScope: documented.length,
      missingDocFiles: missingDocList.length,
      extraDocFiles: extraDocList.length,
    },
    missingSections,
    records,
    missingDocList,
    extraDocList,
  };
}

function buildMarkdownReport(result: AuditResult): string {
  const missingDocsInModules = groupByModule(result.missingDocList);

  const missingByIntent = result.records
    .filter((r) => r.hasDoc && !r.hasIntentSection)
    .map((r) => `- \`${r.sourcePath}\``);
  const missingByBehavior = result.records
    .filter((r) => r.hasDoc && !r.hasBehaviorSection)
    .map((r) => `- \`${r.sourcePath}\``);
  const missingByConstraint = result.records
    .filter((r) => r.hasDoc && !r.hasConstraintSection)
    .map((r) => `- \`${r.sourcePath}\``);

  const moduleBlocks: string[] = [];
  for (const [moduleName, files] of missingDocsInModules.entries()) {
    const rows = files.map((item) => `  - \`${item}\``).join("\n");
    moduleBlocks.push(`### ${moduleName}\n${rows}`);
  }

  const extraDocs = result.extraDocList
    .sort()
    .slice(0, 200)
    .map((f) => `- \`${f}\``);

  const alignmentRows: string[] = [
    "| 源文件 | 文档路径 | 缺失章节 |",
    "| --- | --- | --- |",
  ];
  for (const record of result.records.sort((a, b) => a.sourcePath.localeCompare(b.sourcePath))) {
    const missing = record.missingSections.length === 0 ? "无" : record.missingSections.join("、");
    const docName = record.hasDoc ? `\`${record.docPath}\`` : "（未建）";
    alignmentRows.push(`| \`${record.sourcePath}\` | ${docName} | ${missing} |`);
  }

  return `# 后端文档反推审计报告\n\n` +
    `- 生成时间：${new Date().toISOString()}\n` +
    `- 非测试\`.ts\` 源文件：${result.scope.totalSourceFiles}\n` +
    `- docs/backend-trace/files 有效覆盖：${result.scope.docsInScope}\n` +
    `- 缺文档文件：${result.scope.missingDocFiles}\n` +
    `- Scope 外冗余文档：${result.scope.extraDocFiles}\n\n` +
    `## 行为意图/约束结构检查\n` +
    `- 缺“行为意图/职责”章节：${result.missingSections.intent}\n` +
    `- 缺“行为”章节：${result.missingSections.behavior}\n` +
    `- 缺“约束”章节：${result.missingSections.constraint}\n\n` +
    `## 缺文档清单（按模块）\n${moduleBlocks.join("\n\n") || "- 无"}\n\n` +
    `## 行为意图章节缺失（前 200）\n${missingByIntent.slice(0, 200).join("\n") || "- 无"}\n\n` +
    `## 行为章节缺失（前 200）\n${missingByBehavior.slice(0, 200).join("\n") || "- 无"}\n\n` +
    `## 约束章节缺失（前 200）\n${missingByConstraint.slice(0, 200).join("\n") || "- 无"}\n\n` +
    `## Scope 外冗余文档（前 200）\n${extraDocs.join("\n") || "- 无"}\n\n` +
    `## 文档齐套性（按源文件）\n${alignmentRows.join("\n")}\n`;
}

function writeSeedTemplate(source: string): string {
  return `# ${source}\n\n` +
    `## 行为意图\n` +
    `- 用一句话说明该文件要解决的目标行为。\n\n` +
    `## 行为\n` +
    `- 输入参数：\n` +
    `- 处理流程：\n` +
    `- 输出结果：\n\n` +
    `## 约束\n` +
    `- 安全/权限约束：\n` +
    `- 兼容性约束：\n` +
    `- 运行边界：\n\n` +
    `## 输入输出\n` +
    `- 输入：\n` +
    `- 输出：\n\n` +
    `## 调用链位置\n` +
    `- 上游：\n` +
    `- 下游：\n\n` +
    `## 状态与副作用\n` +
    `- 读写状态：\n` +
    `- 副作用：\n\n` +
    `## 异常处理\n` +
    `- 异常场景：\n` +
    `- 降级行为：\n\n` +
    `## 与 PRD 需求映射\n` +
    `- 待补充\n`;
}

function sortFiles(files: string[], mode: "path" | "module"): string[] {
  if (mode === "path") return [...files].sort();
  return [...files].sort((a, b) => {
    const ma = a.replace(/^src\//, "").split("/")[0];
    const mb = b.replace(/^src\//, "").split("/")[0];
    if (ma === mb) return a.localeCompare(b);
    return ma.localeCompare(mb);
  });
}

async function writeSeedDocs(result: AuditResult, opts: Options) {
  const docsRoot = path.join(result.rootDir, "docs", "backend-trace");
  const targets = sortFiles(result.missingDocList, opts.sort).slice(0, opts.seedLimit);

  const created: string[] = [];
  for (const source of targets) {
    const docPath = path.join(docsRoot, "files", `${source}.md`);
    if (existsSync(docPath)) continue;

    await mkdir(path.dirname(docPath), { recursive: true });
    await writeFile(docPath, writeSeedTemplate(source), "utf8");
    created.push(path.relative(result.rootDir, docPath));
  }

  if (created.length === 0) {
    process.stdout.write("没有生成草稿（目标已存在或缺口为 0）。\n");
    return;
  }

  process.stdout.write(`已生成 ${created.length} 个草稿文档：\n`);
  for (const file of created) {
    process.stdout.write(`- ${file}\n`);
  }
}

function buildTsvRows(result: AuditResult): string {
  const header = "source\thasDoc\thasIntent\thasBehavior\thasConstraint\thasIO\thasCallChain\thasState\tmissingSections";
  const rows = result.records.map((record) =>
    [
      record.sourcePath,
      String(record.hasDoc),
      String(record.hasIntentSection),
      String(record.hasBehaviorSection),
      String(record.hasConstraintSection),
      String(record.hasIOSection),
      String(record.hasCallChainSection),
      String(record.hasStateSection),
      record.missingSections.join(","),
    ].join("\t")
  );
  return `${header}\n${rows.join("\n")}\n`;
}

(async () => {
  const opts = parseOptions(process.argv.slice(2));
  const result = await runAudit();

  if (opts.command === "status") {
    process.stdout.write(`后端源文件: ${result.scope.totalSourceFiles}\n`);
    process.stdout.write(`有文档: ${result.scope.docsInScope}\n`);
    process.stdout.write(`缺口: ${result.scope.missingDocFiles}\n`);
    process.stdout.write(`行为意图口径缺口: ${result.missingSections.intent}\n`);
    process.stdout.write(`行为口径缺口: ${result.missingSections.behavior}\n`);
    process.stdout.write(`约束口径缺口: ${result.missingSections.constraint}\n`);
    process.stdout.write(`scope 外冗余文档: ${result.scope.extraDocFiles}\n`);
    return;
  }

  if (opts.command === "seed") {
    if (!opts.writeSeed) {
      const preview = sortFiles(result.missingDocList, opts.sort).slice(0, Math.min(20, opts.seedLimit));
      process.stdout.write(`计划生成 ${preview.length} 份草稿（可用 --write-seed 真正落盘）：\n`);
      for (const line of preview) {
        process.stdout.write(`- ${line}\n`);
      }
      return;
    }

    await writeSeedDocs(result, opts);
    return;
  }

  if (opts.command === "report" || opts.writeArtifacts || opts.only === "missing") {
    const docsRoot = path.join(result.rootDir, "docs", "backend-trace", "artifacts");
    await mkdir(docsRoot, { recursive: true });

    await writeFile(
      path.join(docsRoot, "backend-docs-audit.json"),
      JSON.stringify(result, null, 2),
      "utf8"
    );

    await writeFile(
      path.join(docsRoot, "backend-docs-audit.md"),
      buildMarkdownReport(result),
      "utf8"
    );

    await writeFile(path.join(docsRoot, "backend-docs-audit-missing.tsv"), buildTsvRows(result), "utf8");

    if (opts.only === "missing") {
      const list = sortFiles(result.missingDocList, opts.sort);
      process.stdout.write(`缺文档文件: ${list.length}\n`);
      for (const item of list.slice(0, 200)) {
        process.stdout.write(`- ${item}\n`);
      }
      if (list.length > 200) {
        process.stdout.write(`... 省略 ${list.length - 200} 条\n`);
      }
      return;
    }

    process.stdout.write("审计已输出：\n");
    process.stdout.write(`- docs/backend-trace/artifacts/backend-docs-audit.json\n`);
    process.stdout.write(`- docs/backend-trace/artifacts/backend-docs-audit.md\n`);
    process.stdout.write(`- docs/backend-trace/artifacts/backend-docs-audit-missing.tsv\n`);
    return;
  }
})();
