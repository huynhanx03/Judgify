export const NOTIFICATION_CATEGORIES = [
  "authorization",
  "campaign",
  "contest",
  "material",
  "operation",
  "security",
  "submission",
] as const;

export const REQUIRED_NOTIFICATION_CATEGORIES = [
  "authorization",
  "operation",
  "security",
] as const;

export const NOTIFICATION_CONTENT_KINDS = ["catalog", "campaign"] as const;

export const NOTIFICATION_POLICY = {
  DEFAULT_PAGE_SIZE: 30,
  DROPDOWN_PREVIEW_SIZE: 6,
  MAXIMUM_PAGE_SIZE: 100,
  MAXIMUM_CURSOR_LENGTH: 128,
  MAXIMUM_TYPE_LENGTH: 128,
  MAXIMUM_MESSAGE_KEY_LENGTH: 128,
  MAXIMUM_ACTION_PATH_LENGTH: 512,
  VISIBILITY_RECONCILIATION_AGE_MS: 30_000,
  LIVE_RECONCILIATION_DELAY_MS: 80,
} as const;

export type NotificationCategory =
  (typeof NOTIFICATION_CATEGORIES)[number];
export type NotificationContentKind =
  (typeof NOTIFICATION_CONTENT_KINDS)[number];
