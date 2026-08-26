"use client";

import { type FormEvent, useMemo, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  CirclePause,
  Eye,
  Filter,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  TimerReset,
  Workflow,
  XCircle,
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
} from "@/constants/authorization";
import {
  OPERATION_KIND_MAXIMUM_LENGTH,
  OPERATION_KIND_PATTERN,
  OPERATION_PAGE_SIZE,
  OPERATION_STATUS,
} from "@/constants/operation";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { formatDateTime, formatNumber } from "@/lib/format";
import {
  operationControlLabel,
  operationKindLabel,
} from "@/lib/operations/presentation";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { operationService } from "@/services/operation.service";
import type {
  Operation,
  OperationControlAction,
  OperationListQuery,
  OperationPage,
  OperationStatus,
} from "@/types/operation";
import { OperationControlDialog } from "./operation-control-dialog";
import { OperationDetailSheet } from "./operation-detail-sheet";
import { OperationProgress } from "./operation-progress";
import { OperationStatusBadge } from "./operation-status";

type LoadState = "loading" | "refreshing" | "ready" | "error";
type StatusFilter = OperationStatus | "all";

interface AppliedFilters {
  status: StatusFilter;
  kind?: string;
}

interface ControlIntent {
  operation: Operation;
  action: OperationControlAction;
}

const ALL_STATUSES = "all" as const;

function initialFilters(): AppliedFilters {
  return { status: ALL_STATUSES };
}

function shortIdentifier(value: string): string {
  return value.length <= 20 ? value : `${value.slice(0, 11)}…${value.slice(-6)}`;
}

function OperationSkeleton() {
  return (
    <div
      className="space-y-4"
      role="status"
      aria-live="polite"
      aria-label={TEXT.COMMON.LOADING}
    >
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-32 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none"
          />
        ))}
      </div>
      <div className="h-80 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none" />
    </div>
  );
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
  tone?: "primary" | "warning" | "danger" | "success";
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
              : tone === "success"
                ? "bg-success"
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

function OperationIdentity({ operation }: { operation: Operation }) {
  const kindLabel = operationKindLabel(operation.kind);
  return (
    <div className="max-w-72">
      <p className="truncate text-sm font-semibold text-foreground">
        {kindLabel}
      </p>
      <p className="mt-1 truncate font-mono text-[11px] text-muted-foreground">
        {operation.kind}
      </p>
      <p
        className="mt-1 truncate font-mono text-xs text-muted-foreground"
        title={operation.id}
      >
        {shortIdentifier(operation.id)}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">
        {ADMIN_TEXT.OPERATIONS.BATCH_SIZE(operation.batch_size)}
      </p>
    </div>
  );
}

