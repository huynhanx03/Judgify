import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("product navigation consumes the canonical browser route catalog", async () => {
  const [adminNavigation, navigation] = await Promise.all([
    source("constants/admin-navigation.ts"),
    source("constants/navigation.ts"),
  ]);

  assert.match(adminNavigation, /APP_ROUTES\.ADMIN_USERS/);
  assert.match(adminNavigation, /APP_ROUTES\.ADMIN_MATERIAL_CATEGORIES/);
  assert.doesNotMatch(adminNavigation, /href:\s*["']\/admin/);
  assert.match(navigation, /APP_ROUTES\.ARENA/);
  assert.doesNotMatch(navigation, /href:\s*["']\//);
});

test("browser acceptance covers every route at every canonical theme and width", async () => {
  const [matrixSource, quality, publicSuite, memberSuite, adminSuite] =
    await Promise.all([
      source("architecture/browser-matrix.json"),
      source("e2e/support/quality.ts"),
      source("e2e/public.spec.ts"),
      source("e2e/member.spec.ts"),
      source("e2e/admin.spec.ts"),
    ]);
  const matrix = JSON.parse(matrixSource);

  assert.deepEqual(matrix.themes, ["light", "dark", "system"]);
  assert.deepEqual(
    Object.values(matrix.viewports).map(({ width }) => width),
    [375, 768, 1280, 1536],
  );
  assert.match(quality, /matrix\.themes[\s\S]*flatMap/);
  assert.match(quality, /Object\.entries\(matrix\.viewports\)/);
  assert.match(quality, /prepareVisualAcceptance/);
  assert.match(quality, /assertVisualAcceptance/);
  for (const suite of [publicSuite, memberSuite, adminSuite]) {
    assert.match(suite, /for \(const variant of visualAcceptanceVariants\)/);
    assert.match(suite, /prepareVisualAcceptance\(page, variant\)/);
    assert.match(suite, /assertVisualAcceptance\(page, variant\)/);
    assert.match(suite, /assertAccessibility\(page, testInfo\)/);
  }
});

test("dynamic administrator contest routes validate branded identifiers", async () => {
  const routes = await Promise.all([
    source("app/admin/(dashboard)/contests/[id]/page.tsx"),
    source("app/admin/(dashboard)/contests/[id]/clarifications/page.tsx"),
  ]);

  for (const route of routes) {
    assert.match(route, /tryEntityID\(id\)/);
    assert.match(route, /if \(!contestID\) notFound\(\)/);
    assert.doesNotMatch(route, /(?:Number|parseInt)\s*\(/);
  }
});
