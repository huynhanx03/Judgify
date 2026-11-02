import assert from "node:assert/strict";
import { test } from "vitest";

import { safeAppDestination } from "@/lib/auth/safe-navigation";

test("post-auth navigation accepts only application-relative destinations", () => {
  assert.equal(safeAppDestination("/admin?tab=users", "/arena"), "/admin?tab=users");
  assert.equal(safeAppDestination("https://evil.test", "/arena"), "/arena");
  assert.equal(safeAppDestination("//evil.test/path", "/arena"), "/arena");
  assert.equal(safeAppDestination("/\\evil.test", "/arena"), "/arena");
  assert.equal(safeAppDestination("javascript:alert(1)", "/arena"), "/arena");
});
