import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("contest keeps the introduction informational and places discovery controls below it", async () => {
  const page = await source("modules/contest/contest-page.tsx");

  assert.match(page, /PublicPageLead/);
  assert.doesNotMatch(page, /actions=|HERO_REGISTER|HERO_SCHEDULE|scrollIntoView/);
  assert.doesNotMatch(page, /ContestHeroSection|MediaHero|PublicHero/);
});

test("contest cards use canonical routes and shared date presentation", async () => {
  const card = await source("modules/contest/contest-card.tsx");

  assert.match(card, /APP_ROUTES\.CONTEST_DETAIL\(contest\.id\)/);
  assert.match(card, /formatDateTimeMinute/);
  assert.doesNotMatch(card, /toLocaleDateString|href=\{`\/contest/);
  assert.match(card, /focus-visible:ring/);
});
