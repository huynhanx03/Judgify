/**
 * Cultivation module types mirroring backend DTOs.
 * Covers immutable trait effects and effective-dated reward profiles.
 */

import type { EntityID, ISODateTime } from "@/types/api";

/** Nested rarity info in trait response. */
export interface TraitRarityInfo {
  id: EntityID;
  name: string;
  code: string;
  /** Administrative catalog responses expose weight; anonymous offers do not. */
  weight?: number;
}

export type TraitType = "root_bone" | "talent";
export type TraitEffectScope = "all" | "element";

interface TraitEffectBase {
  v: 1;
  scope: TraitEffectScope;
  /** Present only for element-scoped effects; codes are canonical and sorted. */
  element_codes?: string[];
}

export interface TraitMultiplierEffect extends TraitEffectBase {
  kind: "exp_multiplier";
  multiplier_delta_bps: number;
}

export interface TraitBonusEffect extends TraitEffectBase {
  kind: "exp_bonus";
  flat_bonus: number;
}

/** Closed, fixed-point reward effect language understood by the API. */
export type TraitEffectSpec = TraitMultiplierEffect | TraitBonusEffect;

export interface TraitEffectRevisionResponse {
  id: EntityID;
  revision: number;
  effect: TraitEffectSpec;
  checksum: string;
  effective_at: ISODateTime;
}

/** Common display fields also used by anonymous onboarding snapshots. */
export interface TraitPresentation {
  id: EntityID;
  type: TraitType;
  name: string;
  rarity?: TraitRarityInfo;
  description?: string;
}

/** Full administrative/public catalog response from /traits. */
export interface TraitResponse extends TraitPresentation {
  code: string;
  display_order: number;
  active: boolean;
  effect_revision?: TraitEffectRevisionResponse;
  version: number;
}

/** Display-safe immutable trait snapshot embedded in an onboarding offer. */
export interface TraitOfferTraitResponse {
  id: EntityID;
  revision: number;
  type: "root_bone" | "talent";
  name: string;
  description?: string;
  rarity_id: EntityID;
  rarity_code: string;
  rarity_name: string;
}

/** Response from POST /onboarding/trait-offers. */
export interface TraitOfferResponse {
  id: EntityID;
  sequence: number;
  catalog_revision_id: EntityID;
  catalog_checksum: string;
  algorithm_version: string;
  offer_checksum: string;
  root_bone: TraitOfferTraitResponse;
  talents: TraitOfferTraitResponse[];
  remaining_offers: number;
  expires_at: ISODateTime;
  server_time: ISODateTime;
}

export interface RewardProfileSelection {
  root_trait_id: EntityID;
  talent_trait_ids: EntityID[];
}

export type RewardProfileSlot = "root_bone" | "talent";
export type RewardProfileSource = "onboarding" | "administrative";

export interface RewardProfileTrait {
  trait_id: EntityID;
  trait_code: string;
  trait_type: TraitType;
  trait_name: string;
  description?: string;
  rarity_code: string;
  rarity_name: string;
  slot: RewardProfileSlot;
  slot_order: number;
  effect_revision: TraitEffectRevisionResponse;
}

export interface RewardProfileResponse {
  id: EntityID;
  user_id: EntityID;
  revision: number;
  assignment_checksum: string;
  source: RewardProfileSource;
  source_offer_id?: EntityID;
  command_id?: EntityID;
  effective_at: ISODateTime;
  traits: RewardProfileTrait[];
  replayed?: boolean;
}

export interface RewardProfileAssignmentPreview {
  user_id: EntityID;
  current_revision: number;
  next_revision: number;
  traits: RewardProfileTrait[];
  preview_checksum: string;
}

export interface ApplyRewardProfileInput extends RewardProfileSelection {
  user_id: EntityID;
  expected_revision: number;
  preview_checksum: string;
  reason: string;
}

/** Rarity response from BE CRUD endpoints. */
export interface RarityResponse {
  id: EntityID;
  name: string;
  code: string;
  weight: number;
  description?: string;
}

/** Element response from BE. */
export interface ElementResponse {
  id: EntityID;
  name: string;
  code: string;
  description?: string;
  version: number;
}

/** Level response from BE. */
export interface LevelResponse {
  id: EntityID;
  name: string;
  min_exp: number;
  description?: string;
}

/** Rank response from BE. */
export interface RankResponse {
  id: EntityID;
  name: string;
  min_rating: number;
  description?: string;
}
