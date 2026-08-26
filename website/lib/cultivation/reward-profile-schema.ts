import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import {
  sha256ChecksumSchema,
  traitEffectRevisionSchema,
  traitTypeSchema,
} from "@/lib/cultivation/trait-schema";
import type {
  RewardProfileAssignmentPreview,
  RewardProfileResponse,
  RewardProfileTrait,
} from "@/types/cultivation";

const rewardProfileSlotSchema = enumSchema(["root_bone", "talent"] as const);
const rewardProfileSourceSchema = enumSchema([
  "onboarding",
  "administrative",
] as const);

const rewardProfileTraitSchema = strictObjectSchema({
  trait_id: entityIDSchema,
  trait_code: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  trait_type: traitTypeSchema,
  trait_name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
  rarity_code: stringSchema({ minimumLength: 1, maximumLength: 20 }),
  rarity_name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
  slot: rewardProfileSlotSchema,
  slot_order: integerSchema({ minimum: 0, maximum: 2 }),
  effect_revision: traitEffectRevisionSchema,
}) as Schema<RewardProfileTrait>;

function assertRewardProfileTraits(
  traits: RewardProfileTrait[],
  path: string,
): void {
  if (traits.length !== 4) {
    throw new TypeError(`${path}: reward profile must contain four traits`);
  }
  const identities = new Set<string>();
  const talentOrders: number[] = [];
  let roots = 0;
  for (const trait of traits) {
    if (identities.has(trait.trait_id)) {
      throw new TypeError(`${path}: duplicate reward profile trait`);
    }
    identities.add(trait.trait_id);
    if (trait.slot === "root_bone") {
      if (trait.trait_type !== "root_bone" || trait.slot_order !== 0) {
        throw new TypeError(`${path}: invalid root-bone assignment`);
      }
      roots += 1;
    } else {
      if (trait.trait_type !== "talent") {
        throw new TypeError(`${path}: invalid talent assignment`);
      }
      talentOrders.push(trait.slot_order);
    }
  }
  talentOrders.sort((left, right) => left - right);
  if (
    roots !== 1 ||
    talentOrders.length !== 3 ||
    talentOrders.some((value, index) => value !== index)
  ) {
    throw new TypeError(`${path}: invalid reward profile slot topology`);
  }
}

const rawRewardProfileSchema = strictObjectSchema({
  id: entityIDSchema,
  user_id: entityIDSchema,
  revision: integerSchema({ minimum: 1, label: "reward profile revision" }),
  assignment_checksum: sha256ChecksumSchema,
  source: rewardProfileSourceSchema,
  source_offer_id: optionalSchema(entityIDSchema),
  command_id: optionalSchema(entityIDSchema),
  effective_at: isoDateTimeSchema,
  traits: arraySchema(rewardProfileTraitSchema, { maximumLength: 4 }),
  replayed: optionalSchema(booleanSchema),
});

export const rewardProfileSchema: Schema<RewardProfileResponse> = {
  parse(value: unknown, path = "$"): RewardProfileResponse {
    const profile = rawRewardProfileSchema.parse(value, path);
    assertRewardProfileTraits(profile.traits, `${path}.traits`);
    if (
      (profile.source === "onboarding" &&
        (!profile.source_offer_id || profile.command_id !== undefined)) ||
      (profile.source === "administrative" &&
        (!profile.command_id || profile.source_offer_id !== undefined))
    ) {
      throw new TypeError(`${path}: reward profile provenance is invalid`);
    }
    const profileTime = Date.parse(profile.effective_at);
    if (
      profile.traits.some(
        (trait) => Date.parse(trait.effect_revision.effective_at) > profileTime,
      )
    ) {
      throw new TypeError(`${path}: reward profile references a future effect`);
    }
    return profile as RewardProfileResponse;
  },
};

const rawRewardProfilePreviewSchema = strictObjectSchema({
  user_id: entityIDSchema,
  current_revision: integerSchema({ minimum: 1 }),
  next_revision: integerSchema({ minimum: 2 }),
  traits: arraySchema(rewardProfileTraitSchema, { maximumLength: 4 }),
  preview_checksum: sha256ChecksumSchema,
});

export const rewardProfilePreviewSchema: Schema<RewardProfileAssignmentPreview> = {
  parse(value: unknown, path = "$"): RewardProfileAssignmentPreview {
    const preview = rawRewardProfilePreviewSchema.parse(value, path);
    assertRewardProfileTraits(preview.traits, `${path}.traits`);
    if (preview.next_revision !== preview.current_revision + 1) {
      throw new TypeError(`${path}: reward profile preview revision is invalid`);
    }
    return preview as RewardProfileAssignmentPreview;
  },
};
