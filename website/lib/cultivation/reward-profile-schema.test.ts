import assert from "node:assert/strict";
import { test } from "vitest";

import {
  rewardProfilePreviewSchema,
  rewardProfileSchema,
} from "@/lib/cultivation/reward-profile-schema";

const USER_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";
const PROFILE_TIME = "2026-08-01T12:00:00.000Z";
const EFFECT_TIME = "2026-08-01T11:59:59.000Z";

function profileTrait(
  index: number,
  traitType: "root_bone" | "talent",
  slotOrder: number,
) {
  const suffix = (index + 32).toString(16).padStart(2, "0");
  return {
    trait_id: `019f6abb-8dd5-7581-8449-2b9ad77873${suffix}`,
    trait_code: `${traitType}_${index}`,
    trait_type: traitType,
    trait_name: `${traitType} ${index}`,
    rarity_code: "rare",
    rarity_name: "Hiếm",
    slot: traitType,
    slot_order: slotOrder,
    effect_revision: {
      id: `019f6abb-8dd5-7581-8449-2b9ad77874${suffix}`,
      revision: 1,
      effect: {
        v: 1,
        kind: "exp_bonus",
        scope: "all",
        flat_bonus: 10,
      },
      checksum: `${index + 1}`.repeat(64),
      effective_at: EFFECT_TIME,
    },
  };
}

const traits = [
  profileTrait(0, "root_bone", 0),
  profileTrait(1, "talent", 0),
  profileTrait(2, "talent", 1),
  profileTrait(3, "talent", 2),
];

const profile = {
  id: "019f6abb-8dd5-7581-8449-2b9ad77873e5",
  user_id: USER_ID,
  revision: 4,
  assignment_checksum: "a".repeat(64),
  source: "administrative",
  command_id: "019f6abb-8dd5-7581-8449-2b9ad77873e6",
  effective_at: PROFILE_TIME,
  traits,
};

test("reward profile schema preserves exact revision evidence", () => {
  const parsed = rewardProfileSchema.parse(profile);
  assert.equal(parsed.revision, 4);
  assert.equal(parsed.traits.length, 4);
  assert.deepEqual(
    parsed.traits.filter((trait) => trait.slot === "talent").map((trait) => trait.slot_order),
    [0, 1, 2],
  );
});

test("reward profile schema rejects invalid topology, provenance, and time travel", () => {
  assert.throws(() =>
    rewardProfileSchema.parse({
      ...profile,
      source: "onboarding",
    }),
  );
  assert.throws(() =>
    rewardProfileSchema.parse({
      ...profile,
      source_offer_id: "019f6abb-8dd5-7581-8449-2b9ad77873e7",
    }),
  );
  assert.throws(() =>
    rewardProfileSchema.parse({
      ...profile,
      traits: [traits[0], traits[1], traits[2], traits[2]],
    }),
  );
  assert.throws(() =>
    rewardProfileSchema.parse({
      ...profile,
      traits: traits.map((trait, index) =>
        index === 3 ? { ...trait, slot_order: 1 } : trait,
      ),
    }),
  );
  assert.throws(
    () =>
      rewardProfileSchema.parse({
        ...profile,
        traits: traits.map((trait, index) =>
          index === 0
            ? {
                ...trait,
                effect_revision: {
                  ...trait.effect_revision,
                  effective_at: "2026-08-01T12:00:01.000Z",
                },
              }
            : trait,
        ),
      }),
    /future effect/,
  );
});

test("reward profile preview binds the next optimistic revision", () => {
  const preview = {
    user_id: USER_ID,
    current_revision: 4,
    next_revision: 5,
    traits,
    preview_checksum: "f".repeat(64),
  };
  assert.equal(rewardProfilePreviewSchema.parse(preview).next_revision, 5);
  assert.throws(() =>
    rewardProfilePreviewSchema.parse({ ...preview, next_revision: 6 }),
  );
});
