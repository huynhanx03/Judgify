export const OPERATION_STATUS = {
  PENDING: "pending",
  RUNNING: "running",
  PAUSED: "paused",
  COMPLETED: "completed",
  FAILED: "failed",
  CANCELLED: "cancelled",
} as const;

export const OPERATION_STATUSES = Object.values(OPERATION_STATUS);

export const OPERATION_CONTROL_ACTION = {
  PAUSE: "pause",
  RESUME: "resume",
  CANCEL: "cancel",
} as const;

export const OPERATION_REQUESTED_ACTION = {
  NONE: "none",
  PAUSE: OPERATION_CONTROL_ACTION.PAUSE,
  CANCEL: OPERATION_CONTROL_ACTION.CANCEL,
} as const;

export const OPERATION_PAGE_SIZE = 25;
export const OPERATION_MAXIMUM_PAGE_SIZE = 100;
export const OPERATION_KIND_MAXIMUM_LENGTH = 128;
export const OPERATION_KIND_PATTERN = /^[a-z][a-z0-9_.-]*$/;

export const OPERATION_KIND = {
  NOTIFICATION_CAMPAIGN: "notification.campaign.delivery",
	CULTIVATION_RANKING_RECONCILIATION: "cultivation.ranking.reconciliation.v1",
  IDENTITY_ATTRIBUTE_VALUE_MIGRATION: "identity.attribute_value_migration.v1",
} as const;

export const OPERATION_CONTROL_REASON_LIMITS = {
  MINIMUM_CHARACTERS: 3,
  MAXIMUM_BYTES: 1024,
} as const;
