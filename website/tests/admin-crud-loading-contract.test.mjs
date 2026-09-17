import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();
const CRUD_PAGES = [
  "contests",
  "difficulties",
  "elements",
  "levels",
  "problems",
  "ranks",
  "rarities",
  "tags",
  "traits",
];

function crudPageSource(pageName) {
  const migratedCatalogRoutes = new Set([
    "difficulties",
    "elements",
    "levels",
    "problems",
    "ranks",
    "rarities",
    "tags",
    "traits",
  ]);
  if (migratedCatalogRoutes.has(pageName)) {
    return `modules/admin/catalog/${pageName}-page.tsx`;
  }
  return pageName === "contests"
    ? "modules/admin/contests-page.tsx"
    : `app/admin/(dashboard)/${pageName}/page.tsx`;
}

test("settled admin CRUD pages retain their table while refreshing", async () => {
  for (const pageName of CRUD_PAGES) {
    const source = await readFile(
      path.join(
        ROOT,
        crudPageSource(pageName),
      ),
      "utf8",
    );
    assert.match(
      source,
      /crud\.isLoading && !crud\.hasSettled/,
      `${pageName} must reserve its blocking loader for the first request`,
    );
    assert.match(
      source,
      /isLoading=\{crud\.isLoading\}/,
      `${pageName} must expose non-blocking refresh state`,
    );
  }
});
