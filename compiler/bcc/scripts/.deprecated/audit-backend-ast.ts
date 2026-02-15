#!/usr/bin/env node

import { access, mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import ts from "typescript";

type Command = "status" | "snapshot";

type Options = {
  command: Command;
  writeArtifacts: boolean;
  examples: number;
};

type ImportBinding = {
  local: string;
  imported?: string;
  alias?: string;
  kind: "default" | "named" | "namespace";
};

type ImportRecord = {
  specifier: string;
  kind: "import" | "export-all" | "export-named";
  bindings: ImportBinding[];
  isLocal: boolean;
  resolvedPath?: string;
};

type ExportRecord = {
  name: string;
  kind:
    | "function"
    | "class"
    | "interface"
    | "type"
    | "enum"
    | "const"
    | "var"
    | "default"
    | "re-export";
  signature?: string;
  line?: number;
};

type CallRecord = {
  callee: string;
  line: number;
  calleeImport?: string;
};

type FileRecord = {
  sourcePath: string;
  locLines: number;
  imports: ImportRecord[];
  exports: ExportRecord[];
  declarations: {
    functionCount: number;
    classCount: number;
    interfaceCount: number;
    typeAliasCount: number;
    enumCount: number;
    constCount: number;
    varCount: number;
  };
  localDependencies: string[];
  calls: CallRecord[];
  localCallTargets: string[];
  sideEffects: {
    hasAsync: boolean;
    hasAwait: boolean;
    hasTopLevelReturn: boolean;
    hasSpawnCall: boolean;
    hasNetworkCall: boolean;
  };
};

type SummaryRecord = {
  rootDir: string;
  sourceRoot: string;
  sourceCount: number;
  generatedAt: string;
  records: FileRecord[];
  withDefaultExport: number;
  withTopLevelReturn: number;
  withAsync: number;
  withNetworkCall: number;
  withSpawnCall: number;
};

const SOURCE_EXT = ".ts";
const DEFAULT_EXAMPLES = 8;
const BACKEND_ROOT = "src";

function parseOptions(argv: string[]): Options {
  const opts: Options = {
    command: "status",
    writeArtifacts: false,
    examples: DEFAULT_EXAMPLES,
  };

  const positional = new Set<Command>(["status", "snapshot"]);

  for (const arg of argv) {
    if (arg === "status" || arg === "snapshot") {
      opts.command = arg;
      continue;
    }
    if (arg === "--write" || arg === "--write-artifacts") {
      opts.writeArtifacts = true;
      continue;
    }
    if (arg.startsWith("--examples=")) {
      const n = Number(arg.slice("--examples=".length));
      if (!Number.isNaN(n) && n >= 0) {
        opts.examples = Math.trunc(n);
      }
      continue;
    }
  }

  return opts;
}

function isBackendSource(file: string): boolean {
  if (!file.endsWith(SOURCE_EXT)) return false;
  if (file.endsWith(".test.ts") || file.endsWith(".spec.ts") || file.endsWith(".e2e.test.ts")) return false;
  if (file.includes(`${path.sep}__tests__${path.sep}`)) return false;
  return true;
}

function toPosix(filePath: string): string {
  return filePath.replaceAll(path.sep, "/");
}

async function collectSourceFiles(root: string): Promise<string[]> {
  const result: string[] = [];
  const stack: string[] = [root];

  while (stack.length > 0) {
    const dir = stack.pop()!;
    const entries = await readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        stack.push(full);
        continue;
      }
      if (!entry.isFile()) continue;

      const rel = path.relative(process.cwd(), full).replaceAll(path.sep, "/");
      if (isBackendSource(rel)) {
        result.push(rel);
      }
    }
  }

  result.sort();
  return result;
}

function declarationLine(source: ts.SourceFile, node: ts.Node): number {
  return source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1;
}

function hasExportModifier(node: ts.Node): boolean {
  const modifiers = ts.canHaveModifiers(node) ? ts.getModifiers(node) : undefined;
  return modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword) ?? false;
}

