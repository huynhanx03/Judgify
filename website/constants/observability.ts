export const DEBUG_WINDOW_DURATION_OPTIONS = [
  60,
  300,
  600,
  900,
] as const;

export type DebugWindowDurationSeconds =
  (typeof DEBUG_WINDOW_DURATION_OPTIONS)[number];

export const DEBUG_WINDOW_REASON_LIMITS = {
  MINIMUM_CHARACTERS: 3,
  MAXIMUM_CHARACTERS: 1024,
  MAXIMUM_UTF8_BYTES: 1024,
} as const;
