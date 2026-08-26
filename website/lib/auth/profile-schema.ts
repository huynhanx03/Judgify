import {
  arraySchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import type {
  CultivationInfo,
  DiffStat,
  ElementBrief,
  ElementExp,
  LevelInfo,
  ProblemStats,
  ProfileRewardInfo,
  ProfileAttrs,
  RankInfo,
  TagStat,
  TraitInfo,
  TraitEffectInfo,
  UserProfile,
} from "@/types/user";
import type { Gender } from "@/constants/gender";

const MAX_SAFE = Number.MAX_SAFE_INTEGER;
const dateSchema = stringSchema({
  pattern: /^\d{4}-\d{2}-\d{2}$/,
  label: "calendar date",
});
const nonNegativeInteger = (label: string) =>
  integerSchema({ minimum: 0, maximum: MAX_SAFE, label });

const signedInteger = (label: string) =>
  integerSchema({ minimum: -MAX_SAFE, maximum: MAX_SAFE, label });

const genderSchema: Schema<Gender> = {
  parse(value: unknown, path = "$"): Gender {
    return integerSchema({ minimum: 0, maximum: 2, label: "gender" }).parse(
      value,
      path,
    ) as Gender;
  },
};

const checksumSchema = stringSchema({
  minimumLength: 64,
  maximumLength: 64,
  pattern: /^[a-f0-9]{64}$/,
  label: "checksum",
});

const traitEffectSchema: Schema<TraitEffectInfo> = strictObjectSchema({
  v: integerSchema({ minimum: 1, maximum: 32, label: "effect version" }),
  kind: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  scope: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  element_codes: optionalSchema(
    arraySchema(stringSchema({ minimumLength: 1, maximumLength: 64 }), {
      maximumLength: 256,
      unique: true,
    }),
  ),
  multiplier_delta_bps: optionalSchema(signedInteger("effect multiplier")),
  flat_bonus: optionalSchema(signedInteger("effect flat bonus")),
});

const traitSchema: Schema<TraitInfo> = strictObjectSchema({
  id: entityIDSchema,
  code: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  type: enumSchema(["root_bone", "talent"] as const),
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  rarity_code: optionalSchema(stringSchema({ maximumLength: 64 })),
  rarity_name: optionalSchema(stringSchema({ maximumLength: 255 })),
  description: optionalSchema(stringSchema({ maximumLength: 16 * 1024 })),
  slot: enumSchema(["root_bone", "talent"] as const),
  slot_order: integerSchema({ minimum: 0, maximum: 3, label: "trait slot" }),
  effect_revision_id: entityIDSchema,
  effect_revision: integerSchema({ minimum: 1, maximum: MAX_SAFE }),
  effect_checksum: checksumSchema,
  effect_effective_at: isoDateTimeSchema,
  effect: traitEffectSchema,
});

const profileRewardSchema: Schema<ProfileRewardInfo> = strictObjectSchema({
  id: entityIDSchema,
  revision: integerSchema({ minimum: 1, maximum: MAX_SAFE }),
  checksum: checksumSchema,
  source: enumSchema(["onboarding", "administrative"] as const),
  effective_at: isoDateTimeSchema,
});

const elementExpSchema: Schema<ElementExp> = strictObjectSchema({
  code: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  exp: nonNegativeInteger("element experience"),
});

const levelInfoSchema: Schema<LevelInfo> = strictObjectSchema({
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  next_name: optionalSchema(stringSchema({ maximumLength: 255 })),
  tier_index: integerSchema({ minimum: 0, maximum: 256, label: "level tier" }),
  next_tier_index: optionalSchema(
    integerSchema({ minimum: 0, maximum: 256, label: "next level tier" }),
  ),
  progress: {
    parse(value: unknown, path = "$") {
      if (
        typeof value !== "number" ||
        !Number.isFinite(value) ||
        value < 0 ||
        value > 100
      ) {
        throw new TypeError(`${path}: invalid level progress`);
      }
      return value;
    },
  },
  exp_to_next: nonNegativeInteger("experience to next level"),
});

const rankInfoSchema: Schema<RankInfo> = strictObjectSchema({
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  next_name: optionalSchema(stringSchema({ maximumLength: 255 })),
  tier_index: integerSchema({ minimum: 0, maximum: 256, label: "rank tier" }),
  next_tier_index: optionalSchema(
    integerSchema({ minimum: 0, maximum: 256, label: "next rank tier" }),
  ),
  progress: {
    parse(value: unknown, path = "$") {
      if (
        typeof value !== "number" ||
        !Number.isFinite(value) ||
        value < 0 ||
        value > 100
      ) {
        throw new TypeError(`${path}: invalid rank progress`);
      }
      return value;
    },
  },
  rating_to_next: nonNegativeInteger("rating to next rank"),
});

const elementBriefSchema: Schema<ElementBrief> = strictObjectSchema({
  code: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
});

const diffStatSchema: Schema<DiffStat> = strictObjectSchema({
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  level: integerSchema({ minimum: 1, maximum: MAX_SAFE, label: "difficulty level" }),
  solved_count: nonNegativeInteger("solved problem count"),
});

const tagStatSchema: Schema<TagStat> = strictObjectSchema({
  name: stringSchema({ minimumLength: 1, maximumLength: 255 }),
  solved_count: nonNegativeInteger("solved problem count"),
  elements: arraySchema(elementBriefSchema, { maximumLength: 4_096 }),
});

const cultivationSchema: Schema<CultivationInfo> = strictObjectSchema({
  total_exp: nonNegativeInteger("total experience"),
  rating: signedInteger("rating"),
  reward_profile: profileRewardSchema,
  level: levelInfoSchema,
  rank: rankInfoSchema,
  root_bone: traitSchema,
  talents: arraySchema(traitSchema, { minimumLength: 3, maximumLength: 3 }),
  elements: arraySchema(elementExpSchema, { maximumLength: 256 }),
});

const problemStatsSchema: Schema<ProblemStats> = {
  parse(value: unknown, path = "$") {
    const parsed = strictObjectSchema({
      total_submissions: nonNegativeInteger("submission count"),
      accepted_count: nonNegativeInteger("accepted submission count"),
      by_difficulty: arraySchema(diffStatSchema, { maximumLength: 1_024 }),
      by_tag: arraySchema(tagStatSchema, { maximumLength: 1_024 }),
    }).parse(value, path);
    if (parsed.accepted_count > parsed.total_submissions) {
      throw new TypeError(`${path}: accepted submissions exceed submissions`);
    }
    return parsed;
  },
};

export const profileAttrsSchema: Schema<ProfileAttrs> = strictObjectSchema({
  id: entityIDSchema,
  username: stringSchema({ minimumLength: 3, maximumLength: 50 }),
  first_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  last_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  gender: genderSchema,
  birthday: optionalSchema(dateSchema),
  joined_at: optionalSchema(isoDateTimeSchema),
}) as Schema<ProfileAttrs>;

const readyProfileSchema = strictObjectSchema({
  profile_state: enumSchema(["ready"] as const),
  id: entityIDSchema,
  username: stringSchema({ minimumLength: 3, maximumLength: 50 }),
  first_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  last_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  gender: genderSchema,
  birthday: optionalSchema(dateSchema),
  joined_at: isoDateTimeSchema,
  cultivation: cultivationSchema,
  problem_stats: problemStatsSchema,
});

const unprovisionedProfileSchema = strictObjectSchema({
  profile_state: enumSchema(["unprovisioned"] as const),
  id: entityIDSchema,
  username: stringSchema({ minimumLength: 3, maximumLength: 50 }),
  first_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  last_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 100 })),
  gender: genderSchema,
  birthday: optionalSchema(dateSchema),
  joined_at: isoDateTimeSchema,
});

export const profileSchema: Schema<UserProfile> = {
  parse(value: unknown, path = "$"): UserProfile {
    const source = recordAt(value, path);
    if (source.profile_state === "ready") {
      return readyProfileSchema.parse(source, path);
    }
    if (source.profile_state === "unprovisioned") {
      return unprovisionedProfileSchema.parse(source, path);
    }
    throw new TypeError(`${path}.profile_state: invalid profile state`);
  },
};
