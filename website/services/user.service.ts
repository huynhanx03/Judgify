import { api } from "@/lib/api/client";
import { USER_API } from "@/constants/api/identity";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  AdminUser,
  ReviewedUserLifecycleCommand,
  ReviewedUserInvitationCommand,
  ReviewedUserInvitationResendCommand,
  ReviewedUserRoleCommand,
  UserInvitationCommandInput,
  UserInvitationPreviewResponse,
  UserInvitationReceipt,
  UserInvitationResendCommandInput,
  UserInvitationResendPreviewResponse,
  UserInvitationResendReceipt,
  UserLifecycleAction,
  UserLifecycleCommandInput,
  UserLifecyclePreviewResponse,
  UserLifecycleReceipt,
  UserRoleCommandInput,
  UserRoleMutationReceipt,
  UserRolePreviewResponse,
} from "@/types/admin";
import {
  adminUserPageSchema,
  userInvitationPreviewResponseSchema,
  userInvitationReceiptSchema,
  userInvitationResendPreviewResponseSchema,
  userInvitationResendReceiptSchema,
  userLifecycleCommandInputSchema,
  userLifecyclePreviewResponseSchema,
  userLifecycleReceiptSchema,
  userRoleCommandInputSchema,
  userRoleMutationReceiptSchema,
  userRolePreviewResponseSchema,
} from "@/lib/admin/identity-schema";
import { profileSchema } from "@/lib/auth/profile-schema";
import { profileAttributesResponseSchema } from "@/lib/profile/profile-attribute-schema";
import type {
  ProfileAttributesResponse,
  UserProfile,
  UpdateProfileAttributesRequest,
} from "@/types/user";
import { entityIDSchema } from "@/lib/api/contracts";
import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { commandPurpose } from "@/lib/api/command-purpose";
import { USER_LIFECYCLE_ACTIONS } from "@/types/admin";

