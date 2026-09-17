import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function loadSubmissionCommandPurpose() {
  const source = await readFile(
    path.join(ROOT, "lib/submissions/idempotency.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const evaluatedModule = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    () => {
      throw new Error("submission idempotency purpose must have no runtime imports");
    },
    evaluatedModule,
    evaluatedModule.exports,
  );
  return evaluatedModule.exports.submissionCommandPurpose;
}

const baseline = Object.freeze({
  problem_id: "00000000-0000-7000-8000-000000000021",
  runtime_key: "go",
  source_code: "package main\nfunc main() {}",
});

test("submission command purpose is stable and does not retain source", async () => {
  const submissionCommandPurpose = await loadSubmissionCommandPurpose();
  const first = await submissionCommandPurpose(baseline);
  const second = await submissionCommandPurpose({ ...baseline });

  assert.equal(first, second);
  assert.match(first, /^submission-[a-f0-9]{64}$/);
  assert.equal(first.includes(baseline.source_code), false);
});

test("submission command purpose changes with problem, runtime, or source", async () => {
  const submissionCommandPurpose = await loadSubmissionCommandPurpose();
  const original = await submissionCommandPurpose(baseline);
  const changed = await Promise.all([
    submissionCommandPurpose({
      ...baseline,
      problem_id: "00000000-0000-7000-8000-000000000022",
    }),
    submissionCommandPurpose({ ...baseline, runtime_key: "cpp17" }),
    submissionCommandPurpose({
      ...baseline,
      source_code: `${baseline.source_code}\n`,
    }),
  ]);

  for (const purpose of changed) assert.notEqual(purpose, original);
  assert.equal(new Set(changed).size, changed.length);
});

test("submission command purpose binds both contest identities", async () => {
  const submissionCommandPurpose = await loadSubmissionCommandPurpose();
  const contest = {
    ...baseline,
    contest_id: "00000000-0000-7000-8000-000000000031",
    contest_problem_id: "00000000-0000-7000-8000-000000000041",
  };
  const original = await submissionCommandPurpose(contest);
  const changed = await Promise.all([
    submissionCommandPurpose({
      ...contest,
      contest_id: "00000000-0000-7000-8000-000000000032",
    }),
    submissionCommandPurpose({
      ...contest,
      contest_problem_id: "00000000-0000-7000-8000-000000000042",
    }),
  ]);

  for (const purpose of changed) assert.notEqual(purpose, original);
});
