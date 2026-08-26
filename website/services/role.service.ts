import { api } from "@/lib/api/client";
import { AUTHORIZATION_API, ROLE_API } from "@/constants/api/identity";
import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";
import {
  authorizationCatalogSchema,
  createRoleRequestSchema,
  roleListSchema,
  rolePolicySchema,
  rolePolicyCommandInputSchema,
  rolePolicyMutationReceiptSchema,
  rolePolicyPreviewResponseSchema,
  roleMetadataCommandInputSchema,
  roleMetadataPreviewResponseSchema,
  roleMutationReceiptSchema,
} from "@/lib/admin/identity-schema";
import type {
  AuthorizationCatalog,
  CreateRoleRequest,
  Role,
  RolePolicySnapshot,
  RolePolicyCommandInput,
  RolePolicyMutationReceipt,
  RolePolicyPreviewResponse,
  ReviewedRolePolicyCommand,
  RoleMetadataCommandInput,
  RoleMetadataPreviewResponse,
  RoleMutationReceipt,
  ReviewedRoleMetadataCommand,
} from "@/types/admin";
import { entityIDSchema } from "@/lib/api/contracts";

type RoleMetadataAction = "update" | "archive";

export const roleService = {
  async getAll(signal?: AbortSignal): Promise<Role[]> {
    return api<Role[], never>(ROLE_API.FIND_ALL, {
      method: "GET",
      signal,
      schema: roleListSchema,
    });
  },

  async create(data: CreateRoleRequest): Promise<Role> {
    const input = canonicalCreateRoleInput(data);
    const purpose = roleCreateCommandPurpose(input.key);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const receipt = await api<
        RoleMutationReceipt,
        CreateRoleRequest & { command_id: string; expected_version: 1 }
      >(ROLE_API.CREATE, {
        method: "POST",
        body: {
          ...input,
          command_id: attempt.attempt_id,
          expected_version: 1,
        },
        idempotencyKey: attempt.attempt_id,
        schema: roleMutationReceiptSchema,
      });
      if (!validCreateRoleReceipt(receipt, attempt.attempt_id, input)) {
        throw invalidRoleMetadataResponse();
      }
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return receipt.role;
    } catch (error) {
      resolveDefinitiveCommandFailure(purpose, attempt.attempt_id, error);
      throw error;
    }
  },

  async previewMetadata(
    id: string,
    input: RoleMetadataCommandInput,
    action: RoleMetadataAction = "update",
  ): Promise<ReviewedRoleMetadataCommand> {
    const roleID = entityIDSchema.parse(id);
    const normalized = canonicalRoleMetadataInput(input, action);
    const purpose = roleMetadataCommandPurpose(roleID, action);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        RoleMetadataPreviewResponse,
        RoleMetadataCommandInput & { command_id: string }
      >(
        action === "archive"
          ? ROLE_API.ARCHIVE_PREVIEW(roleID)
          : ROLE_API.METADATA_PREVIEW(roleID),
        {
          method: "POST",
          body: { ...normalized, command_id: attempt.attempt_id },
          idempotencyKey: attempt.attempt_id,
          schema: roleMetadataPreviewResponseSchema,
        },
      );
      if (!validRoleMetadataReview(review, roleID, normalized, action)) {
        throw invalidRoleMetadataResponse();
      }
      return {
        role_id: roleID,
        command_id: attempt.attempt_id,
        input: normalized,
        review,
      };
    } catch (error) {
      resolveDefinitiveCommandFailure(purpose, attempt.attempt_id, error);
      throw error;
    }
  },

  async applyMetadata(reviewed: ReviewedRoleMetadataCommand): Promise<RoleMutationReceipt> {
    const roleID = entityIDSchema.parse(reviewed.role_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const action = reviewed.review.preview.action;
    const input = canonicalRoleMetadataInput(reviewed.input, action);
    if (!validRoleMetadataReview(reviewed.review, roleID, input, action)) {
      throw invalidRoleMetadataResponse();
    }
    const purpose = roleMetadataCommandPurpose(roleID, action);
    const current = commandAttemptStore.getOrCreate(purpose);
    if (current.attempt_id !== commandID) {
      throw new TypeError("role command identity expired");
    }
    try {
      const receipt = await api<
        RoleMutationReceipt,
        RoleMetadataCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(
        action === "archive"
          ? ROLE_API.ARCHIVE_APPLY(roleID)
          : ROLE_API.METADATA_APPLY(roleID),
        {
          method: "POST",
          body: {
            ...input,
            command_id: commandID,
            confirmation_token: reviewed.review.confirmation_token,
          },
          idempotencyKey: commandID,
          schema: roleMutationReceiptSchema,
        },
      );
      if (
        receipt.command_id !== commandID ||
        receipt.event_id !== commandID ||
        receipt.action !== action ||
        !sameRole(receipt.role, reviewed.review.preview.after)
      ) {
        throw invalidRoleMetadataResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveCommandFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonMetadataCommand(
    roleId: string,
    action: RoleMetadataAction,
    commandId?: string,
  ): void {
    const roleID = entityIDSchema.parse(roleId);
    commandAttemptStore.resolve(
      roleMetadataCommandPurpose(roleID, action),
      commandId ? entityIDSchema.parse(commandId) : undefined,
    );
  },

  async getCatalog(signal?: AbortSignal): Promise<AuthorizationCatalog> {
    return api<AuthorizationCatalog, never>(AUTHORIZATION_API.CATALOG, {
      method: "GET",
      signal,
      schema: authorizationCatalogSchema,
    });
  },

  async getPolicies(
    roleId: string,
    signal?: AbortSignal,
  ): Promise<RolePolicySnapshot> {
    return api<RolePolicySnapshot, never>(
      ROLE_API.POLICIES(entityIDSchema.parse(roleId)),
      { method: "GET", signal, schema: rolePolicySchema },
    );
  },

  async previewPolicies(
    roleId: string,
    input: RolePolicyCommandInput,
  ): Promise<ReviewedRolePolicyCommand> {
    const roleID = entityIDSchema.parse(roleId);
    const commandInput = canonicalRolePolicyInput(input);
    const purpose = rolePolicyCommandPurpose(roleID);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const review = await api<
        RolePolicyPreviewResponse,
        RolePolicyCommandInput & { command_id: string }
      >(ROLE_API.POLICIES_PREVIEW(roleID), {
        method: "POST",
        body: { ...commandInput, command_id: attempt.attempt_id },
        idempotencyKey: attempt.attempt_id,
        schema: rolePolicyPreviewResponseSchema,
      });
      if (!validRolePolicyReview(review, commandInput)) {
        throw invalidRolePolicyResponse();
      }
      return {
        role_id: roleID,
        command_id: attempt.attempt_id,
        input: commandInput,
        review,
      };
    } catch (error) {
      resolveDefinitiveCommandFailure(purpose, attempt.attempt_id, error);
      throw error;
    }
  },

  async applyPolicies(
    reviewed: ReviewedRolePolicyCommand,
  ): Promise<RolePolicyMutationReceipt> {
    const roleID = entityIDSchema.parse(reviewed.role_id);
    const commandID = entityIDSchema.parse(reviewed.command_id);
    const purpose = rolePolicyCommandPurpose(roleID);
    const activeAttempt = commandAttemptStore.getOrCreate(purpose);
    if (activeAttempt.attempt_id !== commandID) {
      throw new TypeError("authorization command identity expired");
    }
    try {
      const receipt = await api<
        RolePolicyMutationReceipt,
        RolePolicyCommandInput & {
          command_id: string;
          confirmation_token: string;
        }
      >(ROLE_API.POLICIES_APPLY(roleID), {
        method: "POST",
        body: {
          ...reviewed.input,
          command_id: commandID,
          confirmation_token: reviewed.review.confirmation_token,
        },
        idempotencyKey: commandID,
        schema: rolePolicyMutationReceiptSchema,
      });
      if (
        receipt.command_id !== commandID ||
        receipt.event_id !== commandID ||
        receipt.role_key !== reviewed.review.preview.role_key ||
        receipt.snapshot.revision !== reviewed.input.expected_revision + 1 ||
        !policyRulesEqual(
          canonicalPolicyRules(receipt.snapshot.rules),
          reviewed.input.rules,
        )
      ) {
        throw invalidRolePolicyResponse();
      }
      commandAttemptStore.resolve(purpose, commandID);
      return receipt;
    } catch (error) {
      resolveDefinitiveCommandFailure(purpose, commandID, error);
      throw error;
    }
  },

  abandonPolicyCommand(roleId: string, commandId?: string): void {
    const roleID = entityIDSchema.parse(roleId);
    commandAttemptStore.resolve(
      rolePolicyCommandPurpose(roleID),
      commandId ? entityIDSchema.parse(commandId) : undefined,
    );
  },
};

function canonicalCreateRoleInput(data: CreateRoleRequest): CreateRoleRequest {
  return createRoleRequestSchema.parse({
    key: data.key.trim(),
    name: data.name.trim(),
    ...(data.description === undefined
      ? {}
      : { description: data.description.trim() }),
    reason: data.reason.trim(),
  });
}

function roleCreateCommandPurpose(roleKey: string): string {
  const encodedKey = Array.from(roleKey, (character) =>
    character.charCodeAt(0).toString(16).padStart(2, "0"),
  ).join("");
  return `role-metadata-create-${encodedKey}`;
}

function canonicalRoleMetadataInput(
  input: RoleMetadataCommandInput,
  action: RoleMetadataAction,
): RoleMetadataCommandInput {
  if (
    action === "archive" &&
    (input.name !== undefined || input.description !== undefined)
  ) {
    throw new TypeError("archive metadata must be empty");
  }
  const parsed = roleMetadataCommandInputSchema.parse({
    expected_version: input.expected_version,
    reason: input.reason.trim(),
    ...(action === "update" && input.name !== undefined
      ? { name: input.name.trim() }
      : {}),
    ...(action === "update" && input.description !== undefined
      ? { description: input.description.trim() }
      : {}),
  });
  if (
    action === "update" &&
    parsed.name === undefined &&
    parsed.description === undefined
  ) {
    throw new TypeError("role metadata update is empty");
  }
  return parsed;
}

function roleMetadataCommandPurpose(
  roleID: string,
  action: RoleMetadataAction,
): string {
  return `role-metadata-${action}-${roleID}`;
}

function validCreateRoleReceipt(
  receipt: RoleMutationReceipt,
  commandID: string,
  input: CreateRoleRequest,
): boolean {
  return (
    receipt.command_id === commandID &&
    receipt.event_id === commandID &&
    receipt.action === "create" &&
    receipt.role.key === input.key &&
    receipt.role.name === input.name &&
    roleDescription(receipt.role) === (input.description ?? "") &&
    receipt.role.status === "active" &&
    receipt.role.version === 1 &&
    !receipt.role.is_system &&
    !receipt.role.is_protected
  );
}

function validRoleMetadataReview(
  review: RoleMetadataPreviewResponse,
  roleID: string,
  input: RoleMetadataCommandInput,
  action: RoleMetadataAction,
): boolean {
  const preview = review.preview;
  if (
    preview.action !== action ||
    preview.role_id !== roleID ||
    preview.expected_version !== input.expected_version ||
    preview.resulting_version !== input.expected_version + 1 ||
    preview.before.id !== roleID ||
    preview.after.id !== roleID ||
    preview.before.version !== input.expected_version ||
    preview.after.version !== input.expected_version + 1 ||
    preview.before.status !== "active" ||
    preview.before.key !== preview.after.key ||
    preview.before.is_system ||
    preview.after.is_system ||
    preview.before.is_protected ||
    preview.after.is_protected ||
    Date.parse(review.expires_at) <= Date.now()
  ) {
    return false;
  }

  if (action === "archive") {
    return (
      preview.after.status === "archived" &&
      preview.after.name === preview.before.name &&
      roleDescription(preview.after) === roleDescription(preview.before)
    );
  }
  return (
    preview.after.status === "active" &&
    preview.after.name === (input.name ?? preview.before.name) &&
    roleDescription(preview.after) ===
      (input.description ?? roleDescription(preview.before))
  );
}

function sameRole(left: Role, right: Role): boolean {
  return (
    left.id === right.id &&
    left.key === right.key &&
    left.name === right.name &&
    roleDescription(left) === roleDescription(right) &&
    left.is_system === right.is_system &&
    left.is_protected === right.is_protected &&
    left.status === right.status &&
    left.version === right.version
  );
}

function roleDescription(role: Role): string {
  return role.description ?? "";
}

function invalidRoleMetadataResponse(): TypeError {
  return new TypeError("invalid role metadata response");
}

function rolePolicyCommandPurpose(roleID: string): string {
  return `authorization-role-policies-${roleID}`;
}

function resolveDefinitiveCommandFailure(
  purpose: string,
  commandID: string,
  error: unknown,
): void {
  if (
    error instanceof ApiError &&
    !error.retryable &&
    error.status < 500 &&
    !(
      error.status === 403 &&
      error.params?.reason === "reauthentication_required"
    )
  ) {
    commandAttemptStore.resolve(purpose, commandID);
  }
}

function canonicalRolePolicyInput(
  input: RolePolicyCommandInput,
): RolePolicyCommandInput {
  const parsed = rolePolicyCommandInputSchema.parse({
    ...input,
    reason: input.reason.trim(),
    rules: input.rules.map(({ resource, action }) => ({ resource, action })),
  });
  const rules = [...parsed.rules].sort(comparePolicyRules);
  for (let index = 1; index < rules.length; index += 1) {
    if (comparePolicyRules(rules[index - 1]!, rules[index]!) === 0) {
      throw new TypeError("duplicate role policy rule");
    }
  }
  return { ...parsed, rules };
}

function validRolePolicyReview(
  review: RolePolicyPreviewResponse,
  input: RolePolicyCommandInput,
): boolean {
  const preview = review.preview;
  if (
    preview.expected_revision !== input.expected_revision ||
    preview.resulting_revision !== input.expected_revision + 1 ||
    preview.before.revision !== input.expected_revision ||
    preview.after.revision !== input.expected_revision + 1 ||
    Date.parse(review.expires_at) <= Date.now()
  ) {
    return false;
  }
  const before = canonicalPolicyRules(preview.before.rules);
  const after = canonicalPolicyRules(preview.after.rules);
  return (
    policyRulesEqual(after, input.rules) &&
    policyRulesEqual(
      canonicalPolicyRules(preview.added),
      policyRuleDifference(after, before),
    ) &&
    policyRulesEqual(
      canonicalPolicyRules(preview.removed),
      policyRuleDifference(before, after),
    )
  );
}

function canonicalPolicyRules(rules: RolePolicyCommandInput["rules"]) {
  return rules
    .map(({ resource, action }) => ({ resource, action }))
    .sort(comparePolicyRules);
}

function comparePolicyRules(
  left: RolePolicyCommandInput["rules"][number],
  right: RolePolicyCommandInput["rules"][number],
): number {
  return (
    left.resource.localeCompare(right.resource) ||
    left.action.localeCompare(right.action)
  );
}

function policyRuleDifference(
  left: RolePolicyCommandInput["rules"],
  right: RolePolicyCommandInput["rules"],
) {
  const rightKeys = new Set(
    right.map(({ resource, action }) => `${resource}\u0000${action}`),
  );
  return left.filter(
    ({ resource, action }) => !rightKeys.has(`${resource}\u0000${action}`),
  );
}

function policyRulesEqual(
  left: RolePolicyCommandInput["rules"],
  right: RolePolicyCommandInput["rules"],
): boolean {
  return (
    left.length === right.length &&
    left.every(
      (rule, index) => comparePolicyRules(rule, right[index]!) === 0,
    )
  );
}

function invalidRolePolicyResponse(): TypeError {
  return new TypeError("invalid role policy response");
}
