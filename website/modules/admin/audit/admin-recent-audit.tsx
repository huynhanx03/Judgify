"use client";

import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  Clock3,
  RefreshCw,
  ScrollText,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AUDIT_RECENT_LIMIT } from "@/constants/audit";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { formatDateTime } from "@/lib/format";
import { getErrorMessage } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { auditService } from "@/services/audit.service";
import type { AuditLogEntry } from "@/types/audit";

export function AdminRecentAudit() {
  const recentResource = useRetryableResource<AuditLogEntry[]>({
    resetKey: `admin-recent-audit:${AUDIT_RECENT_LIMIT}`,
    initialData: [],
    keepPreviousData: true,
    load: async (signal) =>
      (await auditService.list({ limit: AUDIT_RECENT_LIMIT }, signal)).items,
  });
  const items = recentResource.data;
  const error = recentResource.error
    ? getErrorMessage(recentResource.error, ADMIN_TEXT.AUDIT.RECENT_ERROR)
    : null;
  const isInitialLoading =
    recentResource.status === "loading" && items.length === 0;
  const isRefreshing =
    recentResource.status === "loading" && items.length > 0;
  const hasBlockingError =
    recentResource.status === "error" && items.length === 0;
  const hasStaleError = recentResource.status === "error" && items.length > 0;

  return (
    <Card className="overflow-hidden border-border shadow-sm">
      <CardHeader className="flex flex-row items-start justify-between gap-4 border-b border-border bg-muted/20">
        <div>
          <CardTitle className="flex items-center gap-2 text-base">
            <ScrollText className="h-4 w-4 text-primary" aria-hidden="true" />
            {ADMIN_TEXT.AUDIT.RECENT_TITLE}
          </CardTitle>
          <p className="mt-1 text-sm leading-5 text-muted-foreground">
            {ADMIN_TEXT.AUDIT.RECENT_DESCRIPTION}
          </p>
        </div>
        <Link
          href={APP_ROUTES.ADMIN_AUDIT}
          className={cn(
            buttonVariants({ variant: "ghost", size: "sm" }),
            "hidden min-h-11 shrink-0 sm:inline-flex",
          )}
        >
          {ADMIN_TEXT.AUDIT.VIEW_ALL}
          <ArrowRight aria-hidden="true" />
        </Link>
      </CardHeader>
      <CardContent className="p-0" aria-busy={recentResource.status === "loading"}>
        {isInitialLoading ? (
          <div
            className="space-y-3 p-5"
            role="status"
            aria-live="polite"
            aria-label={ADMIN_TEXT.AUDIT.RECENT_LOADING}
          >
            {[0, 1, 2].map((item) => (
              <div
                key={item}
                className="h-14 animate-pulse rounded-xl bg-muted/50 motion-reduce:animate-none"
              />
            ))}
          </div>
        ) : hasBlockingError ? (
          <div className="flex min-h-40 flex-col items-center justify-center p-6 text-center" role="alert">
            <AlertTriangle className="h-5 w-5 text-destructive" aria-hidden="true" />
            <p className="mt-3 text-sm text-foreground">
              {error ?? ADMIN_TEXT.AUDIT.RECENT_ERROR}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-4 min-h-11"
              onClick={recentResource.retry}
            >
              <RefreshCw aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          </div>
        ) : items.length === 0 ? (
          <div className="flex min-h-40 flex-col items-center justify-center p-6 text-center">
            <Clock3 className="h-5 w-5 text-muted-foreground" aria-hidden="true" />
            <p className="mt-3 text-sm text-muted-foreground">
              {ADMIN_TEXT.AUDIT.RECENT_EMPTY}
            </p>
          </div>
        ) : (
          <>
            {isRefreshing ? (
              <div
                className="flex items-center gap-3 border-b border-border bg-muted/20 px-5 py-3 text-sm text-muted-foreground"
                role="status"
                aria-live="polite"
              >
                <RefreshCw
                  className="h-4 w-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
                {ADMIN_TEXT.AUDIT.RECENT_LOADING}
              </div>
            ) : hasStaleError ? (
              <div
                className="flex flex-col gap-3 border-b border-destructive/20 bg-destructive/5 px-5 py-4 sm:flex-row sm:items-center"
                role="alert"
              >
                <AlertTriangle
                  className="h-5 w-5 shrink-0 text-destructive"
                  aria-hidden="true"
                />
                <p className="flex-1 text-sm text-foreground">
                  {error ?? ADMIN_TEXT.AUDIT.RECENT_ERROR}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11"
                  onClick={recentResource.retry}
                >
                  <RefreshCw aria-hidden="true" />
                  {TEXT.COMMON.RETRY}
                </Button>
              </div>
            ) : null}
            <div className="divide-y divide-border">
              {items.map((entry) => (
                <div
                  key={entry.id}
                  className="grid gap-3 px-5 py-4 transition-colors duration-200 hover:bg-muted/35 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center motion-reduce:transition-none"
                >
                  <div className="min-w-0">
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <Badge variant="secondary" className="max-w-full font-mono">
                        <span className="truncate">{entry.action}</span>
                      </Badge>
                      <span className="truncate font-mono text-xs text-muted-foreground">
                        {entry.resource}
                      </span>
                    </div>
                    <p className="mt-2 truncate font-mono text-xs text-muted-foreground">
                      {entry.correlation_id}
                    </p>
                  </div>
                  <time
                    dateTime={entry.occurred_at}
                    className="text-xs tabular-nums text-muted-foreground"
                  >
                    {formatDateTime(entry.occurred_at)}
                  </time>
                </div>
              ))}
            </div>
          </>
        )}
        <div className="border-t border-border p-3 sm:hidden">
          <Link
            href={APP_ROUTES.ADMIN_AUDIT}
            className={cn(
              buttonVariants({ variant: "outline" }),
              "min-h-11 w-full",
            )}
          >
            {ADMIN_TEXT.AUDIT.VIEW_ALL}
            <ArrowRight aria-hidden="true" />
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}
