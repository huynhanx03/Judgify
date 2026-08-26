import {
  DEBUG_WINDOW_DURATION_OPTIONS,
  DEBUG_WINDOW_REASON_LIMITS,
  type DebugWindowDurationSeconds,
} from "@/constants/observability";
import { commandPurpose } from "@/lib/api/command-purpose";
import type { ActivateDebugWindowCommand } from "@/types/observability";

const textEncoder = new TextEncoder();

export function isDebugWindowDuration(
  value: number,
): value is DebugWindowDurationSeconds {
  return DEBUG_WINDOW_DURATION_OPTIONS.some(
    (duration) => duration === value,
  );
}

export function normalizeDebugWindowReason(value: string): string {
  const reason = value.trim();
  if (
    reason.length < DEBUG_WINDOW_REASON_LIMITS.MINIMUM_CHARACTERS ||
    reason.length > DEBUG_WINDOW_REASON_LIMITS.MAXIMUM_CHARACTERS ||
    textEncoder.encode(reason).byteLength >
      DEBUG_WINDOW_REASON_LIMITS.MAXIMUM_UTF8_BYTES ||
    /[\p{C}]/u.test(reason)
  ) {
    throw new TypeError("invalid debug-window reason");
  }
  return reason;
}

export function isDebugWindowReasonValid(value: string): boolean {
  try {
    normalizeDebugWindowReason(value);
    return true;
  } catch {
    return false;
  }
}

export function normalizeActivateDebugWindow(
  command: ActivateDebugWindowCommand,
): ActivateDebugWindowCommand {
  if (!isDebugWindowDuration(command.duration_seconds)) {
    throw new TypeError("invalid debug-window duration");
  }
  return {
    duration_seconds: command.duration_seconds,
    reason: normalizeDebugWindowReason(command.reason),
  };
}

export function debugWindowActivationPurpose(
  command: ActivateDebugWindowCommand,
): Promise<string> {
  return commandPurpose("debug-window-activate", command);
}
