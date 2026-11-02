import { CONTEST_REASON_LIMITS } from "@/constants/contest";

const CONTROL_CHARACTER_PATTERN = /[\u0000-\u001f\u007f]/;

export function contestVersionETag(version: number): string {
  if (!Number.isSafeInteger(version) || version < 1) {
    throw new RangeError("contest version must be a positive integer");
  }
  return `"contest-v${version}"`;
}

export function normalizeContestReason(reason: string): string {
  const normalized = reason.trim();
  if (
    normalized.length < CONTEST_REASON_LIMITS.MINIMUM_CHARACTERS ||
    normalized.length > CONTEST_REASON_LIMITS.MAXIMUM_CHARACTERS ||
    new TextEncoder().encode(normalized).byteLength >
      CONTEST_REASON_LIMITS.MAXIMUM_UTF8_BYTES ||
    CONTROL_CHARACTER_PATTERN.test(normalized)
  ) {
    throw new TypeError("contest reason is outside the boundary");
  }
  return normalized;
}

export function isContestReasonValid(reason: string): boolean {
  try {
    normalizeContestReason(reason);
    return true;
  } catch {
    return false;
  }
}

export const normalizeContestPublishReason = normalizeContestReason;
export const isContestPublishReasonValid = isContestReasonValid;

export function normalizeRatingReratingReason(reason: string): string {
  const normalized = normalizeContestReason(reason);
  if (normalized.length < 3) {
    throw new TypeError("rating rerating reason is too short");
  }
  return normalized;
}

export function isRatingReratingReasonValid(reason: string): boolean {
  try {
    normalizeRatingReratingReason(reason);
    return true;
  } catch {
    return false;
  }
}
