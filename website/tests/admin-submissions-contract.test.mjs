import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function loadTypeScriptModule(relativePath, dependencies = {}) {
  const compiled = ts.transpileModule(await source(relativePath), {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    (specifier) => {
      if (specifier in dependencies) return dependencies[specifier];
      throw new Error(`unexpected import: ${specifier}`);
    },
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

test("admin submission service uses the dedicated safe-list and sensitive-detail endpoints", async () => {
  const calls = [];
  const schema = (contract) => ({
    contract,
    parse: (value) => value,
  });
  const api = async (endpoint, options = {}) => {
    calls.push({
      method: options.method ?? "GET",
      endpoint,
      ...(options.body === undefined ? {} : { body: options.body }),
      signal: options.signal,
      schema: options.schema?.contract,
    });
    return endpoint === "/admin/submissions/find"
      ? { records: [], pagination: {} }
      : {};
  };
  const SUBMISSION_API = {
    ADMIN_FIND: "/admin/submissions/find",
    ADMIN_GET: (id) => `/admin/submissions/${id}`,
  };
  const validId = "01982f5c-a2d3-7abc-8def-1234567890ab";
  const { submissionService } = await loadTypeScriptModule(
    "services/submission.service.ts",
    {
      "@/lib/api/client": { api },
      "@/constants/api": { JUDGE_API: {}, SUBMISSION_API },
      "@/constants/api/submission": { SUBMISSION_API },
      "@/constants/api/judge": { JUDGE_API: {} },
      "@/lib/submissions/runtime-catalog": {
        judgeRuntimeCatalogSchema: schema("runtime-catalog"),
        runtimeKeySchema: schema("runtime-key"),
      },
      "@/lib/submissions/submission-schema": {
        adminSubmissionDetailSchema: schema("admin-submission-detail"),
        adminJudgeOperationReceiptSchema: schema("admin-operation-receipt"),
        adminSubmissionPageSchema: schema("admin-submission-page"),
        submissionAcceptedSnapshotSchema: schema("submission-accepted"),
        submissionCursorPageSchema: schema("submission-cursor-page"),
        submissionSchema: schema("submission"),
        submissionSummaryListSchema: schema("submission-summary-list"),
      },
      "@/lib/api/idempotency": {
        commandAttemptStore: {
          getOrCreate: () => ({ attempt_id: validId }),
          resolve() {},
        },
      },
      "@/lib/api/error": { ApiError: class ApiError extends Error {} },
      "@/lib/api/contracts": {
        cursorSchema: schema("cursor"),
        entityIDSchema: schema("entity-id"),
      },
      "@/constants/submission": {
        SUBMISSION_HISTORY_MAXIMUM_PAGE_SIZE: 100,
        SUBMISSION_HISTORY_PAGE_SIZE: 20,
        ADMIN_JUDGE_OPERATION_REASON_LIMITS: {
          MINIMUM_CHARACTERS: 1,
          MAXIMUM_CHARACTERS: 500,
        },
      },
      "@/lib/submissions/idempotency": {
        submissionCommandPurpose: async () => "submission-test",
      },
      "@/lib/submissions/content-limits": {
        inspectSourceCode: () => ({ valid: true }),
      },
    },
  );
  const query = { pagination: { page: 2, page_size: 25 }, status: "judging" };
  const signal = new AbortController().signal;

  await submissionService.findAdmin(query, signal);
  await submissionService.getAdminById(validId, signal);

  assert.deepEqual(calls, [
    {
      method: "POST",
      endpoint: "/admin/submissions/find",
      body: query,
      signal,
      schema: "admin-submission-page",
    },
    {
      method: "GET",
      endpoint: `/admin/submissions/${validId}`,
      signal,
      schema: "admin-submission-detail",
    },
  ]);
});

test("admin submission filter model trims valid identifiers and rejects malformed ones", async () => {
  const model = await loadTypeScriptModule(
    "lib/submissions/admin-query.ts",
    {
      "@/lib/api/contracts": {
        tryEntityID(value) {
          return typeof value === "string" &&
            /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value)
            ? value
            : null;
        },
      },
    },
  );
  const problemId = "0198-should-not-pass";
  const validId = "01982f5c-a2d3-7abc-8def-1234567890ab";

  assert.deepEqual(
    model.normalizeAdminSubmissionFilters({
      status: "judging",
      language: " go ",
      problemId: ` ${validId} `,
      userId: "",
      contestId: "",
    }),
    {
      ok: true,
      filters: {
        status: "judging",
        language: "go",
        problemId: validId,
      },
    },
  );
  assert.deepEqual(
    model.normalizeAdminSubmissionFilters({
      ...model.createEmptyAdminSubmissionFilters(),
      problemId,
    }),
    { ok: false, invalidFields: ["problemId"] },
  );
  assert.deepEqual(
    model.toAdminSubmissionFindRequest(
      { status: "accepted", userId: validId },
      3,
      25,
    ),
    {
      pagination: { page: 3, page_size: 25 },
      status: "accepted",
      user_id: validId,
    },
  );
});

test("admin submission route and navigation require safe read while detail is inspect-gated", async () => {
  const [authorization, policy, navigation, monitor] = await Promise.all([
    source("constants/authorization.ts"),
    source("lib/auth/admin-policy.ts"),
    source("constants/admin-navigation.ts"),
    source("modules/admin/submissions/admin-submission-monitor.tsx"),
  ]);

  assert.match(authorization, /INSPECT:\s*"inspect"/);
  assert.match(policy, /\/admin\\\/submissions/);
  assert.match(policy, /AUTHORIZATION_RESOURCE\.SUBMISSION[\s\S]*AUTHORIZATION_ACTION\.READ/);
  assert.match(navigation, /href:\s*APP_ROUTES\.ADMIN_SUBMISSIONS/);
  assert.match(navigation, /AUTHORIZATION_RESOURCE\.SUBMISSION/);
  assert.match(monitor, /AUTHORIZATION_ACTION\.INSPECT/);
  assert.match(monitor, /canInspect/);
});

test("admin submission monitor is real-data, recoverable, responsive, and manually refreshed", async () => {
  const [page, monitor, detail, text, types] = await Promise.all([
    source("app/admin/(dashboard)/submissions/page.tsx"),
    source("modules/admin/submissions/admin-submission-monitor.tsx"),
    source("modules/admin/submissions/admin-submission-detail-dialog.tsx"),
    source("i18n/catalog.admin.vi.ts"),
    source("types/submission.ts"),
  ]);

  assert.match(page, /AdminSubmissionMonitor/);
  assert.match(monitor, /submissionService[\s\S]*?\.findAdmin/);
  assert.match(monitor, /submissionService[\s\S]*?\.getRuntimeCatalog/);
  assert.match(monitor, /useRetryableResource<[\s\S]*load:\s*\(signal\)\s*=>\s*submissionService\.findAdmin\(query, signal\)/);
  assert.match(detail, /useRetryableResource<[\s\S]*submissionService\.getAdminById\(submissionId, signal\)/);
  assert.match(monitor, /role="alert"/);
  assert.match(monitor, /aria-live="polite"/);
  assert.match(monitor, /md:hidden/);
  assert.match(monitor, /hidden[^"]*md:block/);
  assert.match(monitor, /result \? metric\.value : TEXT\.COMMON\.NOT_AVAILABLE/);
  assert.match(monitor, /\{loadError && result \?/);
  assert.match(detail, /submissionService[\s\S]*?\.getAdminById/);
  assert.match(detail, /role="status"/);
  assert.doesNotMatch(`${monitor}\n${detail}`, /setInterval|EventSource/);
  assert.match(text, /SUBMISSIONS:\s*{/);
  assert.match(types, /interface AdminSubmissionSummary/);
  assert.match(types, /interface AdminSubmissionDetail extends AdminSubmissionSummary/);
});
