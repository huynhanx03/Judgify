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

test("audit service uses GET /admin/audit with an allowlisted cursor query", async () => {
  const calls = [];
  const auditLogPageSchema = { parse: (value) => value };
  const api = async (endpoint, options) => {
    calls.push({ endpoint, options });
    return { items: [], has_more: false };
  };
  const { auditService } = await loadTypeScriptModule(
    "services/audit.service.ts",
    {
      "@/lib/api/client": { api },
      "@/lib/audit/audit-schema": { auditLogPageSchema },
      "@/constants/api": {
        AUDIT_API: { LIST: (query) => `/admin/audit?${query}` },
      },
      "@/constants/api/audit": {
        AUDIT_API: { LIST: (query) => `/admin/audit?${query}` },
      },
      "@/lib/audit/query": {
        toAuditSearchParams: (query) => {
          const params = new URLSearchParams();
          Object.entries(query).forEach(([key, value]) => {
            if (value !== undefined && value !== "") params.set(key, String(value));
          });
          return params;
        },
      },
    },
  );

  const signal = new AbortController().signal;
  await auditService.list(
    {
      actor_user_id: "01982f5c-a2d3-7abc-8def-1234567890ab",
      action: "identity.role.replace",
      resource: "role",
      resource_id: "moderator",
      correlation_id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
      from: "2026-07-01T00:00:00.000Z",
      to: "2026-07-22T00:00:00.000Z",
      limit: 25,
      cursor: "opaque-next-page",
    },
    signal,
  );

  assert.equal(calls.length, 1);
  const url = new URL(calls[0].endpoint, "https://judgify.local");
  assert.equal(url.pathname, "/admin/audit");
  assert.equal(calls[0].options.method, "GET");
  assert.equal(calls[0].options.signal, signal);
  assert.equal(calls[0].options.schema, auditLogPageSchema);
  assert.deepEqual(
    [...url.searchParams.keys()].sort(),
    [
      "action",
      "actor_user_id",
      "correlation_id",
      "cursor",
      "from",
      "limit",
      "resource",
      "resource_id",
      "to",
    ],
  );
});

test("audit filter model trims exact values and rejects invalid actor or time ranges", async () => {
  const model = await loadTypeScriptModule("lib/audit/query.ts", {
    "@/constants/audit": {
      AUDIT_FILTER_LIMITS: {
        ACTOR_USER_ID: 36,
        ACTION: 128,
        RESOURCE: 128,
        RESOURCE_ID: 256,
        CORRELATION_ID: 128,
      },
      AUDIT_PAGE_SIZE: 25,
      AUDIT_PAGE_SIZE_MAX: 100,
    },
  });
  const validActor = "01982f5c-a2d3-7abc-8def-1234567890ab";

  const valid = model.normalizeAuditFilters({
    actorUserId: ` ${validActor} `,
    action: " identity.role.replace ",
    resource: " role ",
    resourceId: " moderator ",
    correlationId: " cid-42 ",
    from: "2026-07-01T08:00",
    to: "2026-07-22T08:00",
  });
  assert.equal(valid.ok, true);
  assert.equal(valid.query.actor_user_id, validActor);
  assert.equal(valid.query.action, "identity.role.replace");
  assert.equal(valid.query.resource, "role");
  assert.equal(valid.query.resource_id, "moderator");
  assert.equal(valid.query.correlation_id, "cid-42");
  assert.match(valid.query.from, /^2026-07-01T/);
  assert.match(valid.query.to, /^2026-07-22T/);
  assert.equal(valid.query.limit, 25);

  assert.deepEqual(
    model.normalizeAuditFilters({
      ...model.createEmptyAuditFilterDraft(),
      actorUserId: "not-a-uuid",
    }),
    { ok: false, invalidFields: ["actorUserId"] },
  );
  assert.deepEqual(
    model.normalizeAuditFilters({
      ...model.createEmptyAuditFilterDraft(),
      from: "2026-07-22T08:00",
      to: "2026-07-01T08:00",
    }),
    { ok: false, invalidFields: ["timeRange"] },
  );
  assert.deepEqual(
    model.normalizeAuditFilters({
      ...model.createEmptyAuditFilterDraft(),
      from: "2026-07-22T08:00",
      to: "2026-07-22T08:00",
    }),
    { ok: false, invalidFields: ["timeRange"] },
  );
  const uppercaseActor = model.normalizeAuditFilters({
    ...model.createEmptyAuditFilterDraft(),
    actorUserId: validActor.toUpperCase(),
  });
  assert.equal(uppercaseActor.ok, true);
  assert.equal(uppercaseActor.query.actor_user_id, validActor);
  assert.deepEqual(
    model.normalizeAuditFilters({
      ...model.createEmptyAuditFilterDraft(),
      action: "identity role replace",
      resource: "role policy",
      correlationId: "cid with spaces",
    }),
    {
      ok: false,
      invalidFields: ["action", "resource", "correlationId"],
    },
  );
});

test("audit route and navigation require the exact audit read capability", async () => {
  const [authorization, routes, policy, navigation] = await Promise.all([
    source("constants/authorization.ts"),
    source("constants/routes.ts"),
    source("lib/auth/admin-policy.ts"),
    source("constants/admin-navigation.ts"),
  ]);

  assert.match(authorization, /AUDIT:\s*"audit"/);
  assert.match(routes, /ADMIN_AUDIT:\s*"\/admin\/audit"/);
  assert.match(policy, /\/admin\\\/audit/);
  assert.match(
    policy,
    /AUTHORIZATION_RESOURCE\.AUDIT[\s\S]*AUTHORIZATION_ACTION\.READ/,
  );
  assert.match(navigation, /href:\s*APP_ROUTES\.ADMIN_AUDIT/);
  assert.match(navigation, /AUTHORIZATION_RESOURCE\.AUDIT/);
});

