import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("contest hero actions drive the real discovery controls", async () => {
  const [page, hero] = await Promise.all([
    source("modules/contest/contest-page.tsx"),
    source("modules/contest/contest-hero-section.tsx"),
  ]);

  assert.match(page, /onExplore=\{\(\) => focusContestList\("all"\)\}/);
  assert.match(page, /onShowUpcoming=\{\(\) => focusContestList\("upcoming"\)\}/);
  assert.match(page, /scrollIntoView/);
  assert.match(page, /CONTEST_LIST_ANCHOR_ID/);
  assert.match(hero, /onExplore/);
  assert.match(hero, /onShowUpcoming/);
  assert.doesNotMatch(hero, /<button(?:\\s|>)(?![\\s\\S]*onClick)/);
});

test("contest cards use canonical routes and shared date presentation", async () => {
  const card = await source("modules/contest/contest-card.tsx");

  assert.match(card, /APP_ROUTES\.CONTEST_DETAIL\(contest\.id\)/);
  assert.match(card, /formatDateTimeMinute/);
  assert.doesNotMatch(card, /toLocaleDateString|href=\{`\/contest/);
  assert.match(card, /focus-visible:ring/);
});
