"use client";

import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { TEXT } from "@/constants/text";
import {
  formatDateTime,
  formatDuration,
  formatMemory,
} from "@/lib/format";
import { CopyButton } from "@/modules/problem/copy-button";
import type { Submission, SubmissionStatus } from "@/types/submission";

export const STATUS_CONFIG = {
  accepted: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.ACCEPTED,
    className:
      "border-status-success/25 bg-status-success/10 text-status-success",
  },
  wrong_answer: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.WRONG_ANSWER,
    className: "border-destructive/25 bg-destructive/10 text-destructive",
  },
  time_limit_exceeded: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.TIME_LIMIT,
    className:
      "border-status-warning/25 bg-status-warning/10 text-status-warning",
  },
  memory_limit_exceeded: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.MEMORY_LIMIT,
    className:
      "border-status-warning/25 bg-status-warning/10 text-status-warning",
  },
  process_limit_exceeded: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.PROCESS_LIMIT,
    className:
      "border-status-warning/25 bg-status-warning/10 text-status-warning",
  },
  output_limit_exceeded: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.OUTPUT_LIMIT,
    className:
      "border-status-warning/25 bg-status-warning/10 text-status-warning",
  },
  runtime_error: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.RUNTIME_ERROR,
    className: "border-destructive/25 bg-destructive/10 text-destructive",
  },
  compile_error: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.COMPILE_ERROR,
    className: "border-destructive/25 bg-destructive/10 text-destructive",
  },
  pending: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.PENDING,
    className:
      "border-status-info/25 bg-status-info/10 text-status-info motion-safe:animate-pulse",
  },
  judging: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.JUDGING,
    className:
      "border-status-info/25 bg-status-info/10 text-status-info motion-safe:animate-pulse",
  },
  internal_error: {
    label: TEXT.PROBLEM.SUBMISSION_STATUS.INTERNAL_ERROR,
    className: "border-destructive/25 bg-destructive/10 text-destructive",
  },
} as const satisfies Record<
  SubmissionStatus,
  { label: string; className: string }
>;

interface SubmissionDetailProps {
  submission: Submission;
}

export function SubmissionDetail({ submission }: SubmissionDetailProps) {
  const config = STATUS_CONFIG[submission.status];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4 pb-6 pt-4 sm:px-6">
      <div className="flex flex-wrap items-center gap-3">
        <Badge
          variant="outline"
          className={`px-3 py-1 text-sm font-bold ${config.className}`}
        >
          {config.label}
        </Badge>
        <span className="text-sm font-medium text-foreground">
          {submission.language}
        </span>
        <span className="text-xs text-muted-foreground sm:ml-auto">
          {formatDateTime(submission.created_at)}
        </span>
      </div>

      <dl className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-xl border border-border bg-surface-sunken/50 p-3">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROBLEM.DETAIL_STATS_TESTS}
          </dt>
          <dd className="mt-1 font-mono text-base font-bold text-foreground">
            {submission.total_count > 0
              ? `${submission.passed_count}/${submission.total_count}`
              : "-"}
          </dd>
        </div>
        <div className="rounded-xl border border-border bg-surface-sunken/50 p-3">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROBLEM.DETAIL_STATS_TIME}
          </dt>
          <dd className="mt-1 font-mono text-base font-bold text-foreground">
            {submission.time_ms != null
              ? formatDuration(submission.time_ms)
              : "-"}
          </dd>
        </div>
        <div className="rounded-xl border border-border bg-surface-sunken/50 p-3">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROBLEM.DETAIL_STATS_MEMORY}
          </dt>
          <dd className="mt-1 font-mono text-base font-bold text-foreground">
            {submission.memory_kb != null
              ? formatMemory(submission.memory_kb)
              : "-"}
          </dd>
        </div>
      </dl>

      <div className="rounded-xl border border-border bg-muted/20 px-3 py-2">
        <p className="text-xs font-medium text-muted-foreground">
          {TEXT.PROBLEM.DETAIL_SUBMISSION_ID}
        </p>
        <p className="mt-1 break-all font-mono text-xs text-foreground">
          {submission.id}
        </p>
      </div>

      {submission.error_message ? (
        <>
          <Separator />
          <section aria-labelledby="submission-error-heading">
            <h3
              id="submission-error-heading"
              className="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
            >
              {TEXT.PROBLEM.DETAIL_ERROR}
            </h3>
            <pre className="mt-2 overflow-x-auto rounded-xl border border-destructive/25 bg-destructive/5 p-3 font-mono text-xs leading-5 whitespace-pre-wrap break-words text-destructive">
              {submission.error_message}
            </pre>
          </section>
        </>
      ) : null}

      <Separator />

      <section aria-labelledby="submission-source-heading">
        <div className="mb-2 flex items-center justify-between gap-3">
          <h3
            id="submission-source-heading"
            className="text-xs font-semibold tracking-wider text-muted-foreground uppercase"
          >
            {TEXT.PROBLEM.DETAIL_SOURCE_CODE}
          </h3>
          <CopyButton
            value={submission.source_code}
            idleLabel={TEXT.PROBLEM.DETAIL_COPY}
            successLabel={TEXT.PROBLEM.DETAIL_COPIED}
            errorLabel={TEXT.PROBLEM.DETAIL_COPY_ERROR}
            showLabel
          />
        </div>
        <pre className="max-h-[50dvh] overflow-auto rounded-xl border border-border bg-code-background p-4 font-mono text-xs leading-5 whitespace-pre text-media-foreground">
          {submission.source_code}
        </pre>
      </section>
    </div>
  );
}
