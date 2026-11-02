import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function loadModule(relativePath, dependencies = {}) {
  const source = await readFile(path.join(ROOT, relativePath), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loaded = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    (specifier) => dependencies[specifier] ?? {},
    loaded,
    loaded.exports,
  );
  return loaded.exports;
}

async function loadContentPolicy() {
  const constants = await loadModule("constants/submission.ts");
  const helpers = await loadModule("lib/submissions/content-limits.ts", {
    "@/constants/submission": constants,
  });
  return { ...constants, ...helpers };
}

test("judge content policy measures UTF-8 bytes rather than JavaScript characters", async () => {
  const {
    JUDGE_CONTENT_LIMITS,
    inspectSourceCode,
    utf8ByteLength,
  } = await loadContentPolicy();

  assert.deepEqual(JUDGE_CONTENT_LIMITS, {
    MAXIMUM_SOURCE_CODE_BYTES: 256 * 1024,
    MAXIMUM_TEST_INPUT_BYTES: 1024 * 1024,
    MAXIMUM_EXPECTED_OUTPUT_BYTES: 1024 * 1024,
  });
  assert.equal(utf8ByteLength("界"), 3);
  const exact = "界".repeat(Math.floor(JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES / 3))
    + "a".repeat(JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES % 3);
  assert.deepEqual(inspectSourceCode(exact), {
    valid: true,
    reason: null,
    actualBytes: JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES,
    maximumBytes: JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES,
  });
  assert.equal(inspectSourceCode(`${exact}界`).reason, "too_large");
});

test("judge content policy rejects empty and NUL text", async () => {
  const { inspectExpectedOutput, inspectTestInput } = await loadContentPolicy();

  assert.equal(inspectTestInput("").reason, "required");
  assert.equal(inspectExpectedOutput("ok\0").reason, "contains_nul");
});

test("uploaded source is decoded as strict UTF-8", async () => {
  const { decodeSourceUTF8 } = await loadContentPolicy();

  assert.equal(decodeSourceUTF8(new TextEncoder().encode("xin chào")), "xin chào");
  assert.throws(
    () => decodeSourceUTF8(Uint8Array.from([0xff, 0xfe])),
    (error) => error instanceof Error && error.name === "JudgeSourceDecodeError",
  );
});

test("submission and testcase editors consume the shared content policy", async () => {
  const [submission, problemForm] = await Promise.all([
    readFile(path.join(ROOT, "modules/problem/file-submission.tsx"), "utf8"),
    readFile(path.join(ROOT, "modules/admin/problem-form.tsx"), "utf8"),
  ]);

  assert.match(submission, /inspectJudgeContent/);
  assert.match(submission, /decodeSourceUTF8/);
  assert.match(problemForm, /policy\.test_asset_maximum_bytes/);
  assert.match(problemForm, /byteLength\(testcase\.input\)/);
  assert.match(problemForm, /byteLength\(testcase\.expectedOutput\)/);
  assert.doesNotMatch(
    `${submission}\n${problemForm}`,
    /MAXIMUM_(?:SOURCE_CODE|TEST_INPUT|EXPECTED_OUTPUT)_BYTES\s*[:=]\s*\d/,
  );
});
