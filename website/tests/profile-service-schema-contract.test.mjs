import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();
const source = (relativePath) => readFile(path.join(ROOT, relativePath), "utf8");

test("profile service parses both rich reads and compact mutation responses", async () => {
  const service = await source("services/user.service.ts");
  assert.match(service, /schema: profileSchema/);
  assert.match(service, /schema: profileAttributesResponseSchema/);
  assert.match(service, /getProfile\(signal\?: AbortSignal\)/);
  assert.match(service, /from ["']@\/lib\/api\/client["']/);
});

test("profile network schema owns bounded nested response contracts", async () => {
  const schema = await source("lib/auth/profile-schema.ts");
  assert.match(schema, /accepted submissions exceed submissions/);
  assert.match(schema, /isoDateTimeSchema/);
  assert.match(schema, /MAX_SAFE/);
  assert.match(schema, /strictObjectSchema/);
});
