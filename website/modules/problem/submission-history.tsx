"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Eye,
  FileClock,
  Loader2,
  LogIn,
  Radio,
  RefreshCw,
  WifiOff,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import type { ProblemSubmissionsStatus } from "@/hooks/use-problem-submissions";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeTopicState,
} from "@/lib/realtime/websocket-client";
import {
  formatDuration,
  formatMemory,
  formatTime,
} from "@/lib/format";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import {
  STATUS_CONFIG,
  SubmissionDetail,
} from "@/modules/problem/submission-detail";
import { submissionService } from "@/services/submission.service";
import type { Submission, SubmissionSummary } from "@/types/submission";

interface SubmissionHistoryProps {
  submissions: SubmissionSummary[];
  status: ProblemSubmissionsStatus;
  isAuthenticated: boolean;
  isSessionLoading: boolean;
  isRefreshing: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  topicState: RealtimeTopicState;
  onRefresh: () => Promise<void>;
  onLoadMore: () => Promise<void>;
}

function RealtimeStatus({
  topicState,
}: {
  topicState: RealtimeTopicState;
}) {
  if (topicState === REALTIME_TOPIC_STATE.SUBSCRIBED) {
    return (
      <span className="inline-flex min-h-8 items-center gap-1.5 rounded-full border border-status-success/25 bg-status-success/10 px-2.5 text-xs font-medium text-status-success">
        <Radio className="size-3.5" aria-hidden="true" />
        {TEXT.PROBLEM.HISTORY_LIVE}
      </span>
    );
  }

  if (topicState === REALTIME_TOPIC_STATE.PENDING) {
    return (
      <span className="inline-flex min-h-8 items-center gap-1.5 rounded-full border border-status-info/25 bg-status-info/10 px-2.5 text-xs font-medium text-status-info">
        <Loader2
          className="size-3.5 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
        {TEXT.PROBLEM.HISTORY_CONNECTING}
      </span>
    );
  }

  return (
    <span className="inline-flex min-h-8 items-center gap-1.5 rounded-full border border-status-warning/25 bg-status-warning/10 px-2.5 text-xs font-medium text-status-warning">
      <WifiOff className="size-3.5" aria-hidden="true" />
      {TEXT.PROBLEM.HISTORY_FALLBACK}
    </span>
  );
}

