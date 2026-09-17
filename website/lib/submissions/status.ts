import { TERMINAL_SUBMISSION_STATUSES } from "@/constants/submission";
import type {
  SubmissionStatus,
  SubmissionSummary,
} from "@/types/submission";

const terminalStatuses = new Set<SubmissionStatus>(
  TERMINAL_SUBMISSION_STATUSES,
);

export function isTerminalSubmissionStatus(
  status: SubmissionStatus,
): boolean {
  return terminalStatuses.has(status);
}

export function hasNonterminalSubmission(
  submissions: readonly Pick<SubmissionSummary, "status">[],
): boolean {
  return submissions.some(
    (submission) => !isTerminalSubmissionStatus(submission.status),
  );
}
