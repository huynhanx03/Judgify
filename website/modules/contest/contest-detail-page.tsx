"use client";

/**
 * Contest detail page — info, live standings and rating changes.
 */

import { useState, use } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ContestStandingsTable } from "@/modules/contest/contest-standings-table";
import { ContestRatingTable } from "@/modules/contest/contest-rating-table";
import { ContestContentPanel } from "@/modules/contest/contest-content-panel";
import { contestService } from "@/services/contest.service";
import { useContestStandings } from "@/hooks/use-contest-standings";
import { useContestLifecycleResource } from "@/hooks/use-contest-lifecycle-resource";
import { useContestRatingChanges } from "@/hooks/use-contest-rating-changes";
import { notify, getErrorMessage } from "@/lib/toast";
import { ApiError } from "@/lib/api/error";
import { contestProblemHref } from "@/lib/contest/navigation";
import { TEXT } from "@/constants/text";
import { APP_ROUTES } from "@/constants/routes";
import { useAuth } from "@/contexts/auth-context";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import type { Contest } from "@/types/contest";
import {
  ArrowLeft,
  Calendar,
  Clock,
  Trophy,
  Users,
  FileCode2,
  Info,
  UserX,
  CheckCircle2,
  RefreshCw,
  TrendingUp,
  CircleHelp,
  CircleAlert,
  Loader2,
  LockKeyhole,
} from "lucide-react";
import { tryEntityID } from "@/lib/api/contracts";
import {
  formatDateTimeMinute,
  formatDurationMinutes,
  formatNumber,
} from "@/lib/format";

const STATUS_STYLES: Record<string, string> = {
  draft: "border-border bg-muted text-muted-foreground",
  upcoming: "border-info/25 bg-info/10 text-info",
  running: "border-success/25 bg-success/10 text-success",
  ended: "border-border bg-muted text-muted-foreground",
  cancelled: "border-danger/25 bg-danger/10 text-danger",
};

const STATUS_LABELS: Record<string, string> = {
  draft: TEXT.CONTEST.STATUS_DRAFT,
  upcoming: TEXT.CONTEST.STATUS_UPCOMING,
  running: TEXT.CONTEST.STATUS_RUNNING,
  ended: TEXT.CONTEST.STATUS_ENDED,
  cancelled: TEXT.CONTEST.STATUS_CANCELLED,
};

type Tab = "info" | "standings" | "rating" | "content";

function getDuration(start: string, end: string): string {
  return formatDurationMinutes(
    new Date(end).getTime() - new Date(start).getTime(),
  );
}