function hasAsyncModifier(node: ts.Node): boolean {
  const modifiers = ts.canHaveModifiers(node) ? ts.getModifiers(node) : undefined;
  return modifiers?.some((m) => m.kind === ts.SyntaxKind.AsyncKeyword) ?? false;
}

async function exists(filePath: string): Promise<boolean> {
  try {
    await access(filePath);
    return true;
  } catch {
    return false;
  }
}

async function resolveLocalImport(fromFile: string, specifier: string): Promise<string | undefined> {
  if (!specifier.startsWith(".")) return undefined;

  const root = process.cwd();
  const fromAbs = path.join(root, fromFile);
  const fromDir = path.dirname(fromAbs);
  const normalized = specifier.endsWith(".ts") || specifier.endsWith(".js") ? specifier.slice(0, -3) : specifier;
  const base = path.resolve(fromDir, normalized);

  const candidates = [
    `${base}.ts`,
    `${base}.tsx`,
    `${base}.mts`,
    `${base}.cts`,
    base,
    `${base}/index.ts`,
    `${base}/index.tsx`,
    `${base}/index.mts`,
    `${base}/index.cts`,
  ];

  for (const candidate of candidates) {
    if (!(await exists(candidate))) continue;
    const rel = toPosix(path.relative(root, candidate));
    if (!rel.startsWith(`${BACKEND_ROOT}/`)) continue;
    return rel;
  }

  return undefined;
}

function isNetworkTarget(text: string): boolean {
  return text === "fetch" || text.includes("axios") || text.includes("undici") || text.includes("request");
}

function isSpawnTarget(text: string): boolean {
  return (
    text === "spawn" ||
    text === "exec" ||
    text === "execFile" ||
    text === "spawnSync" ||
    text === "execSync" ||
    text === "fork"
  );
}

