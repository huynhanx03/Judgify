import assert from "node:assert/strict";
import { test } from "vitest";

import { vi } from "@/i18n/catalog.vi";
import { admin } from "@/i18n/catalog.admin.vi";
import { text } from "@/i18n/text";
import { adminText } from "@/i18n/admin-text";

function visitCatalog(
  value: unknown,
  path: string[] = [],
  seen = new Set<string>(),
): void {
  if (typeof value === "string") {
    assert.ok(value.trim().length > 0, `${path.join(".")} must not be empty`);
    assert.ok(!seen.has(path.join(".")), `${path.join(".")} is duplicated`);
    seen.add(path.join("."));
    return;
  }

  if (typeof value === "function") {
    assert.ok(!seen.has(path.join(".")), `${path.join(".")} is duplicated`);
    seen.add(path.join("."));
    return;
  }

  assert.ok(value && typeof value === "object", `${path.join(".")} is not a catalog branch`);
  for (const [key, child] of Object.entries(value)) {
    visitCatalog(child, [...path, key], seen);
  }
}

test("Vietnamese catalog contains only non-empty branches and leaves", () => {
  visitCatalog(vi);
});

test("Vietnamese admin catalog contains only non-empty branches and leaves", () => {
  visitCatalog(admin);
});

test("typed navigation labels resolve through public catalog paths", () => {
  assert.equal(text("NAV.ARENA"), vi.NAV.ARENA);
  assert.equal(text("COMMON.CLOSE"), vi.COMMON.CLOSE);
});

test("typed admin labels resolve through admin catalog paths", () => {
  assert.equal(adminText("NAV.OVERVIEW"), admin.NAV.OVERVIEW);
});

test("named interpolation returns markup-like values as plain text", () => {
  const result = text("COMMON.ITEM_COUNT", { count: "<script>" });

  assert.equal(result, "<script> mục");
});

test("formatter leaves remain callable through the typed path accessor", () => {
  assert.equal(text("COMMON.GO_TO_PAGE", { args: [3] }), "Đi đến trang 3");
});
