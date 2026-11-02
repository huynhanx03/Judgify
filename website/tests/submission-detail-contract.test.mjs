import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("submission lists use the bounded summary DTO", async () => {
  const [types, service, hook, history] = await Promise.all([
    source("types/submission.ts"),
    source("services/submission.service.ts"),
    source("hooks/use-problem-submissions.ts"),
    source("modules/problem/submission-history.tsx"),
  ]);

  assert.match(types, /interface SubmissionSummary\s*{/);
  assert.match(types, /interface Submission extends SubmissionSummary/);
  assert.match(
    service,
    /getMyByProblem\([\s\S]*problemId: string[\s\S]*Promise<SubmissionCursorPage>/,
  );
  assert.match(hook, /submissions:\s*SubmissionSummary\[\]/);
  assert.match(history, /submissions:\s*SubmissionSummary\[\]/);
});

test("submission detail is fetched lazily with loading error and retry states", async () => {
  const [history, service, text] = await Promise.all([
    source("modules/problem/submission-history.tsx"),
    source("services/submission.service.ts"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(service, /getById\([\s\S]*id: string[\s\S]*Promise<Submission>/);
  assert.match(history, /submissionService\.getById\(selectedId, signal\)/);
  assert.match(history, /const isLoadingDetail/);
  assert.match(history, /detailResource\.status === "error"/);
  assert.match(history, /detailResource\.retry/);
  assert.match(history, /useRetryableResource<Submission \| null>/);
  assert.match(text, /DETAIL_LOADING/);
  assert.match(text, /DETAIL_LOAD_ERROR/);
  assert.match(text, /DETAIL_RETRY/);
});

test("output limit exceeded is a localized terminal verdict", async () => {
  const [types, terminalCatalog, detail, text] = await Promise.all([
    source("types/submission.ts"),
    source("constants/submission.ts"),
    source("modules/problem/submission-detail.tsx"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(types, /"output_limit_exceeded"/);
  assert.match(terminalCatalog, /"output_limit_exceeded"/);
  assert.match(detail, /output_limit_exceeded:/);
  assert.match(text, /OUTPUT_LIMIT:/);
});
