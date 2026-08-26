"use client";

import {
  type FormEvent,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Filter,
  Inbox,
  LockKeyhole,
  RefreshCw,
  RotateCcw,
  Search,
  TimerReset,
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PaginationControls } from "@/components/pagination-controls";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  ADMIN_SUBMISSION_PAGE_SIZE,
  SUBMISSION_STATUSES,
} from "@/constants/submission";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  ADMIN_SUBMISSION_FILTER_ALL,
  createEmptyAdminSubmissionFilters,
  normalizeAdminSubmissionFilters,
  toAdminSubmissionFindRequest,
  type AdminSubmissionFilterDraft,
  type AdminSubmissionFilters,
  type AdminSubmissionIdentifierField,
} from "@/lib/submissions/admin-query";
import { formatDateTime, formatDuration, formatMemory } from "@/lib/format";
import { getErrorMessage } from "@/lib/toast";
import { STATUS_CONFIG } from "@/modules/problem/submission-detail";
import { submissionService } from "@/services/submission.service";
import type { Paginated } from "@/types/api";
import type {
  AdminSubmissionSummary,
  JudgeRuntime,
} from "@/types/submission";
import { AdminSubmissionDetailDialog } from "./admin-submission-detail-dialog";

type LoadState = "loading" | "refreshing" | "ready" | "error";

interface SummaryMetric {
  label: string;
  value: number;
  icon: typeof Clock3;
  className: string;
}

const identifierLabels: Record<AdminSubmissionIdentifierField, string> = {
  problemId: ADMIN_TEXT.SUBMISSIONS.PROBLEM_ID,
  userId: ADMIN_TEXT.SUBMISSIONS.USER_ID,
  contestId: ADMIN_TEXT.SUBMISSIONS.CONTEST_ID,
};

function shortIdentifier(value: string): string {
  return value.length <= 12 ? value : `${value.slice(0, 8)}…${value.slice(-4)}`;
}

function SubmissionStatusBadge({
  status,
}: Pick<AdminSubmissionSummary, "status">) {
  const config = STATUS_CONFIG[status];
  return (
    <Badge
      variant="outline"
      className={`${config.className} motion-reduce:animate-none`}
    >
      {config.label}
    </Badge>
  );
}

function SubmissionContext({ submission }: { submission: AdminSubmissionSummary }) {
  return (
    <dl className="grid gap-1 text-xs text-muted-foreground">
      <div className="flex items-center gap-2">
        <dt>{ADMIN_TEXT.SUBMISSIONS.PROBLEM}</dt>
        <dd className="font-mono text-foreground" title={submission.problem_id}>
          {shortIdentifier(submission.problem_id)}
        </dd>
      </div>
      <div className="flex items-center gap-2">
        <dt>{ADMIN_TEXT.SUBMISSIONS.USER}</dt>
        <dd className="font-mono text-foreground" title={submission.user_id}>
          {shortIdentifier(submission.user_id)}
        </dd>
      </div>
      <div className="flex items-center gap-2">
        <dt>{ADMIN_TEXT.SUBMISSIONS.CONTEST}</dt>
        <dd className="font-mono text-foreground" title={submission.contest_id}>
          {submission.contest_id
            ? shortIdentifier(submission.contest_id)
            : ADMIN_TEXT.SUBMISSIONS.NO_CONTEST}
        </dd>
      </div>
    </dl>
  );
}

function SubmissionResources({ submission }: { submission: AdminSubmissionSummary }) {
  return (
    <div className="grid gap-1 text-xs tabular-nums text-muted-foreground">
      <span>
        {submission.time_ms == null ? TEXT.COMMON.NOT_AVAILABLE : formatDuration(submission.time_ms)}
      </span>
      <span>
        {submission.memory_kb == null
          ? TEXT.COMMON.NOT_AVAILABLE
          : formatMemory(submission.memory_kb)}
      </span>
    </div>
  );
}

function MonitorSkeleton() {
  return (
    <div
      className="space-y-3"
      role="status"
      aria-live="polite"
      aria-label={TEXT.COMMON.LOADING}
    >
      {[0, 1, 2, 3].map((item) => (
        <div
          key={item}
          className="h-20 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none"
        />
      ))}
    </div>
  );
}

