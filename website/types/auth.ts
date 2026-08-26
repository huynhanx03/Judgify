/**
 * Authentication-related types mirroring backend identity DTOs.
 */

import type { EntityID, ISODateTime } from "@/types/api";
import type {
  ProfileAttributeDataType,
  ProfileAttributeValidation,
  ProfileAttributeValue,
  ProfileAttributeVisibility,
} from "@/types/user";
import {
	OAUTH_PROVIDERS,
  PASSWORD_RESET_REQUEST_STATUS,
  RECOVERY_CONTACT_DELIVERY_STATE,
  RECOVERY_CONTACT_PURPOSE,
  RECOVERY_CONTACT_REAUTHENTICATION_STATUS,
  RECOVERY_CONTACT_STATUS,
  RECOVERY_CONTACT_VERIFICATION_STATUS,
} from "@/constants/identity";

/** POST /auth/login request body. */
export interface LoginRequest {
  username: string;
  password: string;
}

/** POST /auth/login response data. */
export interface LoginResponse {
  success: true;
}

export type OAuthProvider = (typeof OAUTH_PROVIDERS)[number];

/** GET /auth/oauth/:provider/start response data. */
export interface OAuthStartResponse {
  auth_url: string;
}

export type OAuthPurpose = "login" | "link";

export interface OAuthLoginInspectionResponse {
  onboarding_required: boolean;
  /** Provider-derived values are suggestions only; the user still reviews them. */
  profile_suggestions?: ProfileValueSuggestion[];
  /** Effective upper bound for every completion step using this ticket. */
  expires_at: ISODateTime;
  /** Server wall clock paired with expires_at for client-skew-safe countdowns. */
  server_time: ISODateTime;
}

export interface OAuthOnboardingRequest
  extends TraitOfferSelectionRequest,
    ProfileSchemaEvidence {}

export interface OAuthFinalizeLoginRequest {
  onboarding?: OAuthOnboardingRequest;
}

/** POST /auth/oauth/login/finalize response data. */
export type OAuthFinalizeLoginResponse = LoginResponse;

/** POST /auth/oauth/link/finalize response data. */
export type OAuthFinalizeLinkResponse = LoginResponse;

export interface OAuthIdentity {
  provider: OAuthProvider;
  display_name?: string;
  avatar_url?: string;
  last_authenticated_at?: ISODateTime;
  linked_at: ISODateTime;
}

export interface OAuthIdentityListResponse {
  items: OAuthIdentity[];
}

export interface TraitOfferSelectionRequest {
  /** Selected root bone ID (1 required) */
  root_bone_id: EntityID;
  /** Selected talent IDs (exactly 3 required) */
  talent_ids: EntityID[];
  /** Durable server-owned offer evidence, bound to the browser pre-session. */
  offer_id: EntityID;
  offer_sequence: number;
  catalog_revision_id: EntityID;
  catalog_checksum: string;
  offer_checksum: string;
}

/** One required, immutable profile definition rendered during onboarding. */
export interface OnboardingProfileSchemaField {
  definition_id: EntityID;
  definition_revision_id: EntityID;
  key: string;
  /** Versioned presentation order; equal values are ordered by stable key. */
  display_order: number;
  data_type: ProfileAttributeDataType;
  label: string;
  description: string;
  visibility: ProfileAttributeVisibility;
  user_editable: boolean;
  validation: ProfileAttributeValidation;
}

/** Exact server-owned required-profile snapshot used by every account creator. */
export interface OnboardingProfileSchemaResponse {
  revision: number;
  checksum: string;
  fields: OnboardingProfileSchemaField[];
  server_time: ISODateTime;
}

export interface ProfileValueSuggestion {
  key: string;
  value: ProfileAttributeValue;
}

export interface ProfileValueInput {
  definition_id: EntityID;
  definition_revision_id: EntityID;
  value: ProfileAttributeValue;
}

/** Optimistic schema evidence echoed by registration and administrative creation. */
export interface ProfileSchemaEvidence {
  profile_schema_revision: number;
  profile_schema_checksum: string;
  profile_values: ProfileValueInput[];
}

/** POST /auth/register request body. */
export interface RegisterRequest
  extends TraitOfferSelectionRequest,
    ProfileSchemaEvidence {
  username: string;
  password: string;
  /** Address used only for account recovery and security notifications. */
  recovery_email: string;
}

