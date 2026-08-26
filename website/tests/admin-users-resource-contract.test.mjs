import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("admin users and role catalog load independently with stale-response fencing", async () => {
  const page = await source("modules/admin/users-page.tsx");

  assert.match(
    page,
    /usersResource = useRetryableResource<Paginated<AdminUser> \| null>/,
  );
  assert.match(page, /rolesResource = useRetryableResource<Role\[\]>/);
  assert.match(page, /usersResource\.retry/);
  assert.match(page, /rolesResource\.retry/);
  assert.doesNotMatch(
    page,
    /loadSequence|refreshKey|roleRefreshKey|let active/,
  );
});
