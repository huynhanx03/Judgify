import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function loadRuntimeCatalogModule() {
  const source = await readFile(
    path.join(ROOT, "lib/submissions/runtime-catalog.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loaded = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    () => ({}),
    loaded,
    loaded.exports,
  );
  return loaded.exports;
}

const VALID_CATALOG = {
  version: 1,
  maximum_source_code_bytes: 256 * 1024,
  runtimes: [
    {
      runtime_key: "cpp17.release-2026-07",
      language: "cpp",
      display_name: "C++ 17",
      source_filename: "main.cpp",
      file_extensions: [".CPP", ".cc"],
      compiled: true,
    },
    {
      runtime_key: "python3.release-2026-07",
      language: "python",
      display_name: "Python 3",
      source_filename: "main.py",
      file_extensions: [".py"],
      compiled: false,
    },
  ],
};

test("runtime catalog parser returns a normalized immutable trust-boundary value", async () => {
  const { parseJudgeRuntimeCatalog } = await loadRuntimeCatalogModule();
  const parsed = parseJudgeRuntimeCatalog(VALID_CATALOG);

  assert.equal(parsed.version, 1);
  assert.deepEqual(parsed.runtimes[0].file_extensions, [".cpp", ".cc"]);
  assert.equal(Object.isFrozen(parsed), true);
  assert.equal(Object.isFrozen(parsed.runtimes), true);
  assert.equal(Object.isFrozen(parsed.runtimes[0]), true);
  assert.equal(Object.isFrozen(parsed.runtimes[0].file_extensions), true);
});

test("runtime catalog parser rejects ambiguous and unsafe descriptors", async () => {
  const { JudgeRuntimeCatalogError, parseJudgeRuntimeCatalog } =
    await loadRuntimeCatalogModule();
  const invalidCatalogs = [
    null,
    { ...VALID_CATALOG, runtimes: [] },
    { ...VALID_CATALOG, maximum_source_code_bytes: 0 },
    { ...VALID_CATALOG, maximum_source_code_bytes: 17 * 1024 * 1024 },
    {
      ...VALID_CATALOG,
      runtimes: [VALID_CATALOG.runtimes[0], VALID_CATALOG.runtimes[0]],
    },
    {
      ...VALID_CATALOG,
      runtimes: [
        { ...VALID_CATALOG.runtimes[0], source_filename: "../main.cpp" },
      ],
    },
    {
      ...VALID_CATALOG,
      runtimes: [
        { ...VALID_CATALOG.runtimes[0], file_extensions: [".cpp", ".CPP"] },
      ],
    },
  ];

  for (const catalog of invalidCatalogs) {
    assert.throws(
      () => parseJudgeRuntimeCatalog(catalog),
      (error) => error instanceof JudgeRuntimeCatalogError,
    );
  }
});

test("source file matching compares the exact final extension", async () => {
  const { sourceFileMatchesRuntime } = await loadRuntimeCatalogModule();
  const runtime = VALID_CATALOG.runtimes[1];

  assert.equal(sourceFileMatchesRuntime("solution.PY", runtime), true);
  assert.equal(sourceFileMatchesRuntime("solution.copy", runtime), false);
  assert.equal(sourceFileMatchesRuntime("solution", runtime), false);
});

test("submission workbench consumes release runtime keys without a client allowlist", async () => {
  const source = await readFile(
    path.join(ROOT, "modules/problem/file-submission.tsx"),
    "utf8",
  );

  assert.match(source, /getRuntimeCatalog/);
  assert.match(source, /maximum_source_code_bytes/);
  assert.match(source, /runtime_key/);
  assert.match(source, /display_name/);
  assert.match(source, /file_extensions/);
  assert.doesNotMatch(source, /const\s+(?:LANGUAGES|LANGUAGE_LABELS)\s*=/);
});

test("submission intake sends the exact runtime-key command behind a recoverable UUID", async () => {
  const [types, service, page] = await Promise.all([
    readFile(path.join(ROOT, "types/submission.ts"), "utf8"),
    readFile(path.join(ROOT, "services/submission.service.ts"), "utf8"),
    readFile(path.join(ROOT, "modules/problem/problem-workspace-page.tsx"), "utf8"),
  ]);

  const createRequest = types.slice(
    types.indexOf("interface CreateSubmissionBase"),
    types.indexOf("export type SubmissionGenerationPhase"),
  );
  assert.match(createRequest, /runtime_key:\s*RuntimeKey/);
  assert.match(createRequest, /contest_problem_id:\s*EntityID/);
  assert.doesNotMatch(createRequest, /\blanguage:/);
  assert.match(service, /commandAttemptStore\.getOrCreate/);
  assert.match(service, /idempotencyKey:\s*attempt\.attempt_id/);
  assert.match(service, /expectedStatus:\s*202/);
  assert.match(service, /submissionAcceptedSnapshotSchema/);
  assert.match(page, /contest_id:\s*activeContestId/);
  assert.match(page, /contest_problem_id:\s*contestProblemId/);
  assert.doesNotMatch(page, /\blanguage,\s*\n\s*source_code/);
});