async function analyzeFile(sourcePath: string): Promise<FileRecord> {
  const content = await readFile(sourcePath, "utf8");
  const sourceFile = ts.createSourceFile(sourcePath, content, ts.ScriptTarget.ES2022, true, ts.ScriptKind.TS);

  const sourceAliasToSpecifier = new Map<string, string>();
  const imports: ImportRecord[] = [];

  const declarations = {
    functionCount: 0,
    classCount: 0,
    interfaceCount: 0,
    typeAliasCount: 0,
    enumCount: 0,
    constCount: 0,
    varCount: 0,
  };

  const exports: ExportRecord[] = [];
  const calls: CallRecord[] = [];
  const localDependencySet = new Set<string>();
  const localCallSymbols = new Set<string>();

  const sideEffects = {
    hasAsync: false,
    hasAwait: false,
    hasTopLevelReturn: false,
    hasSpawnCall: false,
    hasNetworkCall: false,
  };

  const visit = (node: ts.Node): void => {
    if (ts.isImportDeclaration(node)) {
      const moduleSpecifier = node.moduleSpecifier;
      if (!ts.isStringLiteral(moduleSpecifier)) return;

      const specifier = moduleSpecifier.text;
      const bindings: ImportBinding[] = [];
      const importRecord: ImportRecord = {
        specifier,
        kind: "import",
        bindings,
        isLocal: specifier.startsWith("."),
      };

      if (node.importClause) {
        if (node.importClause.name) {
          const binding: ImportBinding = {
            local: node.importClause.name.text,
            kind: "default",
          };
          bindings.push(binding);
          sourceAliasToSpecifier.set(binding.local, specifier);
        }
        const namedBindings = node.importClause.namedBindings;
        if (namedBindings) {
          if (ts.isNamespaceImport(namedBindings)) {
            const binding: ImportBinding = {
              local: namedBindings.name.text,
              kind: "namespace",
            };
            bindings.push(binding);
            sourceAliasToSpecifier.set(binding.local, specifier);
          }
          if (ts.isNamedImports(namedBindings)) {
            for (const element of namedBindings.elements) {
              const binding: ImportBinding = {
                local: element.name.text,
                imported: element.propertyName?.text,
                alias: element.propertyName ? element.name.text : undefined,
                kind: "named",
              };
              bindings.push(binding);
              sourceAliasToSpecifier.set(binding.local, specifier);
            }
          }
        }
      }

      imports.push(importRecord);
      return;
    }

    if (ts.isExportDeclaration(node)) {
      const moduleSpecifier = node.moduleSpecifier;
      if (!moduleSpecifier || !ts.isStringLiteral(moduleSpecifier)) {
        if (node.exportClause && ts.isNamedExports(node.exportClause)) {
          for (const element of node.exportClause.elements) {
            const exported = element.name.text;
            const imported = element.propertyName ? element.propertyName.text : undefined;
            exports.push({
              name: exported,
              kind: "re-export",
              signature: imported ? `${imported} as ${exported}` : exported,
              line: declarationLine(sourceFile, node),
            });
          }
        }
        return;
      }

      const specifier = moduleSpecifier.text;
      const exportKind = node.exportClause ? "export-named" : "export-all";

      if (!node.exportClause) {
        imports.push({
          specifier,
          kind: "export-all",
          bindings: [{ local: "*", kind: "named" }],
          isLocal: specifier.startsWith("."),
        });
        return;
      }

      if (ts.isNamedExports(node.exportClause)) {
        const bindings: ImportBinding[] = node.exportClause.elements.map((element) => ({
          local: element.name.text,
          imported: element.propertyName?.text,
          alias: element.propertyName ? element.name.text : undefined,
          kind: "named",
        }));

        imports.push({
          specifier,
          kind: "export-named",
          bindings,
          isLocal: specifier.startsWith("."),
        });

        for (const element of bindings) {
          exports.push({
            name: element.local,
            kind: "re-export",
            signature: element.imported ? `${element.imported} as ${element.local}` : element.local,
            line: declarationLine(sourceFile, node),
          });
        }
      }

      return;
    }

    if (ts.isExportAssignment(node)) {
      exports.push({
        name: "default",
        kind: "default",
        signature: node.expression.getText(sourceFile),
        line: declarationLine(sourceFile, node),
      });
      return;
    }

    if (ts.isFunctionDeclaration(node) && node.name && hasExportModifier(node)) {
      declarations.functionCount += 1;
      exports.push({
        name: node.name.text,
        kind: "function",
        signature: `function ${node.name.text}(${node.parameters.map((p) => p.name.getText(sourceFile)).join(", ")})`,
        line: declarationLine(sourceFile, node),
      });
      if (hasAsyncModifier(node)) sideEffects.hasAsync = true;
    }

    if (ts.isClassDeclaration(node) && node.name && hasExportModifier(node)) {
      declarations.classCount += 1;
      exports.push({
        name: node.name.text,
        kind: "class",
        signature: `class ${node.name.text}`,
        line: declarationLine(sourceFile, node),
      });
    }

    if (ts.isInterfaceDeclaration(node) && node.name && hasExportModifier(node)) {
      declarations.interfaceCount += 1;
      exports.push({
        name: node.name.text,
        kind: "interface",
        signature: `interface ${node.name.text}`,
        line: declarationLine(sourceFile, node),
      });
    }

    if (ts.isTypeAliasDeclaration(node) && hasExportModifier(node)) {
      declarations.typeAliasCount += 1;
      exports.push({
        name: node.name.text,
        kind: "type",
        signature: `type ${node.name.text}`,
        line: declarationLine(sourceFile, node),
      });
    }

    if (ts.isEnumDeclaration(node) && node.name && hasExportModifier(node)) {
      declarations.enumCount += 1;
      exports.push({
        name: node.name.text,
        kind: "enum",
        signature: `enum ${node.name.text}`,
        line: declarationLine(sourceFile, node),
      });
    }

    if (ts.isVariableStatement(node)) {
      if (hasExportModifier(node)) {
        const isConst = (node.declarationList.flags & ts.NodeFlags.Const) !== 0;
        const kind = isConst ? "const" : "var";
        for (const declaration of node.declarationList.declarations) {
          if (!ts.isIdentifier(declaration.name)) continue;
          if (isConst) {
            declarations.constCount += 1;
          } else {
            declarations.varCount += 1;
          }
          exports.push({
            name: declaration.name.text,
            kind,
            signature: `${kind} ${declaration.name.text}`,
            line: declarationLine(sourceFile, declaration.name),
          });
        }
      }
      return;
    }

    if (ts.isCallExpression(node)) {
      const line = declarationLine(sourceFile, node);
      const calleeText = node.expression.getText(sourceFile);
      const rootSymbol = (() => {
        if (ts.isIdentifier(node.expression)) return node.expression.text;
        if (ts.isPropertyAccessExpression(node.expression) && ts.isIdentifier(node.expression.expression)) {
          return node.expression.expression.text;
        }
        return undefined;
      })();

      const callImport = rootSymbol ? sourceAliasToSpecifier.get(rootSymbol) : undefined;
      if (callImport?.startsWith(".")) {
        localCallSymbols.add(rootSymbol);
      }

      calls.push({
        callee: calleeText,
        line,
        calleeImport: callImport,
      });

      const baseName = calleeText.split(".")[0] ?? "";
      if (isSpawnTarget(baseName)) sideEffects.hasSpawnCall = true;
      if (isNetworkTarget(baseName) || calleeText.startsWith("fetch(")) {
        sideEffects.hasNetworkCall = true;
      }
      return;
    }

    if (ts.isAwaitExpression(node)) {
      sideEffects.hasAwait = true;
      return;
    }

    if (ts.isReturnStatement(node) && node.parent?.kind === ts.SyntaxKind.SourceFile) {
      sideEffects.hasTopLevelReturn = true;
      return;
    }

    ts.forEachChild(node, visit);
  };

  ts.forEachChild(sourceFile, visit);

  for (const record of imports) {
    if (!record.isLocal) continue;
    const resolvedPath = await resolveLocalImport(sourcePath, record.specifier);
    if (resolvedPath) {
      record.resolvedPath = resolvedPath;
      localDependencySet.add(resolvedPath);
    }
  }

  const localCallTargets = new Set<string>();
  for (const symbol of localCallSymbols) {
    const specifier = sourceAliasToSpecifier.get(symbol);
    if (!specifier?.startsWith(".")) continue;
    const resolved = await resolveLocalImport(sourcePath, specifier);
    if (resolved) {
      localCallTargets.add(resolved);
    }
  }

  return {
    sourcePath: toPosix(sourcePath),
    locLines: sourceFile.getLineAndCharacterOfPosition(sourceFile.end).line + 1,
    imports,
    exports,
    declarations,
    localDependencies: Array.from(localDependencySet).sort(),
    calls: calls.slice(0, 120),
    localCallTargets: Array.from(localCallTargets).sort(),
    sideEffects,
  };
}

