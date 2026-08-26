import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function loadPureModule(relativePath, dependencies = {}) {
  const code = await source(relativePath);
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loaded = { exports: {} };
  Function("require", "module", "exports", compiled)(
    (specifier) => {
      if (specifier in dependencies) return dependencies[specifier];
      throw new Error(`unexpected import: ${specifier}`);
    },
    loaded,
    loaded.exports,
  );
  return loaded.exports;
}

test("contest problem navigation preserves a validated contest context", async () => {
  const navigation = await loadPureModule("lib/contest/navigation.ts", {
    "@/lib/api/contracts": {
      tryEntityID: (value) =>
        typeof value === "string" &&
        /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value)
          ? value
          : null,
    },
  });
  const contestId = "00000000-0000-7000-8000-000000000001";
  const problemId = "00000000-0000-7000-8000-000000000010";
  const contestProblemId = "00000000-0000-7000-8000-000000000020";

  assert.equal(
    navigation.contestProblemHref(contestId, problemId, contestProblemId),
    `/arena/${problemId}?contest=${contestId}&contest_problem=${contestProblemId}`,
  );
  assert.equal(navigation.readContestContext(contestId), contestId);
  assert.equal(navigation.readContestContext([contestId, "ignored"]), contestId);
  assert.equal(navigation.readContestContext("not-a-uuid"), null);
});

test("contest arena submits and displays only its own contest attempts", async () => {
  const [contestPage, arenaPage, hook, types, context] = await Promise.all([
    source("modules/contest/contest-detail-page.tsx"),
    source("modules/problem/problem-workspace-page.tsx"),
    source("hooks/use-problem-submissions.ts"),
    source("types/submission.ts"),
    loadPureModule("lib/submissions/contest-context.ts"),
  ]);

  assert.match(contestPage, /membership\.contest_problem_id/);
  assert.match(
    contestPage,
    /contestProblemHref\([\s\S]*membership\.problem_id,[\s\S]*membership\.contest_problem_id/,
  );
  assert.match(arenaPage, /searchParams:\s*Promise/);
  assert.match(arenaPage, /readContestContext/);
  assert.match(arenaPage, /contest_id:\s*activeContestId/);
  assert.match(arenaPage, /contest_problem_id:\s*contestProblemId/);
  assert.match(arenaPage, /useProblemSubmissions\([\s\S]*contestId/);
  assert.match(arenaPage, /BACK_TO_CONTEST/);
  assert.match(types, /contest_id:\s*EntityID/);
  assert.match(types, /contest_problem_id:\s*EntityID/);
  assert.match(hook, /submissionMatchesContest/);
  assert.match(hook, /submissionMatchesContest\(event, contestId\)/);

  const contestId = "00000000-0000-7000-8000-000000000001";
  assert.equal(context.submissionMatchesContest({ contest_id: contestId }, contestId), true);
  assert.equal(context.submissionMatchesContest({}, contestId), false);
  assert.equal(context.submissionMatchesContest({ contest_id: "other" }, null), true);
});

test("contest registration requires an established session and offers a login action", async () => {
  const [contestPage, routes, text] = await Promise.all([
    source("modules/contest/contest-detail-page.tsx"),
    source("constants/routes.ts"),
    source("i18n/catalog.vi.ts"),
  ]);

  assert.match(contestPage, /const \{ isAuthenticated, isLoading: isAuthLoading \} = useAuth\(\)/);
  assert.match(contestPage, /isAuthenticated && registrationAvailable/);
  assert.match(contestPage, /!isAuthenticated && !isAuthLoading && registrationAvailable/);
  assert.match(contestPage, /APP_ROUTES\.LOGIN/);
  assert.match(routes, /LOGIN:\s*"\/login"/);
  assert.match(text, /LOGIN_TO_REGISTER/);
});

test("admin publishes one versioned draft command with an audited reason", async () => {
  const [api, service, page, text, client, publish] = await Promise.all([
    source("constants/api/contest.ts"),
    source("services/contest.service.ts"),
    source("modules/admin/contests-page.tsx"),
    source("i18n/catalog.admin.vi.ts"),
    source("lib/api/client.ts"),
    source("lib/contest/publish.ts"),
  ]);

  assert.match(api, /PUBLISH:\s*\(id: EntityID\)[\s\S]*\/publish/);
  assert.match(
    service,
    /async publish\([\s\S]*expectedVersion:\s*number,[\s\S]*reason:\s*string,[\s\S]*Promise<ContestLifecycleReceipt>/,
  );
  assert.match(service, /idempotencyKey:\s*attempt\.attempt_id/);
  assert.match(service, /ifMatch:\s*contestVersionETag\(expectedVersion\)/);
  assert.match(service, /expectedStatus:\s*200/);
  assert.match(service, /schema:\s*contestLifecycleReceiptSchema/);
  assert.match(publish, /`"contest-v\$\{version\}"`/);
  assert.match(publish, /MAXIMUM_UTF8_BYTES/);
  assert.match(client, /headers\.set\(EXPECTED_VERSION_HEADER,\s*requestOptions\.ifMatch\)/);
  assert.match(page, /c\.status === "draft"/);
  assert.match(
    page,
    /contestService\.publish\([\s\S]*publishTarget\.id,[\s\S]*publishTarget\.version,[\s\S]*publishReason/,
  );
  assert.match(page, /confirmDisabled=\{!publishReasonValid\}/);
  assert.match(page, /crud\.refresh/);
  assert.match(text, /PUBLISH_CONFIRM_TITLE/);
  assert.match(text, /PUBLISH_REASON_LABEL/);
  assert.match(text, /PUBLISH_SUCCESS/);
});

test("contest responses are checked membership projections before UI use", async () => {
  const [schema, service, types] = await Promise.all([
    source("lib/contest/contest-schema.ts"),
    source("services/contest.service.ts"),
    source("types/contest.ts"),
  ]);

  assert.match(types, /contest_problem_id:\s*EntityID/);
  assert.match(types, /display_order:\s*number/);
  assert.match(types, /visible_before_start:\s*boolean/);
  assert.match(schema, /const frozenProblemSchema = strictObjectSchema/);
  assert.match(schema, /problems: optionalSchema\(arraySchema\(frozenProblemSchema/);
  assert.match(schema, /duplicate contest problem membership/);
  assert.match(schema, /display order must be contiguous/);
  assert.match(service, /schema:\s*paginatedSchema\(contestSchema\)/);
  assert.match(service, /schema:\s*contestSchema/);
});
