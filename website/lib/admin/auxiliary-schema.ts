import {
  entityIDSchema,
  enumSchema,
  isoDateTimeSchema,
  paginatedSchema,
} from "@/lib/api/contracts";
import { arraySchema, booleanSchema, integerSchema, jsonValueSchema, optionalSchema, strictObjectSchema, stringSchema, type Schema } from "@/lib/api/schema";
import { operationSchema } from "@/lib/operations/operation-schema";
import {
  PROFILE_ATTRIBUTE_KEY_PATTERN,
  PROFILE_ATTRIBUTE_LIMITS,
} from "@/constants/profile-attributes";
import { parseProfileAttributeValidation } from "@/lib/profile/profile-validation-schema";
import {
  ATTRIBUTE_DATA_TYPES,
  type AttributeLifecycleCommandInput,
  type AttributeLifecyclePreviewResponse,
  type AttributeLifecycleReceipt,
  type AttributeDefinition,
  type AttributeDefinitionRevision,
} from "@/types/admin-auxiliary";

const attributeDataTypeSchema = enumSchema(ATTRIBUTE_DATA_TYPES);
const attributeVisibilitySchema = enumSchema(["public", "private", "admin"] as const);
const reviewHashSchema: Schema<string> = {
  parse(value: unknown, path = "$") {
    const parsed = stringSchema({
      minimumLength: 64,
      maximumLength: 64,
      label: "attribute review hash",
    }).parse(value, path);
    if (!/^[0-9a-f]{64}$/.test(parsed)) {
      throw new TypeError(`${path}: invalid attribute review hash`);
    }
    return parsed;
  },
};
const rawAttributeDefinitionRevisionSchema = strictObjectSchema({
  id: entityIDSchema,
  definition_id: entityIDSchema,
  revision: integerSchema({ minimum: 1, label: "attribute revision" }),
  display_order: integerSchema({
    minimum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MIN,
    maximum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MAX,
    label: "attribute display order",
  }),
  data_type: attributeDataTypeSchema,
  label: stringSchema({ minimumLength: 2, maximumLength: 128 }),
  description: stringSchema({ maximumLength: 1024 }),
  visibility: attributeVisibilitySchema,
  user_editable: booleanSchema,
  validation: jsonValueSchema,
  created_at: isoDateTimeSchema,
});

const attributeDefinitionRevisionSchema: Schema<AttributeDefinitionRevision> = {
  parse(value: unknown, path = "$") {
    const revision = rawAttributeDefinitionRevisionSchema.parse(value, path);
    return {
      ...revision,
      validation: parseProfileAttributeValidation(
        revision.validation,
        revision.data_type,
        `${path}.validation`,
      ),
    } as AttributeDefinitionRevision;
  },
};

export const attributeDefinitionRevisionPageSchema = strictObjectSchema({
  records: arraySchema(attributeDefinitionRevisionSchema, { maximumLength: 100 }),
  next_cursor: optionalSchema(
    stringSchema({ minimumLength: 1, maximumLength: 32, label: "attribute revision cursor" }),
  ),
});

const rawAttributeDefinitionSchema = strictObjectSchema({
    id: entityIDSchema,
    key: stringSchema({
      minimumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH,
      maximumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH,
      pattern: PROFILE_ATTRIBUTE_KEY_PATTERN,
      label: "attribute key",
    }),
    kind: enumSchema(["system", "custom"] as const),
    required_on_onboarding: booleanSchema,
    status: enumSchema(["active", "archived"] as const),
    version: integerSchema({ minimum: 1, label: "attribute definition version" }),
    draft_revision_id: entityIDSchema,
    active_revision_id: entityIDSchema,
    draft_revision: attributeDefinitionRevisionSchema,
    active_revision: attributeDefinitionRevisionSchema,
  });