function buildMarkdownReport(result: SummaryRecord, examples: number): string {
  const topDependency = [...result.records]
    .filter((r) => r.localDependencies.length > 0)
    .sort((a, b) => b.localDependencies.length - a.localDependencies.length)
    .slice(0, examples);

  const topAsync = result.records.filter((r) => r.sideEffects.hasAsync).length;
  const topAwait = result.records.filter((r) => r.sideEffects.hasAwait).length;
  const topReturn = result.withTopLevelReturn;
  const topNetwork = result.withNetworkCall;
  const topSpawn = result.withSpawnCall;

  const lines: string[] = [
    "# AST 反推快照（静态结构）",
    "",
    `- 生成时间：${result.generatedAt}`,
    `- 源文件数：${result.sourceCount}`,
    `- 含默认导出文件：${result.withDefaultExport}`,
    `- 含 async 标记：${topAsync}`,
    `- 含 await：${topAwait}`,
    `- 含顶层 return：${topReturn}`,
    `- 含网络调用痕迹：${topNetwork}`,
    `- 含进程调用痕迹：${topSpawn}`,
    "",
    "## 局部依赖数 TopN",
  ];

  for (const item of topDependency) {
    lines.push(`- \`${item.sourcePath}\`（${item.localDependencies.length}）`);
    for (const dep of item.localDependencies.slice(0, 20)) {
      lines.push(`  - ${dep}`);
    }
  }

  if (topDependency.length === 0) {
    lines.push("- 无显式本地依赖");
  }

  lines.push("");
  lines.push("## 无导出文件（先补文档）");
  const noExport = result.records.filter((r) => r.exports.length === 0);
  for (const item of noExport.slice(0, 100)) {
    lines.push(`- \`${item.sourcePath}\``);
  }

  lines.push("");
  lines.push("## 大文件 TopN（按行）");
  const topLoc = [...result.records].sort((a, b) => b.locLines - a.locLines).slice(0, examples);
  for (const item of topLoc) {
    lines.push(`- \`${item.sourcePath}\` ${item.locLines}`);
  }

  return `${lines.join("\n")}\n`;
}

