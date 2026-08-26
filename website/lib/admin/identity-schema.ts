import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  literalSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  entityIDSchema,
  isoDateTimeSchema,
  paginatedSchema,
} from "@/lib/api/contracts";
import type {
  AdminUser,
  AuthorizationCatalog,
  AuthorizationResource,
  AuthorizationAction,
  PolicyRule,
  CreateRoleRequest,
  Role,
  RolePolicyCommandInput,
  RolePolicyMutationReceipt,
  RolePolicyPreview,
  RolePolicyPreviewResponse,
  RolePolicySnapshot,
  RoleMetadataCommandInput,
  RoleMetadataPreview,
  RoleMetadataPreviewResponse,
  RoleMutationReceipt,
  UserRoleCommandInput,
  UserRoleMutationReceipt,
  UserRolePreview,
  UserRolePreviewResponse,
  UserRoleSnapshot,
  UserLifecycleProjection,
  UserLifecycleCommandInput,
  UserLifecyclePreview,
  UserLifecyclePreviewResponse,
  UserLifecycleReceipt,
  UserInvitationPreview,
  UserInvitationPreviewResponse,
  UserInvitationReceipt,
  UserInvitationResendPreview,
  UserInvitationResendPreviewResponse,
  UserInvitationResendReceipt,
} from "@/types/admin";
import {
  USER_LIFECYCLE_ACTIONS,
  USER_LIFECYCLE_STATUSES,
} from "@/types/admin";
import type { EffectiveCapabilitiesResponse } from "@/types/auth";
import type { Paginated } from "@/types/api";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";

const keySchema = stringSchema({
  minimumLength: 1,
  maximumLength: 128,
  pattern: /^[a-z][a-z0-9_.-]*$/,
  label: "authorization key",
});

const roleKeySchema = stringSchema({
  minimumLength: 2,
  maximumLength: 50,
  pattern: /^[a-z][a-z0-9_-]*$/,
  label: "role key",
});

const roleSchema: Schema<Role> = strictObjectSchema({
  id: entityIDSchema,
  key: roleKeySchema,
  name: stringSchema({ minimumLength: 2, maximumLength: 50, label: "role name" }),
  description: optionalSchema(stringSchema({ maximumLength: 255 })),
  is_system: booleanSchema,
  is_protected: booleanSchema,
  status: enumSchema(["active", "archived"] as const),
  version: integerSchema({ minimum: 1, label: "role version" }),
});

export const roleListSchema: Schema<Role[]> = {
  parse(value: unknown, path = "$") {
    const roles = arraySchema(roleSchema, { maximumLength: 256 }).parse(
      value,
      path,
    );
    if (new Set(roles.map((role) => role.id)).size !== roles.length) {
      throw new TypeError(`${path}: duplicate role identity`);
    }
    if (new Set(roles.map((role) => role.key)).size !== roles.length) {
      throw new TypeError(`${path}: duplicate role key`);
    }
    if (roles.some((role) => role.status !== "active")) {
      throw new TypeError(`${path}: archived role in active catalog`);
    }
    return roles;
  },
};

export const rolePageSchema: Schema<Paginated<Role>> = paginatedSchema(
  roleSchema,
);
export { roleSchema };

export const createRoleRequestSchema: Schema<CreateRoleRequest> =
  strictObjectSchema({
    key: roleKeySchema,
    name: stringSchema({
      minimumLength: 2,
      maximumLength: 50,
      label: "role name",
    }),
    description: optionalSchema(stringSchema({ maximumLength: 255 })),
    reason: stringSchema({ minimumLength: 3, maximumLength: 1024 }),
  });

export const roleMetadataCommandInputSchema: Schema<RoleMetadataCommandInput> =
  strictObjectSchema({
    expected_version: integerSchema({ minimum: 1, label: "role version" }),
    reason: stringSchema({ minimumLength: 3, maximumLength: 1024 }),
    name: optionalSchema(
      stringSchema({ minimumLength: 2, maximumLength: 50 }),
    ),
    description: optionalSchema(stringSchema({ maximumLength: 255 })),
  });

const roleMetadataPreviewSchema: Schema<RoleMetadataPreview> =
  strictObjectSchema({
    action: enumSchema(["update", "archive"] as const),
    role_id: entityIDSchema,
    expected_version: integerSchema({ minimum: 1 }),
    resulting_version: integerSchema({ minimum: 2 }),
    before: roleSchema,
    after: roleSchema,
  });

export const roleMetadataPreviewResponseSchema: Schema<RoleMetadataPreviewResponse> =
  strictObjectSchema({
    preview: roleMetadataPreviewSchema,
    confirmation_token: stringSchema({
      minimumLength: 1,
      maximumLength: 2048,
    }),
    expires_at: isoDateTimeSchema,
  });

