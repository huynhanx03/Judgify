import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("permission matrix renders server capability keys through the shared Vietnamese catalog", async () => {
  const [matrix, text] = await Promise.all([
    source("modules/admin/permission-matrix.tsx"),
    source("i18n/catalog.admin.vi.ts"),
  ]);

  assert.match(text, /RESOURCE_LABEL:/);
  assert.match(text, /ACTION_LABEL:/);
  assert.match(matrix, /ADMIN_TEXT\.PERMISSIONS\.RESOURCE_LABEL\(/);
  assert.match(matrix, /ADMIN_TEXT\.PERMISSIONS\.ACTION_LABEL\(/);
  assert.doesNotMatch(matrix, /resource\.label\s*\|\|/);
  assert.doesNotMatch(matrix, /action\.label\s*\|\|/);
});
