import assert from "node:assert/strict";
import { test } from "vitest";

import { auditLogPageSchema } from "@/lib/audit/audit-schema";

const entry = {
  id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
  actor_user_id: "019f6abb-8dd5-7581-8449-2b9ad77873e5",
  action: "problem.updated",
  resource: "problem",
  resource_id: "problem-1",
  reason: "corrected limits",
  before: { time_limit_ms: 1_000 },
  after: { time_limit_ms: 2_000 },
  correlation_id: "019f6abb-8dd5-7581-8449-2b9ad77873e6",
  occurred_at: "2026-08-01T00:00:00Z",
};

test("audit page parses a bounded immutable projection", () => {
  const page = auditLogPageSchema.parse({
    items: [entry],
    has_more: false,
  });
  assert.equal(page.items[0]?.action, "problem.updated");
});

test("audit page rejects identity duplication and cursor drift", () => {
  assert.throws(
    () => auditLogPageSchema.parse({ items: [entry, entry], has_more: false }),
    /duplicate audit identity/,
  );
  assert.throws(
    () => auditLogPageSchema.parse({ items: [entry], has_more: true }),
    /inconsistent audit cursor/,
  );
});
