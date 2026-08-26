import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

async function loadService(relativePath, apiConstants) {
  const code = await source(relativePath);
  const compiled = ts.transpileModule(code, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const calls = [];
  const schema = (contract) => ({
    contract,
    parse: (value) => value,
  });
  const contestSchema = schema("contest");
  const api = async (endpoint, options = {}) => {
    calls.push({
      method: options.method ?? "GET",
      endpoint,
      ...(options.body === undefined ? {} : { body: options.body }),
      ...(options.auth === undefined ? {} : { auth: options.auth }),
      signal: options.signal,
      schema: options.schema?.contract,
    });
    if (endpoint.endsWith("/samples")) return [];
    if ((options.method ?? "GET") === "POST") {
      return { records: [], pagination: {} };
    }
    return {};
  };
  const dependencies = {
    "@/lib/api/client": { api },
    "@/constants/api": apiConstants,
    "@/constants/api/contest": apiConstants,
    "@/constants/api/problem": apiConstants,
    "@/constants/api/cultivation": apiConstants,
    "@/constants/api/auth": apiConstants,
    "@/constants/api/identity": apiConstants,
    "@/constants/api/material": apiConstants,
    "@/constants/api/submission": apiConstants,
    "@/constants/api/judge": apiConstants,
    "@/constants/api/notification": apiConstants,
    "@/constants/api/notification-campaign": apiConstants,
    "@/constants/api/realtime": apiConstants,
    "@/constants/api/audit": apiConstants,
    "@/constants/api/observability": apiConstants,
    "@/constants/api/operation": apiConstants,
    "@/constants/api/public-profile": apiConstants,
    "@/lib/api/contracts": {
      entityIDSchema: schema("entity-id"),
      paginatedSchema: (itemSchema) =>
        schema(`paginated:${itemSchema.contract}`),
    },
    "@/lib/api/schema": { voidSchema: schema("void") },
    "@/lib/problems/public-problem-schema": {
      problemPageSchema: schema("problem-page"),
      problemSchema: schema("problem"),
      sampleTestCaseListSchema: schema("sample-list"),
    },
    "@/lib/problems/authoring-schema": {
      problemAuthoringCatalogSchema: schema("problem-authoring-catalog"),
      problemAuthoringDraftSchema: schema("problem-authoring-draft"),
      problemDraftReceiptSchema: schema("problem-draft-receipt"),
      problemPublicationReceiptSchema: schema("problem-publication-receipt"),
    },
    "@/lib/contest/contest-schema": {
      contestPublishReceiptSchema: schema("contest-publish-receipt"),
      contestProblemDetailSchema: schema("contest-problem-detail"),
      contestRegistrationReceiptSchema: schema("contest-registration-receipt"),
      contestSchema,
      ratingChangesSchema: schema("rating-changes"),
      ratingReratingStartSchema: schema("rating-rerating-start"),
      standingsSnapshotSchema: schema("standings-snapshot"),
    },
    "@/lib/api/idempotency": {
      commandAttemptStore: {
        getOrCreate: () => ({ attempt_id: "test-attempt" }),
        resolve() {},
      },
    },
    "@/lib/api/error": { ApiError: class ApiError extends Error {} },
    "@/lib/contest/publish": {
      contestVersionETag: (version) => `"${version}"`,
      normalizeContestReason: (reason) => reason,
      normalizeContestPublishReason: (reason) => reason,
      normalizeRatingReratingReason: (reason) => reason,
    },
  };
  const loadedModule = { exports: {} };
  const require = (specifier) => {
    if (Object.hasOwn(dependencies, specifier)) return dependencies[specifier];
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loadedModule,
    loadedModule.exports,
  );
  return { exports: loadedModule.exports, calls };
}

test("problem service keeps public samples and administrative reads on distinct endpoints", async () => {
  const PROBLEM_API = {
    SAMPLES: (id) => `/problems/${id}/samples`,
    ADMIN_FIND: "/admin/problems/find",
    ADMIN_GET: (id) => `/admin/problems/${id}`,
  };
  const { exports, calls } = await loadService("services/problem.service.ts", {
    PROBLEM_API,
  });
  const query = { pagination: { page: 1, page_size: 20 } };
  const signal = new AbortController().signal;

  await exports.problemService.getPublicSamples("problem-1", signal);
  await exports.problemService.findAdmin(query, signal);
  await exports.problemService.getAdminById("problem-2", signal);

  assert.deepEqual(calls, [
    {
      method: "GET",
      endpoint: "/problems/problem-1/samples",
      auth: "none",
      signal,
      schema: "sample-list",
    },
    {
      method: "POST",
      endpoint: "/admin/problems/find",
      body: query,
      signal,
      schema: "problem-page",
    },
    {
      method: "GET",
      endpoint: "/admin/problems/problem-2",
      signal,
      schema: "problem",
    },
  ]);
});

test("problem pages use bounded public samples and unrestricted admin reads", async () => {
  const [detail, adminList, adminEdit] = await Promise.all([
    source("modules/problem/problem-workspace-page.tsx"),
    source("modules/admin/catalog/problems-page.tsx"),
    source("modules/admin/problems/edit-problem-page.tsx"),
  ]);

  assert.match(detail, /problemService\.getPublicSamples\(problemId, signal\)/);
  assert.doesNotMatch(detail, /problemService\.getTestCases\(problemId\)/);
  assert.match(adminList, /problemService\.findAdmin/);
  assert.match(adminEdit, /problemService\.getAdminById\(id, signal\)/);
  assert.match(adminEdit, /loadError\.status === 403/);
  assert.match(adminEdit, /loadError\.status === 404/);
});

test("contest service keeps public and administrative reads on distinct endpoints", async () => {
  const CONTEST_API = {
    FIND: "/contests/find",
    GET: (id) => `/contests/${id}`,
    ADMIN_FIND: "/admin/contests/find",
    ADMIN_GET: (id) => `/admin/contests/${id}`,
  };
  const { exports, calls } = await loadService("services/contest.service.ts", {
    CONTEST_API,
  });
  const publicQuery = { pagination: { page: 1, page_size: 12 } };
  const adminQuery = { pagination: { page: 2, page_size: 20 } };
  const signal = new AbortController().signal;

  await exports.contestService.find(publicQuery, signal);
  await exports.contestService.getById("contest-public", signal);
  await exports.contestService.findAdmin(adminQuery, signal);
  await exports.contestService.getAdminById("contest-draft", signal);

  assert.deepEqual(calls, [
    {
      method: "POST",
      endpoint: "/contests/find",
      body: publicQuery,
      signal,
      schema: "paginated:contest",
    },
    {
      method: "GET",
      endpoint: "/contests/contest-public",
      signal,
      schema: "contest",
    },
    {
      method: "POST",
      endpoint: "/admin/contests/find",
      body: adminQuery,
      signal,
      schema: "paginated:contest",
    },
    {
      method: "GET",
      endpoint: "/admin/contests/contest-draft",
      signal,
      schema: "contest",
    },
  ]);
});

test("admin contest list and overview totals use protected all-status reads", async () => {
  const [contestPage, dashboard] = await Promise.all([
    source("modules/admin/contests-page.tsx"),
    source("modules/admin/dashboard/dashboard-page.tsx"),
  ]);

  assert.match(contestPage, /contestService\.findAdmin/);
  assert.doesNotMatch(contestPage, /find:\s*contestService\.find\.bind/);
  assert.match(dashboard, /problemService\.findAdmin\(countQuery, signal\)/);
  assert.match(dashboard, /contestService\.findAdmin\(countQuery, signal\)/);
});

test("material service uses publication-aware slug and admin revision endpoints", async () => {
  const [service, endpoints] = await Promise.all([
    source("services/material.service.ts"),
    source("constants/api/material.ts"),
  ]);

  assert.match(service, /MATERIAL_API\.SEARCH/);
  assert.match(service, /MATERIAL_API\.GET_BY_SLUG/);
  assert.match(service, /MATERIAL_API\.ADMIN_SEARCH/);
  assert.match(service, /MATERIAL_API\.RECORD_VIEW/);
  assert.doesNotMatch(endpoints, /"\/materials\/find"/);
  assert.doesNotMatch(service, /materialService\.getById/);
});

test("admin material pages use protected read services", async () => {
  const [materialsPage, categoriesPage] = await Promise.all([
    source("modules/admin/materials/materials-page.tsx"),
    source("modules/admin/materials/categories-page.tsx"),
  ]);

  assert.match(materialsPage, /materialService\.findAdmin/);
  assert.match(categoriesPage, /materialService\.findCategories/);
});