export const attributeDefinitionSchema: Schema<AttributeDefinition> = {
  parse(value: unknown, path = "$") {
    const definition = rawAttributeDefinitionSchema.parse(value, path);
    if (
      definition.draft_revision_id !== definition.draft_revision.id ||
      definition.active_revision_id !== definition.active_revision.id ||
      definition.draft_revision.definition_id !== definition.id ||
      definition.active_revision.definition_id !== definition.id ||
      (definition.kind === "system" && !definition.required_on_onboarding) ||
      (definition.required_on_onboarding &&
        (!definition.draft_revision.user_editable ||
          definition.draft_revision.visibility === "admin" ||
          !definition.active_revision.user_editable ||
          definition.active_revision.visibility === "admin"))
    ) {
      throw new TypeError(`${path}: invalid attribute revision pointers`);
    }
    return definition as AttributeDefinition;
  },
};

export const attributeDefinitionPageSchema = paginatedSchema(attributeDefinitionSchema);

export const attributeLifecycleCommandInputSchema: Schema<AttributeLifecycleCommandInput> =
  strictObjectSchema({
    expected_version: integerSchema({ minimum: 1, label: "attribute version" }),
    candidate_revision_id: optionalSchema(entityIDSchema),
    reason: stringSchema({ minimumLength: 3, maximumLength: 500 }),
  });

const attributeLifecyclePreviewSchema = strictObjectSchema({
  action: enumSchema(["activate", "archive"] as const),
  definition_id: entityIDSchema,
  expected_version: integerSchema({ minimum: 1, label: "attribute version" }),
  resulting_version: integerSchema({ minimum: 2, label: "resulting attribute version" }),
  current_revision: optionalSchema(attributeDefinitionRevisionSchema),
  candidate_revision: optionalSchema(attributeDefinitionRevisionSchema),
  source_value_count: integerSchema({ minimum: 0, label: "source value count" }),
  target_value_count: integerSchema({ minimum: 0, label: "target value count" }),
  requires_migration: booleanSchema,
});

export const attributeLifecyclePreviewResponseSchema: Schema<AttributeLifecyclePreviewResponse> =
  strictObjectSchema({
    preview: attributeLifecyclePreviewSchema,
    confirmation_token: stringSchema({ minimumLength: 1, maximumLength: 2048 }),
    review_hash: reviewHashSchema,
    expires_at: isoDateTimeSchema,
  });

const attributeLifecycleReceiptPayloadSchema = strictObjectSchema({
  status: enumSchema(["committed", "migration_scheduled"] as const),
  action: enumSchema(["activate", "archive"] as const),
  command_id: entityIDSchema,
  event_id: optionalSchema(entityIDSchema),
  definition_id: entityIDSchema,
  version: integerSchema({ minimum: 1, label: "attribute version" }),
  definition: attributeDefinitionSchema,
  active_revision: optionalSchema(attributeDefinitionRevisionSchema),
  operation: optionalSchema(operationSchema),
  recorded_at: isoDateTimeSchema,
  idempotent_replay: booleanSchema,
});

export const attributeLifecycleReceiptSchema: Schema<AttributeLifecycleReceipt> = {
  parse(value: unknown, path = "$") {
    const receipt = attributeLifecycleReceiptPayloadSchema.parse(value, path);
    const validScheduled =
      receipt.status === "migration_scheduled" &&
      receipt.action === "activate" &&
      receipt.event_id === undefined &&
      receipt.active_revision !== undefined &&
      receipt.operation !== undefined;
    const validCommittedActivation =
      receipt.status === "committed" &&
      receipt.action === "activate" &&
      receipt.event_id !== undefined &&
      receipt.active_revision !== undefined &&
      receipt.operation === undefined;
    const validCommittedArchive =
      receipt.status === "committed" &&
      receipt.action === "archive" &&
      receipt.event_id !== undefined &&
      receipt.active_revision === undefined &&
      receipt.operation === undefined;
    if (!validScheduled && !validCommittedActivation && !validCommittedArchive) {
      throw new TypeError(`${path}: invalid attribute lifecycle receipt variant`);
    }
    return receipt as AttributeLifecycleReceipt;
  },
};