test("audit explorer is real-data, responsive, recoverable, and inspectable", async () => {
  const [page, explorer, details, text, types] = await Promise.all([
    source("app/admin/(dashboard)/audit/page.tsx"),
    source("modules/admin/audit/admin-audit-explorer.tsx"),
    source("modules/admin/audit/audit-detail-sheet.tsx"),
    source("i18n/catalog.admin.vi.ts"),
    source("types/audit.ts"),
  ]);

  assert.match(page, /AdminAuditExplorer/);
  assert.match(explorer, /auditService[\s\S]*?\.list/);
  assert.match(explorer, /useRetryableResource/);
  assert.match(explorer, /auditService\.list\([\s\S]*?signal/);
  assert.match(explorer, /keepPreviousData:\s*true/);
  assert.match(explorer, /normalizeAuditFilters/);
  assert.match(explorer, /role="alert"/);
  assert.match(explorer, /aria-live="polite"/);
  assert.match(explorer, /aria-busy=\{isPending\}/);
  assert.match(explorer, /md:hidden/);
  assert.match(explorer, /hidden[^\"]*md:block/);
  assert.match(explorer, /next_cursor/);
  assert.match(explorer, /has_more/);
  assert.match(details, /SheetContent/);
  assert.match(details, /JSON\.stringify/);
  assert.doesNotMatch(`${explorer}\n${details}`, /mock|setInterval|EventSource/);
  assert.doesNotMatch(explorer, /let active|resultRef|refreshRevision/);
  assert.match(text, /AUDIT:\s*{/);
  assert.match(types, /interface AuditLogEntry/);
  assert.match(types, /interface AuditLogPage/);
});

test("audit filter validation feedback stays attached to its matching field", async () => {
  const explorer = await source("modules/admin/audit/admin-audit-explorer.tsx");

  const resourceStart = explorer.indexOf('<Label htmlFor="audit-resource-id">');
  const correlationStart = explorer.indexOf('<Label htmlFor="audit-correlation-id">');
  assert.notEqual(resourceStart, -1);
  assert.notEqual(correlationStart, -1);

  const resourceField = explorer.slice(resourceStart, correlationStart);
  const correlationField = explorer.slice(correlationStart);

  assert.match(resourceField, /invalidFields\.includes\("resourceId"\)/);
  assert.match(resourceField, /ADMIN_TEXT\.AUDIT\.RESOURCE_ID_INVALID/);
  assert.doesNotMatch(resourceField, /CORRELATION_ID_INVALID/);
  assert.match(correlationField, /invalidFields\.includes\("correlationId"\)/);
  assert.match(correlationField, /ADMIN_TEXT\.AUDIT\.CORRELATION_ID_INVALID/);
});

test("audit pagination keeps a previous action available when a later page fails", async () => {
  const explorer = await source("modules/admin/audit/admin-audit-explorer.tsx");
  const errorStart = explorer.indexOf("hasBlockingError ? (");
  const emptyStart = explorer.indexOf("result && result.items.length === 0", errorStart);
  assert.notEqual(errorStart, -1);
  assert.notEqual(emptyStart, -1);
  const errorBranch = explorer.slice(errorStart, emptyStart);

  assert.match(errorBranch, /pageIndex\s*>\s*0/);
  assert.match(errorBranch, /onClick=\{previousPage\}/);
  assert.match(errorBranch, /ADMIN_TEXT\.AUDIT\.PREVIOUS_PAGE/);
});

test("presentation errors never expose arbitrary backend-owned message text", async () => {
  const toast = await source("lib/toast.ts");

  const { getErrorMessage } = await loadTypeScriptModule("lib/toast.ts", {
    sonner: {
      toast: {
        success() {},
        error() {},
        warning() {},
        info() {},
      },
    },
  });

  assert.doesNotMatch(toast, /instanceof\s+ApiError/);
  assert.match(toast, /return fallback/);
  assert.equal(
    getErrorMessage(new Error("backend secret must not render"), "catalog copy"),
    "catalog copy",
  );
});

test("dashboard recent activity is isolated, capability-aware, and backed by audit API", async () => {
  const [dashboard, recent] = await Promise.all([
    source("modules/admin/dashboard/dashboard-page.tsx"),
    source("modules/admin/audit/admin-recent-audit.tsx"),
  ]);

  assert.match(dashboard, /AUTHORIZATION_RESOURCE\.AUDIT/);
  assert.match(dashboard, /AUTHORIZATION_ACTION\.READ/);
  assert.match(dashboard, /canReadAudit\s*\?/);
  assert.match(dashboard, /AdminRecentAudit/);
  assert.match(recent, /auditService[\s\S]*?\.list/);
  assert.match(recent, /useRetryableResource/);
  assert.match(recent, /auditService\.list\([\s\S]*?signal/);
  assert.match(recent, /keepPreviousData:\s*true/);
  assert.match(recent, /role="alert"/);
  assert.match(recent, /onClick=\{recentResource\.retry\}/);
  assert.match(recent, /hasStaleError/);
  assert.doesNotMatch(recent, /let active|loadRevision/);
  assert.doesNotMatch(recent, /mock|setInterval|EventSource/);
});