export const userService = {
  // Profile (public)
  async getProfile(signal?: AbortSignal): Promise<UserProfile> {
    return api<UserProfile, never>(USER_API.PROFILE, {
      method: "GET",
      signal,
      schema: profileSchema,
    });
  },

  async getProfileAttributes(
    signal?: AbortSignal,
  ): Promise<ProfileAttributesResponse> {
    return api<ProfileAttributesResponse, never>(USER_API.PROFILE_ATTRIBUTES, {
      method: "GET",
      signal,
      schema: profileAttributesResponseSchema,
    });
  },

  async updateProfileAttributes(
    request: UpdateProfileAttributesRequest,
  ): Promise<ProfileAttributesResponse> {
    return api<ProfileAttributesResponse, UpdateProfileAttributesRequest>(
      USER_API.PROFILE_ATTRIBUTES,
      {
        method: "PUT",
        body: request,
        schema: profileAttributesResponseSchema,
      },
    );
  },

  // User management (admin)
  async find(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<AdminUser>> {
    return api<Paginated<AdminUser>, QueryOptions>(
      USER_API.FIND,
      {
        method: "POST",
        body: query ?? { pagination: { page: 1, page_size: 200 } },
        signal,
        schema: adminUserPageSchema,
      },
    );
  },

  async previewInvitation(
    input: UserInvitationCommandInput,
  ): Promise<ReviewedUserInvitationCommand> {
    const canonical = canonicalUserInvitationInput(input);
    const purpose = await userInvitationCommandPurpose(canonical);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        UserInvitationPreviewResponse,
        UserInvitationCommandInput & { command_id: string }
      >(USER_API.INVITATION_PREVIEW, {
        method: "POST",
        body: { ...canonical, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: userInvitationPreviewResponseSchema,
      });
      if (!validUserInvitationReview(review, canonical)) {
        throw invalidUserInvitationResponse();
      }
      return {
        command_id: attempt.attempt_id,
        attempt_purpose: purpose,
        input: canonical,
        review,
      };
    } catch (error) {
      resolveDefinitiveUserInvitationFailure(
        purpose,
        attempt.attempt_id,
        error,
      );
      throw error;
    }
  },

  async applyInvitation(
    reviewed: ReviewedUserInvitationCommand,
  ): Promise<UserInvitationReceipt> {
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const canonical = canonicalUserInvitationInput(reviewed.input);
    const purpose = await userInvitationCommandPurpose(canonical);
    if (purpose !== reviewed.attempt_purpose) {
      throw new TypeError("user invitation command purpose changed");
    }
    const retainedAttempt = commandAttemptStore.getOrCreate(purpose);
    if (retainedAttempt.attempt_id !== commandID) {
      commandAttemptStore.resolve(purpose, retainedAttempt.attempt_id);
      throw new TypeError("user invitation command identity expired");
    }
    try {
      const receipt = await api<
        UserInvitationReceipt,
        UserInvitationCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(USER_API.INVITATION_APPLY, {
        method: "POST",
        body: {
          ...canonical,
          command_id: commandID,
          confirmation_token: reviewed.review.confirmation_token,
        },
        idempotencyKey: commandID,
        schema: userInvitationReceiptSchema,
      });
      if (!validUserInvitationReceipt(receipt, reviewed)) {
        throw invalidUserInvitationResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveUserInvitationFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonInvitationCommand(reviewed: ReviewedUserInvitationCommand): void {
    commandAttemptStore.resolve(
      reviewed.attempt_purpose,
      entityIDSchema.parse(reviewed.command_id),
    );
  },

  async previewInvitationResend(
    user: AdminUser,
    reason: string,
  ): Promise<ReviewedUserInvitationResendCommand> {
    const userID = entityIDSchema.parse(user.id);
    const input: UserInvitationResendCommandInput = {
      expected_user_version: user.version,
      expected_invitation_version: 0,
      reason: reason.trim(),
    };
    const purpose = await userInvitationResendPurpose(userID, input);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        UserInvitationResendPreviewResponse,
        UserInvitationResendCommandInput & { command_id: string }
      >(USER_API.INVITATION_RESEND_PREVIEW(userID), {
        method: "POST",
        body: { ...input, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: userInvitationResendPreviewResponseSchema,
      });
      if (
        review.preview.user.id !== userID ||
        review.preview.user.username !== user.username ||
        review.preview.user.version !== user.version ||
        review.preview.user.status !== "invited" ||
        Date.parse(review.expires_at) <= Date.now()
      ) {
        throw invalidUserInvitationResponse();
      }
      const reviewedInput = {
        ...input,
        expected_invitation_version:
          review.preview.expected_invitation_version,
      };
      return {
        user_id: userID,
        command_id: attempt.attempt_id,
        attempt_purpose: purpose,
        input: reviewedInput,
        review,
      };
    } catch (error) {
      resolveDefinitiveUserInvitationFailure(
        purpose,
        attempt.attempt_id,
        error,
      );
      throw error;
    }
  },

  async applyInvitationResend(
    reviewed: ReviewedUserInvitationResendCommand,
  ): Promise<UserInvitationResendReceipt> {
    const userID = entityIDSchema.parse(reviewed.user_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const originalPurposeInput = {
      ...reviewed.input,
      expected_invitation_version: 0,
    };
    const purpose = await userInvitationResendPurpose(
      userID,
      originalPurposeInput,
    );
    if (purpose !== reviewed.attempt_purpose) {
      throw new TypeError("invitation resend command purpose changed");
    }
    const retainedAttempt = commandAttemptStore.getOrCreate(purpose);
    if (retainedAttempt.attempt_id !== commandID) {
      commandAttemptStore.resolve(purpose, retainedAttempt.attempt_id);
      throw new TypeError("invitation resend command identity expired");
    }
    try {
      const receipt = await api<
        UserInvitationResendReceipt,
        UserInvitationResendCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(USER_API.INVITATION_RESEND_APPLY(userID), {
        method: "POST",
        body: {
          ...reviewed.input,
          command_id: commandID,
          confirmation_token: reviewed.review.confirmation_token,
        },
        idempotencyKey: commandID,
        schema: userInvitationResendReceiptSchema,
      });
      if (
        receipt.command_id !== commandID ||
        receipt.event_id !== commandID ||
        receipt.user.id !== userID ||
        receipt.user.status !== "invited" ||
        receipt.user.version !== reviewed.input.expected_user_version ||
        receipt.invitation_version !==
          reviewed.input.expected_invitation_version + 1 ||
        receipt.delivery_state !== "queued" ||
        Date.parse(receipt.invitation_expires_at) <=
          Date.parse(receipt.committed_at)
      ) {
        throw invalidUserInvitationResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveUserInvitationFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonInvitationResendCommand(
    reviewed: ReviewedUserInvitationResendCommand,
  ): void {
    commandAttemptStore.resolve(
      reviewed.attempt_purpose,
      entityIDSchema.parse(reviewed.command_id),
    );
  },

  async previewRoles(
    id: string,
    input: UserRoleCommandInput,
  ): Promise<ReviewedUserRoleCommand> {
    const userID = entityIDSchema.parse(id);
    const commandInput = canonicalUserRoleInput(input);
    const purpose = userRoleCommandPurpose(userID);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        UserRolePreviewResponse,
        UserRoleCommandInput & { command_id: string }
      >(USER_API.ROLES_PREVIEW(userID), {
        method: "POST",
        body: { ...commandInput, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: userRolePreviewResponseSchema,
      });
      if (!validUserRoleReview(review, userID, commandInput)) {
        throw invalidUserRoleResponse();
      }
      return {
        user_id: userID,
        command_id: attempt.attempt_id,
        input: commandInput,
        review,
      };
    } catch (error) {
      resolveDefinitiveUserRoleFailure(purpose, attempt.attempt_id, error);
      throw error;
    }
  },

  async applyRoles(
    reviewed: ReviewedUserRoleCommand,
  ): Promise<UserRoleMutationReceipt> {
    const userID = entityIDSchema.parse(reviewed.user_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const purpose = userRoleCommandPurpose(userID);
    if (commandAttemptStore.getOrCreate(purpose).attempt_id !== commandID) {
      throw new TypeError("authorization command identity expired");
    }
    try {
      const receipt = await api<
        UserRoleMutationReceipt,
        UserRoleCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(USER_API.ROLES_APPLY(userID), {
        method: "POST",
        body: {
          ...reviewed.input,
          command_id: commandID,
          confirmation_token: reviewed.review.confirmation_token,
        },
        idempotencyKey: commandID,
        schema: userRoleMutationReceiptSchema,
      });
      if (
        receipt.command_id !== commandID ||
        receipt.event_id !== commandID ||
        receipt.user_id !== userID ||
        receipt.snapshot.revision !== reviewed.input.expected_revision + 1 ||
        !stringListsEqual(receipt.snapshot.roles, reviewed.input.roles)
      ) {
        throw invalidUserRoleResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveUserRoleFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonRoleCommand(id: string, commandId?: string): void {
    const userID = entityIDSchema.parse(id);
    commandAttemptStore.resolve(
      userRoleCommandPurpose(userID),
      commandId ? entityIDSchema.parse(commandId) : undefined,
    );
  },

  async previewLifecycle(
    user: AdminUser,
    action: UserLifecycleAction,
    input: UserLifecycleCommandInput,
  ): Promise<ReviewedUserLifecycleCommand> {
    const userID = entityIDSchema.parse(user.id);
    const canonicalAction = canonicalUserLifecycleAction(action);
    const commandInput = canonicalUserLifecycleInput(input);
    if (commandInput.expected_version !== user.version) {
      throw new TypeError("user lifecycle command uses a stale version");
    }
    const purpose = await userLifecycleCommandPurpose(
      userID,
      canonicalAction,
      commandInput,
    );
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        UserLifecyclePreviewResponse,
        UserLifecycleCommandInput & { command_id: string }
      >(USER_API.LIFECYCLE_PREVIEW(userID, canonicalAction), {
        method: "POST",
        body: { ...commandInput, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: userLifecyclePreviewResponseSchema,
      });
      if (
        !validUserLifecycleReview(
          review,
          user,
          canonicalAction,
          commandInput,
        )
      ) {
        throw invalidUserLifecycleResponse();
      }
      return {
        user_id: userID,
        command_id: attempt.attempt_id,
        attempt_purpose: purpose,
        action: canonicalAction,
        input: commandInput,
        review,
      };
    } catch (error) {
      resolveDefinitiveUserLifecycleFailure(
        purpose,
        attempt.attempt_id,
        error,
      );
      throw error;
    }
  },

  async applyLifecycle(
    reviewed: ReviewedUserLifecycleCommand,
  ): Promise<UserLifecycleReceipt> {
    const userID = entityIDSchema.parse(reviewed.user_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const action = canonicalUserLifecycleAction(reviewed.action);
    const input = canonicalUserLifecycleInput(reviewed.input);
    const purpose = await userLifecycleCommandPurpose(userID, action, input);
    if (purpose !== reviewed.attempt_purpose) {
      throw new TypeError("user lifecycle command purpose changed");
    }
    const retainedAttempt = commandAttemptStore.getOrCreate(purpose);
    if (retainedAttempt.attempt_id !== commandID) {
      commandAttemptStore.resolve(purpose, retainedAttempt.attempt_id);
      throw new TypeError("user lifecycle command identity expired");
    }
    try {
      const receipt = await api<
        UserLifecycleReceipt,
        UserLifecycleCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(USER_API.LIFECYCLE_APPLY(userID, action), {
        method: "POST",
        body: {
          ...input,
          command_id: commandID,
          confirmation_token: reviewed.review.confirmation_token,
        },
        idempotencyKey: commandID,
        schema: userLifecycleReceiptSchema,
      });
      if (!validUserLifecycleReceipt(receipt, reviewed)) {
        throw invalidUserLifecycleResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveUserLifecycleFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonLifecycleCommand(reviewed: ReviewedUserLifecycleCommand): void {
    commandAttemptStore.resolve(
      reviewed.attempt_purpose,
      entityIDSchema.parse(reviewed.command_id),
    );
  },
};

function canonicalUserInvitationInput(
  input: UserInvitationCommandInput,
): UserInvitationCommandInput {
  const roles = input.roles.map((role) => role.trim()).sort();
  const profileValues = [...input.profile_values]
    .map((item) => ({ ...item }))
    .sort((left, right) =>
      left.definition_id.localeCompare(right.definition_id),
    );
  if (
    input.profile_schema_revision < 1 ||
    !/^[a-f0-9]{64}$/.test(input.profile_schema_checksum.trim().toLowerCase()) ||
    roles.length === 0 ||
    new Set(roles).size !== roles.length ||
    profileValues.length === 0 ||
    input.reason.trim().length < 3
  ) {
    throw new TypeError("invalid user invitation input");
  }
  return {
    username: input.username.trim(),
    recovery_email: input.recovery_email.trim(),
    roles,
    reason: input.reason.trim(),
    profile_schema_revision: input.profile_schema_revision,
    profile_schema_checksum: input.profile_schema_checksum.trim().toLowerCase(),
    profile_values: profileValues,
  };
}

async function userInvitationCommandPurpose(
  input: UserInvitationCommandInput,
): Promise<string> {
  return commandPurpose("user-invitation", canonicalUserInvitationInput(input));
}

function validUserInvitationReview(
  response: UserInvitationPreviewResponse,
  input: UserInvitationCommandInput,
): boolean {
  return (
    response.preview.username === input.username &&
    response.preview.masked_destination.length > 0 &&
    stringListsEqual(response.preview.roles, input.roles) &&
    response.preview.profile_value_count === input.profile_values.length &&
    Date.parse(response.expires_at) > Date.now()
  );
}

function validUserInvitationReceipt(
  receipt: UserInvitationReceipt,
  reviewed: ReviewedUserInvitationCommand,
): boolean {
  return (
    receipt.command_id === reviewed.command_id &&
    receipt.event_id === reviewed.command_id &&
    receipt.user.username === reviewed.input.username &&
    receipt.user.status === "invited" &&
    receipt.user.version === 1 &&
    stringListsEqual(receipt.roles, reviewed.input.roles) &&
    receipt.profile_value_count === reviewed.input.profile_values.length &&
    receipt.delivery_state === "queued" &&
    Date.parse(receipt.invitation_expires_at) > Date.parse(receipt.committed_at)
  );
}

async function userInvitationResendPurpose(
  userID: string,
  input: UserInvitationResendCommandInput,
): Promise<string> {
  return commandPurpose("invitation-resend", {
    user_id: entityIDSchema.parse(userID),
    expected_user_version: input.expected_user_version,
    reason: input.reason.trim(),
  });
}

function resolveDefinitiveUserInvitationFailure(
  purpose: string,
  commandID: string,
  error: unknown,
): void {
  if (
    error instanceof ApiError &&
    !error.retryable &&
    error.status < 500 &&
    !isRecentAuthenticationRequired(error)
  ) {
    commandAttemptStore.resolve(purpose, commandID);
  }
}

function invalidUserInvitationResponse(): TypeError {
  return new TypeError("invalid user invitation response");
}

function userRoleCommandPurpose(userID: string): string {
  return `authorization-user-roles-${userID}`;
}

function canonicalUserRoleInput(
  input: UserRoleCommandInput,
): UserRoleCommandInput {
  const parsed = userRoleCommandInputSchema.parse({
    ...input,
    reason: input.reason.trim(),
    roles: [...input.roles].sort(),
  });
  return { ...parsed, roles: [...parsed.roles].sort() };
}

function validUserRoleReview(
  review: UserRolePreviewResponse,
  userID: string,
  input: UserRoleCommandInput,
): boolean {
  const preview = review.preview;
  const before = [...preview.before.roles].sort();
  const after = [...preview.after.roles].sort();
  return (
    preview.user_id === userID &&
    preview.expected_revision === input.expected_revision &&
    preview.resulting_revision === input.expected_revision + 1 &&
    preview.before.revision === input.expected_revision &&
    preview.after.revision === input.expected_revision + 1 &&
    Date.parse(review.expires_at) > Date.now() &&
    stringListsEqual(after, input.roles) &&
    stringListsEqual(
      [...preview.added].sort(),
      stringDifference(after, before),
    ) &&
    stringListsEqual(
      [...preview.removed].sort(),
      stringDifference(before, after),
    )
  );
}

function stringDifference(left: string[], right: string[]): string[] {
  const rightValues = new Set(right);
  return left.filter((value) => !rightValues.has(value));
}

function stringListsEqual(left: string[], right: string[]): boolean {
  return (
    left.length === right.length &&
    left.every((value, index) => value === right[index])
  );
}

function resolveDefinitiveUserRoleFailure(
  purpose: string,
  commandID: string,
  error: unknown,
): void {
  if (
    error instanceof ApiError &&
    !error.retryable &&
    error.status < 500 &&
    error.status !== 403
  ) {
    commandAttemptStore.resolve(purpose, commandID);
  }
}

function invalidUserRoleResponse(): TypeError {
  return new TypeError("invalid user role response");
}

function canonicalUserLifecycleAction(
  action: UserLifecycleAction,
): UserLifecycleAction {
  if (!USER_LIFECYCLE_ACTIONS.includes(action)) {
    throw new TypeError("invalid user lifecycle action");
  }
  return action;
}

function canonicalUserLifecycleInput(
  input: UserLifecycleCommandInput,
): UserLifecycleCommandInput {
  return userLifecycleCommandInputSchema.parse({
    ...input,
    reason: input.reason.trim(),
  });
}

async function userLifecycleCommandPurpose(
  userID: string,
  action: UserLifecycleAction,
  input: UserLifecycleCommandInput,
): Promise<string> {
  return commandPurpose("user-lifecycle", {
    user_id: entityIDSchema.parse(userID),
    action: canonicalUserLifecycleAction(action),
    ...canonicalUserLifecycleInput(input),
  });
}

function validUserLifecycleReview(
  response: UserLifecyclePreviewResponse,
  user: AdminUser,
  action: UserLifecycleAction,
  input: UserLifecycleCommandInput,
): boolean {
  const preview = response.preview;
  return (
    preview.action === action &&
    preview.user_id === user.id &&
    preview.expected_version === input.expected_version &&
    preview.resulting_version === input.expected_version + 1 &&
    preview.before.id === user.id &&
    preview.before.username === user.username &&
    preview.before.status === user.status &&
    preview.before.version === user.version &&
    preview.before.suspended_at === user.suspended_at &&
    preview.after.id === user.id &&
    preview.after.username === user.username &&
    Date.parse(response.expires_at) > Date.now()
  );
}

function validUserLifecycleReceipt(
  receipt: UserLifecycleReceipt,
  reviewed: ReviewedUserLifecycleCommand,
): boolean {
  const expectedStatus =
    reviewed.action === "suspend"
      ? "suspended"
      : reviewed.action === "reactivate"
        ? "active"
        : "deleted";
  return (
    receipt.command_id === reviewed.command_id &&
    receipt.event_id === reviewed.command_id &&
    receipt.action === reviewed.action &&
    receipt.user.id === reviewed.user_id &&
    receipt.user.username === reviewed.review.preview.after.username &&
    receipt.user.status === expectedStatus &&
    receipt.user.version === reviewed.input.expected_version + 1
  );
}

function resolveDefinitiveUserLifecycleFailure(
  purpose: string,
  commandID: string,
  error: unknown,
): void {
  if (
    error instanceof ApiError &&
    !error.retryable &&
    error.status < 500 &&
    !isRecentAuthenticationRequired(error)
  ) {
    commandAttemptStore.resolve(purpose, commandID);
  }
}

export function isRecentAuthenticationRequired(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 403 &&
    error.params?.reason === "reauthentication_required"
  );
}

function invalidUserLifecycleResponse(): TypeError {
  return new TypeError("invalid user lifecycle response");
}
