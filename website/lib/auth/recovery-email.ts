import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";

const RECOVERY_EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/u;

export function normalizeRecoveryEmail(value: string): string {
  return value.trim();
}

export function isRecoveryEmailValid(value: string): boolean {
  const normalized = normalizeRecoveryEmail(value);
  return (
    normalized.length <= IDENTITY_INPUT_LIMITS.RECOVERY_EMAIL_MAX_LENGTH &&
    RECOVERY_EMAIL_PATTERN.test(normalized)
  );
}
