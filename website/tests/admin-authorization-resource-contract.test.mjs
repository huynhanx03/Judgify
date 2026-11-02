import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("role, catalog, and selected policy reads are independent abortable resources", async () => {
  const [page, service] = await Promise.all([
    source("modules/admin/roles-page.tsx"),
    source("services/role.service.ts"),
  ]);

  assert.match(page, /rolesResource = useRetryableResource<Role\[\]>/);
  assert.match(
    page,
    /catalogResource = useRetryableResource<AuthorizationCatalog \| null>/,
  );
  assert.match(
    page,
    /policyResource = useRetryableResource<RolePolicySnapshot \| null>/,
  );
  assert.match(page, /policyResource\.retry/);
  assert.doesNotMatch(
    page,
    /Promise\.all\(|policyRequestRef|\bloadFoundation\b|\bloadPolicies\b/,
  );
  assert.match(service, /getCatalog\(signal\?: AbortSignal\)/);
  assert.match(
    service,
    /getPolicies\([\s\S]*signal\?: AbortSignal/,
  );
});
