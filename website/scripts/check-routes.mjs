#!/usr/bin/env node

import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const ROOT = process.cwd();
const APP_ROOT = path.join(ROOT, "app");
const MATRIX_PATH = path.join(ROOT, "architecture/browser-matrix.json");
const SOURCE_ROOTS = [
  "app",
  "components",
  "constants",
  "contexts",
  "hooks",
  "lib",
  "modules",
  "services",
];

const ALLOWED_INTERVAL_FILES = new Set([
  "lib/realtime/websocket-client.ts",
  "modules/profile/recovery-contact-card.tsx",
]);
const MATRIX_AUDIENCES = new Set(["public", "anonymous", "authenticated", "admin"]);
const MATRIX_FIXTURES = new Set([
  "publicProblemId",
  "publicContestId",
  "publicMaterialSlug",
  "publicUsername",
  "adminProblemId",
  "adminContestId",
]);

function toPosix(value) {
  return value.split(path.sep).join("/");
}

async function collect(directory, predicate) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    if (entry.name === "node_modules" || entry.name === ".next") continue;
    const nested = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...(await collect(nested, predicate)));
    else if (predicate(nested)) files.push(nested);
  }
  return files;
}

function routeFromPage(pagePath) {
  const relative = toPosix(path.relative(APP_ROOT, path.dirname(pagePath)));
  const segments = relative
    .split("/")
    .filter(Boolean)
    .filter((segment) => !(segment.startsWith("(") && segment.endsWith(")")));
  return segments.length === 0 ? "/" : `/${segments.join("/")}`;
}

function lineNumber(source, offset) {
  return source.slice(0, offset).split(/\r?\n/).length;
}

function reportViolation(violations, file, source, match, reason) {
  violations.push(
    `${file}:${lineNumber(source, match.index ?? 0)}: ${reason}: ${match[0].trim()}`,
  );
}

