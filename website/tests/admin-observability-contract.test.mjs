import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("admin observability is capability-gated and uses the audited debug-window API", async () => {
  const [routes, policy, navigation, api, service, control, page, text] =
    await Promise.all([
      source("constants/routes.ts"),
      source("lib/auth/admin-policy.ts"),
      source("constants/admin-navigation.ts"),
      source("constants/api/observability.ts"),
      source("services/observability.service.ts"),
      source("lib/observability/control.ts"),
      source("modules/admin/observability/admin-observability.tsx"),
      source("i18n/catalog.admin.vi.ts"),
    ]);

  assert.match(routes, /ADMIN_OBSERVABILITY:\s*"\/admin\/observability"/);
  assert.match(policy, /AUTHORIZATION_RESOURCE\.OBSERVABILITY/);
  assert.match(navigation, /APP_ROUTES\.ADMIN_OBSERVABILITY/);
  assert.match(api, /DEBUG_WINDOW/);
  assert.match(control, /commandPurpose\("debug-window-activate"/);
  assert.match(service, /debugWindowActivationPurpose/);
  assert.match(service, /idempotencyKey:\s*attempt\.attempt_id/);
  assert.match(service, /method:\s*"DELETE"/);
  assert.match(page, /AUTHORIZATION_ACTION\.UPDATE/);
  assert.match(page, /useRetryableResource<DebugWindow>/);
  assert.match(text, /OBSERVABILITY:/);
});

test("debug-window response parsing rejects partial active leases", async () => {
  const schema = await source("lib/observability/debug-window-schema.ts");

  assert.match(schema, /active debug window is incomplete/);
  assert.match(schema, /compareISODateTime/);
});
