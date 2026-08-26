"use client";

import {
  type FormEvent,
  useMemo,
  useState,
} from "react";
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Eye,
  Filter,
  Fingerprint,
  RefreshCw,
  RotateCcw,
  ScrollText,
  ShieldCheck,
  UserRound,
  Waypoints,
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
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { AUDIT_FILTER_LIMITS, AUDIT_PAGE_SIZE } from "@/constants/audit";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  createEmptyAuditFilterDraft,
  normalizeAuditFilters,
  type AuditFilterDraft,
  type AuditInvalidField,
} from "@/lib/audit/query";
import { formatDateTime } from "@/lib/format";
import { getErrorMessage } from "@/lib/toast";
import { auditService } from "@/services/audit.service";
import type { AuditListQuery, AuditLogEntry, AuditLogPage } from "@/types/audit";
import { AuditDetailSheet } from "./audit-detail-sheet";

interface AuditPageSnapshot {
  page: AuditLogPage;
  pageIndex: number;
}

function createDefaultAuditQuery(): AuditListQuery {
  const normalized = normalizeAuditFilters(createEmptyAuditFilterDraft());
  return normalized.ok ? normalized.query : { limit: AUDIT_PAGE_SIZE };
}

function shortIdentifier(value: string): string {
  return value.length <= 18 ? value : `${value.slice(0, 10)}…${value.slice(-6)}`;
}

function AuditSkeleton() {
  return (
    <div
      className="space-y-3"
      role="status"
      aria-live="polite"
      aria-label={TEXT.COMMON.LOADING}
    >
      {[0, 1, 2, 3, 4].map((item) => (
        <div
          key={item}
          className="h-20 animate-pulse rounded-xl border border-border bg-muted/40 motion-reduce:animate-none"
        />
      ))}
    </div>
  );
}

function AuditEventIdentity({ entry }: { entry: AuditLogEntry }) {
  return (
    <div className="min-w-0 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="secondary" className="max-w-full font-mono">
          <span className="truncate">{entry.action}</span>
        </Badge>
        <Badge variant="outline" className="max-w-full font-mono">
          <span className="truncate">{entry.resource}</span>
        </Badge>
      </div>
      <p className="truncate font-mono text-xs text-muted-foreground">
        {entry.resource_id ?? TEXT.COMMON.NOT_AVAILABLE}
      </p>
    </div>
  );
}