function OperationCards({
  operations,
  onSelect,
}: {
  operations: Operation[];
  onSelect: (operation: Operation) => void;
}) {
  return (
    <div className="grid gap-3 lg:hidden">
      {operations.map((operation) => (
        <Card key={operation.id} className="border-border">
          <CardContent className="space-y-4 p-4">
            <div className="flex items-start justify-between gap-3">
              <OperationIdentity operation={operation} />
              <OperationStatusBadge operation={operation} />
            </div>
            <OperationProgress operation={operation} />
            <div className="grid grid-cols-2 gap-3 rounded-xl border border-border bg-muted/30 p-3 text-xs">
              <div>
                <p className="text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.COLUMN_ATTEMPTS}
                </p>
                <p className="mt-1 font-medium tabular-nums">
                  {ADMIN_TEXT.OPERATIONS.ATTEMPT_COUNT(
                    operation.attempt_count,
                  )}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.UPDATED_AT_LABEL}
                </p>
                <p className="mt-1 font-medium tabular-nums">
                  {formatDateTime(operation.updated_at)}
                </p>
              </div>
            </div>
            <Button
              type="button"
              variant="outline"
              className="w-full"
              onClick={() => onSelect(operation)}
              aria-label={ADMIN_TEXT.OPERATIONS.VIEW_DETAILS_ARIA(
                operation.kind,
              )}
            >
              <Eye className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.OPERATIONS.VIEW_DETAILS}
            </Button>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function OperationTable({
  operations,
  onSelect,
}: {
  operations: Operation[];
  onSelect: (operation: Operation) => void;
}) {
  return (
    <div className="hidden lg:block">
      <Table aria-label={ADMIN_TEXT.OPERATIONS.TABLE_LABEL}>
        <TableHeader>
          <TableRow>
            <TableHead>{ADMIN_TEXT.OPERATIONS.COLUMN_OPERATION}</TableHead>
            <TableHead>{ADMIN_TEXT.OPERATIONS.COLUMN_PROGRESS}</TableHead>
            <TableHead>{ADMIN_TEXT.OPERATIONS.COLUMN_STATE}</TableHead>
            <TableHead>{ADMIN_TEXT.OPERATIONS.COLUMN_ATTEMPTS}</TableHead>
            <TableHead>{ADMIN_TEXT.OPERATIONS.COLUMN_TIMING}</TableHead>
            <TableHead className="text-right">
              {ADMIN_TEXT.OPERATIONS.COLUMN_ACTIONS}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {operations.map((operation) => (
            <TableRow key={operation.id}>
              <TableCell>
                <OperationIdentity operation={operation} />
              </TableCell>
              <TableCell>
                <OperationProgress operation={operation} />
              </TableCell>
              <TableCell>
                <div className="max-w-52 space-y-2">
                  <OperationStatusBadge operation={operation} />
                  {operation.last_error_code ? (
                    <p className="truncate font-mono text-xs text-destructive">
                      {operation.last_error_code}
                    </p>
                  ) : null}
                </div>
              </TableCell>
              <TableCell>
                <p className="tabular-nums">
                  {ADMIN_TEXT.OPERATIONS.ATTEMPT_COUNT(
                    operation.attempt_count,
                  )}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.VERSION(operation.version)}
                </p>
              </TableCell>
              <TableCell>
                <p className="text-xs tabular-nums">
                  {formatDateTime(operation.created_at)}
                </p>
                <p className="mt-1 text-xs tabular-nums text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.UPDATED_AT_LABEL}:{" "}
                  {formatDateTime(operation.updated_at)}
                </p>
              </TableCell>
              <TableCell className="text-right">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => onSelect(operation)}
                  aria-label={ADMIN_TEXT.OPERATIONS.VIEW_DETAILS_ARIA(
                    operation.kind,
                  )}
                >
                  <Eye className="size-4" aria-hidden="true" />
                  {ADMIN_TEXT.OPERATIONS.VIEW_DETAILS}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function operationQuery(
  filters: AppliedFilters,
  cursor?: string,
): OperationListQuery {
  const query: OperationListQuery = {
    limit: OPERATION_PAGE_SIZE,
    ...(filters.kind ? { kind: filters.kind } : {}),
    ...(cursor ? { cursor: cursor as OperationListQuery["cursor"] } : {}),
  };
  if (filters.status !== ALL_STATUSES) query.status = filters.status;
  return query;
}

export function AdminOperations() {
  const { authorizationRevision, can } = useAuth();
  const canExecute = can(
    AUTHORIZATION_RESOURCE.OPERATION,
    AUTHORIZATION_ACTION.EXECUTE,
  );
  const [statusDraft, setStatusDraft] =
    useState<StatusFilter>(ALL_STATUSES);
  const [kindDraft, setKindDraft] = useState("");
  const [kindInvalid, setKindInvalid] = useState(false);
  const [filters, setFilters] = useState<AppliedFilters>(initialFilters);
  const [cursorStack, setCursorStack] = useState<Array<string | undefined>>([
    undefined,
  ]);
  const [pageIndex, setPageIndex] = useState(0);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);
  const [selected, setSelected] = useState<Operation | null>(null);
  const [controlIntent, setControlIntent] = useState<ControlIntent | null>(null);
  const [controlPending, setControlPending] = useState(false);
  const [controlError, setControlError] = useState<string | null>(null);
  const cursor = cursorStack[pageIndex];
  const query = useMemo(
    () => operationQuery(filters, cursor),
    [cursor, filters],
  );
  const resultResource = useRetryableResource<OperationPage | null>({
    resetKey: `${authorizationRevision}:${JSON.stringify(query)}`,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => operationService.list(query, signal),
    onSuccess: (next) => {
      if (!next) return;
      setSelected((current) =>
        current
          ? next.items.find(({ id }) => id === current.id) ?? current
          : null,
      );
      setLastUpdatedAt(new Date().toISOString());
    },
  });
  const result = resultResource.isPreviousData ? null : resultResource.data;
  const loadFailed = resultResource.status === "error";
  const loadState: LoadState =
    resultResource.status === "loading"
      ? result
        ? "refreshing"
        : "loading"
      : resultResource.status === "error" && !result
        ? "error"
        : "ready";

  const metrics = useMemo(() => {
    const items = result?.items ?? [];
    return {
      active: items.filter(
        ({ status }) =>
          status === OPERATION_STATUS.PENDING ||
          status === OPERATION_STATUS.RUNNING,
      ).length,
      paused: items.filter(
        ({ status }) => status === OPERATION_STATUS.PAUSED,
      ).length,
      completed: items.filter(
        ({ status }) => status === OPERATION_STATUS.COMPLETED,
      ).length,
      failed: items.filter(
        ({ status }) => status === OPERATION_STATUS.FAILED,
      ).length,
    };
  }, [result]);

  function resetPaging() {
    setCursorStack([undefined]);
    setPageIndex(0);
    setSelected(null);
  }

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const kind = kindDraft.trim();
    if (
      kind &&
      (kind.length > OPERATION_KIND_MAXIMUM_LENGTH ||
        !OPERATION_KIND_PATTERN.test(kind))
    ) {
      setKindInvalid(true);
      return;
    }
    setKindInvalid(false);
    resetPaging();
    setFilters({ status: statusDraft, ...(kind ? { kind } : {}) });
    resultResource.retry();
  }

  function resetFilters() {
    setStatusDraft(ALL_STATUSES);
    setKindDraft("");
    setKindInvalid(false);
    resetPaging();
    setFilters(initialFilters());
    resultResource.retry();
  }

  function goNext() {
    if (
      !result?.next_cursor ||
      resultResource.status === "loading" ||
      resultResource.isPreviousData
    ) {
      return;
    }
    setCursorStack((current) => [
      ...current.slice(0, pageIndex + 1),
      result.next_cursor,
    ]);
    setPageIndex((index) => index + 1);
  }

  async function control(reason: string) {
    if (!controlIntent || controlPending) return;
    const actionLabel = operationControlLabel(controlIntent.action);
    setControlPending(true);
    setControlError(null);
    try {
      const response = await operationService.control(
        controlIntent.operation.id,
        {
          action: controlIntent.action,
          expected_version: controlIntent.operation.version,
          reason,
        },
      );
      setSelected(response.operation);
      setControlIntent(null);
      if (response.replay) {
        notify.info(ADMIN_TEXT.OPERATIONS.CONTROL_REPLAYED);
      } else {
        notify.success(ADMIN_TEXT.OPERATIONS.CONTROL_SUCCESS(actionLabel));
      }
      resultResource.retry();
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setControlIntent(null);
        notify.warning(ADMIN_TEXT.OPERATIONS.CONTROL_CONFLICT);
        resultResource.retry();
      } else {
        setControlError(ADMIN_TEXT.OPERATIONS.CONTROL_ERROR);
      }
    } finally {
      setControlPending(false);
    }
  }

  const items = result?.items ?? [];
  const initialLoading = loadState === "loading" && !result;
  const unavailable = loadFailed && !result;

  return (
    <div className="space-y-6 pb-8">
      <header className="flex flex-col gap-4 border-b border-border pb-6 xl:flex-row xl:items-end xl:justify-between">
        <div className="max-w-3xl">
          <div className="mb-3 flex size-11 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary">
            <Workflow className="size-5" aria-hidden="true" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            {ADMIN_TEXT.OPERATIONS.TITLE}
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            {ADMIN_TEXT.OPERATIONS.SUBTITLE}
          </p>
        </div>
        <div className="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
          {lastUpdatedAt ? (
            <span
              className="text-xs tabular-nums text-muted-foreground"
              aria-live="polite"
            >
              {ADMIN_TEXT.OPERATIONS.UPDATED_AT(
                formatDateTime(lastUpdatedAt),
              )}
            </span>
          ) : null}
          <Button
            type="button"
            variant="outline"
            disabled={resultResource.status === "loading"}
            onClick={resultResource.retry}
          >
            <RefreshCw
              className={cn(
                "size-4",
                loadState === "refreshing" &&
                  "animate-spin motion-reduce:animate-none",
              )}
              aria-hidden="true"
            />
            {loadState === "refreshing"
              ? ADMIN_TEXT.OPERATIONS.REFRESHING
              : ADMIN_TEXT.OPERATIONS.REFRESH}
          </Button>
        </div>
      </header>

      <div className="flex gap-3 rounded-2xl border border-primary/20 bg-primary/5 p-4">
        <ShieldCheck className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />
        <div>
          <p className="text-sm font-semibold text-foreground">
            {ADMIN_TEXT.OPERATIONS.TRUST_LABEL}
          </p>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {ADMIN_TEXT.OPERATIONS.TRUST_DESCRIPTION}
          </p>
        </div>
      </div>

      <Card className="border-border">
        <CardHeader className="border-b border-border bg-muted/20">
          <CardTitle className="flex items-center gap-2">
            <Filter className="size-4 text-primary" aria-hidden="true" />
            {ADMIN_TEXT.OPERATIONS.FILTERS_TITLE}
          </CardTitle>
          <CardDescription>
            {ADMIN_TEXT.OPERATIONS.FILTERS_DESCRIPTION}
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-5">
          <form
            className="grid gap-4 md:grid-cols-[minmax(12rem,0.7fr)_minmax(16rem,1.3fr)_auto]"
            onSubmit={applyFilters}
          >
            <div className="space-y-2">
              <Label htmlFor="operation-status-filter">
                {ADMIN_TEXT.OPERATIONS.STATUS_LABEL}
              </Label>
              <Select
                value={statusDraft}
                onValueChange={(value) =>
                  setStatusDraft(value as StatusFilter)
                }
              >
                <SelectTrigger id="operation-status-filter">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL_STATUSES}>
                    {ADMIN_TEXT.OPERATIONS.ALL_STATUSES}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.PENDING}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.PENDING}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.RUNNING}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.RUNNING}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.PAUSED}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.PAUSED}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.COMPLETED}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.COMPLETED}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.FAILED}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.FAILED}
                  </SelectItem>
                  <SelectItem value={OPERATION_STATUS.CANCELLED}>
                    {ADMIN_TEXT.OPERATIONS.STATUS.CANCELLED}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="operation-kind-filter">
                {ADMIN_TEXT.OPERATIONS.KIND}
              </Label>
              <Input
                id="operation-kind-filter"
                value={kindDraft}
                maxLength={OPERATION_KIND_MAXIMUM_LENGTH}
                aria-invalid={kindInvalid}
                aria-describedby="operation-kind-help"
                placeholder={ADMIN_TEXT.OPERATIONS.KIND_PLACEHOLDER}
                onChange={(event) => {
                  setKindDraft(event.target.value);
                  setKindInvalid(false);
                }}
              />
              <p
                id="operation-kind-help"
                className={cn(
                  "text-xs leading-5",
                  kindInvalid ? "text-destructive" : "text-muted-foreground",
                )}
              >
                {kindInvalid
                  ? ADMIN_TEXT.OPERATIONS.KIND_INVALID
                  : ADMIN_TEXT.OPERATIONS.KIND_HINT}
              </p>
            </div>
            <div className="flex items-end gap-2">
              <Button type="submit">
                <Filter className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.OPERATIONS.APPLY_FILTERS}
              </Button>
              <Button type="button" variant="outline" onClick={resetFilters}>
                <RotateCcw className="size-4" aria-hidden="true" />
                <span className="sr-only">
                  {ADMIN_TEXT.OPERATIONS.RESET_FILTERS}
                </span>
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {initialLoading ? <OperationSkeleton /> : null}

      {unavailable ? (
        <Card className="border-destructive/25 bg-destructive/5">
          <CardContent className="flex min-h-56 flex-col items-center justify-center p-6 text-center">
            <AlertTriangle className="size-7 text-destructive" aria-hidden="true" />
            <h2 className="mt-3 font-semibold text-foreground">
              {ADMIN_TEXT.OPERATIONS.LOAD_ERROR_TITLE}
            </h2>
            <p className="mt-1 max-w-xl text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.OPERATIONS.LOAD_ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-4"
              onClick={resultResource.retry}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {!initialLoading && !unavailable ? (
        <>
          {loadFailed ? (
            <div
              className="flex gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4"
              role="alert"
            >
              <AlertTriangle
                className="mt-0.5 size-5 shrink-0 text-destructive"
                aria-hidden="true"
              />
              <div>
                <p className="text-sm font-semibold text-foreground">
                  {ADMIN_TEXT.OPERATIONS.LOAD_ERROR_TITLE}
                </p>
                <p className="mt-1 text-sm leading-6 text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.LOAD_ERROR_DESCRIPTION}
                </p>
              </div>
            </div>
          ) : null}
          <section
            aria-label={ADMIN_TEXT.OPERATIONS.CURRENT_PAGE}
            className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
          >
            <MetricCard
              label={ADMIN_TEXT.OPERATIONS.ACTIVE}
              value={metrics.active}
              icon={TimerReset}
            />
            <MetricCard
              label={ADMIN_TEXT.OPERATIONS.PAUSED}
              value={metrics.paused}
              icon={CirclePause}
              tone="warning"
            />
            <MetricCard
              label={ADMIN_TEXT.OPERATIONS.COMPLETED}
              value={metrics.completed}
              icon={CheckCircle2}
              tone="success"
            />
            <MetricCard
              label={ADMIN_TEXT.OPERATIONS.FAILED}
              value={metrics.failed}
              icon={XCircle}
              tone="danger"
            />
          </section>

          <Card className="overflow-hidden border-border">
            {loadState === "refreshing" ? (
              <div
                className="flex items-center gap-2 border-b border-border bg-muted/35 px-4 py-2 text-xs text-muted-foreground"
                role="status"
              >
                <LoaderCircle
                  className="size-3.5 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
                {ADMIN_TEXT.OPERATIONS.REFRESHING}
              </div>
            ) : null}
            {items.length === 0 ? (
              <CardContent className="flex min-h-64 flex-col items-center justify-center p-6 text-center">
                <Workflow className="size-7 text-muted-foreground" aria-hidden="true" />
                <h2 className="mt-3 font-semibold text-foreground">
                  {ADMIN_TEXT.OPERATIONS.EMPTY_TITLE}
                </h2>
                <p className="mt-1 max-w-xl text-sm leading-6 text-muted-foreground">
                  {ADMIN_TEXT.OPERATIONS.EMPTY_DESCRIPTION}
                </p>
              </CardContent>
            ) : (
              <>
                <OperationTable operations={items} onSelect={setSelected} />
                <CardContent className="p-3 lg:hidden">
                  <OperationCards operations={items} onSelect={setSelected} />
                </CardContent>
              </>
            )}
          </Card>

          <nav
            className="flex items-center justify-between gap-3"
            aria-label={TEXT.COMMON.PAGINATION}
          >
            <Button
              type="button"
              variant="outline"
              disabled={
                pageIndex === 0 ||
                resultResource.status === "loading" ||
                resultResource.isPreviousData
              }
              onClick={() => setPageIndex((index) => Math.max(0, index - 1))}
            >
              {ADMIN_TEXT.OPERATIONS.PREVIOUS_PAGE}
            </Button>
            <Badge variant="outline">
              {ADMIN_TEXT.OPERATIONS.PAGE_NUMBER(pageIndex + 1)}
            </Badge>
            <Button
              type="button"
              variant="outline"
              disabled={
                !result?.next_cursor ||
                resultResource.status === "loading" ||
                resultResource.isPreviousData
              }
              onClick={goNext}
            >
              {ADMIN_TEXT.OPERATIONS.NEXT_PAGE}
            </Button>
          </nav>
        </>
      ) : null}

      {selected ? (
        <OperationDetailSheet
          operation={selected}
          canExecute={canExecute}
          onClose={() => setSelected(null)}
          onControl={(action) => {
            setControlError(null);
            setControlIntent({ operation: selected, action });
            setSelected(null);
          }}
        />
      ) : null}

      {controlIntent ? (
        <OperationControlDialog
          key={`${controlIntent.operation.id}:${controlIntent.operation.version}:${controlIntent.action}`}
          operation={controlIntent.operation}
          action={controlIntent.action}
          pending={controlPending}
          error={controlError}
          onOpenChange={(open) => {
            if (!open) {
              setControlIntent(null);
              setControlError(null);
            }
          }}
          onConfirm={(reason) => void control(reason)}
        />
      ) : null}
    </div>
  );
}
