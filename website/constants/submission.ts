import type { SubmissionStatus } from "@/types/submission";

/** Statuses whose judge result cannot change without a new submission. */
export const TERMINAL_SUBMISSION_STATUSES = [
  "accepted",
  "wrong_answer",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "process_limit_exceeded",
  "output_limit_exceeded",
  "runtime_error",
  "compile_error",
  "internal_error",
] as const satisfies readonly SubmissionStatus[];

export const ACTIVE_SUBMISSION_STATUSES = [
  "pending",
  "judging",
] as const satisfies readonly SubmissionStatus[];

export const SUBMISSION_STATUSES = [
  ...ACTIVE_SUBMISSION_STATUSES,
  ...TERMINAL_SUBMISSION_STATUSES,
] as const satisfies readonly SubmissionStatus[];

export const ADMIN_SUBMISSION_PAGE_SIZE = 25;
export const SUBMISSION_HISTORY_PAGE_SIZE = 20;
export const SUBMISSION_HISTORY_MAXIMUM_PAGE_SIZE = 50;
export const ADMIN_JUDGE_OPERATION_REASON_LIMITS = Object.freeze({
  MINIMUM_CHARACTERS: 3,
  MAXIMUM_CHARACTERS: 500,
});

/** Must stay aligned with api/pkg/judging. Values are encoded UTF-8 bytes. */
export const JUDGE_CONTENT_LIMITS = Object.freeze({
  MAXIMUM_SOURCE_CODE_BYTES: 256 * 1024,
  MAXIMUM_TEST_INPUT_BYTES: 1024 * 1024,
  MAXIMUM_EXPECTED_OUTPUT_BYTES: 1024 * 1024,
});
