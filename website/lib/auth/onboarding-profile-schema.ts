import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import {
  PROFILE_ATTRIBUTE_KEY_PATTERN,
  PROFILE_ATTRIBUTE_LIMITS,
} from "@/constants/profile-attributes";
import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  jsonValueSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import { ApiError } from "@/lib/api/error";
import {
  parseProfileAttributeValidation,
  profileAttributeValueSchema,
} from "@/lib/profile/profile-validation-schema";
import type {
  OnboardingProfileSchemaField,
  OnboardingProfileSchemaResponse,
  ProfileValueSuggestion,
} from "@/types/auth";
import {
  PROFILE_ATTRIBUTE_DATA_TYPES,
} from "@/types/user";

const SHA256_HEX_PATTERN = /^[0-9a-f]{64}$/;

const rawProfileSchemaField = strictObjectSchema({
  definition_id: entityIDSchema,
  definition_revision_id: entityIDSchema,
  key: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH,
    pattern: PROFILE_ATTRIBUTE_KEY_PATTERN,
    label: "profile attribute key",
  }),
  display_order: integerSchema({
    minimum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MIN,
    maximum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MAX,
    label: "profile attribute display order",
  }),
  data_type: enumSchema(PROFILE_ATTRIBUTE_DATA_TYPES),
  label: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MAX_LENGTH,
    label: "profile attribute label",
  }),
  description: stringSchema({
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.DESCRIPTION_MAX_LENGTH,
  }),
  visibility: enumSchema(["public", "private"] as const),
  user_editable: booleanSchema,
  validation: jsonValueSchema,
});

const profileSchemaFieldSchema: Schema<OnboardingProfileSchemaField> = {
  parse(value: unknown, path = "$") {
    const field = rawProfileSchemaField.parse(value, path);
    return {
      ...field,
      validation: parseProfileAttributeValidation(
        field.validation,
        field.data_type,
        `${path}.validation`,
      ),
    } as OnboardingProfileSchemaField;
  },
};

export const onboardingProfileSchemaResponseSchema: Schema<OnboardingProfileSchemaResponse> = {
  parse(value: unknown, path = "$") {
    const response = strictObjectSchema({
      revision: integerSchema({ minimum: 1, label: "profile schema revision" }),
      checksum: stringSchema({
        minimumLength: 64,
        maximumLength: 64,
        pattern: SHA256_HEX_PATTERN,
        label: "profile schema checksum",
      }),
      fields: arraySchema(profileSchemaFieldSchema, {
        minimumLength: 1,
        maximumLength: PROFILE_ATTRIBUTE_LIMITS.SCHEMA_FIELDS,
      }),
      server_time: isoDateTimeSchema,
    }).parse(value, path);

    const definitionIDs = new Set<string>();
    const revisionIDs = new Set<string>();
    const keys = new Set<string>();
    let previousField: OnboardingProfileSchemaField | null = null;
    for (const field of response.fields) {
      if (
        definitionIDs.has(field.definition_id) ||
        revisionIDs.has(field.definition_revision_id) ||
        keys.has(field.key)
      ) {
        throw new TypeError(`${path}.fields: duplicate profile definition`);
      }
      if (
        previousField &&
        (field.display_order < previousField.display_order ||
          (field.display_order === previousField.display_order &&
            field.key <= previousField.key))
      ) {
        throw new TypeError(`${path}.fields: profile definitions are not canonical`);
      }
      if (!field.user_editable) {
        throw new TypeError(`${path}.fields: required onboarding field is read-only`);
      }
      definitionIDs.add(field.definition_id);
      revisionIDs.add(field.definition_revision_id);
      keys.add(field.key);
      previousField = field;
    }
    return response as OnboardingProfileSchemaResponse;
  },
};

const profileValueSuggestionSchema = strictObjectSchema({
  key: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH,
    pattern: PROFILE_ATTRIBUTE_KEY_PATTERN,
  }),
  value: profileAttributeValueSchema,
});

export const profileValueSuggestionsSchema: Schema<ProfileValueSuggestion[]> = {
  parse(value: unknown, path = "$") {
    const suggestions = arraySchema(profileValueSuggestionSchema, {
      maximumLength: PROFILE_ATTRIBUTE_LIMITS.SCHEMA_FIELDS,
    }).parse(value, path) as ProfileValueSuggestion[];
    if (new Set(suggestions.map((suggestion) => suggestion.key)).size !== suggestions.length) {
      throw new TypeError(`${path}: duplicate profile suggestion key`);
    }
    return suggestions;
  },
};

export function isProfileSchemaConflict(error: unknown): error is ApiError {
  return (
    error instanceof ApiError &&
    error.status === 409 &&
    error.params?.reason === "profile_schema_changed"
  );
}

export function invalidProfileValueDefinitionID(error: unknown): string | null {
  if (
    !(error instanceof ApiError) ||
    error.status !== 400 ||
    error.params?.reason !== "invalid_profile_value"
  ) {
    return null;
  }
  const definitionID = error.params.definition_id;
  try {
    return entityIDSchema.parse(definitionID, "$.error.params.definition_id");
  } catch {
    return null;
  }
}
