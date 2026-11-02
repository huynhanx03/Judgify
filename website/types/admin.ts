/**
 * Admin-related types for dashboard management.
 */

import type { EntityID, ISODateTime } from "@/types/api";
import type { ProfileSchemaEvidence } from "@/types/auth";

/** Role entity — mirrors BE RoleResponse. */
export interface Role {
  id: EntityID;
  key: string;
  name: string;
  description?: string;
  is_system: boolean;
  /** Server-owned guard for roles whose policies cannot be edited. */
  is_protected: boolean;
  status: "active" | "archived";
  version: number;
}

/** One server-owned action available for an authorization resource. */
export interface AuthorizationAction {
  key: string;
  label: string;
  description?: string;
}

/** Server-owned capability resource. Admins assign it; clients never invent it. */
export interface AuthorizationResource {
  key: string;
  label: string;
  description?: string;
  actions: AuthorizationAction[];
}

/** Versioned capability catalog used to render the dynamic policy matrix. */
export interface AuthorizationCatalog {
  revision: number;
  resources: AuthorizationResource[];
}

/** One canonical Casbin policy rule for a role. */
export interface PolicyRule {
  role_key?: string;
  resource: string;
  action: string;
}

/** Optimistically versioned role policy document. */
export interface RolePolicySnapshot {
  revision: number;
  rules: PolicyRule[];
}

export interface RolePolicyPreview {
  role_key: string;
  expected_revision: number;
  resulting_revision: number;
  before: RolePolicySnapshot;
  after: RolePolicySnapshot;
  added: PolicyRule[];
  removed: PolicyRule[];
}

