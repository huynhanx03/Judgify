import assert from "node:assert/strict";
import { test } from "vitest";

import {
  isSafeNotificationActionPath,
  parseNotificationLivePayload,
} from "@/lib/notifications/notification-schema";

const ID = "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a";

test("notification actions allow only exact product routes and reject navigation injection", () => {
  for (const safe of ["/", "/notifications", "/profile/security", `/problems/${ID}`, `/admin/users/${ID}`]) {
    assert.equal(isSafeNotificationActionPath(safe), true, safe);
  }
  for (const unsafe of ["https://evil.example", "//evil.example", "/profile?next=//evil.example", "/problems/not-an-id", "/admin/users/../roles", "/notifications#tab", "/\\evil"]) {
    assert.equal(isSafeNotificationActionPath(unsafe), false, unsafe);
  }
});

test("live notification payload requires aggregate-version reconciliation", () => {
  const payload = { user_id: ID, unread_count: 2, inbox_version: 7 };
  assert.deepEqual(parseNotificationLivePayload(payload, 7), {
    ...payload,
    notification: undefined,
  });
  assert.throws(() => parseNotificationLivePayload(payload, 6), /version mismatch/);
});
