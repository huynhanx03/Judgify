import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();
const SERVICES = ["level", "rank", "rarity", "element"];

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("cultivation catalog services parse every response through runtime schemas", async () => {
  for (const resource of SERVICES) {
    const service = await source(`services/${resource}.service.ts`);

    assert.match(service, /from "@\/lib\/api\/client"/);
    assert.match(service, new RegExp(`schema: ${resource}PageSchema`));
    assert.match(service, new RegExp(`schema: ${resource}ListSchema`));
    assert.equal(
      service.match(new RegExp(`schema: ${resource}Schema`, "g"))?.length,
      2,
      `${resource} create and update must parse their responses`,
    );
    assert.match(service, /schema: voidSchema/);
    assert.equal(
      service.match(/schema: \w+Schema/g)?.length,
      5,
      `${resource} must parse find, list, create, update, and delete`,
    );
    assert.match(service, /entityIDSchema\.parse\(id\)/);
    assert.match(service, /getAll\(signal\?: AbortSignal\)/);
    assert.match(service, /auth: "none"/);
  }
});

test("cultivation schemas share strict UUID, pagination, and safe-integer boundaries", async () => {
  const [schema, types] = await Promise.all([
    source("lib/cultivation/catalog-schema.ts"),
    source("types/cultivation.ts"),
  ]);

  assert.match(schema, /strictObjectSchema/);
  assert.match(schema, /entityIDSchema/);
  assert.match(schema, /paginatedSchema/);
  assert.match(schema, /Number\.MAX_SAFE_INTEGER/);
  assert.match(
    schema,
    /maximumLength: options\.maximumLength \?\? MAXIMUM_CATALOG_ITEMS/,
  );
  assert.match(schema, /duplicate catalog identity/);
  assert.match(schema, /inconsistent catalog pagination/);
  assert.match(schema, /version: integerSchema/);
  assert.match(types, /interface ElementResponse[\s\S]*version: number/);
});
