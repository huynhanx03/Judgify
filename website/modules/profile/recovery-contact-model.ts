/** Returns whether a server-issued recent-authentication window is still open. */
export function isRecentReauthenticationActive(
  reauthenticatedUntil: string | null,
  now = Date.now(),
): boolean {
  if (!reauthenticatedUntil) return false;
  const deadline = Date.parse(reauthenticatedUntil);
  return Number.isFinite(deadline) && deadline > now;
}

/** Rounds up so a visible countdown never enables an action too early. */
export function secondsUntil(timestamp: string, now = Date.now()): number {
  const deadline = Date.parse(timestamp);
  if (!Number.isFinite(deadline)) return 0;
  return Math.max(0, Math.ceil((deadline - now) / 1_000));
}

/** Compact, locale-neutral clock value; surrounding copy lives in TEXT. */
export function formatCountdown(totalSeconds: number): string {
  const bounded = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(bounded / 60);
  const seconds = bounded % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

export type RecoveryContactErrorKind =
  | "reauthentication_required"
  | "conflict"
  | "rate_limited"
  | "unknown";

/** Uses stable HTTP semantics only; backend prose never controls UI behavior. */
export function classifyRecoveryContactError(
  error: unknown,
): RecoveryContactErrorKind {
  if (!error || typeof error !== "object" || !("status" in error)) {
    return "unknown";
  }
  const status = (error as { status?: unknown }).status;
  if (status === 403) return "reauthentication_required";
  if (status === 409) return "conflict";
  if (status === 429) return "rate_limited";
  return "unknown";
}
