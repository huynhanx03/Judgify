import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();
const terminalStatuses = [
  "accepted",
  "wrong_answer",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "output_limit_exceeded",
  "runtime_error",
  "compile_error",
  "internal_error",
];

async function loadStatusHelpers() {
  const source = await readFile(
    path.join(ROOT, "lib/submissions/status.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const evaluatedModule = { exports: {} };
  const require = (specifier) => {
    if (specifier === "@/constants/submission") {
      return { TERMINAL_SUBMISSION_STATUSES: terminalStatuses };
    }
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    evaluatedModule,
    evaluatedModule.exports,
  );
  return evaluatedModule.exports;
}

test("nonterminal detection covers pending work anywhere in the snapshot", async () => {
  const { hasNonterminalSubmission } = await loadStatusHelpers();

  assert.equal(hasNonterminalSubmission([]), false);
  assert.equal(hasNonterminalSubmission([{ status: "accepted" }]), false);
  assert.equal(
    hasNonterminalSubmission([{ status: "output_limit_exceeded" }]),
    false,
  );
  assert.equal(
    hasNonterminalSubmission([
      { status: "accepted" },
      { status: "judging" },
    ]),
    true,
  );
});
