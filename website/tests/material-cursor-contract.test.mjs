import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("public materials consume every cursor page without duplicating feed state", async () => {
  const [page, feed, text] = await Promise.all([
    source("modules/materials/materials-page.tsx"),
    source("hooks/use-cursor-feed.ts"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(page, /useCursorFeed<MaterialArticle>/);
  assert.match(page, /cursor \? \{ cursor \} : \{\}/);
  assert.match(page, /items:\s*page\.records/);
  assert.match(page, /articlesFeed\.loadMore/);
  assert.match(page, /articlesFeed\.hasMore/);
  assert.match(feed, /deduplicates items/);
  assert.match(text, /ARTICLES_LOAD_MORE/);
  assert.match(text, /ARTICLES_LOADING_MORE/);
});

test("material detail separates not-found and related-content resource states", async () => {
  const page = await source("modules/materials/material-detail-page.tsx");

  assert.match(page, /instanceof ApiError/);
  assert.match(page, /status === 404/);
  assert.match(page, /RELATED_LOADING/);
  assert.match(page, /RELATED_LOAD_ERROR_TITLE/);
  assert.match(page, /RELATED_EMPTY/);
});
