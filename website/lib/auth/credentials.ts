import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";

const USERNAME_PATTERN = /^[A-Za-z0-9]+$/;

/** Canonical browser-side projection of the API's `alphanum` username rule. */
export function normalizeUsername(value: string): string {
  return value.trim();
}

export function isUsernameValid(value: string): boolean {
  return (
    value === normalizeUsername(value) &&
    value.length >= IDENTITY_INPUT_LIMITS.USERNAME_MIN_LENGTH &&
    value.length <= IDENTITY_INPUT_LIMITS.USERNAME_MAX_LENGTH &&
    USERNAME_PATTERN.test(value)
  );
}

export function usernameHTMLPattern(): string {
  return USERNAME_PATTERN.source;
}

export function isPasswordValid(value: string): boolean {
  return (
    value.length >= IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH &&
    value.length <= IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH
  );
}
