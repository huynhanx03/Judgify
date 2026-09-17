import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("shared admin CRUD lists use the abortable resource lifecycle", async () => {
  const hook = await source("modules/admin/hooks/use-paginated-crud.ts");

  assert.match(hook, /useRetryableResource<Paginated<T> \| null>/);
  assert.match(hook, /service\.find\(query, signal\)/);
  assert.match(hook, /keepPreviousData:\s*true/);
  assert.match(hook, /listResource\.retry/);
  assert.doesNotMatch(
    hook,
    /let cancelled|refreshKey|setIsLoading|serviceRef/,
  );
});
