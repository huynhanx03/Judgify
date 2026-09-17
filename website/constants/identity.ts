/** Mirrors the bounded identity DTO contracts owned by the API. */
export const IDENTITY_INPUT_LIMITS = {
  USERNAME_MIN_LENGTH: 3,
  USERNAME_MAX_LENGTH: 50,
  PASSWORD_MIN_LENGTH: 8,
  PASSWORD_MAX_LENGTH: 256,
  RECOVERY_EMAIL_MAX_LENGTH: 320,
  NAME_MAX_LENGTH: 50,
  USER_LIFECYCLE_REASON_MIN_LENGTH: 3,
  USER_LIFECYCLE_REASON_MAX_LENGTH: 1024,
  ADMIN_USERS_PAGE_SIZE: 20,
  SEARCH_DEBOUNCE_MS: 300,
} as const;

/** Closed OAuth provider catalog shared by login and account security UI. */
export const OAUTH_PROVIDERS = ["google", "github"] as const;

/** Browser-facing bounds for opaque, single-use identity credentials. */
export const ONE_TIME_CREDENTIAL = {
  FRAGMENT_KEY: "token",
  MAX_LENGTH: 4096,
} as const;

/** Runtime values returned by the recovery-contact verification protocol. */
export const RECOVERY_CONTACT_VERIFICATION_STATUS = {
  VERIFIED: "verified",
} as const;

/** Runtime values returned by the recovery-contact management protocol. */
export const RECOVERY_CONTACT_STATUS = {
  PENDING: "pending",
  VERIFIED: "verified",
} as const;

export const RECOVERY_CONTACT_PURPOSE = {
  REGISTRATION: "registration",
  REPLACEMENT: "replacement",
} as const;

export const RECOVERY_CONTACT_DELIVERY_STATE = {
  QUEUED: "queued",
  SENT: "sent",
  FAILED: "failed",
} as const;

export const RECOVERY_CONTACT_REAUTHENTICATION_STATUS = {
  REAUTHENTICATED: "reauthenticated",
} as const;

/** Runtime value returned by the non-enumerating password-reset request. */
export const PASSWORD_RESET_REQUEST_STATUS = {
  ACCEPTED: "accepted",
} as const;

export const RECOVERY_CONTACT_UI = {
  CLOCK_TICK_MS: 1_000,
} as const;
