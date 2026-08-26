import type {
  ProfileAttribute,
  ProfileAttributeMutation,
  ProfileAttributeValue,
} from "@/types/user"
import {
  PROFILE_ATTRIBUTE_LIMITS,
  isProfileAttributeNumber,
  parseProfileAttributeNumberText,
} from "@/constants/profile-attributes"
import { profileCanonicalJSONByteLength } from "@/lib/profile/profile-validation-schema"

export type ProfileAttributeDraftValue = string | boolean | undefined
export type ProfileAttributeDrafts = Record<string, ProfileAttributeDraftValue>
export type ProfileAttributeFieldError =
  | "required"
  | "invalid"
  | "minimum"
  | "maximum"
  | "min_length"
  | "max_length"
  | "enum"
  | "pattern"
  | "payload_size"
export type ProfileAttributeFieldErrors = Record<
  string,
  ProfileAttributeFieldError
>

export type ProfileAttributeMutationPlan =
  | { ok: true; mutations: ProfileAttributeMutation[] }
  | { ok: false; errors: ProfileAttributeFieldErrors }

export function createProfileAttributeDrafts(
  attributes: ProfileAttribute[],
): ProfileAttributeDrafts {
  return Object.fromEntries(
    attributes.map((attribute) => [
      attribute.definition_id,
      draftValue(attribute.has_value ? attribute.value : undefined),
    ]),
  )
}

export function planProfileAttributeMutations(
  attributes: ProfileAttribute[],
  drafts: ProfileAttributeDrafts,
  options: { requireOnboardingValues?: boolean } = {},
): ProfileAttributeMutationPlan {
  const errors: ProfileAttributeFieldErrors = {}
  const mutations: ProfileAttributeMutation[] = []
  const ordered = [...attributes].sort((left, right) =>
    left.definition_id < right.definition_id ? -1 : left.definition_id === right.definition_id ? 0 : 1,
  )

  for (const attribute of ordered) {
    if (!attribute.user_editable) continue
    const parsed = evaluateProfileAttributeDraft(
      attribute,
      drafts[attribute.definition_id],
      options,
    )
    if (!parsed.ok) {
      errors[attribute.definition_id] = parsed.error
      continue
    }
    const nextValue = parsed.value
    if (nextValue === undefined) {
      if (attribute.has_value) {
        mutations.push(mutationBase(attribute, { unset: true }))
      }
      continue
    }
    if (!attribute.has_value || !Object.is(attribute.value, nextValue)) {
      mutations.push(mutationBase(attribute, { value: nextValue }))
    }
  }

  return Object.keys(errors).length > 0
    ? { ok: false, errors }
    : { ok: true, mutations }
}

export function validateProfileAttributeDraft(
  attribute: ProfileAttribute,
  draft: ProfileAttributeDraftValue,
  options: { requireOnboardingValues?: boolean } = {},
): ProfileAttributeFieldError | null {
  const result = evaluateProfileAttributeDraft(attribute, draft, options)
  return result.ok ? null : result.error
}

export function profileAttributeEnumValues(
  attribute: ProfileAttribute,
): ProfileAttributeValue[] {
  const candidate = attribute.validation.enum
  if (!Array.isArray(candidate)) return []
  const values: ProfileAttributeValue[] = []
  for (const value of candidate) {
    if (!valueMatchesType(value, attribute.data_type)) return []
    if (!values.some((existing) => Object.is(existing, value))) {
      values.push(value)
    }
  }
  return values
}

function mutationBase<T extends { value: ProfileAttributeValue } | { unset: true }>(
  attribute: ProfileAttribute,
  change: T,
): ProfileAttributeMutation {
  return {
    definition_id: attribute.definition_id,
    expected_definition_revision_id: attribute.definition_revision_id,
    expected_value_version: attribute.value_version,
    ...change,
  } as ProfileAttributeMutation
}

function draftValue(
  value: ProfileAttributeValue | undefined,
): ProfileAttributeDraftValue {
  if (typeof value === "boolean" || value === undefined) return value
  return String(value)
}

