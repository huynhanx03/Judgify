"use client";

import { useMemo } from "react";
import {
  Activity,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  Clock3,
  Cpu,
  RefreshCw,
  ServerCog,
  ShieldCheck,
  TimerReset,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { formatDateTime, formatDuration, formatNumber } from "@/lib/format";
import { getErrorMessage } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { judgeOperationsService } from "@/services/judge-operations.service";
import type {
  JudgeActivationSnapshot,
  JudgeOperationsOverview,
  JudgeWorkerSnapshot,
} from "@/types/judge-operations";

type LoadState = "loading" | "refreshing" | "ready" | "error";

function shortIdentifier(value: string): string {
  return value.length <= 22 ? value : `${value.slice(0, 12)}…${value.slice(-7)}`;
}

function timestamp(value: string): number {
  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : 0;
}

function workerState(
  worker: JudgeWorkerSnapshot,
  overview: JudgeOperationsOverview,
): "fresh" | "draining" | "stale" {
  const age = timestamp(overview.generated_at) - timestamp(worker.heartbeat_at);
  if (age < 0 || age > overview.worker_heartbeat_freshness_ms) return "stale";
  return worker.claims_enabled ? "fresh" : "draining";
}

function MetricCard({
  label,
  value,
  icon: Icon,
  tone = "primary",
}: {
  label: string;
  value: number;
  icon: LucideIcon;
  tone?: "primary" | "warning" | "danger";
}) {
  return (
    <Card className="relative min-h-32 overflow-hidden border-border">
      <div
        className={cn(
          "absolute inset-x-0 top-0 h-0.5",
          tone === "danger"
            ? "bg-destructive"
            : tone === "warning"
              ? "bg-warning"
              : "bg-primary",
        )}
      />
      <CardContent className="flex h-full items-center justify-between gap-4 pt-3">
        <div>
          <p className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            {label}
          </p>
          <p className="mt-2 text-3xl font-bold tabular-nums text-foreground">
            {formatNumber(value)}
          </p>
        </div>
        <div className="flex size-11 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </div>
      </CardContent>
    </Card>
  );
}

function OperationsSkeleton() {
  return (
    <div
      className="space-y-5"
      role="status"
      aria-live="polite"
      aria-label={TEXT.COMMON.LOADING}
    >
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((item) => (
          <div key={item} className="h-32 animate-pulse rounded-xl bg-muted/50 motion-reduce:animate-none" />
        ))}
      </div>
      {[0, 1].map((item) => (
        <div key={item} className="h-64 animate-pulse rounded-xl bg-muted/50 motion-reduce:animate-none" />
      ))}
    </div>
  );
}

function WorkerStatus({
  worker,
  overview,
}: {
  worker: JudgeWorkerSnapshot;
  overview: JudgeOperationsOverview;
}) {
  const state = workerState(worker, overview);
  return (
    <Badge
      variant={state === "stale" ? "destructive" : state === "fresh" ? "default" : "secondary"}
    >
      {state === "stale"
        ? ADMIN_TEXT.JUDGE.STALE
        : state === "fresh"
          ? ADMIN_TEXT.JUDGE.FRESH
          : ADMIN_TEXT.JUDGE.DRAINING}
    </Badge>
  );
}

