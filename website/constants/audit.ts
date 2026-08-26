export const AUDIT_PAGE_SIZE = 25;
export const AUDIT_PAGE_SIZE_MAX = 100;
export const AUDIT_RECENT_LIMIT = 6;

/** Browser-side bounds mirror the public audit query contract. */
export const AUDIT_FILTER_LIMITS = {
  ACTOR_USER_ID: 36,
  ACTION: 128,
  RESOURCE: 128,
  RESOURCE_ID: 256,
  CORRELATION_ID: 128,
} as const;
