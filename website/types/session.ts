/** Safe account-session projections returned by the identity service. */
import type { Cursor, EntityID, ISODateTime } from "@/types/api";

export type AuthSessionStatus = "active" | "revoked" | "expired";

export interface AuthSession {
  id: EntityID;
  current: boolean;
  status: AuthSessionStatus;
  created_at: ISODateTime;
  last_seen_at: ISODateTime;
  last_reauthenticated_at: ISODateTime;
  idle_expires_at: ISODateTime;
  absolute_expires_at: ISODateTime;
  revoked_at?: ISODateTime | null;
  user_agent?: string;
}

export interface AuthSessionListResponse {
  items: AuthSession[];
  next_cursor?: Cursor | null;
}

export interface AuthSessionListQuery {
  limit?: number;
  cursor?: string;
}

export interface AuthSessionMutationResponse {
  success: boolean;
}

/** Minimal identity projection safe to hydrate into the application shell. */
export interface SessionPrincipal {
  id: EntityID;
  username: string;
  first_name?: string;
  last_name?: string;
}

/** Current session facts needed for expiry and security UX. */
export interface CurrentSessionView {
  id: EntityID;
  expires_at: ISODateTime;
  reauthenticated_until?: ISODateTime | null;
}

/**
 * Canonical `/api/session/me` response. Capability keys are concrete
 * `resource:action` pairs projected by the backend; raw roles, wildcard rules,
 * Casbin subjects, tokens, and cookies never enter this DTO.
 */
export interface SessionView {
  user: SessionPrincipal;
  session: CurrentSessionView;
  capabilities: readonly string[];
  authorization_revision: number;
}
