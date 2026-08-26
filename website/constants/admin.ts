/** Shared interaction limits for conventional paginated admin catalogs. */
export const ADMIN_RESOURCE_LIST = Object.freeze({
  PAGE_SIZE: 10,
  SEARCH_DEBOUNCE_MS: 300,
});

/** Bounded, server-side entity lookup used by relationship form fields. */
export const ADMIN_USER_LOOKUP = Object.freeze({
  PAGE_SIZE: 20,
  SEARCH_DEBOUNCE_MS: 300,
});
