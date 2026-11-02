import {
  PROFILE_ATTRIBUTE_KEY_PATTERN,
  PROFILE_ATTRIBUTE_LIMITS,
} from "@/constants/profile-attributes"
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts"
import {
  arraySchema,
  enumSchema,
  integerSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema"
import { profileAttributeValueSchema } from "@/lib/profile/profile-validation-schema"
import type {
  ProfilePrivacy,
  ProfilePrivacyMutationReceipt,
  PublicProfile,
  PublicProfileAttribute,
  PublicProfileAttributeValue,
} from "@/types/public-profile"
import { PUBLIC_PROFILE_ATTRIBUTE_DATA_TYPES } from "@/types/public-profile"

export const publicUsernameSchema = stringSchema({
  minimumLength: 3,
  maximumLength: 50,
  pattern: /^[A-Za-z0-9]+$/,
  label: "public username",
})

export const profileVisibilitySchema = enumSchema([
  "private",
  "public",
] as const)

const rawPublicProfileAttributeSchema = strictObjectSchema({
  key: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH,
    pattern: PROFILE_ATTRIBUTE_KEY_PATTERN,
    label: "public profile attribute key",
  }),
  data_type: enumSchema(PUBLIC_PROFILE_ATTRIBUTE_DATA_TYPES),
  label: stringSchema({
    minimumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MIN_LENGTH,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.LABEL_MAX_LENGTH,
    label: "public profile attribute label",
  }),
  description: stringSchema({
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.DESCRIPTION_MAX_LENGTH,
  }),
  value: {
    parse(value: unknown, path = "$"): PublicProfileAttributeValue {
      return profileAttributeValueSchema.parse(value, path)
    },
  } satisfies Schema<PublicProfileAttributeValue>,
})

const publicProfileAttributeSchema: Schema<PublicProfileAttribute> = {
  parse(value: unknown, path = "$") {
    const attribute = rawPublicProfileAttributeSchema.parse(value, path)
    if (!publicValueMatchesType(attribute.value, attribute.data_type)) {
      throw new TypeError(`${path}.value: value does not match data_type`)
    }
    return attribute
  },
}

export const publicProfileSchema: Schema<PublicProfile> = {
  parse(value: unknown, path = "$") {
    const profile = strictObjectSchema({
      username: publicUsernameSchema,
      joined_at: isoDateTimeSchema,
      attributes: arraySchema(publicProfileAttributeSchema, {
        maximumLength: 256,
      }),
    }).parse(value, path)
    const keys = new Set<string>()
    for (const [index, attribute] of profile.attributes.entries()) {
      if (keys.has(attribute.key)) {
        throw new TypeError(
          `${path}.attributes[${index}]: duplicate public attribute`,
        )
      }
      keys.add(attribute.key)
    }
    return profile
  },
}

export const profilePrivacySchema: Schema<ProfilePrivacy> =
  strictObjectSchema({
    visibility: profileVisibilitySchema,
    version: integerSchema({ minimum: 1, label: "profile privacy version" }),
    updated_at: isoDateTimeSchema,
  })

export const profilePrivacyMutationReceiptSchema: Schema<ProfilePrivacyMutationReceipt> = {
  parse(value: unknown, path = "$") {
    const receipt = strictObjectSchema({
      command_id: entityIDSchema,
      event_id: entityIDSchema,
      state: profilePrivacySchema,
      committed_at: isoDateTimeSchema,
    }).parse(value, path)
    if (receipt.command_id !== receipt.event_id) {
      throw new TypeError(`${path}: command and event identities differ`)
    }
    return receipt
  },
}

function publicValueMatchesType(
  value: PublicProfileAttributeValue,
  dataType: PublicProfileAttribute["data_type"],
): boolean {
  if (dataType === "string") return typeof value === "string"
  if (dataType === "number") {
    return typeof value === "number"
  }
  if (dataType === "boolean") return typeof value === "boolean"
  return typeof value === "string" && isCanonicalDate(value)
}

function isCanonicalDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value
}
