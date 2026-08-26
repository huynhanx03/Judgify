import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("identity admin services validate server-owned role and user projections", async () => {
  const [roles, users, authorization] = await Promise.all([
    source("services/role.service.ts"),
    source("services/user.service.ts"),
    source("services/authorization.service.ts"),
  ]);
  assert.match(roles, /roleListSchema/);
  assert.match(roles, /authorizationCatalogSchema/);
  assert.match(roles, /rolePolicySchema/);
  assert.match(users, /adminUserPageSchema/);
  assert.match(users, /userInvitationPreviewResponseSchema/);
  assert.match(users, /userInvitationReceiptSchema/);
  assert.match(users, /userRoleMutationReceiptSchema/);
  assert.match(users, /userLifecyclePreviewResponseSchema/);
  assert.match(users, /userLifecycleReceiptSchema/);
  assert.match(authorization, /effectiveCapabilitiesSchema/);
  assert.match(roles, /from ["']@\/lib\/api\/client["']/);
  assert.match(authorization, /from ["']@\/lib\/api\/client["']/);
});