function Capacity({ worker }: { worker: JudgeWorkerSnapshot }) {
  const used = worker.total_capacity - worker.free_capacity;
  const percentage = Math.max(0, Math.min(100, (used / worker.total_capacity) * 100));
  return (
    <div className="min-w-36 space-y-2">
      <div className="flex items-center justify-between gap-3 text-xs tabular-nums">
        <span>{ADMIN_TEXT.JUDGE.FREE_OF_TOTAL(worker.free_capacity, worker.total_capacity)}</span>
        <span className="text-muted-foreground">{Math.round(percentage)}%</span>
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={worker.total_capacity}
        aria-valuenow={used}
        aria-label={ADMIN_TEXT.JUDGE.CAPACITY}
      >
        <div className="h-full rounded-full bg-primary transition-[width] duration-300 motion-reduce:transition-none" style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}

function Workers({ overview }: { overview: JudgeOperationsOverview }) {
  return (
    <Card className="border-border">
      <CardHeader className="border-b border-border bg-muted/20">
        <CardTitle className="flex items-center gap-2">
          <ServerCog className="size-4 text-primary" aria-hidden="true" />
          {ADMIN_TEXT.JUDGE.WORKERS_TITLE}
        </CardTitle>
        <CardDescription>{ADMIN_TEXT.JUDGE.WORKERS_DESCRIPTION}</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        {overview.workers.length === 0 ? (
          <div className="flex min-h-44 flex-col items-center justify-center p-6 text-center">
            <ServerCog className="size-6 text-muted-foreground" aria-hidden="true" />
            <p className="mt-3 font-medium">{ADMIN_TEXT.JUDGE.WORKERS_EMPTY_TITLE}</p>
            <p className="mt-1 max-w-lg text-sm text-muted-foreground">{ADMIN_TEXT.JUDGE.WORKERS_EMPTY_DESCRIPTION}</p>
          </div>
        ) : (
          <Table aria-label={ADMIN_TEXT.JUDGE.WORKERS_TITLE}>
            <TableHeader>
              <TableRow>
                <TableHead>{ADMIN_TEXT.JUDGE.WORKER}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.HEARTBEAT}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.CAPACITY}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.EVIDENCE}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {overview.workers.map((worker) => (
                <TableRow key={worker.id}>
                  <TableCell>
                    <div className="max-w-72">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-semibold">{worker.worker_key}</span>
                        <WorkerStatus worker={worker} overview={overview} />
                      </div>
                      <p className="mt-1 truncate font-mono text-xs text-muted-foreground" title={worker.boot_id}>
                        {ADMIN_TEXT.JUDGE.BOOT}: {shortIdentifier(worker.boot_id)}
                      </p>
                    </div>
                  </TableCell>
                  <TableCell>
                    <time dateTime={worker.heartbeat_at} className="text-xs tabular-nums">
                      {formatDateTime(worker.heartbeat_at)}
                    </time>
                  </TableCell>
                  <TableCell><Capacity worker={worker} /></TableCell>
                  <TableCell>
                    <p className="text-xs tabular-nums">{ADMIN_TEXT.JUDGE.EVIDENCE_COUNT(worker.evidence_count)}</p>
                    <p className="mt-1 text-xs tabular-nums text-muted-foreground">
                      {worker.latest_probed_at ? formatDateTime(worker.latest_probed_at) : ADMIN_TEXT.JUDGE.NO_PROBE}
                    </p>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function ActivationIdentity({ activation }: { activation: JudgeActivationSnapshot }) {
  return (
    <div className="max-w-80">
      <div className="flex items-center gap-2">
        <span className="truncate font-semibold">{activation.runtime_key}</span>
        <Badge variant={activation.state === "active" ? "default" : "outline"}>
          {activation.state === "active" ? ADMIN_TEXT.JUDGE.ACTIVE : ADMIN_TEXT.JUDGE.RETIRED}
        </Badge>
      </div>
      <p className="mt-1 truncate font-mono text-xs text-muted-foreground" title={activation.capability_profile_hash}>
        {shortIdentifier(activation.capability_profile_hash)}
      </p>
    </div>
  );
}

function Activations({ overview }: { overview: JudgeOperationsOverview }) {
  return (
    <Card className="border-border">
      <CardHeader className="border-b border-border bg-muted/20">
        <CardTitle className="flex items-center gap-2">
          <ShieldCheck className="size-4 text-primary" aria-hidden="true" />
          {ADMIN_TEXT.JUDGE.ACTIVATIONS_TITLE}
        </CardTitle>
        <CardDescription>{ADMIN_TEXT.JUDGE.ACTIVATIONS_DESCRIPTION}</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        {overview.activations.length === 0 ? (
          <div className="flex min-h-44 flex-col items-center justify-center p-6 text-center">
            <ShieldCheck className="size-6 text-muted-foreground" aria-hidden="true" />
            <p className="mt-3 font-medium">{ADMIN_TEXT.JUDGE.ACTIVATIONS_EMPTY_TITLE}</p>
            <p className="mt-1 max-w-lg text-sm text-muted-foreground">{ADMIN_TEXT.JUDGE.ACTIVATIONS_EMPTY_DESCRIPTION}</p>
          </div>
        ) : (
          <Table aria-label={ADMIN_TEXT.JUDGE.ACTIVATIONS_TITLE}>
            <TableHeader>
              <TableRow>
                <TableHead>{ADMIN_TEXT.JUDGE.RUNTIME}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.CHECKER}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.RELEASE}</TableHead>
                <TableHead>{ADMIN_TEXT.JUDGE.ACTIVATED_AT}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {overview.activations.map((activation) => (
                <TableRow key={activation.id}>
                  <TableCell><ActivationIdentity activation={activation} /></TableCell>
                  <TableCell>
                    <p className="font-medium">{activation.checker_key}</p>
                    <p className="text-xs text-muted-foreground">{activation.checker_version}</p>
                  </TableCell>
                  <TableCell>
                    <p className="max-w-64 truncate font-mono text-xs" title={activation.release_id}>{activation.release_id}</p>
                    <p className="mt-1 text-xs text-muted-foreground">{activation.runtime_profile_version}</p>
                  </TableCell>
                  <TableCell>
                    <time dateTime={activation.activated_at} className="text-xs tabular-nums">{formatDateTime(activation.activated_at)}</time>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

export function AdminJudgeOperations() {
  const { authorizationRevision } = useAuth();
  const overviewResource = useRetryableResource<
    JudgeOperationsOverview | null
  >({
    resetKey: authorizationRevision,
    initialData: null,
    keepPreviousData: true,
    load: judgeOperationsService.getOverview,
  });
  const overview = overviewResource.isPreviousData
    ? null
    : overviewResource.data;
  const state: LoadState =
    overviewResource.status === "loading"
      ? overview
        ? "refreshing"
        : "loading"
      : overviewResource.status === "error" && !overview
        ? "error"
        : "ready";
  const error =
    overviewResource.status === "error"
      ? getErrorMessage(
          overviewResource.error,
          ADMIN_TEXT.JUDGE.LOAD_ERROR_DESCRIPTION,
        )
      : null;

  const metrics = useMemo(() => {
    if (!overview) return null;
    const actionable = overview.queue.ready + overview.queue.leased + overview.queue.retry_wait;
    const healthyWorkers = overview.workers.filter((worker) => workerState(worker, overview) === "fresh").length;
    return {
      actionable,
      healthyWorkers,
      activeActivations: overview.activations.filter(({ state }) => state === "active").length,
      totalCapacity: overview.workers.reduce((total, worker) => total + worker.total_capacity, 0),
    };
  }, [overview]);

  const refresh = overviewResource.retry;
  const refreshing = state === "refreshing";

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary shadow-sm shadow-primary/5">
            <ServerCog className="size-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{ADMIN_TEXT.JUDGE.TITLE}</h1>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">{ADMIN_TEXT.JUDGE.SUBTITLE}</p>
          </div>
        </div>
        <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center">
          {overview ? (
            <span className="text-xs tabular-nums text-muted-foreground">{ADMIN_TEXT.JUDGE.UPDATED_AT(formatDateTime(overview.generated_at))}</span>
          ) : null}
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            onClick={refresh}
            disabled={overviewResource.status === "loading"}
          >
            <RefreshCw className={cn("size-4", refreshing && "animate-spin motion-reduce:animate-none")} aria-hidden="true" />
            {refreshing ? ADMIN_TEXT.JUDGE.REFRESHING : ADMIN_TEXT.JUDGE.REFRESH}
          </Button>
        </div>
      </header>

      <div className="flex items-start gap-3 rounded-2xl border border-primary/15 bg-primary/[0.06] p-4">
        <ShieldCheck className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />
        <div>
          <p className="font-semibold text-foreground">{ADMIN_TEXT.JUDGE.SAFE_READ_TITLE}</p>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">{ADMIN_TEXT.JUDGE.SAFE_READ_DESCRIPTION}</p>
        </div>
      </div>

      {state === "loading" ? <OperationsSkeleton /> : state === "error" || !overview || !metrics ? (
        <Card className="border-destructive/30">
          <CardContent className="flex min-h-64 flex-col items-center justify-center p-8 text-center" role="alert">
            <AlertTriangle className="size-7 text-destructive" aria-hidden="true" />
            <h2 className="mt-4 text-lg font-semibold">{ADMIN_TEXT.JUDGE.LOAD_ERROR_TITLE}</h2>
            <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">{error ?? ADMIN_TEXT.JUDGE.LOAD_ERROR_DESCRIPTION}</p>
            <Button type="button" variant="outline" className="mt-5 min-h-11" onClick={refresh}>
              <RefreshCw aria-hidden="true" />{TEXT.COMMON.RETRY}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div
          className="space-y-6"
          aria-label={ADMIN_TEXT.JUDGE.SNAPSHOT_LABEL}
          aria-busy={refreshing}
        >
          {error ? (
            <div
              className="flex items-start gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4"
              role="alert"
            >
              <AlertTriangle
                className="mt-0.5 size-5 shrink-0 text-destructive"
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-semibold">
                  {ADMIN_TEXT.JUDGE.LOAD_ERROR_TITLE}
                </p>
                <p className="mt-1 text-sm leading-6 text-muted-foreground">
                  {error}
                </p>
              </div>
              <Button type="button" variant="outline" onClick={refresh}>
                <RefreshCw className="size-4" aria-hidden="true" />
                <span className="sr-only">{TEXT.COMMON.RETRY}</span>
              </Button>
            </div>
          ) : null}
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard label={ADMIN_TEXT.JUDGE.ACTIONABLE_JOBS} value={metrics.actionable} icon={Activity} tone={overview.queue.dead > 0 ? "danger" : metrics.actionable > 0 ? "warning" : "primary"} />
            <MetricCard label={ADMIN_TEXT.JUDGE.HEALTHY_WORKERS} value={metrics.healthyWorkers} icon={CheckCircle2} tone={overview.workers.length > 0 && metrics.healthyWorkers === 0 ? "danger" : "primary"} />
            <MetricCard label={ADMIN_TEXT.JUDGE.TOTAL_CAPACITY} value={metrics.totalCapacity} icon={Cpu} />
            <MetricCard label={ADMIN_TEXT.JUDGE.ACTIVE_ACTIVATIONS} value={metrics.activeActivations} icon={Boxes} tone={metrics.activeActivations === 0 ? "warning" : "primary"} />
          </div>

          <Card className="border-border">
            <CardHeader className="border-b border-border bg-muted/20">
              <CardTitle className="flex items-center gap-2"><TimerReset className="size-4 text-primary" aria-hidden="true" />{ADMIN_TEXT.JUDGE.QUEUE_TITLE}</CardTitle>
              <CardDescription>{ADMIN_TEXT.JUDGE.QUEUE_DESCRIPTION}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
                {[
                  [ADMIN_TEXT.JUDGE.READY, overview.queue.ready],
                  [ADMIN_TEXT.JUDGE.LEASED, overview.queue.leased],
                  [ADMIN_TEXT.JUDGE.RETRY_WAIT, overview.queue.retry_wait],
                  [ADMIN_TEXT.JUDGE.DONE, overview.queue.done],
                  [ADMIN_TEXT.JUDGE.CANCELLED, overview.queue.cancelled],
                  [ADMIN_TEXT.JUDGE.DEAD, overview.queue.dead],
                ].map(([label, value]) => (
                  <div key={String(label)} className="rounded-xl border border-border bg-background p-3">
                    <p className="text-xs text-muted-foreground">{label}</p>
                    <p className="mt-1 text-xl font-bold tabular-nums">{formatNumber(Number(value))}</p>
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Clock3 className="size-4" aria-hidden="true" />
                <span>{ADMIN_TEXT.JUDGE.OLDEST_READY}:</span>
                <strong className="font-semibold text-foreground">
                  {overview.queue.oldest_ready_at
                    ? formatDuration(Math.max(0, timestamp(overview.generated_at) - timestamp(overview.queue.oldest_ready_at)))
                    : ADMIN_TEXT.JUDGE.NO_WAITING_JOB}
                </strong>
              </div>
            </CardContent>
          </Card>

          <Workers overview={overview} />
          <Activations overview={overview} />
        </div>
      )}
    </div>
  );
}
