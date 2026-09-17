import type { EntityID } from "@/types/api";

const entityPath = (root: string, id: EntityID) =>
  `${root}/${encodeURIComponent(id)}`;

/** Trait endpoints (Cultivation module). */
export const TRAIT_API = {
  FIND_ALL: "/traits",
  FIND: "/traits/find",
  GET: (id: EntityID) => entityPath("/traits", id),
  CREATE: "/traits",
  UPDATE: (id: EntityID) => entityPath("/traits", id),
  DELETE: (id: EntityID) => entityPath("/traits", id),
} as const;

export const ONBOARDING_TRAIT_API = {
  CREATE_OFFER: "/onboarding/trait-offers",
} as const;

/** Effective-dated reward profile for the authenticated user. */
export const REWARD_PROFILE_API = {
  CURRENT: "/reward-profile",
} as const;

/** Element endpoints (Cultivation module). */
export const ELEMENT_API = {
  FIND_ALL: "/elements",
  FIND: "/elements/find",
  GET: (id: EntityID) => entityPath("/elements", id),
  CREATE: "/elements",
  UPDATE: (id: EntityID) => entityPath("/elements", id),
  DELETE: (id: EntityID) => entityPath("/elements", id),
} as const;

/** Level endpoints (Cultivation module). */
export const LEVEL_API = {
  FIND_ALL: "/levels",
  FIND: "/levels/find",
  GET: (id: EntityID) => entityPath("/levels", id),
  CREATE: "/levels",
  UPDATE: (id: EntityID) => entityPath("/levels", id),
  DELETE: (id: EntityID) => entityPath("/levels", id),
} as const;

/** Rarity endpoints (Cultivation module). */
export const RARITY_API = {
  FIND_ALL: "/rarities",
  FIND: "/rarities/find",
  GET: (id: EntityID) => entityPath("/rarities", id),
  CREATE: "/rarities",
  UPDATE: (id: EntityID) => entityPath("/rarities", id),
  DELETE: (id: EntityID) => entityPath("/rarities", id),
} as const;

/** Rank endpoints (Cultivation module). */
export const RANK_API = {
  FIND_ALL: "/ranks",
  FIND: "/ranks/find",
  GET: (id: EntityID) => entityPath("/ranks", id),
  CREATE: "/ranks",
  UPDATE: (id: EntityID) => entityPath("/ranks", id),
  DELETE: (id: EntityID) => entityPath("/ranks", id),
} as const;

/** Ranking endpoints (Cultivation module). */
export const RANKING_API = {
	RATING: (query: string) => `/rankings/rating${query ? `?${query}` : ""}`,
	EXP: (query: string) => `/rankings/exp${query ? `?${query}` : ""}`,
	PRIVACY: "/ranking/privacy",
} as const;

export const PROGRESSION_API = {
	MY_LEDGER: (query: string) => `/progression/ledger${query ? `?${query}` : ""}`,
} as const;

export const ADMIN_CULTIVATION_API = {
	LEDGER: (query: string) => `/admin/cultivation/progression/ledger${query ? `?${query}` : ""}`,
	ADJUSTMENT_PREVIEW: "/admin/cultivation/progression/adjustments/preview",
	ADJUSTMENT_APPLY: "/admin/cultivation/progression/adjustments",
	REVERSAL_PREVIEW: "/admin/cultivation/progression/reversals/preview",
	REVERSAL_APPLY: "/admin/cultivation/progression/reversals",
	REWARD_RULES: (query = "") => `/admin/cultivation/reward-rules${query ? `?${query}` : ""}`,
	REWARD_RULE_PREVIEW: "/admin/cultivation/reward-rules/preview",
	REWARD_RULE_PUBLISH: "/admin/cultivation/reward-rules",
	REWARD_PROFILE: (userID: EntityID) =>
		entityPath("/admin/cultivation/reward-profiles", userID),
	REWARD_PROFILE_PREVIEW: (userID: EntityID) =>
		`${entityPath("/admin/cultivation/reward-profiles", userID)}/preview`,
	RANKING_RECONCILIATION: "/admin/cultivation/ranking/reconciliation",
	RANKING_RECONCILIATION_PREVIEW: "/admin/cultivation/ranking/reconciliation/preview",
} as const;
