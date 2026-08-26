import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("materials filters fail and recover independently", async () => {
  const page = await source("modules/materials/materials-page.tsx");

  assert.match(page, /categoryResource = useRetryableResource<MaterialCategory\[\]>/);
  assert.match(
    page,
    /difficultyResource = useRetryableResource<DifficultyResponse\[\]>/,
  );
  assert.match(page, /onRetry=\{categoryResource\.retry\}/);
  assert.match(page, /onRetry=\{difficultyResource\.retry\}/);
  assert.doesNotMatch(page, /Promise\.all\(/);
  assert.doesNotMatch(page, /filterCatalogResource/);
});

test("material detail uses canonical routing without inventing a browser identity", async () => {
  const [page, service] = await Promise.all([
    source("modules/materials/material-detail-page.tsx"),
    source("services/material.service.ts"),
  ]);

  assert.match(page, /APP_ROUTES\.MATERIAL_DETAIL\(resolution\.canonical_slug\)/);
  assert.match(page, /recordView\(resolution\.material\.id, controller\.signal\)/);
  assert.doesNotMatch(page, /(?:localStorage|sessionStorage|viewerIdentity)/);
  assert.match(service, /api<\{ counted: boolean \}, never>/);
  assert.doesNotMatch(service, /viewer_token/);
});