export default function ContestDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const contestId = tryEntityID(id);
  const { isAuthenticated, isLoading: isAuthLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<Tab>("info");
  const [isRegistering, setIsRegistering] = useState(false);

  const {
    standings,
    view: standingsView,
    frozenAt: standingsFrozenAt,
    myStanding,
    hasSelfError: standingsSelfError,
    hasLoaded: standingsHasLoaded,
    isRefreshing,
    hasError: standingsHasError,
    retry: retryStandings,
  } = useContestStandings(
    contestId,
    activeTab === "standings",
    isAuthenticated,
  );
  const contestResource = useContestLifecycleResource<Contest | null>({
    resourceKey: contestId ?? "invalid-contest-id",
    enabled: contestId !== null,
    initialData: null,
    load: (signal) =>
      contestId
        ? contestService.getById(contestId, signal)
        : Promise.resolve(null),
  });
  const contest = contestResource.data;
  const isLoading =
    contestResource.status === "loading" && contestResource.data === null;
  const ratingResource = useContestRatingChanges(
    contestId,
    activeTab === "rating" && contest?.status === "ended",
  );

  async function handleRegister() {
    if (!contest || !contestId) return;
    setIsRegistering(true);
    try {
      await contestService.register(
        contestId,
        contest.version,
        contest.registration?.version ?? 0,
      );
      notify.success(TEXT.CONTEST.REGISTER_SUCCESS);
      contestResource.retry();
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.CONTEST.REGISTER_ERROR));
    } finally {
      setIsRegistering(false);
    }
  }

  async function handleUnregister() {
    if (!contest?.registration || !contestId) return;
    setIsRegistering(true);
    try {
      await contestService.unregister(
        contestId,
        contest.version,
        contest.registration.version,
      );
      notify.success(TEXT.CONTEST.UNREGISTER_SUCCESS);
      contestResource.retry();
    } catch (err) {
      notify.error(getErrorMessage(err, TEXT.CONTEST.UNREGISTER_ERROR));
    } finally {
      setIsRegistering(false);
    }
  }

  if (isLoading) {
    return (
      <DataLoadingFeedback
        label={TEXT.CONTEST.DETAIL_LOADING}
        className="min-h-[50dvh]"
      />
    );
  }

  const contestNotFound =
    contestResource.error instanceof ApiError &&
    contestResource.error.status === 404;

  if (contestResource.status === "error" && !contestNotFound) {
    return (
      <div className="mx-auto max-w-2xl py-16">
        <DataLoadFeedback
          title={TEXT.CONTEST.DETAIL_LOAD_ERROR_TITLE}
          description={TEXT.CONTEST.DETAIL_LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.CONTEST.RETRY}
          onRetry={contestResource.retry}
        />
      </div>
    );
  }

  if (!contestId || contestNotFound || !contest) {
    return (
      <div className="mx-auto max-w-lg space-y-4 py-20 text-center">
        <h1 className="text-2xl font-bold text-foreground">
          {TEXT.CONTEST.NOT_FOUND}
        </h1>
        <p className="text-sm leading-6 text-muted-foreground">
          {TEXT.CONTEST.NOT_FOUND_DESCRIPTION}
        </p>
        <Link
          href={APP_ROUTES.CONTEST}
          className="inline-flex min-h-11 items-center justify-center rounded-lg px-3 text-sm font-medium text-primary outline-none transition-colors hover:bg-primary/10 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {TEXT.CONTEST.BACK_TO_LIST}
        </Link>
      </div>
    );
  }

  const isEnded = contest.status === "ended";
  const registrationAvailable =
    contest.status === "upcoming" &&
    contest.registration_mode === "registered" &&
    !contest.is_registered &&
    contest.registration?.status !== "banned";
  const canRegister = isAuthenticated && registrationAvailable;
  const showLoginToRegister =
    !isAuthenticated && !isAuthLoading && registrationAvailable;
  const canUnregister =
    contest.status === "upcoming" &&
    contest.registration_mode === "registered" &&
    contest.is_registered;
  const contestProblems = [...contest.problems].sort(
    (left, right) => left.display_order - right.display_order,
  );

  return (
    <div className="space-y-4 animate-in fade-in duration-300 motion-reduce:animate-none">
      <Card className="glass-card border-border/40 overflow-hidden flex flex-col min-h-[calc(100vh-180px)]">
        {/* Top bar */}
        <div className="px-6 py-4 border-b border-border/40 bg-muted/20">
          <div className="flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-3">
              <Link
                href={APP_ROUTES.CONTEST}
                className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <ArrowLeft className="h-4 w-4" />
                {TEXT.CONTEST.BACK_TO_LIST}
              </Link>
            </div>
            <div className="flex items-center gap-3">
              <Badge variant="outline" className={STATUS_STYLES[contest.status] ?? ""}>
                {STATUS_LABELS[contest.status] ?? TEXT.COMMON.UNKNOWN}
              </Badge>
              {contest.is_registered ? (
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="border-success/30 bg-success/10 text-success">
                    <CheckCircle2 className="h-3 w-3 mr-1" aria-hidden="true" />
                    {TEXT.CONTEST.REGISTERED}
                  </Badge>
                  {canUnregister && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="border-danger/30 text-danger hover:bg-danger/10"
                      onClick={handleUnregister}
                      disabled={isRegistering}
                    >
                      <UserX className="h-3.5 w-3.5 mr-1" aria-hidden="true" />
                      {TEXT.CONTEST.UNREGISTER}
                    </Button>
                  )}
                </div>
              ) : (
                canRegister ? (
                  <Button size="sm" onClick={handleRegister} disabled={isRegistering}>
                    {TEXT.CONTEST.REGISTER}
                  </Button>
                ) : showLoginToRegister ? (
                  <Button
                    size="sm"
                    render={<Link href={APP_ROUTES.LOGIN} />}
                  >
                    {TEXT.CONTEST.LOGIN_TO_REGISTER}
                  </Button>
                ) : null
              )}
            </div>
          </div>
        </div>

        {/* Header */}
        <div className="px-6 py-5 border-b border-border/40 space-y-3">
          <h1 className="text-2xl font-bold">{contest.title}</h1>
          <div className="flex flex-wrap items-center gap-5 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Calendar className="h-4 w-4 text-cultivation" aria-hidden="true" />
              <span>
                {formatDateTimeMinute(contest.start_time)} →{" "}
                {formatDateTimeMinute(contest.end_time)}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-cultivation" aria-hidden="true" />
              <span>{getDuration(contest.start_time, contest.end_time)}</span>
            </div>
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-cultivation" aria-hidden="true" />
              <span>{contest.participant_count}{contest.max_participants !== undefined ? `/${contest.max_participants}` : ""} {TEXT.CONTEST.PARTICIPANTS}</span>
            </div>
            <div className="flex items-center gap-2">
              <FileCode2 className="h-4 w-4 text-cultivation" aria-hidden="true" />
              <span>{TEXT.CONTEST.PROBLEMS}: {contest.problem_count}</span>
            </div>
          </div>
        </div>

        {/* Tab bar */}
        <div
          className="flex overflow-x-auto border-b border-border/40 bg-muted/10 px-2"
          role="tablist"
          aria-label={TEXT.CONTEST.DETAIL_TABS_LABEL}
        >
          <button
            type="button"
            id="contest-tab-info"
            role="tab"
            aria-selected={activeTab === "info"}
            aria-controls="contest-tab-panel"
            onClick={() => setActiveTab("info")}
            className={`flex min-h-11 shrink-0 items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
              activeTab === "info"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Info className="h-4 w-4" aria-hidden="true" />
            {TEXT.CONTEST.DETAIL_INFO}
          </button>
          <button
            type="button"
            id="contest-tab-content"
            role="tab"
            aria-selected={activeTab === "content"}
            aria-controls="contest-tab-panel"
            onClick={() => setActiveTab("content")}
            className={`flex min-h-11 shrink-0 items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
              activeTab === "content"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <CircleHelp className="h-4 w-4" aria-hidden="true" />
            {TEXT.CONTEST.CONTENT_TAB}
          </button>
          <button
            type="button"
            id="contest-tab-standings"
            role="tab"
            aria-selected={activeTab === "standings"}
            aria-controls="contest-tab-panel"
            onClick={() => setActiveTab("standings")}
            className={`flex min-h-11 shrink-0 items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
              activeTab === "standings"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Trophy className="h-4 w-4" aria-hidden="true" />
            {TEXT.CONTEST.STANDINGS}
            {activeTab === "standings" && isRefreshing && (
              <RefreshCw className="h-3.5 w-3.5 text-muted-foreground animate-spin motion-reduce:animate-none" aria-hidden="true" />
            )}
          </button>
          {isEnded && (
            <button
              type="button"
              id="contest-tab-rating"
              role="tab"
              aria-selected={activeTab === "rating"}
              aria-controls="contest-tab-panel"
              onClick={() => setActiveTab("rating")}
              className={`flex min-h-11 shrink-0 items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                activeTab === "rating"
                  ? "text-primary border-b-2 border-primary"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <TrendingUp className="h-4 w-4" aria-hidden="true" />
              {TEXT.CONTEST.RATING_CHANGES}
            </button>
          )}
        </div>

        {/* Tab content */}
        <div
          id="contest-tab-panel"
          role="tabpanel"
          aria-labelledby={`contest-tab-${activeTab}`}
          className="flex-1 overflow-y-auto p-4 sm:p-6"
        >
          {activeTab === "info" && (
            <div className="space-y-6 max-w-3xl">
              {contest.description ? (
                <div>
                  <h3 className="text-lg font-bold mb-3">{TEXT.CONTEST.DETAIL_DESCRIPTION}</h3>
                  <p className="text-muted-foreground whitespace-pre-wrap leading-relaxed">
                    {contest.description}
                  </p>
                </div>
              ) : (
                <p className="text-muted-foreground italic">{TEXT.CONTEST.NO_DESCRIPTION}</p>
              )}

              <div className="h-px bg-border/40" />

              {/* Problem list */}
              <div>
                <h3 className="text-lg font-bold mb-3">{TEXT.CONTEST.PROBLEMS}</h3>
                {contestProblems.length > 0 ? (
                  <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                    {contestProblems.map((membership) => (
                      <Link
                        key={membership.contest_problem_id}
                        href={contestProblemHref(
                          contestId,
                          membership.problem_id,
                          membership.contest_problem_id,
                        )}
                        className="flex items-center gap-3 p-4 rounded-xl border border-border/40 bg-muted/20 hover:bg-muted/40 transition-colors"
                      >
                        <div className="h-10 w-10 rounded-lg bg-cultivation/10 text-cultivation flex items-center justify-center font-bold text-sm">
                          {membership.alias}
                        </div>
                        <div>
                          <p className="font-medium text-sm">
                            {membership.problem_title}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {membership.points === undefined
                              ? TEXT.CONTEST.PROBLEM_NUMBER(membership.alias)
                              : TEXT.CONTEST.PROBLEM_POINTS(membership.points)}
                          </p>
                        </div>
                      </Link>
                    ))}
                  </div>
                ) : (
                  <p className="text-muted-foreground italic">{TEXT.CONTEST.NO_PROBLEMS}</p>
                )}
              </div>
            </div>
          )}

          {activeTab === "standings" && (
            !standingsHasLoaded && !standingsHasError ? (
              <div
                className="flex min-h-64 items-center justify-center gap-2 text-sm text-muted-foreground"
                role="status"
              >
                <Loader2
                  className="size-5 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
                {TEXT.CONTEST.STANDINGS_LOADING}
              </div>
            ) : standingsHasError && standings.length === 0 ? (
              <div className="flex min-h-64 flex-col items-center justify-center text-center" role="alert">
                <div className="flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
                  <CircleAlert className="size-5" aria-hidden="true" />
                </div>
                <p className="mt-4 font-semibold">{TEXT.CONTEST.STANDINGS_LOAD_ERROR}</p>
                <p className="mt-1 max-w-md text-sm text-muted-foreground">
                  {TEXT.CONTEST.STANDINGS_LOAD_ERROR_DESCRIPTION}
                </p>
                <Button type="button" variant="outline" className="mt-5" onClick={retryStandings}>
                  <RefreshCw className="size-4" aria-hidden="true" />
                  {TEXT.COMMON.RETRY}
                </Button>
              </div>
            ) : (
              <div className="space-y-3">
                {standingsHasError ? (
                  <div className="flex items-center justify-between gap-3 rounded-xl border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm" role="status">
                    <span className="text-muted-foreground">{TEXT.CONTEST.STANDINGS_REFRESH_ERROR}</span>
                    <Button type="button" variant="ghost" size="sm" onClick={retryStandings}>
                      {TEXT.COMMON.RETRY}
                    </Button>
                  </div>
                ) : null}
                {standingsView === "frozen" && standingsFrozenAt ? (
                  <div
                    className="flex items-start gap-3 rounded-xl border border-warning/25 bg-warning/8 px-4 py-3"
                    role="status"
                  >
                    <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-warning/12 text-warning">
                      <LockKeyhole className="size-4" aria-hidden="true" />
                    </div>
                    <div>
                      <p className="text-sm font-semibold">
                        {TEXT.CONTEST.STANDINGS_FROZEN_LABEL}
                      </p>
                      <p className="mt-0.5 text-xs leading-5 text-muted-foreground">
                        {TEXT.CONTEST.STANDINGS_FROZEN_DESCRIPTION(
                          formatDateTimeMinute(standingsFrozenAt),
                        )}
                      </p>
                    </div>
                  </div>
                ) : null}
                {myStanding ? (
                  <section className="overflow-hidden rounded-2xl border border-primary/20 bg-primary/[0.04]">
                    <div className="flex items-start gap-3 border-b border-primary/10 px-4 py-3 sm:px-5">
                      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                        <Trophy className="size-4" aria-hidden="true" />
                      </div>
                      <div>
                        <h3 className="text-sm font-semibold">
                          {TEXT.CONTEST.STANDINGS_MY_POSITION}
                        </h3>
                        <p className="mt-0.5 text-xs leading-5 text-muted-foreground">
                          {TEXT.CONTEST.STANDINGS_MY_POSITION_DESCRIPTION}
                        </p>
                      </div>
                    </div>
                    <dl className="grid grid-cols-2 divide-x divide-primary/10 sm:grid-cols-4">
                      {[
                        [TEXT.CONTEST.RANK, myStanding.rank],
                        [TEXT.CONTEST.SOLVED, myStanding.solved_count],
                        [TEXT.CONTEST.PENALTY, myStanding.penalty],
                        [TEXT.CONTEST.STANDINGS_PENDING, myStanding.pending_attempts],
                      ].map(([label, value]) => (
                        <div key={String(label)} className="px-4 py-3 sm:px-5">
                          <dt className="text-[0.7rem] font-semibold tracking-wide text-muted-foreground uppercase">
                            {label}
                          </dt>
                          <dd className="mt-1 text-lg font-bold tabular-nums">
                            {formatNumber(Number(value))}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </section>
                ) : standingsSelfError ? (
                  <div className="rounded-xl border border-warning/20 bg-warning/5 px-4 py-3 text-sm text-muted-foreground" role="status">
                    {TEXT.CONTEST.STANDINGS_MY_POSITION_ERROR}
                  </div>
                ) : null}
                <ContestStandingsTable standings={standings} />
              </div>
            )
          )}

          {activeTab === "content" ? (
            <ContestContentPanel
              contestID={contestId}
              isParticipant={Boolean(contest.is_registered)}
              canSubmit={Boolean(contest.is_registered) && contest.status === "running"}
            />
          ) : null}

          {activeTab === "rating" && isEnded && (
            ratingResource.status === "loading" ? (
              <div
                className="flex min-h-64 items-center justify-center gap-2 text-sm text-muted-foreground"
                role="status"
              >
                <Loader2 className="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                {TEXT.CONTEST.RATING_LOADING}
              </div>
            ) : ratingResource.status === "error" ? (
              <div className="flex min-h-64 flex-col items-center justify-center text-center" role="alert">
                <div className="flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
                  <CircleAlert className="size-5" aria-hidden="true" />
                </div>
                <p className="mt-4 font-semibold">{TEXT.CONTEST.RATING_LOAD_ERROR}</p>
                <p className="mt-1 max-w-md text-sm text-muted-foreground">
                  {TEXT.CONTEST.RATING_LOAD_ERROR_DESCRIPTION}
                </p>
                <Button type="button" variant="outline" className="mt-5" onClick={ratingResource.retry}>
                  <RefreshCw className="size-4" aria-hidden="true" />
                  {TEXT.COMMON.RETRY}
                </Button>
              </div>
            ) : (
              <ContestRatingTable ratingChanges={ratingResource.data} />
            )
          )}
        </div>
      </Card>
    </div>
  );
}
