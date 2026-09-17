#!/usr/bin/env node

/**
 * Frontend architecture debt gate.
 *
 * The current baseline is intentionally empty. Any new hard-coded dependency
 * fails CI and must be moved to its canonical contract or presentation catalog.
 *
 * This is intentionally a source architecture check, not a style linter. ESLint
 * remains responsible for syntax and React correctness.
 */

import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import ts from "typescript";

const ROOT = process.cwd();
const BASELINE_PATH = path.join(
  ROOT,
  "architecture/frontend-architecture-baseline.json",
);

const SOURCE_ROOTS = [
  "app",
  "components",
  "config",
  "constants",
  "contexts",
  "contracts",
  "hooks",
  "lib",
  "messages",
  "modules",
  "services",
  "types",
];

const API_CONTRACT_ROOT = "constants/api/";
const API_BOUNDARY_FILES = new Set(["lib/api/client.ts"]);
const MESSAGE_CATALOG_FILES = new Set(["constants/text.ts"]);
const MESSAGE_CATALOG_ROOTS = ["messages/"];
const COLOR_TOKEN_ROOTS = ["theme/", "tokens/"];

const VISIBLE_ATTRIBUTE_NAMES = new Set([
  "alt",
  "aria-label",
  "label",
  "placeholder",
  "title",
]);
const VISIBLE_PROPERTY_NAMES = new Set([
  "description",
  "label",
  "placeholder",
  "subtitle",
  "title",
]);
const PRESENTATION_CALL_NAMES = new Set([
  "error",
  "info",
  "setError",
  "success",
  "toast",
  "warning",
]);

const RULES = Object.freeze({
  AUTH_BROWSER_STORAGE: "auth-browser-storage",
  DIRECT_API_ENDPOINT: "direct-api-endpoint",
  DIRECT_FETCH: "direct-fetch",
  EMOJI_ICON: "emoji-icon",
  PERMISSION_BITMASK: "permission-bitmask",
  RAW_FEATURE_COLOR: "raw-feature-color",
  SSE_TRANSPORT: "sse-transport",
  VISIBLE_TEXT_LITERAL: "visible-text-literal",
});

const RAW_COLOR_PATTERN =
  /#[\da-f]{3,8}\b|(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\([^)]*\)|(?:bg|border|decoration|fill|from|outline|ring|shadow|stroke|text|to|via)-\[(?:[^\]]*#|[^\]]*(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\()/i;
const EMOJI_PATTERN = /\p{Extended_Pictographic}/u;
const AUTH_STORAGE_KEY_PATTERN =
  /access|auth|bearer|credential|jwt|refresh|session|token/i;
const BITWISE_OPERATORS = new Set([
  ts.SyntaxKind.AmpersandToken,
  ts.SyntaxKind.BarToken,
  ts.SyntaxKind.CaretToken,
  ts.SyntaxKind.LessThanLessThanToken,
  ts.SyntaxKind.GreaterThanGreaterThanToken,
  ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken,
]);

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function relativePath(filePath) {
  return toPosix(path.relative(ROOT, filePath));
}

function isUnder(file, roots) {
  return roots.some((root) => file.startsWith(root));
}

function isScannableFile(file) {
  return (
    (file.endsWith(".ts") || file.endsWith(".tsx") || file.endsWith(".css")) &&
    !file.endsWith(".d.ts") &&
    !/\.(?:spec|test)\.[cm]?[jt]sx?$/.test(file) &&
    !file.includes("/__tests__/") &&
    !file.includes("/__generated__/")
  );
}

async function collectFiles(directory) {
  const absolute = path.join(ROOT, directory);
  let entries;
  try {
    entries = await readdir(absolute, { withFileTypes: true });
  } catch (error) {
    if (error?.code === "ENOENT") return [];
    throw error;
  }

  const files = [];
  for (const entry of entries) {
    if (entry.name === "node_modules" || entry.name === ".next") continue;
    const nested = path.join(absolute, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectFiles(relativePath(nested))));
    } else if (isScannableFile(relativePath(nested))) {
      files.push(nested);
    }
  }
  return files;
}

function normalizeEvidence(value) {
  return value.replace(/\s+/g, " ").trim().slice(0, 240);
}

function displayText(value) {
  return value.replace(/\s+/g, " ").trim();
}

function containsVisibleLanguage(value) {
  const text = displayText(value);
  if (!text || text.length === 1) return false;
  if (/^(?:https?:\/\/|\/|\.|\.\.\.|—|–|-|\||\d[\d.,:%+\-]*)$/.test(text)) {
    return false;
  }
  return /\p{L}/u.test(text);
}

