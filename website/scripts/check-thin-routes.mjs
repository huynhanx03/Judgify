#!/usr/bin/env node

/** Routes own server metadata/validation; interactive behavior belongs to modules. */
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";

async function pages(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) result.push(...(await pages(target)));
    else if (entry.name === "page.tsx") result.push(target);
  }
  return result;
}

const root = process.cwd();
const routePages = await pages(path.join(root, "app"));
const violations = [];

for (const file of routePages) {
  const source = await readFile(file, "utf8");
  if (/^\s*["']use client["'];?/m.test(source)) {
    violations.push(path.relative(root, file));
  }
}

if (violations.length > 0) {
  throw new Error(`Client implementations are not allowed in route pages: ${violations.join(", ")}`);
}

console.log(`Thin-route check passed (${routePages.length} route pages).`);