export function AdminAuditExplorer() {
  const [draft, setDraft] = useState<AuditFilterDraft>(
    createEmptyAuditFilterDraft,
  );
  const [invalidFields, setInvalidFields] = useState<AuditInvalidField[]>([]);
  const [appliedQuery, setAppliedQuery] = useState<AuditListQuery>(
    createDefaultAuditQuery,
  );
  const [cursorStack, setCursorStack] = useState<Array<string | undefined>>([
    undefined,
  ]);
  const [pageIndex, setPageIndex] = useState(0);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);
  const [selectedEntry, setSelectedEntry] = useState<AuditLogEntry | null>(null);
  const cursor = cursorStack[pageIndex];
  const resourceKey = JSON.stringify({
    ...appliedQuery,
    cursor: cursor ?? null,
    pageIndex,
  });
  const auditResource = useRetryableResource<AuditPageSnapshot | null>({
    resetKey: resourceKey,
    initialData: null,
    keepPreviousData: true,
    load: async (signal) => ({
      page: await auditService.list(
        { ...appliedQuery, ...(cursor ? { cursor } : {}) },
        signal,
      ),
      pageIndex,
    }),
    onSuccess: () => setLastUpdatedAt(new Date().toISOString()),
  });
  const snapshot = auditResource.data;
  const result = snapshot?.page ?? null;
  const displayedPageIndex = snapshot?.pageIndex ?? pageIndex;
  const loadError = auditResource.error
    ? getErrorMessage(
        auditResource.error,
        ADMIN_TEXT.AUDIT.LOAD_ERROR_DESCRIPTION,
      )
    : null;
  const isPending = auditResource.status === "loading";
  const isInitialLoading = isPending && result === null;
  const isRefreshing = isPending && result !== null;
  const hasBlockingError = auditResource.status === "error" && result === null;

  const metrics = useMemo(() => {
    const items = result?.items ?? [];
    return [
      {
        label: ADMIN_TEXT.AUDIT.EVENTS_ON_PAGE,
        value: items.length,
        icon: ScrollText,
      },
      {
        label: ADMIN_TEXT.AUDIT.ACTORS_ON_PAGE,
        value: new Set(items.map(({ actor_user_id }) => actor_user_id).filter(Boolean))
          .size,
        icon: UserRound,
      },
      {
        label: ADMIN_TEXT.AUDIT.RESOURCES_ON_PAGE,
        value: new Set(items.map(({ resource }) => resource).filter(Boolean)).size,
        icon: Waypoints,
      },
      {
        label: ADMIN_TEXT.AUDIT.TRACE_IDS_ON_PAGE,
        value: new Set(
          items.map(({ correlation_id }) => correlation_id).filter(Boolean),
        ).size,
        icon: Fingerprint,
      },
    ];
  }, [result]);

  function updateDraft<K extends keyof AuditFilterDraft>(
    key: K,
    value: AuditFilterDraft[K],
  ) {
    setDraft((current) => ({ ...current, [key]: value }));
    setInvalidFields([]);
  }

  function resetPaging() {
    setCursorStack([undefined]);
    setPageIndex(0);
  }

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeAuditFilters(draft);
    if (!normalized.ok) {
      setInvalidFields(normalized.invalidFields);
      return;
    }
    setInvalidFields([]);
    resetPaging();
    setAppliedQuery(normalized.query);
    auditResource.retry();
  }

  function resetFilters() {
    const empty = createEmptyAuditFilterDraft();
    const normalized = normalizeAuditFilters(empty);
    setDraft(empty);
    setInvalidFields([]);
    resetPaging();
    if (normalized.ok) setAppliedQuery(normalized.query);
    auditResource.retry();
  }

  function previousPage() {
    if (pageIndex === 0 || isPending) return;
    setPageIndex((page) => page - 1);
  }

  function nextPage() {
    if (
      isPending ||
      auditResource.isPreviousData ||
      !result?.has_more ||
      !result.next_cursor
    ) {
      return;
    }
    const nextCursor = result.next_cursor;
    setCursorStack((current) => [
      ...current.slice(0, pageIndex + 1),
      nextCursor,
    ]);
    setPageIndex((page) => page + 1);
  }

  const hasInvalidLength =
    draft.actorUserId.trim().length > AUDIT_FILTER_LIMITS.ACTOR_USER_ID ||
    draft.action.trim().length > AUDIT_FILTER_LIMITS.ACTION ||
    draft.resource.trim().length > AUDIT_FILTER_LIMITS.RESOURCE ||
    draft.resourceId.trim().length > AUDIT_FILTER_LIMITS.RESOURCE_ID ||
    draft.correlationId.trim().length > AUDIT_FILTER_LIMITS.CORRELATION_ID;

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary shadow-sm shadow-primary/5">
            <ScrollText className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              {ADMIN_TEXT.AUDIT.TITLE}
            </h1>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
              {ADMIN_TEXT.AUDIT.SUBTITLE}
            </p>
          </div>
        </div>
        <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center">
          {lastUpdatedAt ? (
            <span className="text-xs tabular-nums text-muted-foreground">
              {ADMIN_TEXT.AUDIT.UPDATED_AT(formatDateTime(lastUpdatedAt))}
            </span>
          ) : null}
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            onClick={auditResource.retry}
            disabled={isPending}
          >
            <RefreshCw
              className={isRefreshing ? "animate-spin motion-reduce:animate-none" : ""}
              aria-hidden="true"
            />
            {isRefreshing
              ? ADMIN_TEXT.AUDIT.REFRESHING
              : ADMIN_TEXT.AUDIT.REFRESH}
          </Button>
        </div>
      </header>

      <div className="flex items-start gap-3 rounded-2xl border border-primary/20 bg-primary/5 p-4">
        <ShieldCheck className="mt-0.5 h-5 w-5 shrink-0 text-primary" aria-hidden="true" />
        <div>
          <p className="text-sm font-semibold text-foreground">
            {ADMIN_TEXT.AUDIT.TRUST_LABEL}
          </p>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {ADMIN_TEXT.AUDIT.TRUST_DESCRIPTION}
          </p>
        </div>
      </div>

      <Card className="overflow-hidden border-border shadow-sm">
        <CardHeader className="border-b border-border bg-muted/20">
          <div className="flex items-start gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <Filter className="h-4 w-4" aria-hidden="true" />
            </div>
            <div>
              <CardTitle className="text-base">
                {ADMIN_TEXT.AUDIT.FILTERS_TITLE}
              </CardTitle>
              <CardDescription className="mt-1 leading-5">
                {ADMIN_TEXT.AUDIT.FILTERS_DESCRIPTION}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <form className="space-y-5" onSubmit={applyFilters} noValidate>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
              <div className="space-y-2">
                <Label htmlFor="audit-actor-user-id">
                  {ADMIN_TEXT.AUDIT.ACTOR_USER_ID}
                </Label>
                <Input
                  id="audit-actor-user-id"
                  value={draft.actorUserId}
                  onChange={(event) => updateDraft("actorUserId", event.target.value)}
                  placeholder={ADMIN_TEXT.AUDIT.ACTOR_USER_ID_PLACEHOLDER}
                  aria-invalid={invalidFields.includes("actorUserId")}
                  className="min-h-11 font-mono text-xs"
                />
                {invalidFields.includes("actorUserId") ? (
                  <p className="text-xs text-destructive" role="alert">
                    {ADMIN_TEXT.AUDIT.ACTOR_USER_ID_INVALID}
                  </p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="audit-action">{ADMIN_TEXT.AUDIT.ACTION}</Label>
                <Input
                  id="audit-action"
                  value={draft.action}
                  onChange={(event) => updateDraft("action", event.target.value)}
                  placeholder={ADMIN_TEXT.AUDIT.ACTION_PLACEHOLDER}
                  aria-invalid={invalidFields.includes("action")}
                  className="min-h-11 font-mono text-xs"
                />
                {invalidFields.includes("action") ? (
                  <p className="text-xs text-destructive" role="alert">
                    {ADMIN_TEXT.AUDIT.ACTION_INVALID}
                  </p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="audit-resource">{ADMIN_TEXT.AUDIT.RESOURCE}</Label>
                <Input
                  id="audit-resource"
                  value={draft.resource}
                  onChange={(event) => updateDraft("resource", event.target.value)}
                  placeholder={ADMIN_TEXT.AUDIT.RESOURCE_PLACEHOLDER}
                  aria-invalid={invalidFields.includes("resource")}
                  className="min-h-11 font-mono text-xs"
                />
                {invalidFields.includes("resource") ? (
                  <p className="text-xs text-destructive" role="alert">
                    {ADMIN_TEXT.AUDIT.RESOURCE_INVALID}
                  </p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="audit-resource-id">
                  {ADMIN_TEXT.AUDIT.RESOURCE_ID}
                </Label>
                <Input
                  id="audit-resource-id"
                  value={draft.resourceId}
                  onChange={(event) => updateDraft("resourceId", event.target.value)}
                  placeholder={ADMIN_TEXT.AUDIT.RESOURCE_ID_PLACEHOLDER}
                  aria-invalid={invalidFields.includes("resourceId")}
                  aria-describedby={
                    invalidFields.includes("resourceId")
                      ? "audit-resource-id-error"
                      : undefined
                  }
                  className="min-h-11 font-mono text-xs"
                />
                {invalidFields.includes("resourceId") ? (
                  <p
                    id="audit-resource-id-error"
                    className="text-xs text-destructive"
                    role="alert"
                  >
                    {ADMIN_TEXT.AUDIT.RESOURCE_ID_INVALID}
                  </p>
                ) : null}
              </div>
              <div className="space-y-2 sm:col-span-2 xl:col-span-1">
                <Label htmlFor="audit-correlation-id">
                  {ADMIN_TEXT.AUDIT.CORRELATION_ID}
                </Label>
                <Input
                  id="audit-correlation-id"
                  value={draft.correlationId}
                  onChange={(event) =>
                    updateDraft("correlationId", event.target.value)
                  }
                  placeholder={ADMIN_TEXT.AUDIT.CORRELATION_ID_PLACEHOLDER}
                  aria-invalid={invalidFields.includes("correlationId")}
                  aria-describedby={
                    invalidFields.includes("correlationId")
                      ? "audit-correlation-id-error"
                      : undefined
                  }
                  className="min-h-11 font-mono text-xs"
                />
                {invalidFields.includes("correlationId") ? (
                  <p
                    id="audit-correlation-id-error"
                    className="text-xs text-destructive"
                    role="alert"
                  >
                    {ADMIN_TEXT.AUDIT.CORRELATION_ID_INVALID}
                  </p>
                ) : null}
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 xl:max-w-3xl">
              <div className="space-y-2">
                <Label htmlFor="audit-from">{ADMIN_TEXT.AUDIT.FROM}</Label>
                <Input
                  id="audit-from"
                  type="datetime-local"
                  value={draft.from}
                  onChange={(event) => updateDraft("from", event.target.value)}
                  aria-invalid={invalidFields.includes("timeRange")}
                  className="min-h-11"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="audit-to">{ADMIN_TEXT.AUDIT.TO}</Label>
                <Input
                  id="audit-to"
                  type="datetime-local"
                  value={draft.to}
                  onChange={(event) => updateDraft("to", event.target.value)}
                  aria-invalid={invalidFields.includes("timeRange")}
                  className="min-h-11"
                />
              </div>
            </div>

            {invalidFields.includes("timeRange") ? (
              <p className="text-sm text-destructive" role="alert">
                {ADMIN_TEXT.AUDIT.TIME_RANGE_INVALID}
              </p>
            ) : null}
            {hasInvalidLength ? (
              <p className="text-sm text-destructive" role="alert">
                {ADMIN_TEXT.AUDIT.FILTER_TOO_LONG}
              </p>
            ) : null}

            <div className="flex flex-col gap-3 border-t border-border pt-5 sm:flex-row">
              <Button type="submit" className="min-h-11 sm:min-w-40">
                <Filter aria-hidden="true" />
                {ADMIN_TEXT.AUDIT.APPLY_FILTERS}
              </Button>
              <Button
                type="button"
                variant="outline"
                className="min-h-11"
                onClick={resetFilters}
              >
                <RotateCcw aria-hidden="true" />
                {ADMIN_TEXT.AUDIT.RESET_FILTERS}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <section aria-labelledby="audit-summary-title" className="space-y-3">
        <h2
          id="audit-summary-title"
          className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground"
        >
          {ADMIN_TEXT.AUDIT.RESULT_SUMMARY}
        </h2>
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {metrics.map(({ label, value, icon: Icon }) => (
            <Card key={label} className="border-border bg-card shadow-sm">
              <CardContent className="flex items-center gap-3 p-4">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="h-4 w-4" aria-hidden="true" />
                </div>
                <div className="min-w-0">
                  <p className="text-2xl font-bold tabular-nums text-foreground">
                    {result ? value : TEXT.COMMON.NOT_AVAILABLE}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">{label}</p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>

      {loadError && result ? (
        <div
          className="flex flex-col gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4 sm:flex-row sm:items-center"
          role="alert"
        >
          <AlertTriangle className="h-5 w-5 shrink-0 text-destructive" aria-hidden="true" />
          <p className="flex-1 text-sm text-foreground">{loadError}</p>
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            onClick={auditResource.retry}
          >
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}

      <Card className="overflow-hidden border-border shadow-sm">
        <CardContent className="p-0" aria-busy={isPending}>
          {isInitialLoading ? (
            <div className="p-4 sm:p-6">
              <AuditSkeleton />
            </div>
          ) : hasBlockingError ? (
            <div className="flex min-h-72 flex-col items-center justify-center px-6 py-12 text-center" role="alert">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
                <AlertTriangle className="h-5 w-5" aria-hidden="true" />
              </div>
              <h2 className="mt-4 text-lg font-semibold text-foreground">
                {ADMIN_TEXT.AUDIT.LOAD_ERROR_TITLE}
              </h2>
              <p className="mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
                {loadError ?? ADMIN_TEXT.AUDIT.LOAD_ERROR_DESCRIPTION}
              </p>
              <div className="mt-5 flex flex-wrap items-center justify-center gap-2">
                {pageIndex > 0 ? (
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    onClick={previousPage}
                  >
                    <ArrowLeft aria-hidden="true" />
                    {ADMIN_TEXT.AUDIT.PREVIOUS_PAGE}
                  </Button>
                ) : null}
                <Button
                  type="button"
                  className="min-h-11"
                  onClick={auditResource.retry}
                >
                  <RefreshCw aria-hidden="true" />
                  {TEXT.COMMON.RETRY}
                </Button>
              </div>
            </div>
          ) : result && result.items.length === 0 ? (
            <div className="flex min-h-72 flex-col items-center justify-center px-6 py-12 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                <ScrollText className="h-5 w-5" aria-hidden="true" />
              </div>
              <h2 className="mt-4 text-lg font-semibold text-foreground">
                {ADMIN_TEXT.AUDIT.EMPTY_TITLE}
              </h2>
              <p className="mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
                {ADMIN_TEXT.AUDIT.EMPTY_DESCRIPTION}
              </p>
            </div>
          ) : result ? (
            <>
              <div className="md:hidden">
                <div className="divide-y divide-border">
                  {result.items.map((entry) => (
                    <article key={entry.id} className="space-y-4 p-4">
                      <AuditEventIdentity entry={entry} />
                      <dl className="grid gap-3 text-xs sm:grid-cols-2">
                        <div>
                          <dt className="text-muted-foreground">
                            {ADMIN_TEXT.AUDIT.COLUMN_ACTOR}
                          </dt>
                          <dd className="mt-1 break-all font-mono text-foreground">
                            {entry.actor_user_id
                              ? shortIdentifier(entry.actor_user_id)
                              : ADMIN_TEXT.AUDIT.SYSTEM_ACTOR}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground">
                            {ADMIN_TEXT.AUDIT.COLUMN_TIME}
                          </dt>
                          <dd className="mt-1 tabular-nums text-foreground">
                            {formatDateTime(entry.occurred_at)}
                          </dd>
                        </div>
                        <div className="sm:col-span-2">
                          <dt className="text-muted-foreground">
                            {ADMIN_TEXT.AUDIT.CORRELATION_ID}
                          </dt>
                          <dd className="mt-1 break-all font-mono text-foreground">
                            {entry.correlation_id}
                          </dd>
                        </div>
                      </dl>
                      <Button
                        type="button"
                        variant="outline"
                        className="min-h-11 w-full"
                        onClick={() => setSelectedEntry(entry)}
                        aria-label={ADMIN_TEXT.AUDIT.OPEN_DETAILS_ARIA(entry.action)}
                      >
                        <Eye aria-hidden="true" />
                        {ADMIN_TEXT.AUDIT.OPEN_DETAILS}
                      </Button>
                    </article>
                  ))}
                </div>
              </div>

              <div className="hidden md:block">
                <Table>
                  <TableCaption className="sr-only">
                    {ADMIN_TEXT.AUDIT.TABLE_LABEL}
                  </TableCaption>
                  <TableHeader>
                    <TableRow className="bg-muted/25 hover:bg-muted/25">
                      <TableHead className="px-4 py-3">
                        {ADMIN_TEXT.AUDIT.COLUMN_EVENT}
                      </TableHead>
                      <TableHead>{ADMIN_TEXT.AUDIT.COLUMN_ACTOR}</TableHead>
                      <TableHead>{ADMIN_TEXT.AUDIT.COLUMN_TRACE}</TableHead>
                      <TableHead>{ADMIN_TEXT.AUDIT.COLUMN_TIME}</TableHead>
                      <TableHead className="pr-4 text-right">
                        {ADMIN_TEXT.AUDIT.COLUMN_ACTIONS}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.items.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell className="max-w-72 px-4 py-3">
                          <AuditEventIdentity entry={entry} />
                        </TableCell>
                        <TableCell className="max-w-48">
                          <span
                            className="block truncate font-mono text-xs text-foreground"
                            title={entry.actor_user_id ?? undefined}
                          >
                            {entry.actor_user_id
                              ? shortIdentifier(entry.actor_user_id)
                              : ADMIN_TEXT.AUDIT.SYSTEM_ACTOR}
                          </span>
                        </TableCell>
                        <TableCell className="max-w-56">
                          <span
                            className="block truncate font-mono text-xs text-muted-foreground"
                            title={entry.correlation_id}
                          >
                            {shortIdentifier(entry.correlation_id)}
                          </span>
                        </TableCell>
                        <TableCell className="text-xs tabular-nums text-muted-foreground">
                          {formatDateTime(entry.occurred_at)}
                        </TableCell>
                        <TableCell className="pr-4 text-right">
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="min-h-11 min-w-11"
                            onClick={() => setSelectedEntry(entry)}
                            aria-label={ADMIN_TEXT.AUDIT.OPEN_DETAILS_ARIA(entry.action)}
                            title={ADMIN_TEXT.AUDIT.OPEN_DETAILS}
                          >
                            <Eye className="h-4 w-4" aria-hidden="true" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="flex flex-col gap-3 border-t border-border bg-muted/15 p-4 sm:flex-row sm:items-center sm:justify-between">
                <p className="text-center text-sm font-medium tabular-nums text-muted-foreground sm:text-left">
                  {ADMIN_TEXT.AUDIT.PAGE_NUMBER(displayedPageIndex + 1)}
                </p>
                <div className="grid grid-cols-2 gap-2 sm:flex">
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    onClick={previousPage}
                    disabled={pageIndex === 0 || isPending}
                  >
                    <ArrowLeft aria-hidden="true" />
                    {ADMIN_TEXT.AUDIT.PREVIOUS_PAGE}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    onClick={nextPage}
                    disabled={
                      isPending ||
                      auditResource.isPreviousData ||
                      !result.has_more ||
                      !result.next_cursor
                    }
                  >
                    {ADMIN_TEXT.AUDIT.NEXT_PAGE}
                    <ArrowRight aria-hidden="true" />
                  </Button>
                </div>
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

      {selectedEntry ? (
        <AuditDetailSheet
          entry={selectedEntry}
          onClose={() => setSelectedEntry(null)}
        />
      ) : null}
    </div>
  );
}
