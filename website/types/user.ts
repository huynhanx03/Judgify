/**
 * User profile types — mirrors api/internal/identity/core/dto/user.go FullProfileResponse.
 */

import type { Gender } from "@/constants/gender";
import type { EntityID, ISODateTime } from "@/types/api";

export interface TraitInfo {
  id: EntityID
  code: string
  type: "root_bone" | "talent"
  name: string
  rarity_code?: string
  rarity_name?: string
  description?: string
  slot: "root_bone" | "talent"
  slot_order: number
  effect_revision_id: EntityID
  effect_revision: number
  effect_checksum: string
  effect_effective_at: ISODateTime
  effect: TraitEffectInfo
}

export interface TraitEffectInfo {
  v: number
  kind: string
  scope: string
  element_codes?: string[]
  multiplier_delta_bps?: number
  flat_bonus?: number
}

export interface ProfileRewardInfo {
  id: EntityID
  revision: number
  checksum: string
  source: "onboarding" | "administrative"
  effective_at: ISODateTime
}

export interface ElementExp {
  code: string
  name: string
  exp: number
}

export interface LevelInfo {
  name: string
  next_name?: string        // empty if at max level
  tier_index: number        // 0-based index for color palette
  next_tier_index?: number  // tier_index of next level; absent if at max
  progress: number          // 0–100
  exp_to_next: number
}

export interface RankInfo {
  name: string
  next_name?: string        // empty if at max rank
  tier_index: number        // 0-based index for color palette
  next_tier_index?: number  // tier_index of next rank; absent if at max
  progress: number          // 0–100
  rating_to_next: number
}

export interface CultivationInfo {
  total_exp: number
  rating: number
  reward_profile: ProfileRewardInfo
  level: LevelInfo
  rank: RankInfo
  root_bone: TraitInfo
  talents: TraitInfo[]
  elements: ElementExp[]
}

export interface ElementBrief {
  code: string
  name: string
}

export interface DiffStat {
  name: string
  level: number
  solved_count: number
}

export interface TagStat {
  name: string
  solved_count: number
  elements: ElementBrief[]
}

export interface ProblemStats {
  total_submissions: number
  accepted_count: number
  by_difficulty: DiffStat[]
  by_tag: TagStat[]
}

/**
 * Identity fields shared by the compact profile mutation response and the
 * enriched profile query response.
 */
export interface ProfileAttrs {
  id: EntityID
  username: string
  first_name?: string
  last_name?: string
  gender: Gender
  birthday?: string
  joined_at?: ISODateTime
}

export interface ReadyUserProfile extends ProfileAttrs {
  profile_state: "ready"
  joined_at: ISODateTime
  cultivation: CultivationInfo
  problem_stats: ProblemStats
}

export interface UnprovisionedUserProfile extends ProfileAttrs {
  profile_state: "unprovisioned"
  joined_at: ISODateTime
  cultivation?: never
  problem_stats?: never
}

export type UserProfile = ReadyUserProfile | UnprovisionedUserProfile

export const PROFILE_ATTRIBUTE_DATA_TYPES = [
  "string",
  "number",
  "boolean",
  "date",
] as const

export type ProfileAttributeDataType =
  (typeof PROFILE_ATTRIBUTE_DATA_TYPES)[number]
export type ProfileAttributeVisibility = "public" | "private"
export type ProfileAttributeValue = string | number | boolean
export type ProfileAttributeValidation = Record<string, unknown>

export interface ProfileAttribute {
  definition_id: EntityID
  definition_revision_id: EntityID
  key: string
  kind: "system" | "custom"
  required_on_onboarding: boolean
  data_type: ProfileAttributeDataType
  label: string
  description: string
  visibility: ProfileAttributeVisibility
  user_editable: boolean
  display_order: number
  validation: ProfileAttributeValidation
  has_value: boolean
  value?: ProfileAttributeValue
  value_version: number
}

export interface ProfileAttributesResponse {
  profile: ProfileAttrs
  attributes: ProfileAttribute[]
}

interface ProfileAttributeMutationBase {
  definition_id: EntityID
  expected_definition_revision_id: EntityID
  expected_value_version: number
}

export type ProfileAttributeMutation =
  | (ProfileAttributeMutationBase & {
      value: ProfileAttributeValue
      unset?: never
    })
  | (ProfileAttributeMutationBase & {
      unset: true
      value?: never
    })

export interface UpdateProfileAttributesRequest {
  mutations: ProfileAttributeMutation[]
}
