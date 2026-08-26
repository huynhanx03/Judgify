import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("ranking feeds load independently and forward cancellation", async () => {
  const [page, service] = await Promise.all([
    source("modules/ranking/ranking-page.tsx"),
    source("services/ranking.service.ts"),
  ]);

  assert.match(page, /useRankingPage\(["']rating["']\)/);
  assert.match(page, /useRankingPage\(["']experience["']\)/);
  assert.doesNotMatch(page, /Promise\.all/);
  assert.match(page, /new AbortController\(\)/);
  assert.match(service, /pageByRating\([\s\S]*signal\?: AbortSignal/);
  assert.match(service, /pageByExperience\([\s\S]*signal\?: AbortSignal/);
  assert.match(service, /auth:\s*["']none["'],\s*signal/);
});

test("each ranking panel owns loading, empty, stale, error, and retry states", async () => {
  const [page, list] = await Promise.all([
    source("modules/ranking/ranking-page.tsx"),
    source("modules/ranking/ranking-top-list.tsx"),
  ]);

  assert.match(page, /status=\{rating\.status\}/);
  assert.match(page, /status=\{cultivation\.status\}/);
  assert.match(page, /onRetry=\{rating\.retry\}/);
  assert.match(page, /onRetry=\{cultivation\.retry\}/);
  assert.match(page, /onLoadMore=\{rating\.loadMore\}/);
  assert.match(page, /onLoadMore=\{cultivation\.loadMore\}/);
  assert.match(list, /TEXT\.RANKING\.EMPTY_CULTIVATORS/);
  assert.match(list, /TEXT\.RANKING\.LOAD_ERROR_TITLE/);
  assert.match(list, /TEXT\.RANKING\.STALE_DATA/);
  assert.match(list, /TEXT\.COMMON\.RETRY/);
  assert.match(list, /aria-busy=\{isRefreshing\}/);
});