function isMessageKey(value) {
  return /^[A-Z][A-Z\d_]*(?:\.[A-Z][A-Z\d_]*)+$/.test(value.trim());
}

function literalValue(node) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text;
  }
  if (ts.isTemplateExpression(node)) {
    return [node.head.text, ...node.templateSpans.map((span) => span.literal.text)].join("");
  }
  return null;
}

function propertyNameText(name) {
  if (!name) return "";
  if (ts.isIdentifier(name) || ts.isStringLiteral(name)) return name.text;
  return name.getText();
}

function callName(expression) {
  if (ts.isIdentifier(expression)) return expression.text;
  if (ts.isPropertyAccessExpression(expression)) return expression.name.text;
  return "";
}

function isInsideNamedProperty(node, propertyName) {
  let current = node.parent;
  while (current && !ts.isStatement(current)) {
    if (
      ts.isPropertyAssignment(current) &&
      propertyNameText(current.name) === propertyName
    ) {
      return true;
    }
    current = current.parent;
  }
  return false;
}

function isSchemaDiagnosticProperty(node) {
  if (!ts.isPropertyAssignment(node) || propertyNameText(node.name) !== "label") {
    return false;
  }
  const object = node.parent;
  const call = object?.parent;
  return (
    ts.isObjectLiteralExpression(object) &&
    ts.isCallExpression(call) &&
    /^(?:bigint|integer|number|string)Schema$/.test(callName(call.expression))
  );
}

function isMessageCatalog(file) {
  return MESSAGE_CATALOG_FILES.has(file) || isUnder(file, MESSAGE_CATALOG_ROOTS);
}

function isColorTokenFile(file) {
  return isUnder(file, COLOR_TOKEN_ROOTS);
}

function unwrapExpression(node) {
  let current = node;
  while (
    current &&
    (ts.isAsExpression(current) ||
      ts.isParenthesizedExpression(current) ||
      ts.isSatisfiesExpression(current) ||
      ts.isTypeAssertionExpression(current))
  ) {
    current = current.expression;
  }
  return current;
}

function isLiteralNode(node) {
  return (
    ts.isStringLiteral(node) ||
    ts.isNoSubstitutionTemplateLiteral(node) ||
    ts.isTemplateExpression(node)
  );
}