export const roleMutationReceiptSchema: Schema<RoleMutationReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    action: enumSchema(["create", "update", "archive"] as const),
    role: roleSchema,
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

const authorizationActionSchema: Schema<AuthorizationAction> =
  strictObjectSchema({
    key: keySchema,
    label: stringSchema({ minimumLength: 1, maximumLength: 128 }),
    description: stringSchema({ maximumLength: 512 }),
  });

const authorizationResourceSchema: Schema<AuthorizationResource> =
  strictObjectSchema({
    key: keySchema,
    label: stringSchema({ minimumLength: 1, maximumLength: 128 }),
    description: stringSchema({ maximumLength: 512 }),
    actions: arraySchema(authorizationActionSchema, { maximumLength: 64 }),
  });

export const authorizationCatalogSchema: Schema<AuthorizationCatalog> = {
  parse(value: unknown, path = "$") {
    const parsed = strictObjectSchema({
      revision: integerSchema({ minimum: 1, label: "authorization revision" }),
      resources: arraySchema(authorizationResourceSchema, {
        maximumLength: 128,
      }),
    }).parse(value, path);
    const resourceKeys = new Set<string>();
    for (const resource of parsed.resources) {
      if (resourceKeys.has(resource.key)) {
        throw new TypeError(`${path}.resources: duplicate resource key`);
      }
      resourceKeys.add(resource.key);
      const actionKeys = resource.actions.map((action) => action.key);
      if (new Set(actionKeys).size !== actionKeys.length) {
        throw new TypeError(`${path}.resources.${resource.key}: duplicate action key`);
      }
    }
    return parsed as AuthorizationCatalog;
  },
};

const policyRuleSchema: Schema<PolicyRule> = strictObjectSchema({
  role_key: optionalSchema(keySchema),
  resource: keySchema,
  action: keySchema,
});

export const rolePolicyCommandInputSchema: Schema<RolePolicyCommandInput> =
  strictObjectSchema({
    expected_revision: integerSchema({
      minimum: 1,
      label: "expected authorization revision",
    }),
    reason: stringSchema({ minimumLength: 3, maximumLength: 500 }),
    rules: arraySchema(policyRuleSchema, { maximumLength: 256 }),
  });

export const rolePolicySchema: Schema<RolePolicySnapshot> = strictObjectSchema({
  revision: integerSchema({ minimum: 1, label: "policy revision" }),
  rules: arraySchema(policyRuleSchema, { maximumLength: 256 }),
});

const rolePolicyPreviewSchema: Schema<RolePolicyPreview> = strictObjectSchema({
  role_key: keySchema,
  expected_revision: integerSchema({ minimum: 1, label: "expected authorization revision" }),
  resulting_revision: integerSchema({ minimum: 2, label: "resulting authorization revision" }),
  before: rolePolicySchema,
  after: rolePolicySchema,
  added: arraySchema(policyRuleSchema, { maximumLength: 256 }),
  removed: arraySchema(policyRuleSchema, { maximumLength: 256 }),
});

export const rolePolicyPreviewResponseSchema: Schema<RolePolicyPreviewResponse> =
  strictObjectSchema({
    preview: rolePolicyPreviewSchema,
    confirmation_token: stringSchema({ minimumLength: 1, maximumLength: 2048 }),
    expires_at: isoDateTimeSchema,
  });

export const rolePolicyMutationReceiptSchema: Schema<RolePolicyMutationReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    role_key: keySchema,
    snapshot: rolePolicySchema,
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

const capabilityGrantSchema = strictObjectSchema({
  resource: keySchema,
  actions: arraySchema(keySchema, { maximumLength: 64, unique: true }),
});

export const effectiveCapabilitiesSchema: Schema<EffectiveCapabilitiesResponse> =
  strictObjectSchema({
    revision: integerSchema({ minimum: 0, label: "authorization revision" }),
    capabilities: arraySchema(capabilityGrantSchema, {
      maximumLength: 128,
    }),
  }) as Schema<EffectiveCapabilitiesResponse>;

const adminUserRawSchema = strictObjectSchema({
  id: entityIDSchema,
  username: stringSchema({
    minimumLength: 3,
    maximumLength: 50,
    pattern: /^[A-Za-z0-9]+$/,
  }),
  status: enumSchema(USER_LIFECYCLE_STATUSES),
  version: integerSchema({ minimum: 1, label: "user security version" }),
  suspended_at: optionalSchema(isoDateTimeSchema),
  roles: arraySchema(roleKeySchema, { maximumLength: 64, unique: true }),
  authorization_revision: integerSchema({
    minimum: 1,
    label: "user authorization revision",
  }),
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
});

