import { AUTH_API } from "@/constants/api/auth";
import { api } from "@/lib/api/client";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { preSessionBootstrap } from "@/lib/auth/pre-session";
import { ApiError } from "@/lib/api/error";
import {
  activateUserInvitationResponseSchema,
  commandSuccessSchema,
  forgotPasswordResponseSchema,
  oauthLoginInspectionSchema,
  oauthStartSchema,
  oauthIdentityListSchema,
  reauthenticateRecoveryContactResponseSchema,
  recoveryContactResponseSchema,
  verifyRecoveryContactResponseSchema,
} from "@/lib/auth/auth-schema";
import { onboardingProfileSchemaResponseSchema } from "@/lib/auth/onboarding-profile-schema";
import type {
  ActivateUserInvitationRequest,
  ActivateUserInvitationResponse,
  ForgotPasswordResponse,
  ChangePasswordResponse,
  LoginRequest,
  LoginResponse,
  OAuthFinalizeLinkResponse,
  OAuthFinalizeLoginRequest,
  OAuthFinalizeLoginResponse,
  OAuthLoginInspectionResponse,
  OAuthIdentityListResponse,
  OnboardingProfileSchemaResponse,
  OAuthProvider,
  OAuthStartResponse,
  ReauthenticateRecoveryContactResponse,
  RecoveryContactResponse,
  RegisterRequest,
  RegisterResponse,
  ResetPasswordResponse,
  VerifyRecoveryContactResponse,
} from "@/types/auth";

function issuancePurpose(prefix: string, provider?: string): string {
  return provider ? `${prefix}-${provider}` : prefix;
}

async function runRecoverableIssuance<T>(
  purpose: string,
  command: (idempotencyKey: string) => Promise<T>,
): Promise<T> {
  await preSessionBootstrap.ensure();
  const attempt = commandAttemptStore.getOrCreate(purpose);
  try {
    const result = await command(attempt.attempt_id);
    commandAttemptStore.resolve(purpose, attempt.attempt_id);
    return result;
  } catch (error) {
    if (
      error instanceof ApiError &&
      !error.retryable &&
      error.status < 500
    ) {
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
    }
    throw error;
  }
}

