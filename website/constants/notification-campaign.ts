export const CAMPAIGN_STATUS = {
  DRAFT: "draft",
  SCHEDULED: "scheduled",
  SENDING: "sending",
  COMPLETED: "completed",
  FAILED: "failed",
  CANCELLED: "cancelled",
} as const;

export const CAMPAIGN_STATUSES = Object.values(CAMPAIGN_STATUS);

export const CAMPAIGN_AUDIENCE = {
  EXPLICIT_USERS: "explicit_users",
  ROLE: "role",
  ALL_ACTIVE: "all_active",
} as const;

export const CAMPAIGN_AUDIENCES = Object.values(CAMPAIGN_AUDIENCE);

export const CAMPAIGN_RECIPIENT_STATUS = {
  PENDING: "pending",
  CREATED: "created",
  SKIPPED: "skipped",
  FAILED: "failed",
} as const;

export const CAMPAIGN_RECIPIENT_STATUSES = Object.values(
  CAMPAIGN_RECIPIENT_STATUS,
);

export const CAMPAIGN_LIMITS = {
  TITLE_BYTES: 160,
  BODY_BYTES: 64 * 1024,
  ACTION_PATH_BYTES: 512,
  REASON_BYTES: 1024,
  EXPLICIT_USERS: 5_000,
  PAGE_SIZE: 25,
  MAXIMUM_PAGE_SIZE: 100,
  SAMPLE_SIZE: 20,
  CURSOR_LENGTH: 128,
  SCHEDULE_AHEAD_DAYS: 365,
} as const;