export const adminUserSchema: Schema<AdminUser> = {
  parse(value: unknown, path = "$") {
    const user = adminUserRawSchema.parse(value, path);
    if (!validUserLifecycleStatusTimestamp(user.status, user.suspended_at)) {
      throw new TypeError(`${path}: inconsistent user lifecycle state`);
    }
    return user;
  },
};

const userLifecycleProjectionRawSchema = strictObjectSchema({
  id: entityIDSchema,
  username: stringSchema({
    minimumLength: 3,
    maximumLength: 50,
    pattern: /^[A-Za-z0-9]+$/,
  }),
  status: enumSchema(USER_LIFECYCLE_STATUSES),
  version: integerSchema({ minimum: 1, label: "user security version" }),
  suspended_at: optionalSchema(isoDateTimeSchema),
});

export const userLifecycleProjectionSchema: Schema<UserLifecycleProjection> = {
  parse(value: unknown, path = "$") {
    const projection = userLifecycleProjectionRawSchema.parse(value, path);
    if (
      !validUserLifecycleStatusTimestamp(
        projection.status,
        projection.suspended_at,
      )
    ) {
      throw new TypeError(`${path}: inconsistent user lifecycle projection`);
    }
    return projection;
  },
};

const userLifecyclePreviewRawSchema = strictObjectSchema({
  action: enumSchema(USER_LIFECYCLE_ACTIONS),
  user_id: entityIDSchema,
  expected_version: integerSchema({ minimum: 1 }),
  resulting_version: integerSchema({ minimum: 2 }),
  before: userLifecycleProjectionSchema,
  after: userLifecycleProjectionSchema,
});

const userLifecyclePreviewSchema: Schema<UserLifecyclePreview> = {
  parse(value: unknown, path = "$") {
    const preview = userLifecyclePreviewRawSchema.parse(value, path);
    if (
      preview.resulting_version !== preview.expected_version + 1 ||
      preview.before.id !== preview.user_id ||
      preview.after.id !== preview.user_id ||
      preview.before.username !== preview.after.username ||
      preview.before.version !== preview.expected_version ||
      preview.after.version !== preview.resulting_version ||
      !validUserLifecycleTransition(
        preview.action,
        preview.before.status,
        preview.after.status,
      )
    ) {
      throw new TypeError(`${path}: inconsistent user lifecycle preview`);
    }
    return preview;
  },
};

export const userLifecyclePreviewResponseSchema: Schema<UserLifecyclePreviewResponse> =
  strictObjectSchema({
    preview: userLifecyclePreviewSchema,
    confirmation_token: stringSchema({
      minimumLength: 1,
      maximumLength: 2048,
    }),
    expires_at: isoDateTimeSchema,
  });

export const userLifecycleReceiptSchema: Schema<UserLifecycleReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    action: enumSchema(USER_LIFECYCLE_ACTIONS),
    user: userLifecycleProjectionSchema,
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

export const userLifecycleCommandInputSchema: Schema<UserLifecycleCommandInput> =
  strictObjectSchema({
  expected_version: integerSchema({
    minimum: 1,
    label: "expected user security version",
  }),
  reason: stringSchema({
    minimumLength: IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MIN_LENGTH,
    maximumLength: IDENTITY_INPUT_LIMITS.USER_LIFECYCLE_REASON_MAX_LENGTH,
  }),
  });

function validUserLifecycleStatusTimestamp(
  status: (typeof USER_LIFECYCLE_STATUSES)[number],
  suspendedAt: string | undefined,
): boolean {
  return status === "suspended" ? suspendedAt !== undefined : suspendedAt === undefined;
}

function validUserLifecycleTransition(
  action: (typeof USER_LIFECYCLE_ACTIONS)[number],
  before: (typeof USER_LIFECYCLE_STATUSES)[number],
  after: (typeof USER_LIFECYCLE_STATUSES)[number],
): boolean {
  if (action === "suspend") return before === "active" && after === "suspended";
  if (action === "reactivate") {
    return before === "suspended" && after === "active";
  }
  return (
    before === "invited" ||
    before === "active" ||
    before === "suspended"
  ) && after === "deleted";
}

export const userRoleCommandInputSchema: Schema<UserRoleCommandInput> =
  strictObjectSchema({
    expected_revision: integerSchema({
      minimum: 1,
      label: "expected authorization revision",
    }),
    reason: stringSchema({ minimumLength: 3, maximumLength: 500 }),
    roles: arraySchema(roleKeySchema, {
      minimumLength: 1,
      maximumLength: 64,
      unique: true,
    }),
  });

const userRoleSnapshotSchema: Schema<UserRoleSnapshot> = strictObjectSchema({
  revision: integerSchema({ minimum: 1, label: "authorization revision" }),
  roles: arraySchema(roleKeySchema, {
    maximumLength: 64,
    unique: true,
  }),
});