export const authService = {
  async getOnboardingProfileSchema(
    signal?: AbortSignal,
  ): Promise<OnboardingProfileSchemaResponse> {
    return api<OnboardingProfileSchemaResponse, never>(
      AUTH_API.ONBOARDING_PROFILE_SCHEMA,
      {
        method: "GET",
        signal,
        auth: "none",
        retryUnauthorized: false,
        schema: onboardingProfileSchemaResponseSchema,
      },
    );
  },

  async login(
    request: LoginRequest,
    idempotencyKey?: string,
  ): Promise<LoginResponse> {
    await preSessionBootstrap.ensure();
    const execute = (key: string) =>
      api<LoginResponse, LoginRequest>(AUTH_API.LOGIN, {
        method: "POST",
        body: request,
        auth: "none",
        retryUnauthorized: false,
        idempotencyKey: key,
        schema: commandSuccessSchema,
      });
    return idempotencyKey
      ? execute(idempotencyKey)
      : runRecoverableIssuance("password-login", execute);
  },

  async register(
    request: RegisterRequest,
    idempotencyKey?: string,
  ): Promise<RegisterResponse> {
    await preSessionBootstrap.ensure();
    const execute = (key: string) =>
      api<RegisterResponse, RegisterRequest>(AUTH_API.REGISTER, {
        method: "POST",
        body: request,
        auth: "none",
        retryUnauthorized: false,
        idempotencyKey: key,
        schema: commandSuccessSchema,
      });
    return idempotencyKey
      ? execute(idempotencyKey)
      : runRecoverableIssuance("password-registration", execute);
  },

  async startOAuth(provider: OAuthProvider): Promise<OAuthStartResponse> {
    const purpose = issuancePurpose("oauth-start", provider);
    return runRecoverableIssuance(purpose, (idempotencyKey) =>
      api<OAuthStartResponse, never>(AUTH_API.OAUTH_START(provider), {
        method: "POST",
        auth: "none",
        retryUnauthorized: false,
        idempotencyKey,
        schema: oauthStartSchema,
      }),
    );
  },

  async startOAuthLink(provider: OAuthProvider): Promise<OAuthStartResponse> {
    return api<OAuthStartResponse, never>(AUTH_API.OAUTH_LINK_START(provider), {
      method: "POST",
      retryUnauthorized: true,
      schema: oauthStartSchema,
    });
  },

  async finalizeOAuthLogin(
    request: OAuthFinalizeLoginRequest = {},
    idempotencyKey?: string,
  ): Promise<OAuthFinalizeLoginResponse> {
    await preSessionBootstrap.ensure();
    const purpose = "oauth-finalize-login";
    const execute = (key: string) =>
      api<OAuthFinalizeLoginResponse, OAuthFinalizeLoginRequest>(
        AUTH_API.OAUTH_FINALIZE_LOGIN,
        {
          method: "POST",
          body: request,
          auth: "none",
          retryUnauthorized: false,
          idempotencyKey: key,
          schema: commandSuccessSchema,
        },
      );
    return idempotencyKey
      ? execute(idempotencyKey)
      : runRecoverableIssuance(purpose, execute);
  },

  async inspectOAuthLogin(): Promise<OAuthLoginInspectionResponse> {
    await preSessionBootstrap.ensure();
    return api<OAuthLoginInspectionResponse, never>(
      AUTH_API.OAUTH_INSPECT_LOGIN,
      {
        method: "POST",
        auth: "none",
        retryUnauthorized: false,
        schema: oauthLoginInspectionSchema,
      },
    );
  },

  async finalizeOAuthLink(): Promise<OAuthFinalizeLinkResponse> {
    return api<OAuthFinalizeLinkResponse, never>(AUTH_API.OAUTH_FINALIZE_LINK, {
      method: "POST",
      retryUnauthorized: true,
      schema: commandSuccessSchema,
    });
  },

  async listOAuthIdentities(
    signal?: AbortSignal,
  ): Promise<OAuthIdentityListResponse> {
    return api<OAuthIdentityListResponse, never>(AUTH_API.OAUTH_IDENTITIES, {
      method: "GET",
      signal,
      retryUnauthorized: true,
      schema: oauthIdentityListSchema,
    });
  },

  async unlinkOAuth(
    provider: OAuthProvider,
  ): Promise<OAuthFinalizeLinkResponse> {
    return api<OAuthFinalizeLinkResponse, never>(
      AUTH_API.OAUTH_IDENTITY(provider),
      {
        method: "DELETE",
        retryUnauthorized: false,
        schema: commandSuccessSchema,
      },
    );
  },

  async refresh(idempotencyKey: string): Promise<void> {
    await preSessionBootstrap.ensure();
    await api<LoginResponse, never>(AUTH_API.REFRESH, {
      method: "POST",
      auth: "none",
      retryUnauthorized: false,
      idempotencyKey,
      schema: commandSuccessSchema,
    });
  },

  async changePassword(
    currentPassword: string,
    newPassword: string,
  ): Promise<ChangePasswordResponse> {
    return api<ChangePasswordResponse, { current_password: string; new_password: string }>(
      AUTH_API.CHANGE_PASSWORD,
      {
        method: "POST",
        body: {
          current_password: currentPassword,
          new_password: newPassword,
        },
        retryUnauthorized: false,
        schema: commandSuccessSchema,
      },
    );
  },

  async forgotPassword(username: string): Promise<ForgotPasswordResponse> {
    return runRecoverableIssuance("password-recovery-request", (key) =>
      api<ForgotPasswordResponse, { username: string }>(
        AUTH_API.FORGOT_PASSWORD,
        {
          method: "POST",
          body: { username },
          auth: "none",
          retryUnauthorized: false,
          idempotencyKey: key,
          schema: forgotPasswordResponseSchema,
        },
      ),
    );
  },

  async resetPassword(
    token: string,
    newPassword: string,
  ): Promise<ResetPasswordResponse> {
    return runRecoverableIssuance("password-reset", (key) =>
      api<ResetPasswordResponse, {
        token: string;
        new_password: string;
      }>(AUTH_API.RESET_PASSWORD, {
        method: "POST",
        body: { token, new_password: newPassword },
        auth: "none",
        retryUnauthorized: false,
        idempotencyKey: key,
        schema: commandSuccessSchema,
      }),
    );
  },

  async activateUserInvitation(
    token: string,
    newPassword: string,
  ): Promise<ActivateUserInvitationResponse> {
    return runRecoverableIssuance("account-invitation-activation", (key) =>
      api<ActivateUserInvitationResponse, ActivateUserInvitationRequest>(
        AUTH_API.ACTIVATE_INVITATION,
        {
          method: "POST",
          body: { token, new_password: newPassword },
          auth: "none",
          retryUnauthorized: false,
          idempotencyKey: key,
          schema: activateUserInvitationResponseSchema,
        },
      ),
    );
  },

  async verifyRecoveryContact(
    token: string,
  ): Promise<VerifyRecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-verification", (key) =>
      api<VerifyRecoveryContactResponse, { token: string }>(
        AUTH_API.VERIFY_RECOVERY_CONTACT,
        {
          method: "POST",
          body: { token },
          auth: "none",
          retryUnauthorized: false,
          idempotencyKey: key,
          schema: verifyRecoveryContactResponseSchema,
        },
      ),
    );
  },

  getRecoveryContact(signal?: AbortSignal): Promise<RecoveryContactResponse> {
    return api<RecoveryContactResponse, never>(AUTH_API.RECOVERY_CONTACT, {
      signal,
      schema: recoveryContactResponseSchema,
    });
  },

  reauthenticateRecoveryContact(
    currentPassword: string,
  ): Promise<ReauthenticateRecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-reauthentication", (key) =>
      api<
        ReauthenticateRecoveryContactResponse,
        { current_password: string }
      >(AUTH_API.REAUTHENTICATE, {
        method: "POST",
        body: { current_password: currentPassword },
        idempotencyKey: key,
        retryUnauthorized: false,
        schema: reauthenticateRecoveryContactResponseSchema,
      }),
    );
  },

  replaceRecoveryContact(
    recoveryEmail: string,
    expectedCurrentVersion: number,
  ): Promise<RecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-replacement", (key) =>
      api<
        RecoveryContactResponse,
        { recovery_email: string; expected_current_version: number }
      >(AUTH_API.RECOVERY_CONTACT, {
        method: "PUT",
        body: {
          recovery_email: recoveryEmail,
          expected_current_version: expectedCurrentVersion,
        },
        idempotencyKey: key,
        schema: recoveryContactResponseSchema,
      }),
    );
  },

  resendRecoveryContactVerification(
    expectedPendingVersion: number,
  ): Promise<RecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-resend", (key) =>
      api<RecoveryContactResponse, { expected_pending_version: number }>(
        AUTH_API.RECOVERY_CONTACT_RESEND,
        {
          method: "POST",
          body: { expected_pending_version: expectedPendingVersion },
          idempotencyKey: key,
          schema: recoveryContactResponseSchema,
        },
      ),
    );
  },

  cancelPendingRecoveryContact(
    expectedPendingVersion: number,
  ): Promise<RecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-cancel", (key) =>
      api<RecoveryContactResponse, { expected_pending_version: number }>(
        AUTH_API.RECOVERY_CONTACT_PENDING,
        {
          method: "DELETE",
          body: { expected_pending_version: expectedPendingVersion },
          idempotencyKey: key,
          schema: recoveryContactResponseSchema,
        },
      ),
    );
  },

  removeRecoveryContact(
    expectedCurrentVersion: number,
  ): Promise<RecoveryContactResponse> {
    return runRecoverableIssuance("recovery-contact-removal", (key) =>
      api<RecoveryContactResponse, { expected_current_version: number }>(
        AUTH_API.RECOVERY_CONTACT,
        {
          method: "DELETE",
          body: { expected_current_version: expectedCurrentVersion },
          idempotencyKey: key,
          schema: recoveryContactResponseSchema,
        },
      ),
    );
  },

  async logout(idempotencyKey?: string): Promise<void> {
    await preSessionBootstrap.ensure();
    const execute = async (key: string): Promise<void> => {
      await api<LoginResponse, never>(AUTH_API.LOGOUT, {
        method: "POST",
        idempotencyKey: key,
        retryUnauthorized: false,
        schema: commandSuccessSchema,
      });
    };
    if (idempotencyKey) {
      await execute(idempotencyKey);
      return;
    }
    await runRecoverableIssuance("session-logout", execute);
  },
};
