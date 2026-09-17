import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("admin dashboard aborts obsolete loads while isolating individual stats", async () => {
  const page = await source("modules/admin/dashboard/dashboard-page.tsx");

  assert.match(page, /useRetryableResource<DashboardProjection>/);
  assert.match(page, /Promise\.allSettled/);
  assert.match(page, /card\.load\(signal\)/);
  assert.match(page, /dashboardResource\.retry/);
  assert.doesNotMatch(page, /let active|loadAttempt|useEffect\(/);
});

test("dashboard count services accept AbortSignal", async () => {
  const sources = await Promise.all([
    source("services/user.service.ts"),
    source("services/problem.service.ts"),
    source("services/contest.service.ts"),
    source("services/tag.service.ts"),
    source("services/role.service.ts"),
  ]);

  for (const service of sources) {
    assert.match(service, /signal\?: AbortSignal/);
  }
});