const userRolePreviewSchema: Schema<UserRolePreview> = strictObjectSchema({
  user_id: entityIDSchema,
  expected_revision: integerSchema({
    minimum: 1,
    label: "expected authorization revision",
  }),
  resulting_revision: integerSchema({
    minimum: 2,
    label: "resulting authorization revision",
  }),
  before: userRoleSnapshotSchema,
  after: userRoleSnapshotSchema,
  added: arraySchema(roleKeySchema, { maximumLength: 64, unique: true }),
  removed: arraySchema(roleKeySchema, { maximumLength: 64, unique: true }),
});

export const userRolePreviewResponseSchema: Schema<UserRolePreviewResponse> =
  strictObjectSchema({
    preview: userRolePreviewSchema,
    confirmation_token: stringSchema({ minimumLength: 1, maximumLength: 2048 }),
    expires_at: isoDateTimeSchema,
  });

export const userRoleMutationReceiptSchema: Schema<UserRoleMutationReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    user_id: entityIDSchema,
    snapshot: userRoleSnapshotSchema,
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

const userInvitationPreviewRawSchema = strictObjectSchema({
  username: stringSchema({
    minimumLength: 3,
    maximumLength: 50,
    pattern: /^[A-Za-z0-9]+$/,
  }),
  masked_destination: stringSchema({ minimumLength: 3, maximumLength: 320 }),
  roles: arraySchema(roleKeySchema, {
    minimumLength: 1,
    maximumLength: 64,
    unique: true,
  }),
  profile_value_count: integerSchema({ minimum: 1, maximum: 128 }),
});

const userInvitationPreviewSchema: Schema<UserInvitationPreview> = {
  parse(value: unknown, path = "$") {
    const preview = userInvitationPreviewRawSchema.parse(value, path);
    if (!isStrictlySorted(preview.roles)) {
      throw new TypeError(`${path}.roles: roles are not canonical`);
    }
    return preview;
  },
};

export const userInvitationPreviewResponseSchema: Schema<UserInvitationPreviewResponse> =
  strictObjectSchema({
    preview: userInvitationPreviewSchema,
    confirmation_token: stringSchema({ minimumLength: 1, maximumLength: 2048 }),
    expires_at: isoDateTimeSchema,
  });

export const userInvitationReceiptSchema: Schema<UserInvitationReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    user: userLifecycleProjectionSchema,
    roles: arraySchema(roleKeySchema, {
      minimumLength: 1,
      maximumLength: 64,
      unique: true,
    }),
    profile_value_count: integerSchema({ minimum: 1, maximum: 128 }),
    invitation_expires_at: isoDateTimeSchema,
    delivery_state: literalSchema("queued"),
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

const userInvitationResendPreviewRawSchema = strictObjectSchema({
  user: userLifecycleProjectionSchema,
  masked_destination: stringSchema({ minimumLength: 3, maximumLength: 320 }),
  expected_invitation_version: integerSchema({ minimum: 1 }),
  resulting_invitation_version: integerSchema({ minimum: 2 }),
  current_expires_at: isoDateTimeSchema,
});

const userInvitationResendPreviewSchema: Schema<UserInvitationResendPreview> = {
  parse(value: unknown, path = "$") {
    const preview = userInvitationResendPreviewRawSchema.parse(value, path);
    if (
      preview.user.status !== "invited" ||
      preview.resulting_invitation_version !==
        preview.expected_invitation_version + 1
    ) {
      throw new TypeError(`${path}: inconsistent invitation resend preview`);
    }
    return preview;
  },
};

export const userInvitationResendPreviewResponseSchema: Schema<UserInvitationResendPreviewResponse> =
  strictObjectSchema({
    preview: userInvitationResendPreviewSchema,
    confirmation_token: stringSchema({ minimumLength: 1, maximumLength: 2048 }),
    expires_at: isoDateTimeSchema,
  });

export const userInvitationResendReceiptSchema: Schema<UserInvitationResendReceipt> =
  strictObjectSchema({
    command_id: entityIDSchema,
    event_id: entityIDSchema,
    user: userLifecycleProjectionSchema,
    invitation_version: integerSchema({ minimum: 2 }),
    invitation_expires_at: isoDateTimeSchema,
    delivery_state: literalSchema("queued"),
    committed_at: isoDateTimeSchema,
    idempotent_replay: booleanSchema,
  });

export const adminUserPageSchema: Schema<Paginated<AdminUser>> =
  paginatedSchema(adminUserSchema);

export const commandSuccessSchema: Schema<{ success: true }> =
  strictObjectSchema({ success: literalSchema(true) });

function isStrictlySorted(values: string[]): boolean {
  for (let index = 1; index < values.length; index += 1) {
    if (values[index - 1] >= values[index]) return false;
  }
  return true;
}
