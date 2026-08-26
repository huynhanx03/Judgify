import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("ranking responses cross one bounded runtime boundary", async () => {
  const service = await source("services/ranking.service.ts");
  assert.match(service, /rankingPageSchema/);
  assert.match(service, /rankingLimitSchema\.parse\(limit\)/);
  assert.match(service, /auth:\s*["']none["']/);
  assert.match(service, /signal/);
  assert.match(service, /payload\.next_cursor === cursor/);
  assert.match(service, /from ["']@\/lib\/api\/client["']/);
});