function firstPathSegment(value) {
  const candidate = value.trim().replace(/^["'`]/, "");
  if (candidate.startsWith("//")) return null;
  const match =
    candidate.match(/^\/([a-z][a-z\d-]*)/i) ??
    candidate.match(/}\/[\s]*([a-z][a-z\d-]*)/i);
  return match ? `/${match[1]}` : null;
}

async function discoverApiPrefixes(files) {
  const prefixes = new Set();
  for (const filePath of files) {
    const file = relativePath(filePath);
    if (!file.startsWith(API_CONTRACT_ROOT) || file.endsWith(".css")) continue;
    const source = await readFile(filePath, "utf8");
    const sourceFile = ts.createSourceFile(
      file,
      source,
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TS,
    );

    function visit(node) {
      if (isLiteralNode(node)) {
        const segment = firstPathSegment(literalValue(node) ?? "");
        if (segment) prefixes.add(segment);
      }
      ts.forEachChild(node, visit);
    }
    visit(sourceFile);
  }
  return prefixes;
}

async function scanStyle(filePath) {
  const file = relativePath(filePath);
  if (file === "app/globals.css" || isColorTokenFile(file)) return [];
  const source = await readFile(filePath, "utf8");
  const violations = [];
  for (const [index, line] of source.split(/\r?\n/).entries()) {
    if (!RAW_COLOR_PATTERN.test(line)) continue;
    violations.push({
      rule: RULES.RAW_FEATURE_COLOR,
      file,
      line: index + 1,
      column: Math.max(1, line.search(/#|(?:rgba?|hsla?|oklch?)\(/i) + 1),
      evidence: normalizeEvidence(line),
    });
  }
  return violations;
}

function isPermissionContext(file, node, sourceFile) {
  if (/permission|roles\//i.test(file)) return true;
  const start = Math.max(0, node.getStart(sourceFile) - 180);
  const end = Math.min(sourceFile.getFullText().length, node.getEnd() + 180);
  return /permission|scope/i.test(sourceFile.getFullText().slice(start, end));
}

function apiEndpointContext(node, sourceFile) {
  let current = node;
  while (current.parent) {
    const parent = current.parent;
    if (ts.isVariableDeclaration(parent) && parent.initializer) {
      const name = parent.name.getText(sourceFile);
      if (/(?:api|endpoint|request|socket|stream)/i.test(name)) return "strong";
      if (/url/i.test(name)) return "url";
    }
    if (ts.isPropertyAssignment(parent)) {
      const name = propertyNameText(parent.name);
      if (/(?:api|endpoint|request|socket|stream)/i.test(name)) return "strong";
      if (/url/i.test(name)) return "url";
    }
    if (ts.isCallExpression(parent) || ts.isNewExpression(parent)) {
      const expression = parent.expression;
      const name = callName(expression);
      if (
        name === "fetch" ||
        name === "api" ||
        name === "EventSource" ||
        name === "WebSocket"
      ) {
        return "strong";
      }
    }
    if (ts.isStatement(parent)) break;
    current = parent;
  }
  return null;
}

function scanSource(filePath, apiPrefixes) {
  const file = relativePath(filePath);
  return readFile(filePath, "utf8").then((source) => {
    const sourceFile = ts.createSourceFile(
      file,
      source,
      ts.ScriptTarget.Latest,
      true,
      file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
    );
    const violations = [];
    const endpointNodes = new WeakSet();

    function add(rule, node, evidence = node.getText(sourceFile)) {
      const position = sourceFile.getLineAndCharacterOfPosition(
        node.getStart(sourceFile),
      );
      violations.push({
        rule,
        file,
        line: position.line + 1,
        column: position.character + 1,
        evidence: normalizeEvidence(evidence),
      });
    }

    function addVisibleLiteral(node, value) {
      if (
        !isMessageCatalog(file) &&
        !isMessageKey(value) &&
        containsVisibleLanguage(value)
      ) {
        add(RULES.VISIBLE_TEXT_LITERAL, node, value);
      }
    }

    function visit(node) {
      if (ts.isCallExpression(node)) {
        const name = callName(node.expression);

        if (name === "fetch" && !API_BOUNDARY_FILES.has(file)) {
          add(RULES.DIRECT_FETCH, node.expression);
        }

        if (
          ts.isPropertyAccessExpression(node.expression) &&
          (node.expression.expression.getText(sourceFile) === "localStorage" ||
            node.expression.expression.getText(sourceFile) === "sessionStorage")
        ) {
          const key = node.arguments[0]?.getText(sourceFile) ?? "";
          if (AUTH_STORAGE_KEY_PATTERN.test(key)) {
            add(RULES.AUTH_BROWSER_STORAGE, node, key);
          }
        }

        if (
          !file.startsWith(API_CONTRACT_ROOT) &&
          name === "api" &&
          node.arguments[0] &&
          isLiteralNode(node.arguments[0]) &&
          firstPathSegment(literalValue(node.arguments[0]) ?? "")
        ) {
          add(RULES.DIRECT_API_ENDPOINT, node.arguments[0]);
          endpointNodes.add(node.arguments[0]);
        }

        if (
          !isMessageCatalog(file) &&
          PRESENTATION_CALL_NAMES.has(name) &&
          !(ts.isPropertyAccessExpression(node.expression) &&
            node.expression.expression.getText(sourceFile) === "console")
        ) {
          for (const argument of node.arguments) {
            const value = literalValue(argument, sourceFile);
            if (value !== null) addVisibleLiteral(argument, value);
          }
        }
      }

      if (
        ts.isNewExpression(node) &&
        node.expression.getText(sourceFile) === "EventSource"
      ) {
        add(RULES.SSE_TRANSPORT, node.expression);
      }
      if (
        ts.isTypeReferenceNode(node) &&
        node.typeName.getText(sourceFile) === "EventSource"
      ) {
        add(RULES.SSE_TRANSPORT, node.typeName);
      }

      if (isLiteralNode(node)) {
        const value = literalValue(node, sourceFile) ?? "";

        if (EMOJI_PATTERN.test(value)) {
          add(RULES.EMOJI_ICON, node, value);
        }

        if (!isColorTokenFile(file) && RAW_COLOR_PATTERN.test(value)) {
          add(RULES.RAW_FEATURE_COLOR, node, value);
        }

        if (
          !file.startsWith(API_CONTRACT_ROOT) &&
          !endpointNodes.has(node)
        ) {
          const context = apiEndpointContext(node, sourceFile);
          const segment = firstPathSegment(value);
          if (
            segment &&
            context &&
            (context === "strong" || apiPrefixes.has(segment))
          ) {
            add(RULES.DIRECT_API_ENDPOINT, node, value);
          }
        }
      }

      if (ts.isJsxText(node)) {
        addVisibleLiteral(node, node.getText(sourceFile));
      }

      if (ts.isJsxAttribute(node)) {
        const name = propertyNameText(node.name);
        if (VISIBLE_ATTRIBUTE_NAMES.has(name) && node.initializer) {
          if (ts.isStringLiteral(node.initializer)) {
            addVisibleLiteral(node.initializer, node.initializer.text);
          } else if (
            ts.isJsxExpression(node.initializer) &&
            node.initializer.expression &&
            isLiteralNode(node.initializer.expression)
          ) {
            const value = literalValue(node.initializer.expression, sourceFile);
            if (value !== null) addVisibleLiteral(node.initializer.expression, value);
          }
        }
      }

      if (
        ts.isJsxExpression(node) &&
        node.expression &&
        isLiteralNode(node.expression) &&
        !ts.isJsxAttribute(node.parent) &&
        !(
          ts.isJsxElement(node.parent) &&
          ["script", "style"].includes(
            node.parent.openingElement.tagName.getText(sourceFile),
          )
        )
      ) {
        const value = literalValue(node.expression, sourceFile);
        if (value !== null) addVisibleLiteral(node.expression, value);
      }

      if (
        ts.isJsxExpression(node) &&
        node.expression &&
        !ts.isJsxAttribute(node.parent) &&
        ts.isConditionalExpression(node.expression)
      ) {
        for (const branch of [
          node.expression.whenTrue,
          node.expression.whenFalse,
        ]) {
          const expression = unwrapExpression(branch);
          if (!isLiteralNode(expression)) continue;
          const value = literalValue(expression, sourceFile);
          if (value !== null) addVisibleLiteral(expression, value);
        }
      }

      if (
        ts.isParameter(node) &&
        node.initializer &&
        isLiteralNode(node.initializer) &&
        /(?:alt|label|message|placeholder|subtitle|title)$/i.test(
          propertyNameText(node.name),
        )
      ) {
        const value = literalValue(node.initializer, sourceFile);
        if (value !== null) addVisibleLiteral(node.initializer, value);
      }

      if (ts.isPropertyAssignment(node)) {
        const name = propertyNameText(node.name);
        if (
          VISIBLE_PROPERTY_NAMES.has(name) &&
          isLiteralNode(node.initializer) &&
          !isInsideNamedProperty(node, "classNames") &&
          !isSchemaDiagnosticProperty(node)
        ) {
          const value = literalValue(node.initializer, sourceFile);
          if (value !== null) addVisibleLiteral(node.initializer, value);
        }

        if (
          /^role_?id$/i.test(name) &&
          ts.isNumericLiteral(node.initializer) &&
          Number(node.initializer.text) > 0
        ) {
          add(RULES.PERMISSION_BITMASK, node, node.getText(sourceFile));
        }
      }

      if (
        (ts.isPropertySignature(node) ||
          ts.isPropertyDeclaration(node) ||
          ts.isParameter(node)) &&
        /^(?:permission|permissions|scope|scopes)$/i.test(
          propertyNameText(node.name),
        ) &&
        node.type?.kind === ts.SyntaxKind.NumberKeyword
      ) {
        add(RULES.PERMISSION_BITMASK, node);
      }

      if (
        ts.isVariableDeclaration(node) &&
        /^(?:[A-Z][A-Z\d]*_)*SCOPES?$/i.test(node.name.getText(sourceFile)) &&
        node.initializer &&
        ts.isArrayLiteralExpression(unwrapExpression(node.initializer)) &&
        /\b(?:1|2|4|8|16|32|64|128)\b/.test(node.initializer.getText(sourceFile))
      ) {
        add(RULES.PERMISSION_BITMASK, node);
      }

      if (
        ts.isBinaryExpression(node) &&
        BITWISE_OPERATORS.has(node.operatorToken.kind) &&
        isPermissionContext(file, node, sourceFile)
      ) {
        add(RULES.PERMISSION_BITMASK, node);
      }
      if (
        ts.isPrefixUnaryExpression(node) &&
        node.operator === ts.SyntaxKind.TildeToken &&
        isPermissionContext(file, node, sourceFile)
      ) {
        add(RULES.PERMISSION_BITMASK, node);
      }

      ts.forEachChild(node, visit);
    }

    visit(sourceFile);
    return violations;
  });
}

function scanFile(filePath, apiPrefixes) {
  return filePath.endsWith(".css")
    ? scanStyle(filePath)
    : scanSource(filePath, apiPrefixes);
}

function assignStableIds(violations) {
  const occurrences = new Map();
  return violations
    .sort(
      (a, b) =>
        a.file.localeCompare(b.file) ||
        a.line - b.line ||
        a.column - b.column ||
        a.rule.localeCompare(b.rule),
    )
    .map((violation) => {
      const digest = createHash("sha256")
        .update(violation.evidence)
        .digest("hex")
        .slice(0, 16);
      const base = `${violation.rule}:${violation.file}:${digest}`;
      const occurrence = (occurrences.get(base) ?? 0) + 1;
      occurrences.set(base, occurrence);
      return { ...violation, id: `${base}:${occurrence}` };
    });
}

function countBy(values, key) {
  const counts = new Map();
  for (const value of values) {
    const name = value[key];
    counts.set(name, (counts.get(name) ?? 0) + 1);
  }
  return Object.fromEntries([...counts.entries()].sort(([a], [b]) => a.localeCompare(b)));
}

async function loadBaseline() {
  try {
    const raw = await readFile(BASELINE_PATH, "utf8");
    const parsed = JSON.parse(raw);
    if (parsed.version !== 1 || !Array.isArray(parsed.allowed)) {
      throw new Error("unsupported baseline format");
    }
    return parsed;
  } catch (error) {
    if (error?.code === "ENOENT") return { version: 1, allowed: [] };
    throw new Error(`Cannot read architecture baseline: ${error.message}`);
  }
}

function baselineDocument(violations) {
  return {
    version: 1,
    policy:
      "Known debt only. New entries require explicit architecture review; remove entries as debt is fixed.",
    allowed: violations.map(({ id }) => id),
  };
}

function printViolation(violation) {
  process.stderr.write(
    `  ${violation.file}:${violation.line}:${violation.column} ` +
      `[${violation.rule}] ${violation.evidence}\n`,
  );
}

async function main() {
  const files = (
    await Promise.all(SOURCE_ROOTS.map((directory) => collectFiles(directory)))
  )
    .flat()
    .sort();
  const apiPrefixes = await discoverApiPrefixes(files);
  const violations = assignStableIds(
    (await Promise.all(files.map((file) => scanFile(file, apiPrefixes)))).flat(),
  );

  if (process.argv.includes("--print-baseline")) {
    process.stdout.write(`${JSON.stringify(baselineDocument(violations), null, 2)}\n`);
    return;
  }

  if (process.argv.includes("--print-inventory")) {
    process.stdout.write(
      `${JSON.stringify(
        {
          scanned_files: files.length,
          api_prefixes: [...apiPrefixes].sort(),
          total: violations.length,
          by_rule: countBy(violations, "rule"),
          by_file: countBy(violations, "file"),
          violations,
        },
        null,
        2,
      )}\n`,
    );
    return;
  }

  const baseline = await loadBaseline();
  const allowed = new Set(baseline.allowed);
  const current = new Set(violations.map(({ id }) => id));
  const introduced = violations.filter(({ id }) => !allowed.has(id));
  const resolved = baseline.allowed.filter((id) => !current.has(id));

  if (introduced.length > 0) {
    process.stderr.write(
      `Frontend architecture check failed: ${introduced.length} new violation(s).\n`,
    );
    for (const violation of introduced.slice(0, 80)) printViolation(violation);
    if (introduced.length > 80) {
      process.stderr.write(`  ...and ${introduced.length - 80} more.\n`);
    }
    process.stderr.write(
      "Move text/endpoints/colors into their catalog, use the API boundary, and use string capabilities.\n",
    );
    process.exitCode = 1;
    return;
  }

  const counts = countBy(violations, "rule");
  process.stdout.write(
    `Frontend architecture check passed (${files.length} files, ${violations.length} baselined violation(s)).\n`,
  );
  for (const [rule, count] of Object.entries(counts)) {
    process.stdout.write(`  ${rule}: ${count}\n`);
  }
  if (resolved.length > 0) {
    process.stdout.write(
      `  ${resolved.length} baseline entr${resolved.length === 1 ? "y is" : "ies are"} now resolved; remove ${
        resolved.length === 1 ? "it" : "them"
      } in the next debt cleanup.\n`,
    );
  }
}

await main();
