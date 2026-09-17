import { GENDER_OPTIONS } from "@/constants/gender"
import { TEXT } from "@/constants/text"
import { NUMBER_FORMAT_SEPARATORS } from "@/lib/format"
import type { ProfileAttributeValue } from "@/types/user"

export const SYSTEM_PROFILE_ATTRIBUTE_KEYS = {
  GENDER: "gender",
} as const

export const PROFILE_ATTRIBUTE_KEY_PATTERN = /^[a-z][a-z0-9_.-]{1,49}$/

// Browser-side mirrors of the identity profile contract. These bounds keep
// parsers and editors fail-closed before a request reaches the authoritative
// Go codec; the backend remains the source of truth.
export const PROFILE_ATTRIBUTE_LIMITS = {
  CANONICAL_BYTES: 8 * 1_024,
  TEXT_INPUT_CODE_UNITS: 8_190,
  SCHEMA_FIELDS: 128,
  PROFILE_ATTRIBUTES: 256,
  ENUM_VALUES: 256,
  VALIDATION_LENGTH: 8 * 1_024,
  PATTERN_LENGTH: 1_024,
  CLIENT_PATTERN_LENGTH: 256,
  CLIENT_PATTERN_QUANTIFIERS: 8,
  KEY_MIN_LENGTH: 2,
  KEY_MAX_LENGTH: 50,
  LABEL_MIN_LENGTH: 2,
  LABEL_MAX_LENGTH: 128,
  DESCRIPTION_MAX_LENGTH: 1_024,
  DISPLAY_ORDER_MIN: 0,
  DISPLAY_ORDER_MAX: 10_000,
} as const

const PROFILE_NUMBER_MINIMUM_NON_ZERO_MAGNITUDE = 1e-6
const PROFILE_NUMBER_MAXIMUM_MAGNITUDE_EXCLUSIVE = 1e21
const PROFILE_NUMBER_TEXT_PATTERN = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$/

/**
 * The exact exponent-free JSON number domain shared by Go's profile codec and
 * ECMAScript. Keeping this at the contract boundary prevents JSON.parse from
 * silently rounding a persisted decimal before the UI can validate it.
 */
export function isProfileAttributeNumber(value: unknown): value is number {
  if (
    typeof value !== "number" ||
    !Number.isFinite(value) ||
    Object.is(value, -0)
  ) {
    return false
  }
  const magnitude = Math.abs(value)
  const serialized = JSON.stringify(value)
  return (
    (value === 0 ||
      (magnitude >= PROFILE_NUMBER_MINIMUM_NON_ZERO_MAGNITUDE &&
        magnitude < PROFILE_NUMBER_MAXIMUM_MAGNITUDE_EXCLUSIVE)) &&
    (!Number.isInteger(value) || Number.isSafeInteger(value)) &&
    typeof serialized === "string" &&
    !/[eE]/.test(serialized)
  )
}

/**
 * Parses a profile-number editor value without allowing ECMAScript to round
 * an authored decimal silently. The lexical normalization deliberately
 * mirrors the authoritative Go codec before the numeric round-trip check.
 */
export function parseProfileAttributeNumberText(value: string): number | null {
  if (
    value.length === 0 ||
    value.length > PROFILE_ATTRIBUTE_LIMITS.CANONICAL_BYTES ||
    !PROFILE_NUMBER_TEXT_PATTERN.test(value)
  ) {
    return null
  }

  const negative = value.startsWith("-")
  const unsigned = negative ? value.slice(1) : value
  const separator = unsigned.indexOf(".")
  const integer = separator < 0 ? unsigned : unsigned.slice(0, separator)
  const fraction = separator < 0
    ? ""
    : unsigned.slice(separator + 1).replace(/0+$/, "")
  const magnitude = fraction.length > 0 ? `${integer}.${fraction}` : integer
  const canonical = negative && magnitude !== "0" ? `-${magnitude}` : magnitude
  const parsed = Number(canonical)

  return isProfileAttributeNumber(parsed) && JSON.stringify(parsed) === canonical
    ? parsed
    : null
}

/**
 * Adds locale grouping without asking Intl.NumberFormat to round the scalar.
 * The accepted profile-number domain always has an exponent-free shortest
 * representation, so grouping that token preserves every persisted digit.
 */
export function formatProfileAttributeNumber(value: number): string {
  if (!isProfileAttributeNumber(value)) return TEXT.COMMON.UNKNOWN
  const canonical = String(value)
  const negative = canonical.startsWith("-")
  const unsigned = negative ? canonical.slice(1) : canonical
  const [integer, fraction] = unsigned.split(".", 2)
  const groupedInteger = integer.replace(
    /\B(?=(\d{3})+(?!\d))/g,
    NUMBER_FORMAT_SEPARATORS.group,
  )
  return `${negative ? "-" : ""}${groupedInteger}${
    fraction ? NUMBER_FORMAT_SEPARATORS.decimal + fraction : ""
  }`
}

export function profileAttributeOptionLabel(
  key: string,
  value: ProfileAttributeValue,
): string {
  if (key === SYSTEM_PROFILE_ATTRIBUTE_KEYS.GENDER && typeof value === "number") {
    return (
      GENDER_OPTIONS.find((option) => option.value === value)?.label ??
      TEXT.PROFILE.ATTRIBUTES.OPTION_LABEL(String(value))
    )
  }
  return TEXT.PROFILE.ATTRIBUTES.OPTION_LABEL(String(value))
}
