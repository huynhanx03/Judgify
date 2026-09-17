import assert from "node:assert/strict";
import { test } from "vitest";

import { profileAttrsSchema, profileSchema } from "@/lib/auth/profile-schema";

const USER_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";
const JOINED_AT = "2026-08-01T00:00:00Z";

const profile = {
  profile_state: "ready",
  id: USER_ID,
  username: "judgify_user",
  first_name: "Judge",
  last_name: "User",
  gender: 0,
  birthday: "2000-01-01",
  joined_at: JOINED_AT,
  cultivation: {
    total_exp: 100,
    rating: 1_200,
    reward_profile: {
      id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e18",
      revision: 1,
      checksum: "a".repeat(64),
      source: "onboarding",
      effective_at: JOINED_AT,
    },
    level: {
      name: "Nhập môn",
      tier_index: 0,
      progress: 50,
      exp_to_next: 100,
    },
    rank: {
      name: "Đồng",
      tier_index: 0,
      progress: 20,
      rating_to_next: 200,
    },
    root_bone: {
      id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e19",
      code: "steadfast",
      type: "root_bone",
      name: "Steadfast",
      slot: "root_bone",
      slot_order: 0,
      effect_revision_id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e20",
      effect_revision: 1,
      effect_checksum: "b".repeat(64),
      effect_effective_at: JOINED_AT,
      effect: { v: 1, kind: "experience", scope: "global" },
    },
    talents: [1, 2, 3].map((slot) => ({
      id: `018f5fbe-3d77-7e10-8fd1-8b6f1ca32e2${slot}`,
      code: `talent-${slot}`,
      type: "talent",
      name: `Talent ${slot}`,
      slot: "talent",
      slot_order: slot,
      effect_revision_id: `018f5fbe-3d77-7e10-8fd1-8b6f1ca32e3${slot}`,
      effect_revision: 1,
      effect_checksum: "c".repeat(64),
      effect_effective_at: JOINED_AT,
      effect: { v: 1, kind: "experience", scope: "global" },
    })),
    elements: [],
  },
  problem_stats: {
    total_submissions: 10,
    accepted_count: 4,
    by_difficulty: [],
    by_tag: [],
  },
};

test("rich profile response is parsed recursively", () => {
  const parsed = profileSchema.parse(profile);
  assert.equal(parsed.profile_state, "ready");
  if (parsed.profile_state !== "ready") {
    throw new Error("expected a ready profile");
  }
  assert.equal(parsed.id, USER_ID);
  assert.equal(parsed.cultivation.rating, 1_200);
  assert.equal(parsed.problem_stats.accepted_count, 4);
});

test("profile boundary rejects impossible aggregates and response drift", () => {
  assert.throws(
    () =>
      profileSchema.parse({
        ...profile,
        problem_stats: {
          ...profile.problem_stats,
          accepted_count: 11,
        },
      }),
    /accepted submissions exceed submissions/,
  );
  assert.throws(
    () => profileSchema.parse({ ...profile, internal_secret: true }),
    /unknown field/,
  );
});

test("compact profile mutation keeps joined_at optional but validates it", () => {
  assert.equal(
    profileAttrsSchema.parse({
      id: USER_ID,
      username: "judgify_user",
      first_name: "Judge",
      last_name: "User",
      gender: 0,
      birthday: "2000-01-01",
    }).joined_at,
    undefined,
  );
  assert.throws(
    () =>
      profileAttrsSchema.parse({
        id: USER_ID,
        username: "judgify_user",
        first_name: "Judge",
        last_name: "User",
        gender: 0,
        joined_at: "01/08/2026",
      }),
    /RFC 3339/,
  );
});