export function SubmissionHistory({
  submissions,
  status,
  isAuthenticated,
  isSessionLoading,
  isRefreshing,
  isLoadingMore,
  hasMore,
  topicState,
  onRefresh,
  onLoadMore,
}: SubmissionHistoryProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selectedStatus = submissions.find(
    (submission) => submission.id === selectedId,
  )?.status;
  const detailResource = useRetryableResource<Submission | null>({
    resetKey: selectedId
      ? JSON.stringify([selectedId, selectedStatus])
      : null,
    enabled: selectedId !== null,
    initialData: null,
    load: async (signal) => {
      if (!selectedId) throw new TypeError("submission detail identity missing");
      const submission = await submissionService.getById(selectedId, signal);
      if (submission.id !== selectedId) {
        throw new TypeError("submission detail identity mismatch");
      }
      return submission;
    },
  });
  const isLoadingDetail = detailResource.status === "loading";
  const detailError = detailResource.status === "error";
  const detail = detailResource.data;
  const latestSubmission = submissions[0];

  function openDetail(submissionId: string) {
    setSelectedId(submissionId);
  }

  if (isSessionLoading) {
    return (
      <DataLoadingFeedback
        label={TEXT.PROBLEM.SESSION_CHECKING}
        className="min-h-48"
      />
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="flex min-h-64 flex-col items-center justify-center rounded-2xl border border-border bg-surface-sunken/40 px-5 text-center">
        <span className="flex size-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <LogIn className="size-5" aria-hidden="true" />
        </span>
        <h2 className="mt-4 text-base font-semibold text-foreground">
          {TEXT.PROBLEM.HISTORY_AUTH_REQUIRED_TITLE}
        </h2>
        <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
          {TEXT.PROBLEM.HISTORY_AUTH_REQUIRED_DESCRIPTION}
        </p>
        <Link
          href={APP_ROUTES.LOGIN}
          className="mt-5 inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground outline-none transition-colors hover:bg-primary/90 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {TEXT.PROBLEM.HISTORY_AUTH_REQUIRED_ACTION}
        </Link>
      </div>
    );
  }

  if (status === "loading" && submissions.length === 0) {
    return (
      <DataLoadingFeedback
        label={TEXT.PROBLEM.LOADING_SUBMISSIONS}
        className="min-h-48"
      />
    );
  }

  if (status === "error" && submissions.length === 0) {
    return (
      <DataLoadFeedback
        title={TEXT.PROBLEM.HISTORY_LOAD_ERROR_TITLE}
        description={TEXT.PROBLEM.HISTORY_LOAD_ERROR_DESCRIPTION}
        retryLabel={TEXT.PROBLEM.HISTORY_REFRESH}
        onRetry={() => void onRefresh()}
      />
    );
  }

  return (
    <>
      <div className="space-y-4">
        <div
          className="flex flex-wrap items-center justify-between gap-3"
          aria-label={TEXT.PROBLEM.HISTORY_STATUS_LABEL}
        >
          <RealtimeStatus topicState={topicState} />
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="cursor-pointer"
            onClick={() => void onRefresh()}
            disabled={isRefreshing}
          >
            <RefreshCw
              className={
                isRefreshing
                  ? "animate-spin motion-reduce:animate-none"
                  : undefined
              }
              aria-hidden="true"
            />
            {isRefreshing
              ? TEXT.PROBLEM.HISTORY_REFRESHING
              : TEXT.PROBLEM.HISTORY_REFRESH}
          </Button>
        </div>

        {status === "error" ? (
          <DataLoadFeedback
            compact
            title={TEXT.PROBLEM.HISTORY_LOAD_ERROR_TITLE}
            description={TEXT.PROBLEM.HISTORY_LOAD_ERROR_DESCRIPTION}
            retryLabel={TEXT.PROBLEM.HISTORY_REFRESH}
            onRetry={() => void onRefresh()}
          />
        ) : null}

        {submissions.length === 0 ? (
          <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-surface-sunken/30 px-5 text-center">
            <span className="flex size-11 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <FileClock className="size-5" aria-hidden="true" />
            </span>
            <h2 className="mt-4 text-sm font-semibold text-foreground">
              {TEXT.PROBLEM.NO_SUBMISSIONS}
            </h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
              {TEXT.PROBLEM.NO_SUBMISSIONS_DESCRIPTION}
            </p>
          </div>
        ) : (
          <>
            <div className="space-y-3 md:hidden">
              {submissions.map((submission) => {
                const config = STATUS_CONFIG[submission.status];
                return (
                  <article
                    key={submission.id}
                    className="rounded-2xl border border-border bg-card p-4 shadow-sm"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <Badge
                        variant="outline"
                        className={`text-xs font-bold ${config.className}`}
                      >
                        {config.label}
                      </Badge>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        onClick={() => openDetail(submission.id)}
                        aria-label={TEXT.PROBLEM.HISTORY_OPEN_SUBMISSION(
                          submission.id,
                        )}
                      >
                        <Eye aria-hidden="true" />
                      </Button>
                    </div>
                    <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-3 text-xs">
                      <div>
                        <dt className="text-muted-foreground">
                          {TEXT.PROBLEM.COL_LANGUAGE}
                        </dt>
                        <dd className="mt-1 font-medium text-foreground">
                          {submission.language}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">
                          {TEXT.PROBLEM.COL_TESTS}
                        </dt>
                        <dd className="mt-1 font-mono font-medium text-foreground">
                          {submission.total_count > 0
                            ? `${submission.passed_count}/${submission.total_count}`
                            : "-"}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">
                          {TEXT.PROBLEM.COL_TIME}
                        </dt>
                        <dd className="mt-1 font-mono text-foreground">
                          {submission.time_ms != null
                            ? formatDuration(submission.time_ms)
                            : "-"}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">
                          {TEXT.PROBLEM.COL_MEMORY}
                        </dt>
                        <dd className="mt-1 font-mono text-foreground">
                          {submission.memory_kb != null
                            ? formatMemory(submission.memory_kb)
                            : "-"}
                        </dd>
                      </div>
                    </dl>
                    <p className="mt-3 border-t border-border pt-3 text-xs text-muted-foreground">
                      {formatTime(submission.created_at)}
                    </p>
                  </article>
                );
              })}
            </div>

            <div className="hidden overflow-hidden rounded-xl border border-border md:block">
              <Table>
                <caption className="sr-only">
                  {TEXT.PROBLEM.HISTORY_TABLE_LABEL}
                </caption>
                <TableHeader>
                  <TableRow className="bg-surface-sunken/60 hover:bg-surface-sunken/60">
                    <TableHead className="w-44">
                      {TEXT.PROBLEM.COL_RESULT}
                    </TableHead>
                    <TableHead>{TEXT.PROBLEM.COL_LANGUAGE}</TableHead>
                    <TableHead className="text-center">
                      {TEXT.PROBLEM.COL_TESTS}
                    </TableHead>
                    <TableHead className="text-right">
                      {TEXT.PROBLEM.COL_TIME}
                    </TableHead>
                    <TableHead className="text-right">
                      {TEXT.PROBLEM.COL_MEMORY}
                    </TableHead>
                    <TableHead className="text-right">
                      {TEXT.PROBLEM.COL_SUBMITTED_AT}
                    </TableHead>
                    <TableHead className="w-14">
                      <span className="sr-only">
                        {TEXT.PROBLEM.HISTORY_VIEW_DETAIL}
                      </span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {submissions.map((submission) => {
                    const config = STATUS_CONFIG[submission.status];
                    return (
                      <TableRow key={submission.id}>
                        <TableCell>
                          <Badge
                            variant="outline"
                            className={`text-xs font-bold whitespace-nowrap ${config.className}`}
                          >
                            {config.label}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {submission.language}
                        </TableCell>
                        <TableCell className="text-center font-mono text-xs">
                          {submission.total_count > 0
                            ? `${submission.passed_count}/${submission.total_count}`
                            : "-"}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs">
                          {submission.time_ms != null
                            ? formatDuration(submission.time_ms)
                            : "-"}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs">
                          {submission.memory_kb != null
                            ? formatMemory(submission.memory_kb)
                            : "-"}
                        </TableCell>
                        <TableCell className="text-right text-xs text-muted-foreground">
                          {formatTime(submission.created_at)}
                        </TableCell>
                        <TableCell>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            onClick={() => openDetail(submission.id)}
                            aria-label={TEXT.PROBLEM.HISTORY_OPEN_SUBMISSION(
                              submission.id,
                            )}
                          >
                            <Eye aria-hidden="true" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </>
        )}

        {hasMore ? (
          <div className="flex justify-center pt-1">
            <Button
              type="button"
              variant="outline"
              className="min-w-48 cursor-pointer"
              onClick={() => void onLoadMore()}
              disabled={isLoadingMore || isRefreshing}
            >
              {isLoadingMore ? (
                <Loader2
                  className="animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : null}
              {isLoadingMore
                ? TEXT.PROBLEM.HISTORY_LOADING_MORE
                : TEXT.PROBLEM.HISTORY_LOAD_MORE}
            </Button>
          </div>
        ) : submissions.length > 0 ? (
          <p className="text-center text-xs text-muted-foreground">
            {TEXT.PROBLEM.HISTORY_END}
          </p>
        ) : null}
      </div>

      <span className="sr-only" role="status" aria-live="polite">
        {latestSubmission
          ? TEXT.PROBLEM.HISTORY_LATEST_RESULT(
              STATUS_CONFIG[latestSubmission.status].label,
            )
          : ""}
      </span>

      <Sheet
        open={selectedId !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedId(null);
        }}
      >
        <SheetContent
          side="right"
          className="w-full overflow-hidden p-0 sm:max-w-2xl"
        >
          <SheetHeader className="border-b border-border px-4 pb-3 pt-4 sm:px-6">
            <SheetTitle>{TEXT.PROBLEM.DETAIL_TITLE}</SheetTitle>
          </SheetHeader>
          {isLoadingDetail ? (
            <DataLoadingFeedback
              label={TEXT.PROBLEM.DETAIL_LOADING}
              className="m-4 min-h-48 sm:m-6"
            />
          ) : null}
          {detailError && !isLoadingDetail ? (
            <DataLoadFeedback
              title={TEXT.PROBLEM.DETAIL_LOAD_ERROR}
              retryLabel={TEXT.PROBLEM.DETAIL_RETRY}
              onRetry={detailResource.retry}
              className="m-4 sm:m-6"
            />
          ) : null}
          {detail && !isLoadingDetail && !detailError ? (
            <SubmissionDetail submission={detail} />
          ) : null}
        </SheetContent>
      </Sheet>
    </>
  );
}
