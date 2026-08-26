"use client";

import Link from "next/link";
import { useEffect } from "react";
import {
  ArrowLeft,
  CalendarClock,
  ExternalLink,
  Gauge,
  ListChecks,
  Megaphone,
  RefreshCw,
  ShieldCheck,
  Swords,
  Users,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  contestStaffTopic,
  contestStandingsTopic,
  EVENT_CONTEST_LIFECYCLE_CHANGED_V1,
  EVENT_CONTEST_RATING_CALCULATED_V1,
  EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
  EVENT_CONTEST_STANDINGS_CHANGED_V1,
} from "@/constants/realtime";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { APP_ROUTES } from "@/constants/routes";
import { useAuth } from "@/contexts/auth-context";
import { useRealtime } from "@/contexts/realtime-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { formatDateTime, formatNumber } from "@/lib/format";
import { parseContestInvalidationV1Envelope } from "@/lib/realtime/contest-event-schema";
import { parseContestStandingsChangedV1Envelope } from "@/lib/realtime/contest-standings-event-schema";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeEnvelope,
} from "@/lib/realtime/websocket-client";
import {
  AdminAccessLoading,
  AdminAccessState,
} from "@/modules/admin/admin-access-state";
import { ContestRatingTable } from "@/modules/contest/contest-rating-table";
import { ContestStandingsTable } from "@/modules/contest/contest-standings-table";
import { contestService } from "@/services/contest.service";
import type {
  Contest,
  RatingChange,
  StandingsSnapshot,
} from "@/types/contest";

interface ContestOverview {
  contest: Contest | null;
  standings: StandingsSnapshot | null;
  ratingChanges: RatingChange[];
}

const EMPTY_OVERVIEW: ContestOverview = {
  contest: null,
  standings: null,
  ratingChanges: [],
};

const STATUS_TONE: Record<string, string> = {
  draft: "bg-muted text-muted-foreground",
  upcoming: "bg-info/10 text-info",
  running: "bg-success/10 text-success",
  ended: "bg-muted text-muted-foreground",
  cancelled: "bg-destructive/10 text-destructive",
};

const STATUS_LABEL: Record<string, string> = {
  draft: TEXT.CONTEST.STATUS_DRAFT,
  upcoming: TEXT.CONTEST.STATUS_UPCOMING,
  running: TEXT.CONTEST.STATUS_RUNNING,
  ended: TEXT.CONTEST.STATUS_ENDED,
  cancelled: TEXT.CONTEST.STATUS_CANCELLED,
};

