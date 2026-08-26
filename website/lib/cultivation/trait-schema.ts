import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  literalSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type { Paginated } from "@/types/api";
import {
  uniqueCatalogListSchema,
  uniqueCatalogPageSchema,
} from "@/lib/cultivation/catalog-schema";
import type {
  TraitOfferResponse,
  TraitOfferTraitResponse,
  TraitEffectRevisionResponse,
  TraitEffectSpec,
  TraitResponse,
  TraitRarityInfo,
} from "@/types/cultivation";
import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";

export const traitTypeSchema = enumSchema(["root_bone", "talent"] as const);
export const traitEffectScopeSchema = enumSchema(["all", "element"] as const);

const elementCodeSchema = stringSchema({
  minimumLength: 1,
  maximumLength: 64,
  pattern: /^[a-z][a-z0-9_]{0,63}$/,
  label: "element code",
});

const elementCodesSchema = arraySchema(elementCodeSchema, {
  maximumLength: 16,
  unique: true,
});

const multiplierEffectSchema = strictObjectSchema({
  v: literalSchema(1),
  kind: literalSchema("exp_multiplier"),
  scope: traitEffectScopeSchema,
  element_codes: optionalSchema(elementCodesSchema),
  multiplier_delta_bps: integerSchema({
    minimum: -10_000,
    maximum: 90_000,
    label: "multiplier basis-point delta",
  }),
});

const bonusEffectSchema = strictObjectSchema({
  v: literalSchema(1),
  kind: literalSchema("exp_bonus"),
  scope: traitEffectScopeSchema,
  element_codes: optionalSchema(elementCodesSchema),
  flat_bonus: integerSchema({
    minimum: -1_000_000_000,
    maximum: 1_000_000_000,
    label: "flat experience bonus",
  }),
});

function canonicalEffectElements(
  scope: "all" | "element",
  elementCodes: string[] | undefined,
  path: string,
): string[] | undefined {
  if (scope === "all") {
    if (elementCodes !== undefined && elementCodes.length > 0) {
      throw new TypeError(`${path}.element_codes: all-scoped effect cannot target elements`);
    }
    return undefined;
  }
  if (!elementCodes || elementCodes.length === 0) {
    throw new TypeError(`${path}.element_codes: element-scoped effect requires targets`);
  }
  const sorted = [...elementCodes].sort();
  if (sorted.some((code, index) => code !== elementCodes[index])) {
    throw new TypeError(`${path}.element_codes: element codes must be canonical`);
  }
  return elementCodes;
}

export const traitEffectSchema: Schema<TraitEffectSpec> = {
  parse(value: unknown, path = "$"): TraitEffectSpec {
    const source = recordAt(value, path);
    if (source.kind === "exp_multiplier") {
      const effect = multiplierEffectSchema.parse(source, path);
      const elementCodes = canonicalEffectElements(
        effect.scope,
        effect.element_codes,
        path,
      );
      return {
        v: 1,
        kind: "exp_multiplier",
        scope: effect.scope,
        ...(elementCodes ? { element_codes: elementCodes } : {}),
        multiplier_delta_bps: effect.multiplier_delta_bps,
      };
    }
    if (source.kind === "exp_bonus") {
      const effect = bonusEffectSchema.parse(source, path);
      const elementCodes = canonicalEffectElements(
        effect.scope,
        effect.element_codes,
        path,
      );
      return {
        v: 1,
        kind: "exp_bonus",
        scope: effect.scope,
        ...(elementCodes ? { element_codes: elementCodes } : {}),
        flat_bonus: effect.flat_bonus,
      };
    }
    throw new TypeError(`${path}.kind: unsupported trait effect kind`);
  },
};

export const sha256ChecksumSchema = stringSchema({
  minimumLength: 64,
  maximumLength: 64,
  pattern: /^[0-9a-f]{64}$/,
  label: "SHA-256 checksum",
});

export const traitEffectRevisionSchema = strictObjectSchema({
  id: entityIDSchema,
  revision: integerSchema({ minimum: 1, label: "effect revision" }),
  effect: traitEffectSchema,
  checksum: sha256ChecksumSchema,
  effective_at: isoDateTimeSchema,
}) as Schema<TraitEffectRevisionResponse>;

