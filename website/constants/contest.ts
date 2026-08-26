export const CONTEST_REASON_LIMITS = {
  MINIMUM_CHARACTERS: 1,
  MAXIMUM_CHARACTERS: 1024,
  MAXIMUM_UTF8_BYTES: 1024,
} as const;

/** Stable resource identity for the public contest discovery projection. */
export const CONTEST_LIFECYCLE = {
  PUBLIC_LIST_RESOURCE_KEY: "public-contest-list",
} as const;

export const CONTEST_DISCOVERY = {
  PAGE_SIZE: 12,
  SEARCH_DEBOUNCE_MS: 350,
  MAXIMUM_SEARCH_CHARACTERS: 120,
} as const;

/** Stable in-page navigation target shared by contest discovery controls. */
export const CONTEST_LIST_ANCHOR_ID = "contest-list";

export const CONTEST_RATING = {
  RERATING_OPERATION_KIND: "contest.rating.rerating.v1",
  MAXIMUM_RERATING_SEQUENCES: 10_000,
} as const;

export const CONTEST_ADMIN_CATALOG = {
  PAGE_SIZE: 50,
  SEARCH_DEBOUNCE_MS: 250,
} as const;
