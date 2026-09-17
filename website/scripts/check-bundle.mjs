#!/usr/bin/env node

import { gzipSync } from "node:zlib";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const ROOT = process.cwd();
const NEXT_ROOT = path.join(ROOT, ".next");
const ROUTE_MANIFEST = path.join(NEXT_ROOT, "app-path-routes-manifest.json");
const BUILD_MANIFEST = path.join(NEXT_ROOT, "build-manifest.json");
const OUTPUT = path.join(ROOT, "test-results", "bundle-budget.json");
// Measured against the current stack. The framework floor alone — react-dom,
// the Next.js client runtime, the webpack runtime and main-app — is ~126 KB
// compressed, so this budget buys roughly 154 KB for product code. Lower it
// when a reduction lands; never raise it to absorb a regression.
const PUBLIC_ROUTE_BUDGET_BYTES = 280 * 1024;
const ORDINARY_PUBLIC_ROUTES = [
  "/",
  "/about",
  "/arena",
  "/contest",
  "/materials",
  "/ranking",
  "/login",
  "/register",
  "/forgot-password",
];

async function json(file) {
  try {
    return JSON.parse(await readFile(file, "utf8"));
  } catch (error) {
    if (error?.code === "ENOENT") {
      throw new Error(`missing production build artifact: ${path.relative(ROOT, file)}`);
    }
    throw error;
  }
}

function clientManifestPath(appPath) {
  const segment = appPath.replace(/^\//, "");
  const directory = segment === "page" ? "" : segment.replace(/\/page$/, "");
  return path.join(
    NEXT_ROOT,
    "server",
    "app",
    directory,
    "page_client-reference-manifest.js",
  );
}

async function clientManifest(appPath) {
  const file = clientManifestPath(appPath);
  const source = await readFile(file, "utf8");
  const marker = `__RSC_MANIFEST[${JSON.stringify(appPath)}]=`;
  const start = source.indexOf(marker);
  if (start < 0) throw new Error(`cannot parse ${path.relative(ROOT, file)}`);
  return JSON.parse(source.slice(start + marker.length).trim().replace(/;$/, ""));
}

function javascriptChunks(manifest) {
  const chunks = new Set();
  for (const clientModule of Object.values(manifest.clientModules ?? {})) {
    for (const chunk of clientModule.chunks ?? []) {
      if (typeof chunk === "string" && chunk.endsWith(".js")) chunks.add(chunk);
    }
  }
  return chunks;
}

async function chunkSize(relative) {
  const file = path.join(NEXT_ROOT, relative);
  const content = await readFile(file);
  return {
    file: relative,
    bytes: (await stat(file)).size,
    gzipBytes: gzipSync(content, { level: 9 }).byteLength,
  };
}

const routes = await json(ROUTE_MANIFEST);
const build = await json(BUILD_MANIFEST);
// Next.js serves polyfillFiles behind `nomodule`, so every browser this product
// supports skips them. They are still measured and reported below, but charging
// them to the budget would overstate what a real client downloads.
const polyfillChunks = new Set(build.polyfillFiles ?? []);
const rootChunks = new Set([...polyfillChunks, ...(build.rootMainFiles ?? [])]);
const results = [];
const failures = [];

for (const route of ORDINARY_PUBLIC_ROUTES) {
  const match = Object.entries(routes).find(([, publicRoute]) => publicRoute === route);
  if (!match) {
    failures.push(`${route}: route is absent from the production manifest`);
    continue;
  }
  const [appPath] = match;
  const manifest = await clientManifest(appPath);
  const chunks = new Set([...rootChunks, ...javascriptChunks(manifest)]);
  const sizes = await Promise.all([...chunks].sort().map(chunkSize));
  const budgeted = sizes.filter((item) => !polyfillChunks.has(item.file));
  const gzipBytes = budgeted.reduce((total, item) => total + item.gzipBytes, 0);
  const bytes = budgeted.reduce((total, item) => total + item.bytes, 0);
  const polyfillGzipBytes = sizes
    .filter((item) => polyfillChunks.has(item.file))
    .reduce((total, item) => total + item.gzipBytes, 0);
  results.push({
    route,
    budgetBytes: PUBLIC_ROUTE_BUDGET_BYTES,
    bytes,
    gzipBytes,
    polyfillGzipBytes,
    legacyGzipBytes: gzipBytes + polyfillGzipBytes,
    chunks: sizes,
  });
  if (gzipBytes > PUBLIC_ROUTE_BUDGET_BYTES) {
    failures.push(
      `${route}: ${gzipBytes} compressed bytes exceeds ${PUBLIC_ROUTE_BUDGET_BYTES}`,
    );
  }
}

const report = {
  version: 1,
  metric:
    "sum of unique initial JavaScript chunks compressed with gzip level 9, " +
    "excluding the nomodule polyfill bundle (reported separately as polyfillGzipBytes)",
  generatedAt: new Date().toISOString(),
  routes: results,
  failures,
};
await mkdir(path.dirname(OUTPUT), { recursive: true });
await writeFile(OUTPUT, `${JSON.stringify(report, null, 2)}\n`, { mode: 0o600 });

if (failures.length > 0) {
  console.error("Frontend bundle budget failed:\n");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}
console.log(
  `Frontend bundle budget passed (${results.length} routes, ${PUBLIC_ROUTE_BUDGET_BYTES} bytes each).`,
);