/** POST /auth/register response data. */
export interface RegisterResponse {
  success: true;
}

/** POST /auth/refresh response data. */
export interface RefreshTokenResponse {
  success: true;
}

/** POST /auth/change-password request body. */
export interface ChangePasswordRequest {
  current_password: string;
  new_password: string;
}

/** POST /auth/change-password response data. */
export interface ChangePasswordResponse {
  success: boolean;
}

/** POST /auth/forgot-password request body. */
export interface ForgotPasswordRequest {
  username: string;
}

/** POST /auth/forgot-password response data. */
export interface ForgotPasswordResponse {
  status: typeof PASSWORD_RESET_REQUEST_STATUS.ACCEPTED;
}

/** POST /auth/reset-password request body. */
export interface ResetPasswordRequest {
  token: string;
  new_password: string;
}

/** POST /auth/reset-password response data. */
export interface ResetPasswordResponse {
  success: boolean;
}

/** POST /auth/invitations/activate request body. */
export interface ActivateUserInvitationRequest {
  token: string;
  new_password: string;
}

/** POST /auth/invitations/activate response data. */
export interface ActivateUserInvitationResponse {
  success: true;
}

/** POST /auth/recovery-contact/verify request body. */
export interface VerifyRecoveryContactRequest {
  token: string;
}

/** POST /auth/recovery-contact/verify response data. */
export interface VerifyRecoveryContactResponse {
  status: typeof RECOVERY_CONTACT_VERIFICATION_STATUS.VERIFIED;
}

export type RecoveryContactStatus =
  (typeof RECOVERY_CONTACT_STATUS)[keyof typeof RECOVERY_CONTACT_STATUS];

export type RecoveryContactPurpose =
  (typeof RECOVERY_CONTACT_PURPOSE)[keyof typeof RECOVERY_CONTACT_PURPOSE];

export type RecoveryContactDeliveryState =
  (typeof RECOVERY_CONTACT_DELIVERY_STATE)[keyof typeof RECOVERY_CONTACT_DELIVERY_STATE];

export interface RecoveryContactCurrent {
  status: typeof RECOVERY_CONTACT_STATUS.VERIFIED;
  masked_address: string;
  version: number;
}

export interface RecoveryContactPending {
  purpose: RecoveryContactPurpose;
  status: typeof RECOVERY_CONTACT_STATUS.PENDING;
  masked_address: string;
  version: number;
  verification_expires_at: ISODateTime;
  resend_available_at: ISODateTime;
  delivery_state: RecoveryContactDeliveryState;
}

/** GET /auth/recovery-contact and every successful management mutation. */
export interface RecoveryContactResponse {
  /** Server-owned policy decision; clients must not infer credential safety. */
  can_remove: boolean;
  current: RecoveryContactCurrent | null;
  pending: RecoveryContactPending | null;
  reauthenticated_until: ISODateTime | null;
}

export interface ReauthenticateRecoveryContactRequest {
  current_password: string;
}

export interface ReauthenticateRecoveryContactResponse {
  status: typeof RECOVERY_CONTACT_REAUTHENTICATION_STATUS.REAUTHENTICATED;
  reauthenticated_until: ISODateTime;
}

export interface ReplaceRecoveryContactRequest {
  recovery_email: string;
  expected_current_version: number;
}

export interface ResendRecoveryContactRequest {
  expected_pending_version: number;
}

export interface CancelPendingRecoveryContactRequest {
  expected_pending_version: number;
}

export interface RemoveRecoveryContactRequest {
  expected_current_version: number;
}

/** A server-projected, code-owned authorization grant. */
export interface CapabilityGrant {
  resource: string;
  actions: string[];
}

/** Effective capabilities for the authenticated principal at one policy revision. */
export interface EffectiveCapabilitiesResponse {
  revision: number;
  capabilities: CapabilityGrant[];
}

/** An exact resource/action requirement used by route and action guards. */
export interface CapabilityRequirement {
  resource: string;
  action: string;
}

export type CapabilityStatus = "idle" | "loading" | "ready" | "error";
export type BootstrapStatus = "loading" | "ready" | "error";