export function AdminContestOverview({
  contestID,
}: {
  contestID: string;
}) {
  const {
    isAuthenticated,
    isLoading,
    bootstrapStatus,
    can,
    retryBootstrap,
  } = useAuth();
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
  } = useRealtime();
  const canRead = can(
    AUTHORIZATION_RESOURCE.CONTEST,
    AUTHORIZATION_ACTION.READ,
  );
  const resource = useRetryableResource<ContestOverview>({
    resetKey: contestID,
    enabled: isAuthenticated && canRead,
    initialData: EMPTY_OVERVIEW,
    keepPreviousData: true,
    load: async (signal) => {
      const [contest, standings, ratingChanges] = await Promise.all([
        contestService.getAdminById(contestID, signal),
        contestService.getAdminStandings(contestID, signal),
        contestService.getRatingChanges(contestID, signal),
      ]);
      return { contest, standings, ratingChanges };
    },
  });
  const retryOverview = resource.retry;

  useEffect(() => {
    if (!isAuthenticated || !canRead) return;
    const staffTopic = contestStaffTopic(contestID);
    const standingsTopic = contestStandingsTopic(contestID);
    const staffTopics = new Set([staffTopic]);
    const refresh = () => {
      if (document.visibilityState === "visible") {
        retryOverview();
      }
    };
    const parseContestChange = (
      payload: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
      try {
        parseContestInvalidationV1Envelope(
          payload,
          envelope,
          contestID,
          staffTopics,
        );
        refresh();
      } catch {
        // Cross-topic and malformed frames never invalidate admin state.
      }
    };
    const parseStandingChange = (
      payload: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
      try {
        parseContestStandingsChangedV1Envelope(
          payload,
          envelope,
          contestID,
          standingsTopic,
        );
        refresh();
      } catch {
        // The REST snapshot remains authoritative.
      }
    };
    const removers = [
      subscribeToTopic(staffTopic),
      subscribeToTopic(standingsTopic),
      subscribe<unknown>(
        EVENT_CONTEST_LIFECYCLE_CHANGED_V1,
        parseContestChange,
      ),
      subscribe<unknown>(
        EVENT_CONTEST_RATING_CALCULATED_V1,
        parseContestChange,
      ),
      subscribe<unknown>(
        EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
        parseContestChange,
      ),
      subscribe<unknown>(
        EVENT_CONTEST_STANDINGS_CHANGED_V1,
        parseStandingChange,
      ),
      onResyncRequired(refresh),
      onStateChange((state) => {
        if (state === "open") refresh();
      }),
      onTopicStateChange(staffTopic, (state) => {
        if (state === REALTIME_TOPIC_STATE.SUBSCRIBED) refresh();
      }),
      onTopicStateChange(standingsTopic, (state) => {
        if (state === REALTIME_TOPIC_STATE.SUBSCRIBED) refresh();
      }),
    ];
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      removers.forEach((remove) => remove());
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [
    canRead,
    contestID,
    isAuthenticated,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
    retryOverview,
    subscribe,
    subscribeToTopic,
  ]);

  if (isLoading) return <AdminAccessLoading />;
  if (bootstrapStatus === "error") {
    return (
      <AdminAccessState kind="unavailable" onRetry={retryBootstrap} />
    );
  }
  if (!isAuthenticated || !canRead) {
    return <AdminAccessState kind="forbidden" />;
  }
  if (resource.status === "loading" && !resource.data.contest) {
    return <AdminAccessLoading />;
  }
  if (resource.status === "error" || !resource.data.contest) {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-2xl items-center">
        <Card className="w-full border-destructive/20">
          <CardHeader>
            <CardTitle>
              {ADMIN_TEXT.CONTESTS.OVERVIEW_LOAD_ERROR}
            </CardTitle>
            <CardDescription>
              {ADMIN_TEXT.CONTESTS.OVERVIEW_LOAD_ERROR_DESCRIPTION}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={resource.retry}>
              <RefreshCw className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.CONTESTS.OVERVIEW_RETRY}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const { contest, standings, ratingChanges } = resource.data;
  const ratingState = !contest.rated
    ? ADMIN_TEXT.CONTESTS.RATING_DISABLED
    : contest.rating_completed_at
      ? ADMIN_TEXT.CONTESTS.RATING_APPLIED
      : ADMIN_TEXT.CONTESTS.RATING_PENDING;

  return (
    <div className="mx-auto w-full max-w-[96rem] space-y-6">
      <header className="relative overflow-hidden rounded-2xl border border-border bg-card px-5 py-6 shadow-sm sm:px-7">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/70 to-transparent" />
        <div className="flex flex-col justify-between gap-5 lg:flex-row lg:items-end">
          <div>
            <Link
              href={APP_ROUTES.ADMIN_CONTESTS}
              className="inline-flex min-h-11 items-center gap-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:rounded-md focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.CONTESTS.OVERVIEW_BACK}
            </Link>
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <h1 className="text-balance text-2xl font-bold tracking-tight sm:text-3xl">
                {contest.title}
              </h1>
              <Badge
                variant="secondary"
                className={STATUS_TONE[contest.status] ?? ""}
              >
                {STATUS_LABEL[contest.status] ?? TEXT.COMMON.UNKNOWN}
              </Badge>
              <Badge variant="outline" className="gap-1.5">
                <ShieldCheck className="size-3.5" aria-hidden="true" />
                {ADMIN_TEXT.CONTESTS.OVERVIEW_LIVE}
              </Badge>
            </div>
            <p className="mt-2 max-w-3xl text-pretty text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.CONTESTS.OVERVIEW_SUBTITLE}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link
              href={APP_ROUTES.ADMIN_CONTEST_COMMUNICATIONS(contest.id)}
              className={buttonVariants({ variant: "outline" })}
            >
              <Megaphone className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.CONTESTS.OVERVIEW_COMMUNICATIONS}
            </Link>
            <Link
              href={APP_ROUTES.CONTEST_DETAIL(contest.id)}
              className={buttonVariants()}
            >
              <ExternalLink className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.CONTESTS.OVERVIEW_PUBLIC}
            </Link>
          </div>
        </div>
      </header>

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          icon={Users}
          label={ADMIN_TEXT.CONTESTS.OVERVIEW_PARTICIPANTS}
          value={formatNumber(contest.participant_count)}
        />
        <Metric
          icon={Swords}
          label={ADMIN_TEXT.CONTESTS.OVERVIEW_PROBLEMS}
          value={formatNumber(contest.problem_count)}
        />
        <Metric
          icon={ListChecks}
          label={ADMIN_TEXT.CONTESTS.OVERVIEW_STANDINGS_VERSION}
          value={formatNumber(contest.standings_projection_version)}
        />
        <Metric
          icon={Gauge}
          label={ADMIN_TEXT.CONTESTS.OVERVIEW_RATING_STATE}
          value={ratingState}
        />
      </section>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CalendarClock className="size-5 text-primary" aria-hidden="true" />
            {ADMIN_TEXT.CONTESTS.OVERVIEW_SCHEDULE}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm sm:grid-cols-2 xl:grid-cols-4">
          <Fact
            label={ADMIN_TEXT.CONTESTS.OVERVIEW_START}
            value={formatDateTime(contest.start_time)}
          />
          <Fact
            label={ADMIN_TEXT.CONTESTS.OVERVIEW_END}
            value={formatDateTime(contest.end_time)}
          />
          <Fact
            label={ADMIN_TEXT.CONTESTS.OVERVIEW_FREEZE}
            value={
              contest.freeze_standings_at
                ? formatDateTime(contest.freeze_standings_at)
                : ADMIN_TEXT.CONTESTS.OVERVIEW_NO_FREEZE
            }
          />
          <Fact
            label={ADMIN_TEXT.CONTESTS.OVERVIEW_ALGORITHM}
            value={contest.rating_algorithm_version}
          />
        </CardContent>
      </Card>

      <Tabs defaultValue="standings">
        <TabsList className="w-full justify-start overflow-x-auto sm:w-fit">
          <TabsTrigger value="standings">
            {ADMIN_TEXT.CONTESTS.OVERVIEW_OFFICIAL_TAB}
          </TabsTrigger>
          <TabsTrigger value="rating">
            {ADMIN_TEXT.CONTESTS.OVERVIEW_RATING_TAB}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="standings" className="pt-3">
          <Card>
            <CardHeader>
              <CardTitle>
                {ADMIN_TEXT.CONTESTS.OVERVIEW_OFFICIAL_TAB}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.CONTESTS.OVERVIEW_OFFICIAL_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="px-0">
              <ContestStandingsTable
                standings={standings?.items ?? []}
              />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="rating" className="pt-3">
          <Card>
            <CardHeader>
              <CardTitle>
                {ADMIN_TEXT.CONTESTS.OVERVIEW_RATING_TAB}
              </CardTitle>
              <CardDescription>
                {ADMIN_TEXT.CONTESTS.OVERVIEW_RATING_DESCRIPTION}
              </CardDescription>
            </CardHeader>
            <CardContent className="px-0">
              <ContestRatingTable ratingChanges={ratingChanges} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function Metric({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  return (
    <Card className="overflow-hidden">
      <CardContent className="flex items-center gap-4 pt-6">
        <div className="flex size-11 shrink-0 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            {label}
          </p>
          <p className="mt-1 truncate text-xl font-bold tabular-nums">
            {value}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border bg-muted/20 p-4">
      <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {label}
      </p>
      <p className="mt-2 font-medium text-foreground">{value}</p>
    </div>
  );
}
