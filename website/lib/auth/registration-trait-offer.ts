import { REGISTRATION_TRAIT_RULES } from "@/constants/registration";
import { isoDateTimeSchema } from "@/lib/api/contracts";
import type { EntityID, ISODateTime } from "@/types/api";
import type {
  TraitOfferResponse,
  TraitOfferTraitResponse,
  TraitPresentation,
} from "@/types/cultivation";

export interface RegistrationTraitOffer {
  id: EntityID;
  sequence: number;
  catalogRevisionId: EntityID;
  catalogChecksum: string;
  algorithmVersion: string;
  offerChecksum: string;
  rootBone: TraitPresentation;
  talents: TraitPresentation[];
  remainingOffers: number;
  expiresAt: ISODateTime;
  expiresAtEpochMs: number;
}

function toPresentationTrait(trait: TraitOfferTraitResponse): TraitPresentation {
  return {
    id: trait.id,
    type: trait.type,
    name: trait.name,
    ...(trait.description === undefined
      ? {}
      : { description: trait.description }),
    rarity: {
      id: trait.rarity_id,
      code: trait.rarity_code,
      name: trait.rarity_name,
    },
  };
}

// The service schema has already rejected unknown or malformed network data;
// this adapter enforces the UI-specific lifetime and cardinality invariants and
// returns a fresh presentation model so React never owns the transport object.
export function parseRegistrationTraitOffer(
  response: TraitOfferResponse,
  requestStartedAtEpochMs: number,
): RegistrationTraitOffer | null {
  if (
    response.root_bone.type !== "root_bone" ||
    response.talents.length !== REGISTRATION_TRAIT_RULES.OFFERED_TALENT_COUNT ||
    response.talents.some((trait) => trait.type !== "talent")
  ) {
    return null;
  }
  const identities = new Set(response.talents.map((trait) => trait.id));
  if (
    identities.size !== REGISTRATION_TRAIT_RULES.OFFERED_TALENT_COUNT ||
    identities.has(response.root_bone.id)
  ) {
    return null;
  }
  const serverExpiresAtEpochMs = Date.parse(response.expires_at);
  const serverTimeEpochMs = Date.parse(response.server_time);
  if (
    !Number.isFinite(serverExpiresAtEpochMs) ||
    !Number.isFinite(serverTimeEpochMs) ||
    !Number.isFinite(requestStartedAtEpochMs) ||
    serverExpiresAtEpochMs <= serverTimeEpochMs
  ) {
    return null;
  }
  // Project the server-owned remaining duration onto the local request-start
  // point. Static browser clock skew cannot extend validity, and anchoring
  // before the network round trip deliberately expires conservatively early.
  const expiresAtEpochMs =
    requestStartedAtEpochMs + (serverExpiresAtEpochMs - serverTimeEpochMs);
  return {
    id: response.id,
    sequence: response.sequence,
    catalogRevisionId: response.catalog_revision_id,
    catalogChecksum: response.catalog_checksum,
    algorithmVersion: response.algorithm_version,
    offerChecksum: response.offer_checksum,
    rootBone: toPresentationTrait(response.root_bone),
    talents: response.talents.map(toPresentationTrait),
    remainingOffers: response.remaining_offers,
    expiresAt: isoDateTimeSchema.parse(
      new Date(expiresAtEpochMs).toISOString(),
    ),
    expiresAtEpochMs,
  };
}
