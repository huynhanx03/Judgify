"use client";

import type { ReactNode } from "react";
import { AlertTriangle, Boxes, CheckCircle2, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useCursorFeed } from "@/hooks/use-cursor-feed";
import { formatDateTime } from "@/lib/format";
import { operationService } from "@/services/operation.service";
import type { OperationBatch } from "@/types/operation";

export function OperationBatchHistory({
  operationID,
  operationVersion,
}: {
  operationID: string;
  operationVersion: number;
}) {
  const feed = useCursorFeed<OperationBatch>({
    resetKey: `${operationID}:${operationVersion}`,
    load: async (cursor, signal) => {
      const page = await operationService.listBatches(operationID, cursor, signal);
      return { items: page.items, next_cursor: page.next_cursor };
    },
    keyOf: (batch) => `${batch.operation_id}:${batch.sequence}`,
  });
  const initialLoading = feed.status === "loading";
  const loadingMore = feed.status === "loading_more";

  return (
    <section className="space-y-3">
      <h3 className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
        <Boxes className="size-4" aria-hidden="true" />
        {ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_TITLE}
      </h3>
      <p className="text-sm leading-6 text-muted-foreground">
        {ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_DESCRIPTION}
      </p>

      {initialLoading ? (
        <BatchState>
          <Loader2 className="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
          <span>{ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_LOADING}</span>
        </BatchState>
      ) : feed.error && feed.items.length === 0 ? (
        <BatchState>
          <AlertTriangle className="size-5 text-destructive" aria-hidden="true" />
          <span>{ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_ERROR}</span>
          <Button type="button" size="sm" variant="outline" onClick={feed.reload}>
            {TEXT.COMMON.RETRY}
          </Button>
        </BatchState>
      ) : feed.items.length === 0 ? (
        <BatchState>
          <Boxes className="size-5" aria-hidden="true" />
          <span>{ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_EMPTY}</span>
        </BatchState>
      ) : (
        <ol className="space-y-3" aria-label={ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_LIST_LABEL}>
          {feed.items.map((batch) => (
            <li key={batch.sequence} className="rounded-2xl border border-border bg-card p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="font-semibold tabular-nums">
                    {ADMIN_TEXT.OPERATIONS.BATCH_SEQUENCE(batch.sequence)}
                  </p>
                  <p className="mt-1 text-xs tabular-nums text-muted-foreground">
                    {formatDateTime(batch.completed_at)}
                  </p>
                </div>
                <Badge variant={batch.failed > 0 ? "destructive" : "outline"}>
                  {batch.failed > 0 ? (
                    <AlertTriangle data-icon="inline-start" aria-hidden="true" />
                  ) : (
                    <CheckCircle2 data-icon="inline-start" aria-hidden="true" />
                  )}
                  {ADMIN_TEXT.OPERATIONS.BATCH_RESULT(batch.succeeded, batch.failed)}
                </Badge>
              </div>

              {batch.failures.length > 0 ? (
                <div className="mt-3 space-y-2 border-t border-border pt-3">
                  <p className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.OPERATIONS.BATCH_FAILURES_LABEL}
                  </p>
                  <ul className="space-y-2">
                    {batch.failures.map((failure, index) => (
                      <li
                        key={`${failure.reference}:${failure.code}:${index}`}
                        className="grid gap-1 rounded-xl bg-destructive/5 px-3 py-2 text-xs sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
                      >
                        <span className="break-all font-mono text-foreground">
                          {failure.reference}
                        </span>
                        <span className="break-all font-mono text-destructive">
                          {failure.code}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </li>
          ))}
        </ol>
      )}

      {feed.error && feed.items.length > 0 ? (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          <span>{ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_MORE_ERROR}</span>
          <Button type="button" size="sm" variant="outline" onClick={() => void feed.loadMore()}>
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}
      {feed.hasMore ? (
        <Button
          type="button"
          variant="outline"
          className="w-full"
          disabled={loadingMore}
          onClick={() => void feed.loadMore()}
        >
          {loadingMore ? (
            <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
          ) : null}
          {loadingMore
            ? ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_LOADING_MORE
            : ADMIN_TEXT.OPERATIONS.BATCH_HISTORY_LOAD_MORE}
        </Button>
      ) : null}
    </section>
  );
}

function BatchState({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-32 flex-col items-center justify-center gap-2 rounded-2xl border border-dashed border-border px-5 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
