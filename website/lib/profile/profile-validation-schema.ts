import {
  PROFILE_ATTRIBUTE_LIMITS,
  isProfileAttributeNumber,
} from "@/constants/profile-attributes"
import {
  assertOnlyKeys,
  integerSchema,
  recordAt,
  stringSchema,
  type Schema,
} from "@/lib/api/schema"
import type {
  ProfileAttributeDataType,
  ProfileAttributeValidation,
  ProfileAttributeValue,
} from "@/types/user"

const PROFILE_VALIDATION_KEYS = [
  "enum",
  "min_length",
  "max_length",
  "pattern",
  "minimum",
  "maximum",
] as const

export const profileAttributeValueSchema: Schema<ProfileAttributeValue> = {
  parse(value: unknown, path = "$") {
    if (
      typeof value !== "string" &&
      typeof value !== "boolean" &&
      !isProfileAttributeNumber(value)
    ) {
      throw new TypeError(`${path}: expected a typed profile scalar`)
    }
    if (
      profileCanonicalJSONByteLength(value) >
      PROFILE_ATTRIBUTE_LIMITS.CANONICAL_BYTES
    ) {
      throw new TypeError(`${path}: profile scalar exceeds canonical boundary`)
    }
    return value
  },
}

export function parseProfileAttributeValidation(
  value: unknown,
  dataType: ProfileAttributeDataType,
  path = "$",
): ProfileAttributeValidation {
  const source = recordAt(value, path)
  if (
    profileCanonicalJSONByteLength(source) >
    PROFILE_ATTRIBUTE_LIMITS.VALIDATION_LENGTH
  ) {
    throw new TypeError(`${path}: profile validation exceeds canonical boundary`)
  }
  assertOnlyKeys(source, PROFILE_VALIDATION_KEYS, path)
  const result: ProfileAttributeValidation = {}

  if (source.enum !== undefined) {
    if (
      !Array.isArray(source.enum) ||
      source.enum.length > PROFILE_ATTRIBUTE_LIMITS.ENUM_VALUES
    ) {
      throw new TypeError(`${path}.enum: invalid enum values`)
    }
    const values = source.enum.map((entry, index) =>
      profileAttributeValueSchema.parse(entry, `${path}.enum[${index}]`),
    )
    if (
      values.some((entry) => !profileValueMatchesType(entry, dataType)) ||
      new Set(values.map((entry) => JSON.stringify(entry))).size !== values.length
    ) {
      throw new TypeError(`${path}.enum: values do not match the field type`)
    }
    result.enum = values
  }

  const minimumLength = optionalValidationInteger(
    source.min_length,
    `${path}.min_length`,
  )
  const maximumLength = optionalValidationInteger(
    source.max_length,
    `${path}.max_length`,
  )
  const pattern = optionalValidationPattern(source.pattern, `${path}.pattern`)
  const minimum = optionalValidationNumber(source.minimum, `${path}.minimum`)
  const maximum = optionalValidationNumber(source.maximum, `${path}.maximum`)

  if (
    (minimumLength !== undefined ||
      maximumLength !== undefined ||
      pattern !== undefined) &&
    dataType !== "string"
  ) {
    throw new TypeError(`${path}: string validation on a non-string field`)
  }
  if (
    (minimum !== undefined || maximum !== undefined) &&
    dataType !== "number"
  ) {
    throw new TypeError(`${path}: numeric validation on a non-number field`)
  }
  if (
    minimumLength !== undefined &&
    maximumLength !== undefined &&
    minimumLength > maximumLength
  ) {
    throw new TypeError(`${path}: minimum length exceeds maximum length`)
  }
  if (minimum !== undefined && maximum !== undefined && minimum > maximum) {
    throw new TypeError(`${path}: minimum exceeds maximum`)
  }

  if (minimumLength !== undefined) result.min_length = minimumLength
  if (maximumLength !== undefined) result.max_length = maximumLength
  if (pattern !== undefined) result.pattern = pattern
  if (minimum !== undefined) result.minimum = minimum
  if (maximum !== undefined) result.maximum = maximum
  return result
}

// JSON.parse has already converted numeric tokens by the time ordinary schema
// validation runs. Admin-authored validation uses this lexical check as well,
// so a decimal that JavaScript would silently round is rejected before the
// request body is serialized.
export function isCanonicalProfileValidationText(
  raw: string,
  value: unknown,
): boolean {
  const serialized = JSON.stringify(value)
  if (typeof serialized !== "string") return false
  let compact = ""
  let inString = false
  let escaped = false
  for (const character of raw) {
    if (inString) {
      compact += character
      if (escaped) {
        escaped = false
      } else if (character === "\\") {
        escaped = true
      } else if (character === '"') {
        inString = false
      }
      continue
    }
    if (character === '"') {
      inString = true
      compact += character
      continue
    }
    if (
      character === " " ||
      character === "\t" ||
      character === "\r" ||
      character === "\n"
    ) {
      continue
    }
    compact += character
  }
  return !inString && !escaped && compact === serialized &&
    profileCanonicalJSONByteLength(value) <=
      PROFILE_ATTRIBUTE_LIMITS.VALIDATION_LENGTH
}

// Go's encoding/json escapes HTML-sensitive runes and the two Unicode line
// separators. Mirroring that representation keeps browser-side byte limits
// equal to the persisted canonical document rather than to UTF-16 code units.
export function profileCanonicalJSONByteLength(value: unknown): number {
  let serialized: string | undefined
  try {
    serialized = JSON.stringify(value)
  } catch {
    return Number.POSITIVE_INFINITY
  }
  if (typeof serialized !== "string") return Number.POSITIVE_INFINITY
  const goCompatible = serialized.replace(
    /[<>&\u2028\u2029]/g,
    (character) =>
      `\\u${character.codePointAt(0)!.toString(16).padStart(4, "0")}`,
  )
  return new TextEncoder().encode(goCompatible).byteLength
}

function optionalValidationInteger(
  value: unknown,
  path: string,
): number | undefined {
  if (value === undefined) return undefined
  return integerSchema({
    minimum: 0,
    maximum: PROFILE_ATTRIBUTE_LIMITS.VALIDATION_LENGTH,
    label: "profile validation length",
  }).parse(value, path)
}

function optionalValidationNumber(
  value: unknown,
  path: string,
): number | undefined {
  if (value === undefined) return undefined
  if (!isProfileAttributeNumber(value)) {
    throw new TypeError(`${path}: expected an interoperable profile number`)
  }
  return value
}

function optionalValidationPattern(
  value: unknown,
  path: string,
): string | undefined {
  if (value === undefined) return undefined
  return stringSchema({
    minimumLength: 1,
    maximumLength: PROFILE_ATTRIBUTE_LIMITS.PATTERN_LENGTH,
    label: "profile validation pattern",
  }).parse(value, path)
}

function profileValueMatchesType(
  value: ProfileAttributeValue,
  dataType: ProfileAttributeDataType,
): boolean {
  if (dataType === "boolean") return typeof value === "boolean"
  if (dataType === "number") return isProfileAttributeNumber(value)
  if (typeof value !== "string") return false
  return dataType !== "date" || isCalendarDate(value)
}

function isCalendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return !Number.isNaN(parsed.valueOf()) &&
    parsed.toISOString().slice(0, 10) === value
}
