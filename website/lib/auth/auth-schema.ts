import {
	arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  literalSchema,
  nullableSchema,
	optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { isoDateTimeSchema } from "@/lib/api/contracts";
import { profileValueSuggestionsSchema } from "@/lib/auth/onboarding-profile-schema";
import {
	OAUTH_PROVIDERS,
  PASSWORD_RESET_REQUEST_STATUS,
  RECOVERY_CONTACT_DELIVERY_STATE,
  RECOVERY_CONTACT_PURPOSE,
  RECOVERY_CONTACT_REAUTHENTICATION_STATUS,
  RECOVERY_CONTACT_STATUS,
  RECOVERY_CONTACT_VERIFICATION_STATUS,
} from "@/constants/identity";
import type {
  ActivateUserInvitationResponse,
  ForgotPasswordResponse,
  ChangePasswordResponse,
  LoginResponse,
  OAuthStartResponse,
  OAuthLoginInspectionResponse,
  OAuthIdentityListResponse,
  ReauthenticateRecoveryContactResponse,
  RecoveryContactResponse,
  RegisterResponse,
  ResetPasswordResponse,
  VerifyRecoveryContactResponse,
} from "@/types/auth";

const maskedAddressSchema = stringSchema({
  minimumLength: 1,
  maximumLength: 320,
  label: "masked recovery address",
});
const versionSchema = integerSchema({
  minimum: 1,
  label: "recovery contact version",
});

export const commandSuccessSchema = strictObjectSchema({
  success: literalSchema(true),
}) as Schema<
  LoginResponse & RegisterResponse & ResetPasswordResponse & ChangePasswordResponse
>;

/** Public invitation activation is fail-closed against response drift. */
export const activateUserInvitationResponseSchema = strictObjectSchema({
  success: literalSchema(true),
}) as Schema<ActivateUserInvitationResponse>;

export const oauthStartSchema: Schema<OAuthStartResponse> = {
  parse(value: unknown, path = "$"): OAuthStartResponse {
    const parsed = strictObjectSchema({
      auth_url: stringSchema({
        minimumLength: 1,
        maximumLength: 4096,
        label: "OAuth authorization URL",
      }),
    }).parse(value, path);
    const url = new URL(parsed.auth_url);
    if (url.protocol !== "https:" || url.username || url.password) {
      throw new TypeError(
        `${path}.auth_url: provider URL must use HTTPS without userinfo`,
      );
    }
    return parsed;
  },
};

const rawOAuthLoginInspectionSchema = strictObjectSchema({
  onboarding_required: booleanSchema,
  profile_suggestions: optionalSchema(profileValueSuggestionsSchema),
  expires_at: isoDateTimeSchema,
  server_time: isoDateTimeSchema,
});

export const oauthLoginInspectionSchema: Schema<OAuthLoginInspectionResponse> = {
  parse(value: unknown, path = "$"): OAuthLoginInspectionResponse {
    const inspection = rawOAuthLoginInspectionSchema.parse(value, path);
    if (Date.parse(inspection.expires_at) <= Date.parse(inspection.server_time)) {
      throw new TypeError(`${path}: OAuth completion ticket is already expired`);
    }
    return inspection;
  },
};

const oauthIdentitySchema = strictObjectSchema({
	provider: enumSchema(OAUTH_PROVIDERS),
	display_name: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 256 })),
	avatar_url: optionalSchema(stringSchema({ minimumLength: 1, maximumLength: 2048 })),
	last_authenticated_at: optionalSchema(isoDateTimeSchema),
	linked_at: isoDateTimeSchema,
});

export const oauthIdentityListSchema = strictObjectSchema({
	items: arraySchema(oauthIdentitySchema, { maximumLength: OAUTH_PROVIDERS.length }),
}) as Schema<OAuthIdentityListResponse>;

export const forgotPasswordResponseSchema =
  strictObjectSchema({
    status: literalSchema(PASSWORD_RESET_REQUEST_STATUS.ACCEPTED),
  }) as Schema<ForgotPasswordResponse>;

export const verifyRecoveryContactResponseSchema =
  strictObjectSchema({
    status: literalSchema(
      RECOVERY_CONTACT_VERIFICATION_STATUS.VERIFIED,
    ),
  }) as Schema<VerifyRecoveryContactResponse>;

const recoveryContactCurrentSchema = strictObjectSchema({
  status: literalSchema(RECOVERY_CONTACT_STATUS.VERIFIED),
  masked_address: maskedAddressSchema,
  version: versionSchema,
});

const recoveryContactPendingSchema = strictObjectSchema({
  purpose: enumSchema([
    RECOVERY_CONTACT_PURPOSE.REGISTRATION,
    RECOVERY_CONTACT_PURPOSE.REPLACEMENT,
  ] as const),
  status: literalSchema(RECOVERY_CONTACT_STATUS.PENDING),
  masked_address: maskedAddressSchema,
  version: versionSchema,
  verification_expires_at: isoDateTimeSchema,
  resend_available_at: isoDateTimeSchema,
  delivery_state: enumSchema([
    RECOVERY_CONTACT_DELIVERY_STATE.QUEUED,
    RECOVERY_CONTACT_DELIVERY_STATE.SENT,
    RECOVERY_CONTACT_DELIVERY_STATE.FAILED,
  ] as const),
});

export const recoveryContactResponseSchema =
  strictObjectSchema({
    can_remove: booleanSchema,
    current: nullableSchema(recoveryContactCurrentSchema),
    pending: nullableSchema(recoveryContactPendingSchema),
    reauthenticated_until: nullableSchema(isoDateTimeSchema),
  }) as Schema<RecoveryContactResponse>;

export const reauthenticateRecoveryContactResponseSchema =
  strictObjectSchema({
    status: literalSchema(
      RECOVERY_CONTACT_REAUTHENTICATION_STATUS.REAUTHENTICATED,
    ),
    reauthenticated_until: isoDateTimeSchema,
  }) as Schema<ReauthenticateRecoveryContactResponse>;
