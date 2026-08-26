import assert from "node:assert/strict";
import { test } from "vitest";

import {
  rankingPageSchema,
  rankingPrivacySchema,
} from "@/lib/cultivation/ranking-schema";

const PUBLIC_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";
const SECOND_PUBLIC_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e6";

const publicEntry = {
  rank: 1,
  public_id: PUBLIC_ID,
  display_name: "Đạo hữu An",
  display_mode: "public",
  rating: 1400,
  rank_title: "Kim Đan",
  total_exp: 8000,
  level_name: "Trúc Cơ",
  level: 3,
};

test("ranking pages accept only privacy-safe stable projections", () => {
  const page = rankingPageSchema.parse({
    entries: [
      publicEntry,
      {
        ...publicEntry,
        rank: 2,
        public_id: SECOND_PUBLIC_ID,
        display_mode: "anonymous",
        display_name: undefined,
      },
    ],
    next_cursor: "opaque-cursor",
    projection_revision: 8,
  });
  assert.equal(page.entries[1]?.display_name, undefined);

  assert.throws(
    () => rankingPageSchema.parse({
      entries: [{ ...publicEntry, display_mode: "anonymous" }],
      projection_revision: 8,
    }),
    /privacy mode/,
  );
  assert.throws(
    () => rankingPageSchema.parse({
      entries: [publicEntry, { ...publicEntry, rank: 2 }],
      projection_revision: 8,
    }),
    /duplicate public identity/,
  );
  assert.throws(
    () => rankingPageSchema.parse({
      entries: [publicEntry, { ...publicEntry, rank: 1, public_id: SECOND_PUBLIC_ID }],
      projection_revision: 8,
    }),
    /unstable ranking order/,
  );
});

test("ranking contracts reject private identity leakage and invalid privacy state", () => {
  assert.throws(
    () => rankingPageSchema.parse({
      entries: [{ ...publicEntry, user_id: SECOND_PUBLIC_ID }],
      projection_revision: 8,
    }),
    /unknown field/,
  );
  assert.throws(
    () => rankingPrivacySchema.parse({
      visibility: "anonymous",
      version: 0,
      projection_revision: 8,
    }),
    /privacy version/,
  );
});