function parseDraft(
  attribute: ProfileAttribute,
  value: ProfileAttributeDraftValue,
):
  | { ok: true; value: ProfileAttributeValue | undefined }
  | { ok: false; error: ProfileAttributeFieldError } {
  if (value === undefined || value === "") {
    if (attribute.data_type === "string" && value === "") {
      return { ok: true, value }
    }
    return { ok: true, value: undefined }
  }
  if (attribute.data_type === "boolean") {
    return typeof value === "boolean"
      ? { ok: true, value }
      : { ok: false, error: "invalid" }
  }
  if (typeof value !== "string") return { ok: false, error: "invalid" }
  if (attribute.data_type === "number") {
    const number = parseProfileAttributeNumberText(value)
    if (number === null) {
      return { ok: false, error: "invalid" }
    }
    return { ok: true, value: number }
  }
  return { ok: true, value }
}

function evaluateProfileAttributeDraft(
  attribute: ProfileAttribute,
  draft: ProfileAttributeDraftValue,
  options: { requireOnboardingValues?: boolean },
):
  | { ok: true; value: ProfileAttributeValue | undefined }
  | { ok: false; error: ProfileAttributeFieldError } {
  const parsed = parseDraft(attribute, draft)
  if (!parsed.ok) return parsed
  if (parsed.value === undefined) {
    if (
      options.requireOnboardingValues === true &&
      attribute.required_on_onboarding
    ) {
      return { ok: false, error: "required" }
    }
    return parsed
  }
  const error = validateValue(attribute, parsed.value)
  return error ? { ok: false, error } : parsed
}

function validateValue(
  attribute: ProfileAttribute,
  value: ProfileAttributeValue,
): ProfileAttributeFieldError | null {
  if (!valueMatchesType(value, attribute.data_type)) return "invalid"
  if (
    profileCanonicalJSONByteLength(value) >
    PROFILE_ATTRIBUTE_LIMITS.CANONICAL_BYTES
  ) {
    return "payload_size"
  }
  const allowed = profileAttributeEnumValues(attribute)
  if (
    allowed.length > 0 &&
    !allowed.some((candidate) => Object.is(candidate, value))
  ) {
    return "enum"
  }
  if (typeof value === "string") {
    if (attribute.data_type === "date" && !isCalendarDate(value)) {
      return "invalid"
    }
    const length = Array.from(value).length
    const minimumLength = safeNumber(attribute.validation.min_length)
    const maximumLength = safeNumber(attribute.validation.max_length)
    if (minimumLength !== null && length < minimumLength) return "min_length"
    if (maximumLength !== null && length > maximumLength) return "max_length"
    const pattern = clientSafePattern(attribute.validation.pattern)
    if (pattern && !pattern.test(value)) return "pattern"
  }
  if (typeof value === "number") {
    const minimum = safeNumber(attribute.validation.minimum)
    const maximum = safeNumber(attribute.validation.maximum)
    if (minimum !== null && value < minimum) return "minimum"
    if (maximum !== null && value > maximum) return "maximum"
  }
  return null
}

// Attribute patterns are authored for Go's linear-time regexp engine. Only a
// deliberately small, non-grouping subset is evaluated in the browser; every
// value remains authoritatively validated by the server.
function clientSafePattern(value: unknown): RegExp | null {
  if (
    typeof value !== "string" ||
    value.length === 0 ||
    value.length > PROFILE_ATTRIBUTE_LIMITS.CLIENT_PATTERN_LENGTH ||
    /[()|]/.test(value) ||
    (value.match(/[+*?{]/g)?.length ?? 0) >
      PROFILE_ATTRIBUTE_LIMITS.CLIENT_PATTERN_QUANTIFIERS
  ) {
    return null
  }
  try {
    return new RegExp(value, "u")
  } catch {
    return null
  }
}

function valueMatchesType(
  value: unknown,
  dataType: ProfileAttribute["data_type"],
): value is ProfileAttributeValue {
  if (dataType === "boolean") return typeof value === "boolean"
  if (dataType === "number") return isProfileAttributeNumber(value)
  return typeof value === "string"
}

function safeNumber(value: unknown): number | null {
  return isProfileAttributeNumber(value) ? value : null
}

function isCalendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return !Number.isNaN(parsed.valueOf()) && parsed.toISOString().slice(0, 10) === value
}
