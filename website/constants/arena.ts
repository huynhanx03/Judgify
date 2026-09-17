/** Stable client-side behavior and URL vocabulary for the public arena. */
export const ARENA_POLICY = Object.freeze({
  PAGE_SIZE: 20,
  SEARCH_DEBOUNCE_MS: 350,
  MAXIMUM_SEARCH_CHARACTERS: 120,
  MAXIMUM_TAG_FILTERS: 16,
  MAXIMUM_PAGE: 10_000,
});

export const ARENA_QUERY_PARAMETER = Object.freeze({
  SEARCH: "q",
  DIFFICULTY: "difficulty",
  TAG: "tag",
  SORT: "sort",
  DIRECTION: "direction",
  PAGE: "page",
});

export const ARENA_SORT_FIELD = Object.freeze({
  NONE: "none",
  ACCEPTANCE: "acceptance",
  SUBMISSIONS: "submissions",
});

export const ARENA_SORT_DIRECTION = Object.freeze({
  ASCENDING: "asc",
  DESCENDING: "desc",
});

export type ArenaSortField =
  (typeof ARENA_SORT_FIELD)[keyof typeof ARENA_SORT_FIELD];

export type ArenaSortDirection =
  (typeof ARENA_SORT_DIRECTION)[keyof typeof ARENA_SORT_DIRECTION];