export function AdminSubmissionMonitor() {
  const { authorizationRevision, can } = useAuth();
  const canInspect = can(
    AUTHORIZATION_RESOURCE.SUBMISSION,
    AUTHORIZATION_ACTION.INSPECT,
  );
  const canUpdate = can(
    AUTHORIZATION_RESOURCE.SUBMISSION,
    AUTHORIZATION_ACTION.UPDATE,
  );
  const [draftFilters, setDraftFilters] = useState<AdminSubmissionFilterDraft>(
    createEmptyAdminSubmissionFilters,
  );
  const [appliedFilters, setAppliedFilters] = useState<AdminSubmissionFilters>(
    {},
  );
  const [invalidFields, setInvalidFields] = useState<
    AdminSubmissionIdentifierField[]
  >([]);
  const [page, setPage] = useState(1);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);
  const [selectedSubmissionId, setSelectedSubmissionId] = useState<
    string | null
  >(null);

  const query = useMemo(
    () =>
      toAdminSubmissionFindRequest(
        appliedFilters,
        page,
        ADMIN_SUBMISSION_PAGE_SIZE,
      ),
    [appliedFilters, page],
  );
  const resultResource = useRetryableResource<
    Paginated<AdminSubmissionSummary> | null
  >({
    resetKey: `${authorizationRevision}:${JSON.stringify(query)}`,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => submissionService.findAdmin(query, signal),
    onSuccess: () => setLastUpdatedAt(new Date().toISOString()),
  });
  const runtimeResource = useRetryableResource<readonly JudgeRuntime[]>({
    resetKey: "judge-runtime-catalog",
    initialData: [],
    keepPreviousData: true,
    load: async (signal) =>
      (await submissionService.getRuntimeCatalog(signal)).runtimes,
  });
  const result = resultResource.isPreviousData ? null : resultResource.data;
  const runtimes = runtimeResource.data;
  const runtimeError = runtimeResource.status === "error";
  const loadError =
    resultResource.status === "error"
      ? getErrorMessage(
          resultResource.error,
          ADMIN_TEXT.SUBMISSIONS.LOAD_ERROR_DESCRIPTION,
        )
      : null;
  const loadState: LoadState =
    resultResource.status === "loading"
      ? result
        ? "refreshing"
        : "loading"
      : resultResource.status === "error" && !result
        ? "error"
        : "ready";

  useEffect(() => {
    if (!canInspect) setSelectedSubmissionId(null);
  }, [canInspect]);

  const records = useMemo(() => result?.records ?? [], [result]);
  const metrics = useMemo<SummaryMetric[]>(() => {
    const queued = records.filter(({ status }) => status === "pending").length;
    const inProgress = records.filter(
      ({ status }) => status === "judging",
    ).length;
    return [
      {
        label: ADMIN_TEXT.SUBMISSIONS.TOTAL_ON_PAGE,
        value: records.length,
        icon: Activity,
        className: "bg-primary/10 text-primary",
      },
      {
        label: ADMIN_TEXT.SUBMISSIONS.QUEUED,
        value: queued,
        icon: Clock3,
        className: "bg-warning/10 text-warning",
      },
      {
        label: ADMIN_TEXT.SUBMISSIONS.IN_PROGRESS,
        value: inProgress,
        icon: TimerReset,
        className: "bg-info/10 text-info",
      },
      {
        label: ADMIN_TEXT.SUBMISSIONS.FINISHED,
        value: records.length - queued - inProgress,
        icon: CheckCircle2,
        className: "bg-success/10 text-success",
      },
    ];
  }, [records]);

  function updateDraftFilter<K extends keyof AdminSubmissionFilterDraft>(
    key: K,
    value: AdminSubmissionFilterDraft[K],
  ) {
    setDraftFilters((current) => ({ ...current, [key]: value }));
    if (key === "problemId" || key === "userId" || key === "contestId") {
      setInvalidFields((current) => current.filter((field) => field !== key));
    }
  }

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeAdminSubmissionFilters(draftFilters);
    if (!normalized.ok) {
      setInvalidFields(normalized.invalidFields);
      return;
    }
    setInvalidFields([]);
    setAppliedFilters(normalized.filters);
    setPage(1);
    resultResource.retry();
  }

  function resetFilters() {
    setDraftFilters(createEmptyAdminSubmissionFilters());
    setInvalidFields([]);
    setAppliedFilters({});
    setPage(1);
    resultResource.retry();
  }

  const isRefreshing = loadState === "refreshing";

  return (
    <div className="mx-auto max-w-[1600px] space-y-6">
      <section className="relative overflow-hidden rounded-2xl border border-border bg-card/80 p-5 shadow-sm sm:p-6">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/60 to-transparent" />
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary ring-4 ring-primary/5">
              <Activity className="size-5" aria-hidden="true" />
            </span>
            <div>
              <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
                {ADMIN_TEXT.SUBMISSIONS.TITLE}
              </h1>
              <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">
                {ADMIN_TEXT.SUBMISSIONS.SUBTITLE}
              </p>
              <p
                className="mt-2 min-h-5 text-xs text-muted-foreground"
                aria-live="polite"
              >
                {isRefreshing
                  ? ADMIN_TEXT.SUBMISSIONS.REFRESHING
                  : lastUpdatedAt
                    ? ADMIN_TEXT.SUBMISSIONS.UPDATED_AT(
                        formatDateTime(lastUpdatedAt),
                      )
                    : null}
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            className="min-h-11 cursor-pointer gap-2 self-stretch sm:self-auto"
            disabled={loadState === "loading" || isRefreshing}
            onClick={resultResource.retry}
          >
            <RefreshCw
              className={`size-4 ${isRefreshing ? "animate-spin motion-reduce:animate-none" : ""}`}
              aria-hidden="true"
            />
            {isRefreshing
              ? ADMIN_TEXT.SUBMISSIONS.REFRESHING
              : ADMIN_TEXT.SUBMISSIONS.REFRESH}
          </Button>
        </div>
      </section>

      <section aria-labelledby="submission-summary-heading">
        <h2 id="submission-summary-heading" className="sr-only">
          {ADMIN_TEXT.SUBMISSIONS.CURRENT_PAGE}
        </h2>
        <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
          {metrics.map((metric) => (
            <Card key={metric.label} size="sm" className="bg-card/70 shadow-sm">
              <CardContent className="flex items-center gap-3">
                <span
                  className={`flex size-9 items-center justify-center rounded-xl ${metric.className}`}
                >
                  <metric.icon className="size-4" aria-hidden="true" />
                </span>
                <div>
                  <p className="text-xl font-bold tabular-nums sm:text-2xl">
                    {result ? metric.value : TEXT.COMMON.NOT_AVAILABLE}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {metric.label}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>

      <Card className="bg-card/70 shadow-sm">
        <CardHeader className="border-b border-border">
          <div className="flex items-start gap-3">
            <span className="flex size-9 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <Filter className="size-4" aria-hidden="true" />
            </span>
            <div>
              <CardTitle>{ADMIN_TEXT.SUBMISSIONS.FILTERS_TITLE}</CardTitle>
              <CardDescription className="mt-1">
                {ADMIN_TEXT.SUBMISSIONS.FILTERS_DESCRIPTION}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={applyFilters} noValidate>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
              <div className="space-y-2">
                <Label htmlFor="submission-status-filter">
                  {ADMIN_TEXT.SUBMISSIONS.STATUS}
                </Label>
                <Select
                  value={draftFilters.status}
                  onValueChange={(value) =>
                    updateDraftFilter(
                      "status",
                      value as AdminSubmissionFilterDraft["status"],
                    )
                  }
                >
                  <SelectTrigger
                    id="submission-status-filter"
                    className="min-h-11 cursor-pointer"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={ADMIN_SUBMISSION_FILTER_ALL}>
                      {ADMIN_TEXT.SUBMISSIONS.ALL_STATUSES}
                    </SelectItem>
                    {SUBMISSION_STATUSES.map((status) => (
                      <SelectItem key={status} value={status}>
                        {STATUS_CONFIG[status].label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="submission-language-filter">
                  {ADMIN_TEXT.SUBMISSIONS.LANGUAGE}
                </Label>
                <Select
                  value={draftFilters.language}
                  onValueChange={(value) =>
                    updateDraftFilter("language", value)
                  }
                  disabled={runtimes.length === 0}
                >
                  <SelectTrigger
                    id="submission-language-filter"
                    className="min-h-11 cursor-pointer"
                  >
                    <SelectValue
                      placeholder={ADMIN_TEXT.SUBMISSIONS.RUNTIME_LOADING}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={ADMIN_SUBMISSION_FILTER_ALL}>
                      {ADMIN_TEXT.SUBMISSIONS.ALL_LANGUAGES}
                    </SelectItem>
                    {[...new Map(
                      runtimes.map((runtime) => [
                        runtime.language,
                        runtime,
                      ]),
                    ).values()].map((runtime) => (
                      <SelectItem
                        key={runtime.language}
                        value={runtime.language}
                      >
                        {runtime.display_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {runtimeError ? (
                  <button
                    type="button"
                    className="min-h-11 cursor-pointer text-left text-xs text-destructive underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={runtimeResource.retry}
                  >
                    {ADMIN_TEXT.SUBMISSIONS.RUNTIME_UNAVAILABLE}
                  </button>
                ) : null}
              </div>

              {(
                ["problemId", "userId", "contestId"] as const satisfies readonly AdminSubmissionIdentifierField[]
              ).map((field) => {
                const isInvalid = invalidFields.includes(field);
                const inputId = `submission-${field}-filter`;
                return (
                  <div key={field} className="space-y-2">
                    <Label htmlFor={inputId}>{identifierLabels[field]}</Label>
                    <Input
                      id={inputId}
                      value={draftFilters[field]}
                      onChange={(event) =>
                        updateDraftFilter(field, event.target.value)
                      }
                      placeholder={ADMIN_TEXT.SUBMISSIONS.UUID_PLACEHOLDER}
                      aria-invalid={isInvalid}
                      aria-describedby={
                        isInvalid ? `${inputId}-error` : undefined
                      }
                      className="min-h-11 font-mono text-xs"
                      autoComplete="off"
                    />
                    {isInvalid ? (
                      <p
                        id={`${inputId}-error`}
                        className="text-xs text-destructive"
                      >
                        {ADMIN_TEXT.SUBMISSIONS.UUID_INVALID}
                      </p>
                    ) : null}
                  </div>
                );
              })}
            </div>

            <div className="flex flex-col-reverse gap-2 border-t border-border pt-4 sm:flex-row sm:justify-end">
              <Button
                type="button"
                variant="ghost"
                className="min-h-11 cursor-pointer gap-2"
                onClick={resetFilters}
              >
                <RotateCcw className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.SUBMISSIONS.RESET_FILTERS}
              </Button>
              <Button
                type="submit"
                className="min-h-11 cursor-pointer gap-2"
              >
                <Search className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.SUBMISSIONS.APPLY_FILTERS}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {!canInspect ? (
        <div
          className="flex items-start gap-3 rounded-xl border border-border bg-muted/30 p-4 text-sm text-muted-foreground"
          role="status"
        >
          <LockKeyhole className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <p>{ADMIN_TEXT.SUBMISSIONS.INSPECT_RESTRICTED}</p>
        </div>
      ) : null}

      {loadError && result ? (
        <div
          className="flex items-start gap-3 rounded-xl border border-destructive/20 bg-destructive/5 p-4"
          role="alert"
        >
          <AlertTriangle
            className="mt-0.5 size-5 shrink-0 text-destructive"
            aria-hidden="true"
          />
          <div className="min-w-0 flex-1">
            <h2 className="font-semibold">
              {ADMIN_TEXT.SUBMISSIONS.LOAD_ERROR_TITLE}
            </h2>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">
              {loadError}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            className="min-h-11 cursor-pointer gap-2"
            onClick={resultResource.retry}
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            <span className="hidden sm:inline">{TEXT.COMMON.RETRY}</span>
          </Button>
        </div>
      ) : null}

      {loadState === "loading" ? <MonitorSkeleton /> : null}

      {loadState === "error" && !result ? (
        <Card
          className="border-destructive/20 bg-card/70 py-12 text-center"
          role="alert"
        >
          <CardContent>
            <AlertTriangle
              className="mx-auto size-9 text-destructive"
              aria-hidden="true"
            />
            <h2 className="mt-4 font-semibold">
              {ADMIN_TEXT.SUBMISSIONS.LOAD_ERROR_TITLE}
            </h2>
            <p className="mx-auto mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.SUBMISSIONS.LOAD_ERROR_DESCRIPTION}
            </p>
          </CardContent>
        </Card>
      ) : null}

      {result && records.length === 0 ? (
        <Card className="border-dashed bg-card/50 py-12 text-center">
          <CardContent>
            <Inbox
              className="mx-auto size-10 text-muted-foreground"
              aria-hidden="true"
            />
            <h2 className="mt-4 font-semibold">
              {ADMIN_TEXT.SUBMISSIONS.EMPTY_TITLE}
            </h2>
            <p className="mx-auto mt-2 max-w-lg text-sm text-muted-foreground">
              {ADMIN_TEXT.SUBMISSIONS.EMPTY_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-5 min-h-11 cursor-pointer gap-2"
              onClick={resetFilters}
            >
              <RotateCcw className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.SUBMISSIONS.RESET_FILTERS}
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {result && records.length > 0 ? (
        <section className="space-y-4" aria-busy={isRefreshing}>
          <div className="space-y-3 md:hidden">
            {records.map((submission) => (
              <Card key={submission.id} size="sm" className="bg-card/80 shadow-sm">
                <CardHeader className="border-b border-border">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <CardTitle className="truncate font-mono text-xs">
                        {submission.id}
                      </CardTitle>
                      <CardDescription className="mt-1">
                        {formatDateTime(submission.created_at)}
                      </CardDescription>
                    </div>
                    <SubmissionStatusBadge status={submission.status} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-3 gap-2 rounded-xl bg-muted/30 p-3 text-center text-xs">
                    <div>
                      <p className="text-muted-foreground">
                        {ADMIN_TEXT.SUBMISSIONS.COLUMN_RUNTIME}
                      </p>
                      <p className="mt-1 font-mono font-semibold">
                        {submission.language}
                      </p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">
                        {ADMIN_TEXT.SUBMISSIONS.COLUMN_PROGRESS}
                      </p>
                      <p className="mt-1 font-semibold tabular-nums">
                        {submission.passed_count}/{submission.total_count}
                      </p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">
                        {ADMIN_TEXT.SUBMISSIONS.COLUMN_RESOURCES}
                      </p>
                      <div className="mt-1">
                        <SubmissionResources submission={submission} />
                      </div>
                    </div>
                  </div>
                  <SubmissionContext submission={submission} />
                  {canInspect ? (
                    <Button
                      type="button"
                      variant="outline"
                      className="min-h-11 w-full cursor-pointer gap-2"
                      onClick={() => setSelectedSubmissionId(submission.id)}
                      aria-label={ADMIN_TEXT.SUBMISSIONS.INSPECT_ARIA(
                        submission.id,
                      )}
                    >
                      <Search className="size-4" aria-hidden="true" />
                      {ADMIN_TEXT.SUBMISSIONS.INSPECT}
                    </Button>
                  ) : null}
                </CardContent>
              </Card>
            ))}
          </div>

          <div className="hidden overflow-hidden rounded-xl border border-border bg-card/70 shadow-sm md:block">
            <Table>
              <TableCaption className="sr-only">
                {ADMIN_TEXT.SUBMISSIONS.TABLE_LABEL}
              </TableCaption>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="pl-4">
                    {ADMIN_TEXT.SUBMISSIONS.COLUMN_SUBMISSION}
                  </TableHead>
                  <TableHead>{ADMIN_TEXT.SUBMISSIONS.COLUMN_STATUS}</TableHead>
                  <TableHead>{ADMIN_TEXT.SUBMISSIONS.COLUMN_RUNTIME}</TableHead>
                  <TableHead>{ADMIN_TEXT.SUBMISSIONS.COLUMN_PROGRESS}</TableHead>
                  <TableHead>{ADMIN_TEXT.SUBMISSIONS.COLUMN_RESOURCES}</TableHead>
                  <TableHead>{ADMIN_TEXT.SUBMISSIONS.COLUMN_CONTEXT}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {records.map((submission) => (
                  <TableRow key={submission.id}>
                    <TableCell className="pl-4">
                      <div className="grid gap-1">
                        <span className="font-mono text-xs font-medium">
                          {shortIdentifier(submission.id)}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {formatDateTime(submission.created_at)}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <SubmissionStatusBadge status={submission.status} />
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {submission.language}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {submission.passed_count}/{submission.total_count}
                    </TableCell>
                    <TableCell>
                      <SubmissionResources submission={submission} />
                    </TableCell>
                    <TableCell>
                      <SubmissionContext submission={submission} />
                    </TableCell>
                    <TableCell className="pr-4 text-right">
                      {canInspect ? (
                        <Button
                          type="button"
                          variant="ghost"
                          className="min-h-11 cursor-pointer gap-2"
                          onClick={() => setSelectedSubmissionId(submission.id)}
                          aria-label={ADMIN_TEXT.SUBMISSIONS.INSPECT_ARIA(
                            submission.id,
                          )}
                        >
                          <Search className="size-4" aria-hidden="true" />
                          {ADMIN_TEXT.SUBMISSIONS.INSPECT}
                        </Button>
                      ) : (
                        <LockKeyhole
                          className="ml-auto size-4 text-muted-foreground"
                          aria-label={ADMIN_TEXT.SUBMISSIONS.INSPECT_RESTRICTED}
                        />
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <PaginationControls
            pagination={result.pagination}
            onPageChange={setPage}
          />
        </section>
      ) : null}

      <AdminSubmissionDetailDialog
        submissionId={selectedSubmissionId}
        canUpdate={canUpdate}
        onOperationSuccess={resultResource.retry}
        onOpenChange={(open) => {
          if (!open) setSelectedSubmissionId(null);
        }}
      />
    </div>
  );
}
