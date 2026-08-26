import type { EntityID } from "@/types/api";

/** Authentication endpoints (Identity module). */
export const AUTH_API = {
  /** Anonymous, idempotent signed CSRF/pre-session bootstrap. */
  PRE_SESSION: "/auth/pre-session",
  LOGIN: "/auth/login",
  REGISTER: "/auth/register",
  ONBOARDING_PROFILE_SCHEMA: "/onboarding/profile-schema",
  REFRESH: "/auth/refresh",
  LOGOUT: "/auth/logout",
  /** Canonical safe user/session/capability projection. */
  SESSION_ME: "/session/me",
  /** Account-owned authentication session management. */
  SESSIONS: "/auth/sessions",
  SESSION: (id: EntityID) => `/auth/sessions/${encodeURIComponent(id)}`,
  LOGOUT_ALL_SESSIONS: "/auth/logout-all",
  CHANGE_PASSWORD: "/auth/change-password",
  FORGOT_PASSWORD: "/auth/forgot-password",
  RESET_PASSWORD: "/auth/reset-password",
  ACTIVATE_INVITATION: "/auth/invitations/activate",
  VERIFY_RECOVERY_CONTACT: "/auth/recovery-contact/verify",
  RECOVERY_CONTACT: "/auth/recovery-contact",
  REAUTHENTICATE: "/auth/reauthenticate",
  RECOVERY_CONTACT_RESEND: "/auth/recovery-contact/resend",
  RECOVERY_CONTACT_PENDING: "/auth/recovery-contact/pending",
  OAUTH_START: (provider: string) => `/auth/oauth/${provider}/start`,
  OAUTH_LINK_START: (provider: string) => `/auth/oauth/${provider}/link/start`,
  OAUTH_INSPECT_LOGIN: "/auth/oauth/login/inspect",
  OAUTH_FINALIZE_LOGIN: "/auth/oauth/login/finalize",
  OAUTH_FINALIZE_LINK: "/auth/oauth/link/finalize",
	OAUTH_IDENTITIES: "/auth/oauth/identities",
	OAUTH_IDENTITY: (provider: string) =>
		`/auth/oauth/identities/${encodeURIComponent(provider)}`,
} as const;