(async () => {
  const opts = parseOptions(process.argv.slice(2));

  const rootDir = process.cwd();
  const sourceRoot = path.join(rootDir, BACKEND_ROOT);
  const sourceFiles = await collectSourceFiles(sourceRoot);
  if (sourceFiles.length === 0) {
    console.error("未找到符合条件的 src 下 ts 文件");
    process.exit(1);
  }

  const records: FileRecord[] = [];
  for (const sourceFile of sourceFiles) {
    records.push(await analyzeFile(sourceFile));
  }

  const sorted = records.sort((a, b) => a.sourcePath.localeCompare(b.sourcePath));
  const result: SummaryRecord = {
    rootDir,
    sourceRoot: toPosix(BACKEND_ROOT),
    sourceCount: sorted.length,
    generatedAt: new Date().toISOString(),
    records: sorted,
    withDefaultExport: sorted.filter((r) => r.exports.some((e) => e.kind === "default")).length,
    withTopLevelReturn: sorted.filter((r) => r.sideEffects.hasTopLevelReturn).length,
    withAsync: sorted.filter((r) => r.sideEffects.hasAsync).length,
    withNetworkCall: sorted.filter((r) => r.sideEffects.hasNetworkCall).length,
    withSpawnCall: sorted.filter((r) => r.sideEffects.hasSpawnCall).length,
  };

  if (opts.command === "status" && !opts.writeArtifacts) {
    console.log(`AST 文件数：${result.sourceCount}`);
    console.log(`含默认导出：${result.withDefaultExport}`);
    console.log(`含 async：${result.withAsync}`);
    console.log(`含 await：${result.records.filter((r) => r.sideEffects.hasAwait).length}`);
    console.log(`含顶层 return：${result.withTopLevelReturn}`);
    console.log(`含网络行为：${result.withNetworkCall}`);
    console.log(`含进程行为：${result.withSpawnCall}`);
    return;
  }

  const artifactsDir = path.join(rootDir, "docs", "backend-trace", "artifacts");
  const jsonPath = path.join(artifactsDir, "backend-docs-ast.json");
  const mdPath = path.join(artifactsDir, "backend-docs-ast.md");

  await mkdir(artifactsDir, { recursive: true });
  await writeFile(jsonPath, JSON.stringify(result, null, 2), "utf8");
  await writeFile(mdPath, buildMarkdownReport(result, opts.examples), "utf8");

  console.log("已输出 AST 审计产物：");
  console.log(`- ${jsonPath}`);
  console.log(`- ${mdPath}`);
})();
