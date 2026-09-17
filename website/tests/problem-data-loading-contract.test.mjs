import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("problem screens never turn request failures into empty data silently", async () => {
  const files = await Promise.all([
    source("modules/arena/arena-page.tsx"),
    source("modules/problem/problem-workspace-page.tsx"),
    source("modules/admin/catalog/problems-page.tsx"),
    source("modules/admin/problem-form.tsx"),
  ]);

  for (const file of files) {
    assert.doesNotMatch(file, /\.catch\(\(\)\s*=>\s*\{\s*\}\)/);
  }
  assert.match(files.join("\n"), /DataLoadFeedback/);
  assert.match(files.join("\n"), /\.retry/);
});

test("arena degrades optional filter catalogs without interrupting problem discovery", async () => {
  const [arena, filterBar, collectionFeedback] = await Promise.all([
    source("modules/arena/arena-page.tsx"),
    source("modules/arena/components/arena-filter-bar.tsx"),
    source("modules/shared/resource-feedback.tsx"),
  ]);

  assert.doesNotMatch(arena, /FILTERS_LOAD_ERROR_TITLE/);
  assert.doesNotMatch(arena, /retryReferenceData/);
  assert.match(arena, /difficultyFilterAvailable/);
  assert.match(arena, /tagFilterAvailable/);
  assert.match(filterBar, /difficultyFilterAvailable/);
  assert.match(filterBar, /tagFilterAvailable/);
  assert.match(
    filterBar,
    /tags\.find\(\(candidate\) => candidate\.id === id\)\?\.name[\s\S]*TEXT\.COMMON\.UNKNOWN/,
  );
  assert.match(arena, /ResourceCollectionFeedback/);
  assert.match(collectionFeedback, /ResourceCollectionFeedback/);
  assert.match(collectionFeedback, /role="alert"/);
});

test("admin catalog lists degrade optional filter resources without replacing their data view", async () => {
  const [adminProblems, traits, tags, materials] = await Promise.all([
    source("modules/admin/catalog/problems-page.tsx"),
    source("modules/admin/catalog/traits-page.tsx"),
    source("modules/admin/catalog/tags-page.tsx"),
    source("modules/admin/materials/materials-page.tsx"),
  ]);

  assert.match(adminProblems, /difficultyCatalog\.status !== "error"/);
  assert.doesNotMatch(adminProblems, /FILTER_DIFFICULTY_ERROR/);
  assert.doesNotMatch(adminProblems, /FILTER_DIFFICULTY_RETRY/);
  assert.match(traits, /rarityCatalog\.status !== "error"/);
  assert.match(traits, /aria-label=\{ADMIN_TEXT\.TRAITS\.FILTER_TYPE_ALL\}/);
  assert.match(traits, /aria-label=\{ADMIN_TEXT\.TRAITS\.FILTER_RARITY_ALL\}/);
  assert.match(tags, /elementCatalog\.status !== "error"/);
  assert.match(materials, /categoryCatalog\.status !== "error"/);
  assert.match(materials, /difficultyCatalog\.status !== "error"/);
  for (const file of [traits, tags, materials]) {
    assert.doesNotMatch(file, /RELATED_DATA_LOAD_ERROR/);
  }
});

test("problem detail keeps problem and public sample failures independent", async () => {
  const [detail, panel] = await Promise.all([
    source("modules/problem/problem-workspace-page.tsx"),
    source("modules/arena/problem-description-panel.tsx"),
  ]);

  assert.match(detail, /problemResource\s*=\s*useRetryableResource/);
  assert.match(detail, /samplesResource\s*=\s*useRetryableResource/);
  assert.match(detail, /error instanceof ApiError/);
  assert.match(detail, /error\.status === 404/);
  assert.doesNotMatch(detail, /Promise\.all\(\s*\[\s*problemService\.getById/);
  assert.match(panel, /sampleStatus === "error"/);
  assert.match(panel, /onRetrySamples/);
});

test("problem authoring fences catalogs and uses shared backend-aligned limits", async () => {
  const [form, resourceHook, limits] = await Promise.all([
    source("modules/admin/problem-form.tsx"),
    source("hooks/use-retryable-resource.ts"),
    source("constants/problem.ts"),
  ]);

  assert.match(form, /catalogResource\.retry/);
  assert.match(form, /difficultyResource\.retry/);
  assert.match(form, /tagResource\.retry/);
  assert.match(form, /maximum_runtimes/);
  assert.doesNotMatch(form, /PROBLEM_AUTHORING_LIMITS/);
  assert.match(form, /ConfirmDialog/);
  assert.doesNotMatch(form, /router\.push\("\/admin\/problems/);
  assert.match(resourceHook, /requestSequenceRef/);
  assert.match(resourceHook, /requestSequence !== requestSequenceRef\.current/);
  assert.match(limits, /PROBLEM_TEST_KIND/);
  assert.match(limits, /PROBLEM_CHECKER_CONFIG_KIND/);
});
