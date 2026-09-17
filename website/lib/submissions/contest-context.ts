/** Minimal shared shape for REST summaries and realtime submission events. */
export interface ContestScopedSubmission {
  contest_id?: string | null;
}

/** Practice mode includes every attempt; contest mode is strictly isolated. */
export function submissionMatchesContest(
  submission: ContestScopedSubmission,
  contestId: string | null,
): boolean {
  return contestId === null || submission.contest_id === contestId;
}
