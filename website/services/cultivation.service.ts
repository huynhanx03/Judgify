import { ONBOARDING_TRAIT_API, TRAIT_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import { preSessionBootstrap } from "@/lib/auth/pre-session";
import {
  traitEffectSchema,
  traitOfferSchema,
  traitListSchema,
  traitPageSchema,
  traitSchema,
  traitTypeSchema,
} from "@/lib/cultivation/trait-schema";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  TraitEffectSpec,
  TraitOfferResponse,
  TraitResponse,
  TraitType,
} from "@/types/cultivation";

export type TraitMutationInput = {
  code: string;
  type: TraitType;
  name: string;
  rarity_id: string;
  description?: string;
  display_order: number;
  effect: TraitEffectSpec;
  reason: string;
};

export type TraitUpdateInput = Partial<Omit<TraitMutationInput, "code" | "reason">> & {
  expected_version: number;
  reason: string;
  active?: boolean;
};

function normalizeTraitInput(data: TraitMutationInput | TraitUpdateInput) {
  const effect = data.effect
    ? traitEffectSchema.parse({
        ...data.effect,
        ...(data.effect.scope === "element"
          ? { element_codes: [...(data.effect.element_codes ?? [])].sort() }
          : { element_codes: undefined }),
      })
    : undefined;
  return {
    ...data,
    ...(data.type === undefined
      ? {}
      : { type: traitTypeSchema.parse(data.type) }),
    ...(data.rarity_id === undefined
      ? {}
      : { rarity_id: entityIDSchema.parse(data.rarity_id) }),
    ...(effect === undefined ? {} : { effect }),
  };
}

export const cultivationService = {
  // Traits — public reads.
  getAllTraits(signal?: AbortSignal): Promise<TraitResponse[]> {
    return api<TraitResponse[], never>(TRAIT_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: traitListSchema,
    });
  },

  findTraits(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<TraitResponse>> {
    return api<Paginated<TraitResponse>, QueryOptions>(TRAIT_API.FIND, {
      method: "POST",
      auth: "none",
      body: query,
      signal,
      schema: traitPageSchema,
    });
  },

  // Traits — administrative mutations.
  createTrait(data: TraitMutationInput): Promise<TraitResponse> {
    const body = normalizeTraitInput(data);
    return api<TraitResponse, typeof body>(TRAIT_API.CREATE, {
      method: "POST",
      body,
      schema: traitSchema,
    });
  },

  updateTrait(id: string, data: TraitUpdateInput): Promise<TraitResponse> {
    const body = normalizeTraitInput(data);
    return api<TraitResponse, typeof body>(
      TRAIT_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body,
        schema: traitSchema,
      },
    );
  },

  async deleteTrait(
    id: string,
    input: { expected_version: number; reason: string },
  ): Promise<void> {
    const body = {
      expected_version: input.expected_version,
      reason: input.reason.trim(),
    };
    await api<void, typeof body>(
      TRAIT_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", body, schema: voidSchema },
    );
  },

  // Every browser retry reuses one semantic idempotency key; a deliberate
  // reroll receives a fresh key only after the preceding offer is acknowledged.
  async createTraitOffer(signal?: AbortSignal): Promise<TraitOfferResponse> {
    await preSessionBootstrap.ensure();
    return runIdempotentCommand("onboarding-trait-offer", (idempotencyKey) =>
      api<TraitOfferResponse, never>(ONBOARDING_TRAIT_API.CREATE_OFFER, {
        method: "POST",
        auth: "none",
        retryUnauthorized: false,
        idempotencyKey,
        signal,
        schema: traitOfferSchema,
      }),
    );
  },
};
