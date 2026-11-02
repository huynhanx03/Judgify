import {
  PROFILE_ATTRIBUTE_KEY_PATTERN,
  PROFILE_ATTRIBUTE_LIMITS,
  isProfileAttributeNumber,
} from "@/constants/profile-attributes"
import { entityIDSchema } from "@/lib/api/contracts"
import {
  arraySchema,
  assertOnlyKeys,
  booleanSchema,
  enumSchema,
  integerSchema,
  jsonValueSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema"
import { profileAttrsSchema } from "@/lib/auth/profile-schema"
import {
  parseProfileAttributeValidation,
  profileAttributeValueSchema,
} from "@/lib/profile/profile-validation-schema"
import type {
  ProfileAttribute,
  ProfileAttributeValue,
  ProfileAttributesResponse,
} from "@/types/user"
import { PROFILE_ATTRIBUTE_DATA_TYPES } from "@/types/user"

const rawAttributeSchema = strictObjectSchema({
  definition_id: entityIDSchema,
  definition_revision_id: entityIDSchema,
  key: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH,
    pattern: PROFILE_ATTRIBUTE_KEY_PATTERN,
    label: "attribute key",
  }),
  kind: enumSchema(["system", "custom"] as const),
  required_on_onboarding: booleanSchema,
  data_type: enumSchema(PROFILE_ATTRIBUTE_DATA_TYPES),
  label: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MAX_LENGTH,
    label: "attribute label",
  }),
  description: stringSchema({
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.DESCRIPTION_MAX_LENGTH,
    label: "attribute description",
  }),
  visibility: enumSchema(["public", "private"] as const),
  user_editable: booleanSchema,
  display_order: integerSchema({
    minimum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MIN,
    maximum: PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MAX,
    label: "attribute display order",
  }),
  validation: jsonValueSchema,
  has_value: booleanSchema,
  value: optionalSchema(profileAttributeValueSchema),
  value_version: integerSchema({ minimum: 0, label: "attribute value version" }),
})

export const profileAttributeSchema: Schema<ProfileAttribute> = {
  parse(value: unknown, path = "$") {
    const attribute = rawAttributeSchema.parse(value, path)
    const validation = parseProfileAttributeValidation(
      attribute.validation,
      attribute.data_type,
      `${path}.validation`,
    )
    const hasValue = attribute.value !== undefined
    if (
      attribute.has_value !== hasValue ||
      (attribute.has_value && attribute.value_version < 1) ||
      (!attribute.has_value && attribute.value_version !== 0)
    ) {
      throw new TypeError(`${path}: inconsistent profile value state`)
    }
    if (hasValue && !valueMatchesType(attribute.value, attribute.data_type)) {
      throw new TypeError(`${path}.value: value does not match data_type`)
    }
    return { ...attribute, validation } as ProfileAttribute
  },
}

export const profileAttributesResponseSchema: Schema<ProfileAttributesResponse> = {
  parse(value: unknown, path = "$") {
    const source = recordAt(value, path)
    assertOnlyKeys(source, ["profile", "attributes"], path)
    const profile = profileAttrsSchema.parse(source.profile, `${path}.profile`)
    const attributes = arraySchema(profileAttributeSchema, {
      maximumLength: PROFILE_ATTRIBUTE_LIMITS.PROFILE_ATTRIBUTES,
    }).parse(source.attributes, `${path}.attributes`)
    const seen = new Set<string>()
    let previous: ProfileAttribute | undefined
    for (const attribute of attributes) {
      if (seen.has(attribute.definition_id)) {
        throw new TypeError(`${path}.attributes: duplicate definition`)
      }
      if (previous && compareProfileAttributes(previous, attribute) >= 0) {
        throw new TypeError(`${path}.attributes: attributes are not canonical`)
      }
      seen.add(attribute.definition_id)
      previous = attribute
    }
    return { profile, attributes }
  },
}

function compareProfileAttributes(
  left: ProfileAttribute,
  right: ProfileAttribute,
): number {
  if (left.display_order !== right.display_order) {
    return left.display_order - right.display_order
  }
  const keyOrder = compareCanonicalText(left.key, right.key)
  if (keyOrder !== 0) return keyOrder
  return compareCanonicalText(left.definition_id, right.definition_id)
}

function compareCanonicalText(left: string, right: string): number {
  if (left === right) return 0
  return left < right ? -1 : 1
}

function valueMatchesType(
  value: ProfileAttributeValue | undefined,
  dataType: ProfileAttribute["data_type"],
): boolean {
  if (value === undefined) return false
  if (dataType === "string") return typeof value === "string"
  if (dataType === "number") return isProfileAttributeNumber(value)
  if (dataType === "boolean") return typeof value === "boolean"
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return false
  }
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return !Number.isNaN(parsed.valueOf()) &&
    parsed.toISOString().slice(0, 10) === value
}