export interface RolePolicyPreviewResponse {
  preview: RolePolicyPreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface RolePolicyMutationReceipt {
  command_id: EntityID;
  event_id: EntityID;
  role_key: string;
  snapshot: RolePolicySnapshot;
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface RolePolicyCommandInput {
  expected_revision: number;
  reason: string;
  rules: PolicyRule[];
}

export interface ReviewedRolePolicyCommand {
  role_id: EntityID;
  command_id: EntityID;
  input: RolePolicyCommandInput;
  review: RolePolicyPreviewResponse;
}

export interface UserRoleSnapshot {
  revision: number;
  roles: string[];
}

export interface UserRolePreview {
  user_id: EntityID;
  expected_revision: number;
  resulting_revision: number;
  before: UserRoleSnapshot;
  after: UserRoleSnapshot;
  added: string[];
  removed: string[];
}

export interface UserRolePreviewResponse {
  preview: UserRolePreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface UserRoleMutationReceipt {
  command_id: EntityID;
  event_id: EntityID;
  user_id: EntityID;
  snapshot: UserRoleSnapshot;
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface UserRoleCommandInput {
  expected_revision: number;
  reason: string;
  roles: string[];
}

export interface ReviewedUserRoleCommand {
  user_id: EntityID;
  command_id: EntityID;
  input: UserRoleCommandInput;
  review: UserRolePreviewResponse;
}

export const USER_LIFECYCLE_ACTIONS = [
  "suspend",
  "reactivate",
  "delete",
] as const;

export type UserLifecycleAction = (typeof USER_LIFECYCLE_ACTIONS)[number];

export const USER_LIFECYCLE_STATUSES = [
  "invited",
  "active",
  "suspended",
  "deleted",
] as const;

export type UserLifecycleStatus = (typeof USER_LIFECYCLE_STATUSES)[number];

export interface UserLifecycleCommandInput {
  expected_version: number;
  reason: string;
}

export interface UserLifecycleProjection {
  id: EntityID;
  username: string;
  status: UserLifecycleStatus;
  version: number;
  suspended_at?: ISODateTime;
}

export interface UserLifecyclePreview {
  action: UserLifecycleAction;
  user_id: EntityID;
  expected_version: number;
  resulting_version: number;
  before: UserLifecycleProjection;
  after: UserLifecycleProjection;
}

export interface UserLifecyclePreviewResponse {
  preview: UserLifecyclePreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface UserLifecycleReceipt {
  command_id: EntityID;
  event_id: EntityID;
  action: UserLifecycleAction;
  user: UserLifecycleProjection;
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

/** Client-held immutable command reviewed by the server before apply. */
export interface ReviewedUserLifecycleCommand {
  user_id: EntityID;
  command_id: EntityID;
  attempt_purpose: string;
  action: UserLifecycleAction;
  input: UserLifecycleCommandInput;
  review: UserLifecyclePreviewResponse;
}

/** Versioned, audited replacement of one user's direct role assignments. */
export interface CreateRoleRequest {
  key: string;
  name: string;
  description?: string;
  /** Audit reason for the durable creation command. */
  reason: string;
}

export interface RoleMetadataCommandInput {
  expected_version: number;
  reason: string;
  name?: string;
  description?: string;
}
export interface RoleMetadataPreview {
  action: "update" | "archive";
  role_id: EntityID;
  expected_version: number;
  resulting_version: number;
  before: Role;
  after: Role;
}

export interface RoleMetadataPreviewResponse {
  preview: RoleMetadataPreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface RoleMutationReceipt {
  command_id: EntityID;
  event_id: EntityID;
  action: "create" | "update" | "archive";
  role: Role;
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface ReviewedRoleMetadataCommand {
  role_id: EntityID;
  command_id: EntityID;
  input: RoleMetadataCommandInput;
  review: RoleMetadataPreviewResponse;
}

export interface UpdateRoleRequest {
  key?: string;
  name?: string;
  description?: string;
}

/** Admin user listing. */
export interface AdminUser {
  id: EntityID;
  username: string;
  status: UserLifecycleStatus;
  version: number;
  suspended_at?: ISODateTime;
  roles: string[];
  authorization_revision: number;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

/** Reviewed administrative invitation. The administrator never handles a password. */
export interface UserInvitationCommandInput extends ProfileSchemaEvidence {
  username: string;
  recovery_email: string;
  roles: string[];
  reason: string;
}

export interface UserInvitationPreview {
  username: string;
  masked_destination: string;
  roles: string[];
  profile_value_count: number;
}

export interface UserInvitationPreviewResponse {
  preview: UserInvitationPreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface UserInvitationReceipt {
  command_id: EntityID;
  event_id: EntityID;
  user: UserLifecycleProjection;
  roles: string[];
  profile_value_count: number;
  invitation_expires_at: ISODateTime;
  delivery_state: "queued";
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface ReviewedUserInvitationCommand {
  command_id: EntityID;
  attempt_purpose: string;
  input: UserInvitationCommandInput;
  review: UserInvitationPreviewResponse;
}

export interface UserInvitationResendCommandInput {
  expected_user_version: number;
  expected_invitation_version: number;
  reason: string;
}

export interface UserInvitationResendPreview {
  user: UserLifecycleProjection;
  masked_destination: string;
  expected_invitation_version: number;
  resulting_invitation_version: number;
  current_expires_at: ISODateTime;
}

export interface UserInvitationResendPreviewResponse {
  preview: UserInvitationResendPreview;
  confirmation_token: string;
  expires_at: ISODateTime;
}

export interface UserInvitationResendReceipt {
  command_id: EntityID;
  event_id: EntityID;
  user: UserLifecycleProjection;
  invitation_version: number;
  invitation_expires_at: ISODateTime;
  delivery_state: "queued";
  committed_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface ReviewedUserInvitationResendCommand {
  user_id: EntityID;
  command_id: EntityID;
  attempt_purpose: string;
  input: UserInvitationResendCommandInput;
  review: UserInvitationResendPreviewResponse;
}

/** Dashboard stats overview. */
export interface DashboardStats {
  totalProblems: number;
  totalTags: number;
  totalRoles: number;
  totalUsers: number;
}
