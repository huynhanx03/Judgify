"use client";

import { use, useMemo, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowLeft,
  Code2,
  FileText,
  History,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { useAuth } from "@/contexts/auth-context";
import { useProblemSubmissions } from "@/hooks/use-problem-submissions";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { tryEntityID } from "@/lib/api/contracts";
import { readContestContext } from "@/lib/contest/navigation";
import { getErrorMessage, notify } from "@/lib/toast";
import { ProblemDescriptionPanel } from "@/modules/arena/problem-description-panel";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import { FileSubmission } from "@/modules/problem/file-submission";
import { SubmissionHistory } from "@/modules/problem/submission-history";
import { contestService } from "@/services/contest.service";
import { problemService } from "@/services/problem.service";
import { submissionService } from "@/services/submission.service";
import { ApiError } from "@/lib/api/error";
import type {
  Problem,
  ProblemWorkspaceStatement,
} from "@/types/problem";
import type { ContestProblemDetail } from "@/types/contest";
import type {
  JudgeRuntime,
  SampleTestCaseResponse,
  SubmissionAcceptedSnapshot,
  SubmissionStatus,
  SubmissionSummary,
  TestCase,
} from "@/types/submission";

function toDisplayTestCase(testCase: SampleTestCaseResponse): TestCase {
  return {
    input: testCase.input,
    output: testCase.expected_output,
  };
}

function toContestWorkspaceStatement(
  problem: ContestProblemDetail,
): ProblemWorkspaceStatement {
  return {
    title: problem.title,
    description: problem.statement_markdown,
    difficulty: {
      name: problem.difficulty.name,
      level: problem.difficulty.level,
    },
    time_limit_ms: problem.cpu_time_ms,
    memory_limit_kb: problem.memory_limit_kb,
    submission_count: problem.submission_count,
    acceptance_rate: problem.acceptance_rate,
    tags: problem.tags.map((tag) => ({
      id: tag.id,
      name: tag.name,
    })),
  };
}

function intakeStatus(
  snapshot: SubmissionAcceptedSnapshot,
): SubmissionStatus {
  if (snapshot.latest_generation_phase === "queued") return "pending";
  if (snapshot.latest_generation_phase === "terminal") {
    return snapshot.active_verdict ?? "internal_error";
  }
  if (snapshot.latest_generation_phase === "cancelled") {
    return "internal_error";
  }
  return "judging";
}

type ProblemTab = "description" | "history";
type MobileWorkspacePane = "statement" | "workbench";

export default function ProblemDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{
    contest?: string | string[];
    contest_problem?: string | string[];
  }>;
}) {
  const { id } = use(params);
  const { contest, contest_problem: contestProblemParameter } = use(searchParams);
  const problemId = tryEntityID(id);
  const contestId = readContestContext(contest);
  const contestProblemId = readContestContext(contestProblemParameter);
  const contestContextRequested =
    contest !== undefined || contestProblemParameter !== undefined;
  const contestContextValid =
    typeof contest === "string" &&
    typeof contestProblemParameter === "string" &&
    contestId !== null &&
    contestProblemId !== null;
  const contestContextInvalid =
    contestContextRequested && !contestContextValid;
  const activeContestId = contestContextValid ? contestId : null;
  const backHref = contestId
    ? APP_ROUTES.CONTEST_DETAIL(contestId)
    : APP_ROUTES.ARENA;
  const backLabel = contestId
    ? TEXT.PROBLEM.BACK_TO_CONTEST
    : TEXT.PROBLEM.BACK_TO_ARENA;
  const { user, isLoading: isLoadingAuth } = useAuth();
  const {
    submissions,
    status: submissionStatus,
    isRefreshing: isRefreshingSubmissions,
    isLoadingMore: isLoadingMoreSubmissions,
    hasMore: hasMoreSubmissions,
    topicState,
    recordSubmission,
    refresh: refreshSubmissions,
    loadMore: loadMoreSubmissions,
  } = useProblemSubmissions(
    contestContextInvalid ? null : problemId,
    user?.id ?? null,
    activeContestId,
  );

  const problemResource = useRetryableResource<
    Problem | ContestProblemDetail | null
  >({
    resetKey: contestContextValid
      ? `${problemId ?? "invalid"}:${contestId}:${contestProblemId}`
      : problemId,
    enabled: problemId !== null,
    initialData: null,
    load: async (signal) => {
      if (!problemId) return null;
      if (contestContextValid && contestId && contestProblemId) {
        const frozen = await contestService.getProblem(
          contestId,
          contestProblemId,
          signal,
        );
        if (frozen.problem_id !== problemId) {
          throw new TypeError("contest problem does not match arena route");
        }
        return frozen;
      }
      return problemService.getById(problemId, signal);
    },
  });
  const contestProblem =
    problemResource.data &&
    "contest_problem_id" in problemResource.data
      ? problemResource.data
      : null;
  const problem: ProblemWorkspaceStatement | null = contestProblem
    ? toContestWorkspaceStatement(contestProblem)
    : problemResource.data &&
        !("contest_problem_id" in problemResource.data)
      ? problemResource.data
      : null;
  const samplesResource = useRetryableResource<SampleTestCaseResponse[]>({
    resetKey: problemId,
    enabled: problemId !== null && !contestContextValid,
    initialData: [],
    load: (signal) =>
      problemId
        ? problemService.getPublicSamples(problemId, signal)
        : Promise.resolve([]),
  });
  const testCases = useMemo(
    () =>
      [...(contestProblem?.samples ?? samplesResource.data)]
        .sort((left, right) => left.order_index - right.order_index)
        .map(toDisplayTestCase),
    [contestProblem, samplesResource.data],
  );
  const sampleStatus = contestContextValid
    ? problemResource.status
    : samplesResource.status;
  const retrySamples = contestContextValid
    ? problemResource.retry
    : samplesResource.retry;
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [activeTab, setActiveTab] =
    useState<ProblemTab>("description");
  const [mobilePane, setMobilePane] =
    useState<MobileWorkspacePane>("statement");

  const submissionDisabledReason = contestContextInvalid
    ? TEXT.PROBLEM.CONTEST_CONTEXT_INVALID
    : isLoadingAuth
      ? TEXT.PROBLEM.SESSION_CHECKING
      : !user
        ? TEXT.PROBLEM.AUTH_REQUIRED_TO_SUBMIT
        : undefined;

  async function handleSubmit(
    code: string,
    runtime: JudgeRuntime,
  ): Promise<boolean> {
    setSubmitError(null);
    if (!problemId || !user) {
      setSubmitError(TEXT.PROBLEM.AUTH_REQUIRED_TO_SUBMIT);
      return false;
    }
    if (contestContextInvalid) {
      setSubmitError(TEXT.PROBLEM.CONTEST_CONTEXT_INVALID);
      return false;
    }

    setIsSubmitting(true);
    try {
      const accepted = await submissionService.submit(
        activeContestId && contestProblemId
          ? {
              problem_id: problemId,
              runtime_key: runtime.runtime_key,
              source_code: code,
              contest_id: activeContestId,
              contest_problem_id: contestProblemId,
            }
          : {
              problem_id: problemId,
              runtime_key: runtime.runtime_key,
              source_code: code,
            },
      );
      const optimisticSubmission: SubmissionSummary = {
        id: accepted.id,
        problem_id: problemId,
        user_id: user.id,
        ...(activeContestId ? { contest_id: activeContestId } : {}),
        language: runtime.language,
        status: intakeStatus(accepted),
        passed_count: 0,
        total_count: 0,
        created_at: accepted.submitted_at,
      };

      recordSubmission(optimisticSubmission);
      setActiveTab("history");
      notify.success(TEXT.PROBLEM.SUBMIT_SUCCESS_SHORT);
      void refreshSubmissions();
      return true;
    } catch (error: unknown) {
      setSubmitError(
        getErrorMessage(error, TEXT.PROBLEM.SUBMIT_ERROR),
      );
      return false;
    } finally {
      setIsSubmitting(false);
    }
  }

  if (problemResource.status === "loading") {
    return (
      <DataLoadingFeedback
        label={TEXT.PROBLEM.LOADING}
        className="min-h-[50dvh]"
      />
    );
  }

  const problemNotFound =
    problemResource.error instanceof ApiError &&
    problemResource.error.status === 404;

  if (problemResource.status === "error" && !problemNotFound) {
    return (
      <div className="mx-auto max-w-2xl py-16">
        <DataLoadFeedback
          title={TEXT.PROBLEM.LOAD_ERROR_TITLE}
          description={TEXT.PROBLEM.LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.PROBLEM.RETRY}
          onRetry={problemResource.retry}
        />
      </div>
    );
  }

  if (problemNotFound || !problem) {
    return (
      <div className="mx-auto max-w-lg space-y-4 py-20 text-center">
        <h1 className="text-2xl font-bold text-foreground">
          {TEXT.PROBLEM.NOT_FOUND}
        </h1>
        <p className="text-sm leading-6 text-muted-foreground">
          {TEXT.PROBLEM.NOT_FOUND_DESCRIPTION}
        </p>
        <Link
          href={backHref}
          className="inline-flex min-h-11 items-center justify-center rounded-lg px-3 text-sm font-medium text-primary outline-none transition-colors hover:bg-primary/10 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {backLabel}
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-4 motion-safe:animate-in motion-safe:fade-in">
      {submitError ? (
        <div
          className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive"
          role="alert"
        >
          {submitError}
        </div>
      ) : null}

      {contestContextInvalid ? (
        <div
          className="flex items-start gap-3 rounded-xl border border-status-warning/25 bg-status-warning/10 px-4 py-3 text-status-warning"
          role="alert"
        >
          <AlertTriangle
            className="mt-0.5 size-5 shrink-0"
            aria-hidden="true"
          />
          <div>
            <p className="text-sm font-semibold">
              {TEXT.PROBLEM.CONTEST_CONTEXT_INVALID_TITLE}
            </p>
            <p className="mt-1 text-sm leading-6">
              {TEXT.PROBLEM.CONTEST_CONTEXT_INVALID}
            </p>
          </div>
        </div>
      ) : null}

      <Card className="glass-card min-h-[calc(100dvh-10rem)] gap-0 overflow-hidden border-border/50 py-0">
        <div className="border-b border-border bg-surface-sunken/30 px-3 py-2 sm:px-5">
          <Link
            href={backHref}
            className="inline-flex min-h-11 items-center gap-2 rounded-lg px-2 text-sm font-medium text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {backLabel}
          </Link>
        </div>

        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as ProblemTab)}
          className="min-h-0 flex-1 gap-0"
        >
          <TabsList
            variant="line"
            aria-label={TEXT.PROBLEM.DESCRIPTION}
            className="w-full justify-start gap-0 border-b border-border bg-background px-2 sm:px-4"
          >
            <TabsTrigger value="description" className="flex-none px-4">
              <FileText aria-hidden="true" />
              {TEXT.PROBLEM.DESCRIPTION}
            </TabsTrigger>
            <TabsTrigger
              value="history"
              disabled={contestContextInvalid}
              className="flex-none px-4"
            >
              <History aria-hidden="true" />
              {TEXT.PROBLEM.SUBMISSION_HISTORY}
            </TabsTrigger>
          </TabsList>

          <div className="min-h-0 flex-1 overflow-y-auto">
            <TabsContent
              value="description"
              keepMounted
              className="min-h-full"
            >
              <nav
                className="sticky top-0 z-10 grid grid-cols-2 border-b border-border bg-surface/95 p-2 backdrop-blur lg:hidden"
                aria-label={TEXT.PROBLEM.PROBLEM_MOBILE_NAVIGATION}
              >
                <button
                  type="button"
                  className={`min-h-11 rounded-lg px-3 text-sm font-semibold outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 ${
                    mobilePane === "statement"
                      ? "bg-primary/10 text-primary"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  }`}
                  aria-pressed={mobilePane === "statement"}
                  onClick={() => setMobilePane("statement")}
                >
                  <FileText
                    className="mr-2 inline size-4"
                    aria-hidden="true"
                  />
                  {TEXT.PROBLEM.DESCRIPTION}
                </button>
                <button
                  type="button"
                  className={`min-h-11 rounded-lg px-3 text-sm font-semibold outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 ${
                    mobilePane === "workbench"
                      ? "bg-primary/10 text-primary"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  }`}
                  aria-pressed={mobilePane === "workbench"}
                  onClick={() => setMobilePane("workbench")}
                >
                  <Code2
                    className="mr-2 inline size-4"
                    aria-hidden="true"
                  />
                  {TEXT.PROBLEM.SUBMIT}
                </button>
              </nav>

              <div className="grid min-h-full lg:grid-cols-[minmax(0,1.08fr)_minmax(23rem,0.92fr)]">
                <section
                  aria-label={TEXT.PROBLEM.DESCRIPTION}
                  className={`min-w-0 p-4 sm:p-6 lg:block lg:border-r lg:border-border ${
                    mobilePane === "statement" ? "block" : "hidden"
                  }`}
                >
                  <ProblemDescriptionPanel
                    problem={problem}
                    testCases={testCases}
                    sampleStatus={sampleStatus}
                    onRetrySamples={retrySamples}
                  />
                </section>
                <aside
                  className={`min-w-0 bg-surface-sunken/20 p-3 sm:p-4 lg:block ${
                    mobilePane === "workbench" ? "block" : "hidden"
                  }`}
                >
                  <FileSubmission
                    onSubmit={handleSubmit}
                    isSubmitting={isSubmitting}
                    submissionDisabledReason={submissionDisabledReason}
                    loginRequired={
                      !contestContextInvalid && !isLoadingAuth && !user
                    }
                    allowedRuntimeKeys={
                      contestProblem?.allowed_runtime_keys
                    }
                  />
                </aside>
              </div>
            </TabsContent>

            <TabsContent
              value="history"
              keepMounted
              className="p-4 sm:p-6"
            >
              <SubmissionHistory
                submissions={submissions}
                status={submissionStatus}
                isAuthenticated={user !== null}
                isSessionLoading={isLoadingAuth}
                isRefreshing={isRefreshingSubmissions}
                isLoadingMore={isLoadingMoreSubmissions}
                hasMore={hasMoreSubmissions}
                topicState={topicState}
                onRefresh={refreshSubmissions}
                onLoadMore={loadMoreSubmissions}
              />
            </TabsContent>
          </div>
        </Tabs>
      </Card>
    </div>
  );
}
