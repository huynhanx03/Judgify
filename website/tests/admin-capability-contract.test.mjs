import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function loadPureTypeScriptModule(relativePath, dependencies = {}) {
  const source = await readFile(path.join(ROOT, relativePath), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    (specifier) => dependencies[specifier] ?? {},
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

test("capability index performs exact catalog grant checks", async () => {
  const {
    createCapabilityIndex,
    hasCapability,
    hasAnyCapability,
    hasAnyRequirement,
  } = await loadPureTypeScriptModule("lib/auth/capabilities.ts");
  const index = createCapabilityIndex({
    revision: 12,
    capabilities: [
      { resource: "problem", actions: ["read", "update", "read"] },
      { resource: "authorization", actions: ["manage"] },
    ],
  });

  assert.equal(index.revision, 12);
  assert.equal(hasAnyCapability(index), true);
  assert.equal(hasCapability(index, "problem", "read"), true);
  assert.equal(hasCapability(index, "problem", "delete"), false);
  assert.equal(hasCapability(index, "*", "*"), false);
  assert.equal(
    hasAnyRequirement(index, [
      { resource: "role", action: "read" },
      { resource: "authorization", action: "manage" },
    ]),
    true,
  );
  assert.equal(hasAnyRequirement(index, []), false);
});

test("admin policy applies exact and longest route capabilities", async () => {
  const capabilities = await loadPureTypeScriptModule("lib/auth/capabilities.ts");
  const authorization = await loadPureTypeScriptModule("constants/authorization.ts");
  const routes = await loadPureTypeScriptModule("constants/routes.ts");
  const policy = await loadPureTypeScriptModule("lib/auth/admin-policy.ts", {
    "@/lib/auth/capabilities": capabilities,
    "@/constants/authorization": authorization,
    "@/constants/routes": routes,
  });
  const index = capabilities.createCapabilityIndex({
    revision: 3,
    capabilities: [
      { resource: "problem", actions: ["read", "create"] },
      { resource: "material_category", actions: ["read"] },
      { resource: "authorization", actions: ["manage"] },
    ],
  });

  assert.equal(policy.adminRouteAllowed("/admin", index), true);
  assert.equal(policy.adminRouteAllowed("/admin/problems", index), true);
  assert.equal(policy.adminRouteAllowed("/admin/problems/create", index), true);
  assert.equal(policy.adminRouteAllowed("/admin/problems/42/edit", index), false);
  assert.equal(policy.adminRouteAllowed("/admin/materials/categories", index), true);
  assert.equal(policy.adminRouteAllowed("/admin/materials", index), false);
  assert.equal(policy.adminRouteAllowed("/admin/contests/contest-42", index), false);
  assert.equal(
    policy.adminRouteAllowed(
      "/admin/contests/contest-42/clarifications",
      index,
    ),
    false,
  );
  assert.equal(policy.adminRouteAllowed("/admin/roles", index), true);
  assert.equal(policy.adminRouteAllowed("/admin/users", index), false);
  assert.equal(policy.adminRouteAllowed("/admin/unknown", index), false);

  const editor = capabilities.createCapabilityIndex({
    revision: 4,
    capabilities: [{ resource: "problem", actions: ["read", "update"] }],
  });
  const updateOnly = capabilities.createCapabilityIndex({
    revision: 5,
    capabilities: [{ resource: "problem", actions: ["update"] }],
  });
  assert.equal(policy.adminRouteAllowed("/admin/problems/42/edit", editor), true);
  assert.equal(policy.adminRouteAllowed("/admin/problems/42/edit", updateOnly), false);

  const contestReader = capabilities.createCapabilityIndex({
    revision: 6,
    capabilities: [{ resource: "contest", actions: ["read"] }],
  });
  assert.equal(
    policy.adminRouteAllowed("/admin/contests/contest-42", contestReader),
    true,
  );
  assert.equal(
    policy.adminRouteAllowed(
      "/admin/contests/contest-42/clarifications?view=pending",
      contestReader,
    ),
    true,
  );
  assert.equal(
    policy.adminRouteAllowed(
      "/admin/contests/contest-42/clarifications/unexpected",
      contestReader,
    ),
    false,
  );
});

test("admin policy rejects an empty snapshot and filters inaccessible UI", async () => {
  const capabilities = await loadPureTypeScriptModule("lib/auth/capabilities.ts");
  const authorization = await loadPureTypeScriptModule("constants/authorization.ts");
  const routes = await loadPureTypeScriptModule("constants/routes.ts");
  const policy = await loadPureTypeScriptModule("lib/auth/admin-policy.ts", {
    "@/lib/auth/capabilities": capabilities,
    "@/constants/authorization": authorization,
    "@/constants/routes": routes,
  });
  const empty = capabilities.createCapabilityIndex({ revision: 7, capabilities: [] });
  assert.equal(policy.adminRouteAllowed("/admin", empty), false);

  const index = capabilities.createCapabilityIndex({
    revision: 8,
    capabilities: [{ resource: "tag", actions: ["read"] }],
  });
  const sections = [
    {
      title: "Content",
      items: [
        { id: "dashboard", requiresAnyCapability: true },
        { id: "tags", requirements: [{ resource: "tag", action: "read" }] },
        { id: "problems", requirements: [{ resource: "problem", action: "read" }] },
      ],
    },
    {
      title: "System",
      items: [
        { id: "users", requirements: [{ resource: "user", action: "read" }] },
      ],
    },
  ];
  const visible = policy.filterAuthorizedSections(sections, index);

  assert.deepEqual(visible.map((section) => section.title), ["Content"]);
  assert.deepEqual(visible[0].items.map((item) => item.id), ["dashboard", "tags"]);
  assert.deepEqual(
    policy.filterAuthorizedItems(sections[0].items.slice(1), index).map((item) => item.id),
    ["tags"],
  );
});

test("empty capability response remains a ready revisioned snapshot", async () => {
  const { createCapabilityIndex, hasAnyCapability } =
    await loadPureTypeScriptModule("lib/auth/capabilities.ts");
  const index = createCapabilityIndex({ revision: 9, capabilities: [] });

  assert.equal(index.revision, 9);
  assert.equal(index.keys.size, 0);
  assert.equal(hasAnyCapability(index), false);
});

test("authorization self endpoint and auth context expose capability state", async () => {
  const [api, service, context] = await Promise.all([
    readFile(path.join(ROOT, "constants/api/identity.ts"), "utf8"),
    readFile(path.join(ROOT, "services/authorization.service.ts"), "utf8"),
    readFile(path.join(ROOT, "contexts/auth-context.tsx"), "utf8"),
  ]);

  assert.match(api, /ME:\s*"\/authorization\/me"/);
  assert.match(service, /getMyCapabilities/);
  assert.match(context, /capabilityStatus/);
  assert.match(context, /refreshCapabilities/);
  assert.match(context, /hasAnyCapability/);
  assert.match(context, /canAny/);
});