const traitRaritySchema: Schema<TraitRarityInfo> = strictObjectSchema({
  id: entityIDSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
  code: stringSchema({ minimumLength: 1, maximumLength: 20 }),
  weight: optionalSchema(integerSchema({
    minimum: 1,
    maximum: Number.MAX_SAFE_INTEGER,
    label: "trait rarity weight",
  })),
}) as Schema<TraitRarityInfo>;

const nullableOptionalSchema = <T>(schema: Schema<T>): Schema<T | undefined> => ({
  parse(value: unknown, path = "$"): T | undefined {
    if (value === undefined || value === null) return undefined;
    return schema.parse(value, path);
  },
});

const rawTraitSchema = strictObjectSchema({
  id: entityIDSchema,
  code: stringSchema({
    minimumLength: 1,
    maximumLength: 64,
    pattern: /^[a-z][a-z0-9_]{0,63}$/,
    label: "trait code",
  }),
  type: traitTypeSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  rarity: nullableOptionalSchema(traitRaritySchema),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
  display_order: integerSchema({ minimum: 0, maximum: 1_000_000 }),
  active: booleanSchema,
  effect_revision: nullableOptionalSchema(traitEffectRevisionSchema),
  version: integerSchema({ minimum: 1, label: "trait version" }),
});

export const traitSchema: Schema<TraitResponse> = {
  parse(value: unknown, path = "$"): TraitResponse {
    const trait = rawTraitSchema.parse(value, path);
    return {
      id: trait.id,
      code: trait.code,
      type: trait.type,
      name: trait.name,
      ...(trait.rarity === undefined ? {} : { rarity: trait.rarity }),
      ...(trait.description === undefined ? {} : { description: trait.description }),
      display_order: trait.display_order,
      active: trait.active,
      ...(trait.effect_revision === undefined
        ? {}
        : { effect_revision: trait.effect_revision }),
      version: trait.version,
    };
  },
};

export const traitListSchema = uniqueCatalogListSchema(traitSchema);
export const traitPageSchema: Schema<Paginated<TraitResponse>> =
  uniqueCatalogPageSchema(traitSchema);

const traitOfferTraitSchema = strictObjectSchema({
  id: entityIDSchema,
  revision: integerSchema({ minimum: 1, label: "trait revision" }),
  type: traitTypeSchema,
  name: stringSchema({ minimumLength: 1, maximumLength: 100 }),
  description: optionalSchema(stringSchema({ maximumLength: 500 })),
  rarity_id: entityIDSchema,
  rarity_code: stringSchema({ minimumLength: 1, maximumLength: 20 }),
  rarity_name: stringSchema({ minimumLength: 1, maximumLength: 50 }),
}) as Schema<TraitOfferTraitResponse>;

const rawTraitOfferSchema = strictObjectSchema({
  id: entityIDSchema,
  sequence: integerSchema({ minimum: 1, label: "offer sequence" }),
  catalog_revision_id: entityIDSchema,
  catalog_checksum: sha256ChecksumSchema,
  algorithm_version: stringSchema({ minimumLength: 1, maximumLength: 64 }),
  offer_checksum: sha256ChecksumSchema,
  root_bone: traitOfferTraitSchema,
  talents: arraySchema(traitOfferTraitSchema, {
    maximumLength: REGISTRATION_TRAIT_RULES.OFFERED_TALENT_COUNT,
  }),
  remaining_offers: integerSchema({ minimum: 0, maximum: 10 }),
  expires_at: isoDateTimeSchema,
  server_time: isoDateTimeSchema,
});

export const traitOfferSchema: Schema<TraitOfferResponse> = {
  parse(value: unknown, path = "$"): TraitOfferResponse {
    const offer = rawTraitOfferSchema.parse(value, path);
    if (
      offer.root_bone.type !== "root_bone" ||
      offer.talents.length !== REGISTRATION_TRAIT_RULES.OFFERED_TALENT_COUNT
    ) {
      throw new TypeError(`${path}: invalid onboarding trait offer cardinality`);
    }
    const identities = new Set<string>([offer.root_bone.id]);
    for (const trait of offer.talents) {
      if (trait.type !== "talent" || identities.has(trait.id)) {
        throw new TypeError(`${path}: invalid onboarding trait offer member`);
      }
      identities.add(trait.id);
    }
    if (Date.parse(offer.expires_at) <= Date.parse(offer.server_time)) {
      throw new TypeError(`${path}: onboarding trait offer is already expired`);
    }
    return offer;
  },
};
