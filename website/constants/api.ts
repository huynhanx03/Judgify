/**
 * Centralized API configuration and endpoint paths.
 * All backend URLs are managed here so changes only require edits in one place.
 */

/** Base URL for the backend API. Reads from environment variable or falls back to localhost. */
export const BASE_API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/** Authentication endpoints (Identity module). */
export const AUTH_API = {
  LOGIN: "/auth/login",
  REGISTER: "/auth/register",
  REFRESH: "/auth/refresh",
  CHANGE_PASSWORD: "/auth/change-password",
  FORGOT_PASSWORD: "/auth/forgot-password",
  RESET_PASSWORD: "/auth/reset-password",
  OAUTH_CALLBACK: (provider: string) => `/auth/oauth/${provider}/callback`,
  OAUTH_REGISTER: (provider: string) => `/auth/oauth/${provider}/register`,
  OAUTH_LINK: (provider: string) => `/auth/oauth/${provider}/link`,
} as const;

/** User endpoints (Identity module). */
export const USER_API = {
  FIND: "/users/find",
  CREATE: "/users",
  UPDATE: (id: number) => `/users/${id}`,
  DELETE: (id: number) => `/users/${id}`,
  PROFILE: "/users/profile",
} as const;

/** Problem endpoints (Problem module). */
export const PROBLEM_API = {
  FIND: "/problems/find",
  GET: (id: number) => `/problems/${id}`,
  CREATE: "/problems",
  UPDATE: (id: number) => `/problems/${id}`,
  DELETE: (id: number) => `/problems/${id}`,
  TEST_CASES: (id: number) => `/problems/${id}/test-cases`,
  TEST_CASE_UPDATE: (id: number) => `/test-cases/${id}`,
  TEST_CASE_DELETE: (id: number) => `/test-cases/${id}`,
} as const;

/** Trait endpoints (Cultivation module). */
export const TRAIT_API = {
  FIND_ALL: "/traits",
  FIND: "/traits/find",
  GET: (id: number) => `/traits/${id}`,
  CREATE: "/traits",
  UPDATE: (id: number) => `/traits/${id}`,
  DELETE: (id: number) => `/traits/${id}`,
  ROLL: "/traits/roll",
} as const;

/** User-Trait endpoints (Cultivation module). */
export const USER_TRAIT_API = {
  CREATE: "/user-traits",
  FIND: "/user-traits/find",
} as const;

/** Tag endpoints (Problem module). */
export const TAG_API = {
  FIND_ALL: "/tags",
  FIND: "/tags/find",
  GET: (id: number) => `/tags/${id}`,
  CREATE: "/tags",
  UPDATE: (id: number) => `/tags/${id}`,
  DELETE: (id: number) => `/tags/${id}`,
} as const;

/** Role endpoints (Identity module). */
export const ROLE_API = {
  FIND_ALL: "/roles",
  FIND: "/roles/find",
  GET: (id: number) => `/roles/${id}`,
  CREATE: "/roles",
  UPDATE: (id: number) => `/roles/${id}`,
  DELETE: (id: number) => `/roles/${id}`,
} as const;

/** Permission endpoints (Identity module). */
export const PERMISSION_API = {
  FIND_ALL: "/permissions",
  FIND: "/permissions/find",
  GET: (id: number) => `/permissions/${id}`,
  CREATE: "/permissions",
  UPDATE: (id: number) => `/permissions/${id}`,
  DELETE: (id: number) => `/permissions/${id}`,
} as const;

/** Resource endpoints (Identity module). */
export const RESOURCE_API = {
  FIND_ALL: "/resources",
  FIND: "/resources/find",
  GET: (id: number) => `/resources/${id}`,
  CREATE: "/resources",
  UPDATE: (id: number) => `/resources/${id}`,
  DELETE: (id: number) => `/resources/${id}`,
} as const;

/** Difficulty endpoints (Problem module). */
export const DIFFICULTY_API = {
  FIND_ALL: "/difficulties",
  FIND: "/difficulties/find",
  GET: (id: number) => `/difficulties/${id}`,
  CREATE: "/difficulties",
  UPDATE: (id: number) => `/difficulties/${id}`,
  DELETE: (id: number) => `/difficulties/${id}`,
} as const;

/** Element endpoints (Cultivation module). */
export const ELEMENT_API = {
  FIND_ALL: "/elements",
  FIND: "/elements/find",
  GET: (id: number) => `/elements/${id}`,
  CREATE: "/elements",
  UPDATE: (id: number) => `/elements/${id}`,
  DELETE: (id: number) => `/elements/${id}`,
} as const;

/** Level endpoints (Cultivation module). */
export const LEVEL_API = {
  FIND_ALL: "/levels",
  FIND: "/levels/find",
  GET: (id: number) => `/levels/${id}`,
  CREATE: "/levels",
  UPDATE: (id: number) => `/levels/${id}`,
  DELETE: (id: number) => `/levels/${id}`,
} as const;

/** Rarity endpoints (Cultivation module). */
export const RARITY_API = {
  FIND_ALL: "/rarities",
  FIND: "/rarities/find",
  GET: (id: number) => `/rarities/${id}`,
  CREATE: "/rarities",
  UPDATE: (id: number) => `/rarities/${id}`,
  DELETE: (id: number) => `/rarities/${id}`,
} as const;

/** Rank endpoints (Cultivation module). */
export const RANK_API = {
  FIND_ALL: "/ranks",
  FIND: "/ranks/find",
  GET: (id: number) => `/ranks/${id}`,
  CREATE: "/ranks",
  UPDATE: (id: number) => `/ranks/${id}`,
  DELETE: (id: number) => `/ranks/${id}`,
} as const;

/** Submission endpoints (Submission module). */
export const SUBMISSION_API = {
  CREATE: "/submissions",
  GET: (id: number) => `/submissions/${id}`,
  /** Get all submissions for a problem. */
  BY_PROBLEM: (problemId: number) => `/problems/${problemId}/submissions`,
  /** Get current user's submissions for a problem. */
  MY_BY_PROBLEM: (problemId: number) => `/problems/${problemId}/my-submissions`,
} as const;

/** Contest endpoints (Contest module). */
export const CONTEST_API = {
  FIND: "/contests/find",
  GET: (id: number) => `/contests/${id}`,
  CREATE: "/contests",
  UPDATE: (id: number) => `/contests/${id}`,
  DELETE: (id: number) => `/contests/${id}`,
  REGISTER: (id: number) => `/contests/${id}/register`,
  UNREGISTER: (id: number) => `/contests/${id}/unregister`,
  STANDINGS: (id: number) => `/contests/${id}/standings`,
  RATING_CHANGES: (id: number) => `/contests/${id}/rating-changes`,
} as const;

/** Ranking endpoints (Cultivation module). */
export const RANKING_API = {
  TOP_RATING: (limit?: number) => `/rankings/rating${limit ? `?limit=${limit}` : ""}`,
  TOP_EXP: (limit?: number) => `/rankings/exp${limit ? `?limit=${limit}` : ""}`,
} as const;
