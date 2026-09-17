import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

const CRUD_PAGES = [
  ["difficulties", "DIFFICULTY"],
  ["elements", "ELEMENT"],
  ["levels", "LEVEL"],
  ["ranks", "RANK"],
  ["rarities", "RARITY"],
  ["tags", "TAG"],
  ["traits", "TRAIT"],
  ["problems", "PROBLEM"],
  ["contests", "CONTEST"],
  ["materials", "MATERIAL"],
  ["materials/categories", "MATERIAL_CATEGORY"],
];

function crudPageSource(route) {
  const migratedCatalogRoutes = new Set([
    "difficulties",
    "elements",
    "levels",
    "problems",
    "ranks",
    "rarities",
    "tags",
    "traits",
  ]);
  if (migratedCatalogRoutes.has(route)) {
    return `modules/admin/catalog/${route}-page.tsx`;
  }
  const migratedFeatureRoutes = {
    contests: "modules/admin/contests-page.tsx",
    materials: "modules/admin/materials/materials-page.tsx",
    "materials/categories": "modules/admin/materials/categories-page.tsx",
  };
  return migratedFeatureRoutes[route] ?? `app/admin/(dashboard)/${route}/page.tsx`;
}

test("shared admin CRUD controls require exact create update and delete grants", async () => {
  const [actions, shell, hook] = await Promise.all([
    readFile(path.join(ROOT, "modules/admin/admin-resource-actions.tsx"), "utf8"),
    readFile(path.join(ROOT, "modules/admin/data-table-shell.tsx"), "utf8"),
    readFile(path.join(ROOT, "modules/admin/hooks/use-paginated-crud.ts"), "utf8"),
  ]);

  assert.match(actions, /AUTHORIZATION_ACTION\.UPDATE/);
  assert.match(actions, /AUTHORIZATION_ACTION\.DELETE/);
  assert.match(actions, /can\(/);
  assert.match(shell, /canCreate/);
  assert.match(shell, /canCreate\s*\?/);
  assert.match(hook, /canCreate = can\(resource, AUTHORIZATION_ACTION\.CREATE\)/);
  assert.match(hook, /canUpdate = can\(resource, AUTHORIZATION_ACTION\.UPDATE\)/);
  assert.match(
    hook,
    /const canDelete\s*=\s*[\s\S]*can\(resource, AUTHORIZATION_ACTION\.DELETE\)/,
  );
  assert.match(hook, /dialogRevision === authorizationRevision/);
  assert.match(hook, /deleteRevision === authorizationRevision/);
});

for (const [route, resource] of CRUD_PAGES) {
  test(`${route} binds every CRUD action to ${resource}`, async () => {
    const source = await readFile(
      path.join(ROOT, crudPageSource(route)),
      "utf8",
    );

    assert.match(source, new RegExp(`AUTHORIZATION_RESOURCE\\.${resource}`));
    assert.match(source, /AdminResourceActions/);
    assert.match(source, /canCreate=/);
    assert.match(
      source,
      /usePaginatedCRUD[\s\S]*\(\{[\s\S]*service,[\s\S]*resource/,
    );
  });
}

test("contest publish is guarded by contest update", async () => {
  const source = await readFile(
    path.join(ROOT, "modules/admin/contests-page.tsx"),
    "utf8",
  );
  assert.match(source, /canPublish/);
  assert.match(source, /AUTHORIZATION_ACTION\.UPDATE/);
  assert.match(source, /canPublish\s*&&/);
});

test("user and role security-sensitive mutations require authorization manage", async () => {
  const [users, roles] = await Promise.all([
    readFile(path.join(ROOT, "modules/admin/users-page.tsx"), "utf8"),
    readFile(path.join(ROOT, "modules/admin/roles-page.tsx"), "utf8"),
  ]);

  assert.match(users, /canManageUsers/);
  assert.match(users, /AUTHORIZATION_ACTION\.MANAGE/);
  assert.match(users, /refreshCapabilities/);
  assert.match(roles, /canManagePolicies/);
  assert.match(roles, /canDeleteRole/);
  assert.match(roles, /roleService\.previewMetadata/);
  assert.match(roles, /roleService\.applyMetadata/);
  assert.match(roles, /"archive"/);
  assert.match(roles, /AUTHORIZATION_ACTION\.MANAGE/);
  assert.match(roles, /refreshCapabilities/);
});

test("problem testcase controls are committed through one versioned problem draft", async () => {
  const source = await readFile(
    path.join(ROOT, "modules/admin/problem-form.tsx"),
    "utf8",
  );
  assert.match(source, /AUTHORIZATION_RESOURCE\.PROBLEM/);
  assert.match(source, /problemService\.saveDraft/);
  assert.match(source, /groups:\s*value\.groups\.map/);
  assert.doesNotMatch(source, /testCaseService|testcaseService/);
});