async function verifySourceContracts() {
  const files = [];
  for (const root of SOURCE_ROOTS) {
    const absolute = path.join(ROOT, root);
    try {
      files.push(
        ...(await collect(absolute, (file) => /\.(?:ts|tsx|js|jsx)$/.test(file))),
      );
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
    }
  }

  const violations = [];
  let websocketConstructors = 0;
  for (const absolute of files) {
    const file = toPosix(path.relative(ROOT, absolute));
    const source = await readFile(absolute, "utf8");
    const rules = [
      {
        pattern: /\bEventSource\s*\(/g,
        reason: "SSE is forbidden; browser realtime uses the shared WebSocket",
      },
      {
        pattern: /\b(?:Number|parseInt)\s*\(\s*(?:params(?:\.|\[)|[^)]*\b(?:entity|problem|submission|contest|material|user|role)_?id\b)/gi,
        reason: "domain identifiers must remain UUID strings",
      },
      {
        pattern: /eslint-disable(?:-next-line|-line)?\s*(?:$|\*|eslint)/gm,
        reason: "blanket ESLint suppression is forbidden",
      },
      {
        pattern: /\b(?:repeat|animationIterationCount)\s*:\s*(?:Infinity|["']infinite["'])/g,
        reason: "unbounded authored animation is forbidden",
      },
      {
        pattern: /\bwhile\s*\(\s*true\s*\)/g,
        reason: "unbounded browser loop is forbidden",
      },
      {
        pattern: /(?:^|[/\\])(?:__mocks__|mocks?|fixtures?)(?:[/\\]|\.)/gi,
        reason: "runtime mock/fixture imports are forbidden in production source",
      },
    ];
    for (const rule of rules) {
      for (const match of source.matchAll(rule.pattern)) {
        reportViolation(violations, file, source, match, rule.reason);
      }
    }

    const constructorPattern = /\bnew\s+WebSocket\s*\(/g;
    const constructors = [...source.matchAll(constructorPattern)];
    websocketConstructors += constructors.length;
    if (
      constructors.length > 0 &&
      file !== "lib/realtime/websocket-client.ts"
    ) {
      for (const match of constructors) {
        reportViolation(
          violations,
          file,
          source,
          match,
          "feature code must not create another WebSocket",
        );
      }
    }

    if (!ALLOWED_INTERVAL_FILES.has(file)) {
      for (const match of source.matchAll(/\b(?:window\.)?setInterval\s*\(/g)) {
        reportViolation(
          violations,
          file,
          source,
          match,
          "timer polling is forbidden",
        );
      }
    }
  }

  if (websocketConstructors !== 1) {
    violations.push(
      `expected exactly one canonical WebSocket constructor, found ${websocketConstructors}`,
    );
  }
  return violations;
}

async function verifyBrowserTestContracts() {
  const e2eRoot = path.join(ROOT, "e2e");
  let files = [];
  try {
    files = await collect(e2eRoot, (file) => /\.(?:ts|tsx|js|jsx)$/.test(file));
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }

  const violations = [];
  const interceptionRules = [
    {
      pattern: /\b(?:page|context|browserContext)\.route\s*\(/g,
      reason: "browser acceptance must not intercept or fulfill application traffic",
    },
    {
      pattern: /\brouteFromHAR\s*\(/g,
      reason: "browser acceptance must use real services instead of HAR replay",
    },
    {
      pattern: /\b(?:mock|fixture)\s*(?:API|Server|Response)/gi,
      reason: "browser acceptance must not introduce a mock service boundary",
    },
  ];

  for (const absolute of files) {
    const file = toPosix(path.relative(ROOT, absolute));
    const source = await readFile(absolute, "utf8");
    for (const rule of interceptionRules) {
      for (const match of source.matchAll(rule.pattern)) {
        reportViolation(violations, file, source, match, rule.reason);
      }
    }
  }
  return violations;
}

const matrix = JSON.parse(await readFile(MATRIX_PATH, "utf8"));
if (matrix.version !== 1 || !Array.isArray(matrix.routes)) {
  throw new Error("browser matrix must use version 1 and contain a routes array");
}

const pageFiles = await collect(APP_ROOT, (file) => file.endsWith(`${path.sep}page.tsx`));
const discovered = [...new Set(pageFiles.map(routeFromPage))].sort();
const declared = matrix.routes.map((entry) => entry.route).sort();
const duplicateRoutes = declared.filter((route, index) => declared.indexOf(route) !== index);
const missing = discovered.filter((route) => !declared.includes(route));
const stale = declared.filter((route) => !discovered.includes(route));
const invalidEntries = matrix.routes.filter(
  (entry) =>
    typeof entry.route !== "string" ||
    !entry.route.startsWith("/") ||
    (entry.route.length > 1 && entry.route.endsWith("/")) ||
    typeof entry.audience !== "string" ||
    !MATRIX_AUDIENCES.has(entry.audience) ||
    (entry.route.includes("[") && !MATRIX_FIXTURES.has(entry.fixture)) ||
    (!entry.route.includes("[") && entry.fixture !== undefined) ||
    typeof entry.journey !== "string",
);
const invalidThemes =
  !Array.isArray(matrix.themes) ||
  matrix.themes.length !== 3 ||
  !["light", "dark", "system"].every((theme) => matrix.themes.includes(theme));
const invalidViewports = ["mobile", "tablet", "desktop", "wide"].filter(
  (name) =>
    !Number.isInteger(matrix.viewports?.[name]?.width) ||
    matrix.viewports[name].width <= 0 ||
    !Number.isInteger(matrix.viewports?.[name]?.height) ||
    matrix.viewports[name].height <= 0,
);
const sourceViolations = await verifySourceContracts();
const browserTestViolations = await verifyBrowserTestContracts();

const failures = [];
if (duplicateRoutes.length) failures.push(`duplicate routes: ${duplicateRoutes.join(", ")}`);
if (missing.length) failures.push(`routes missing from browser matrix: ${missing.join(", ")}`);
if (stale.length) failures.push(`stale browser-matrix routes: ${stale.join(", ")}`);
if (invalidEntries.length) failures.push("browser-matrix entries have an invalid route, audience, journey or fixture");
if (invalidThemes) failures.push("browser matrix must declare light, dark and system themes exactly once");
if (invalidViewports.length) failures.push(`browser matrix has invalid viewports: ${invalidViewports.join(", ")}`);
failures.push(...sourceViolations);
failures.push(...browserTestViolations);

if (failures.length) {
  console.error("Frontend route/acceptance contract failed:\n");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(
  `Frontend route/acceptance contract passed (${discovered.length} routes, one shared WebSocket).`,
);
