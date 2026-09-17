import assert from "node:assert/strict";
import { test } from "vitest";

import {
  traitEffectSchema,
  traitListSchema,
  traitOfferSchema,
  traitPageSchema,
  traitSchema,
} from "@/lib/cultivation/trait-schema";

const ROOT_BONE_ID = "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17";
const TALENT_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";
const RARITY_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e6";
const EFFECT_REVISION_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e7";
const EXPIRY = "2026-08-01T12:00:00.000Z";
const SERVER_TIME = "2026-08-01T11:50:00.000Z";

const rarity = {
  id: RARITY_ID,
  name: "Hiếm",
  code: "rare",
  weight: 100,
};

const rootBone = {
  id: ROOT_BONE_ID,
  code: "spirit_root",
  type: "root_bone",
  name: "Linh căn",
  rarity,
  description: "Tăng kinh nghiệm ổn định.",
  display_order: 10,
  active: true,
  effect_revision: {
    id: EFFECT_REVISION_ID,
    revision: 1,
    effect: {
      v: 1,
      kind: "exp_multiplier",
      scope: "all",
      multiplier_delta_bps: 1_500,
    },
    checksum: "c".repeat(64),
    effective_at: SERVER_TIME,
  },
  version: 1,
};

const offerTrait = {
  id: TALENT_ID,
  revision: 1,
  type: "talent",
  name: "Tâm pháp",
  rarity_id: RARITY_ID,
  rarity_code: "rare",
  rarity_name: "Hiếm",
};

const offer = {
  id: "019f6abb-8dd5-7581-8449-2b9ad77873ec",
  sequence: 1,
  catalog_revision_id: "019f6abb-8dd5-7581-8449-2b9ad77873ed",
  catalog_checksum: "a".repeat(64),
  algorithm_version: "weighted-without-replacement-v1",
  offer_checksum: "b".repeat(64),
  root_bone: { ...offerTrait, id: ROOT_BONE_ID, type: "root_bone" },
  talents: [
    offerTrait,
    { ...offerTrait, id: "019f6abb-8dd5-7581-8449-2b9ad77873e8" },
    { ...offerTrait, id: "019f6abb-8dd5-7581-8449-2b9ad77873e9" },
    { ...offerTrait, id: "019f6abb-8dd5-7581-8449-2b9ad77873ea" },
    { ...offerTrait, id: "019f6abb-8dd5-7581-8449-2b9ad77873eb" },
    { ...offerTrait, id: "019f6abb-8dd5-7581-8449-2b9ad77873ef" },
  ],
  remaining_offers: 2,
  expires_at: EXPIRY,
  server_time: SERVER_TIME,
};

test("trait schema accepts the closed immutable effect contract", () => {
  const parsed = traitSchema.parse(rootBone);
  assert.equal(parsed.effect_revision?.effect.kind, "exp_multiplier");
  assert.equal(
    parsed.effect_revision?.effect.kind === "exp_multiplier"
      ? parsed.effect_revision.effect.multiplier_delta_bps
      : undefined,
    1_500,
  );

  const withoutOptionalJoins = traitSchema.parse({
    ...rootBone,
    rarity: null,
    effect_revision: null,
  });
  assert.equal(withoutOptionalJoins.rarity, undefined);
  assert.equal(withoutOptionalJoins.effect_revision, undefined);

  assert.throws(() => traitSchema.parse({ ...rootBone, type: "unknown" }));
  assert.throws(() => traitSchema.parse({ ...rootBone, unexpected: true }), /unknown field/);
});

test("trait effect schema rejects semantic drift and non-canonical targets", () => {
  assert.deepEqual(
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_bonus",
      scope: "element",
      element_codes: ["fire", "water"],
      flat_bonus: 25,
    }),
    {
      v: 1,
      kind: "exp_bonus",
      scope: "element",
      element_codes: ["fire", "water"],
      flat_bonus: 25,
    },
  );

  assert.throws(() =>
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_multiplier",
      scope: "all",
      multiplier_delta_bps: 1.5,
    }),
  );
  assert.throws(() =>
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_bonus",
      scope: "all",
      element_codes: ["fire"],
      flat_bonus: 25,
    }),
  );
  assert.throws(() =>
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_bonus",
      scope: "element",
      flat_bonus: 25,
    }),
  );
  assert.throws(() =>
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_bonus",
      scope: "element",
      element_codes: ["water", "fire"],
      flat_bonus: 25,
    }),
  );
  assert.throws(() =>
    traitEffectSchema.parse({
      v: 1,
      kind: "exp_bonus",
      scope: "all",
      flat_bonus: 25,
      daily_limit: 3,
    }),
  );
});

test("trait pages and lists reject duplicate identities and invalid pagination", () => {
  const pagination = {
    current_page: 1,
    page_size: 10,
    total_pages: 1,
    total_items: 1,
    has_next: false,
    has_prev: false,
  };
  assert.equal(traitListSchema.parse([rootBone])[0]?.id, ROOT_BONE_ID);
  assert.equal(
    traitPageSchema.parse({ records: [rootBone], pagination }).records.length,
    1,
  );
  assert.throws(
    () => traitListSchema.parse([rootBone, rootBone]),
    /duplicate catalog identity/,
  );
  assert.throws(
    () =>
      traitPageSchema.parse({
        records: [rootBone],
        pagination: { ...pagination, total_items: 0 },
      }),
    /inconsistent catalog pagination/,
  );
});

test("trait offer schema exposes display evidence only and enforces uniqueness", () => {
  const parsed = traitOfferSchema.parse(offer);
  assert.equal(parsed.root_bone.type, "root_bone");
  assert.equal(parsed.talents.length, 6);

  assert.throws(() =>
    traitOfferSchema.parse({
      ...offer,
      root_bone: { ...offer.root_bone, effect_revision: rootBone.effect_revision },
    }),
  );
  assert.throws(
    () => traitOfferSchema.parse({ ...offer, talents: offer.talents.slice(0, 5) }),
    /cardinality/,
  );
  assert.throws(
    () =>
      traitOfferSchema.parse({
        ...offer,
        talents: [
          { ...offer.talents[0], id: ROOT_BONE_ID },
          ...offer.talents.slice(1),
        ],
      }),
    /invalid onboarding trait offer member/,
  );
  assert.throws(() =>
    traitOfferSchema.parse({ ...offer, expires_at: "not-a-timestamp" }),
  );
  assert.throws(() => traitOfferSchema.parse({ ...offer, offer_checksum: "not-a-digest" }));
  assert.throws(
    () => traitOfferSchema.parse({ ...offer, server_time: EXPIRY }),
    /already expired/,
  );
});
