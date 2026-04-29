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

/** Ranking endpoints (Cultivation module). */
export const RANKING_API = {
  TOP_RATING: (limit?: number) => `/rankings/rating${limit ? `?limit=${limit}` : ""}`,
  TOP_EXP: (limit?: number) => `/rankings/exp${limit ? `?limit=${limit}` : ""}`,
} as const;
