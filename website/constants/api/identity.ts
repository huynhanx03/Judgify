import type { EntityID } from "@/types/api";
import type { UserLifecycleAction } from "@/types/admin";

/** User endpoints (Identity module). */
export const USER_API = {
  FIND: "/users/find",
  INVITATION_PREVIEW: "/users/invitations/preview",
  INVITATION_APPLY: "/users/invitations/apply",
  INVITATION_RESEND_PREVIEW: (id: EntityID) =>
    `/users/${encodeURIComponent(id)}/invitation/resend/preview`,
  INVITATION_RESEND_APPLY: (id: EntityID) =>
    `/users/${encodeURIComponent(id)}/invitation/resend/apply`,
  ROLES_PREVIEW: (id: EntityID) =>
    `/users/${encodeURIComponent(id)}/roles/preview`,
  ROLES_APPLY: (id: EntityID) =>
    `/users/${encodeURIComponent(id)}/roles/apply`,
  LIFECYCLE_PREVIEW: (id: EntityID, action: UserLifecycleAction) =>
    `/users/${encodeURIComponent(id)}/lifecycle/${action}/preview`,
  LIFECYCLE_APPLY: (id: EntityID, action: UserLifecycleAction) =>
    `/users/${encodeURIComponent(id)}/lifecycle/${action}/apply`,
  PROFILE: "/users/profile",
  PROFILE_ATTRIBUTES: "/users/profile/attributes",
} as const;

/** Role endpoints (Identity module). */
export const ROLE_API = {
  FIND_ALL: "/roles",
  FIND: "/roles/find",
  GET: (id: EntityID) => `/roles/${encodeURIComponent(id)}`,
  CREATE: "/roles",
  METADATA_PREVIEW: (id: EntityID) => `/roles/${encodeURIComponent(id)}/metadata/preview`,
  METADATA_APPLY: (id: EntityID) => `/roles/${encodeURIComponent(id)}/metadata/apply`,
  ARCHIVE_PREVIEW: (id: EntityID) => `/roles/${encodeURIComponent(id)}/archive/preview`,
  ARCHIVE_APPLY: (id: EntityID) => `/roles/${encodeURIComponent(id)}/archive/apply`,
  POLICIES: (id: EntityID) => `/roles/${encodeURIComponent(id)}/policies`,
  POLICIES_PREVIEW: (id: EntityID) =>
    `/roles/${encodeURIComponent(id)}/policies/preview`,
  POLICIES_APPLY: (id: EntityID) =>
    `/roles/${encodeURIComponent(id)}/policies/apply`,
} as const;

/** Canonical authorization catalog; resources/actions are server-owned. */
export const AUTHORIZATION_API = {
  ME: "/authorization/me",
  CATALOG: "/authorization/catalog",
} as const;

/** Attribute-definition endpoints (Identity module). */
export const ATTRIBUTE_DEFINITION_API = {
  FIND: "/attribute-definitions/find",
  GET: (id: EntityID) => `/attribute-definitions/${encodeURIComponent(id)}`,
  REVISIONS: (id: EntityID) =>
    `/attribute-definitions/${encodeURIComponent(id)}/revisions`,
  CREATE: "/attribute-definitions",
  UPDATE: (id: EntityID) => `/attribute-definitions/${encodeURIComponent(id)}`,
  ACTIVATION_PREVIEW: (id: EntityID) =>
    `/attribute-definitions/${encodeURIComponent(id)}/activation/preview`,
  ACTIVATION_APPLY: (id: EntityID) =>
    `/attribute-definitions/${encodeURIComponent(id)}/activation/apply`,
  ARCHIVE_PREVIEW: (id: EntityID) =>
    `/attribute-definitions/${encodeURIComponent(id)}/archive/preview`,
  ARCHIVE_APPLY: (id: EntityID) =>
    `/attribute-definitions/${encodeURIComponent(id)}/archive/apply`,
} as const;
