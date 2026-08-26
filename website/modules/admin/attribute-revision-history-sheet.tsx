"use client";

import type { ReactNode } from "react";
import { Database, History, Loader2, RotateCcw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useCursorFeed } from "@/hooks/use-cursor-feed";
import { attributeDataTypeLabel } from "@/lib/admin/auxiliary-form";
import { formatDateTime } from "@/lib/format";
import { attributeDefinitionService } from "@/services/attribute-definition.service";
import type {
  AttributeDefinition,
  AttributeDefinitionRevision,
} from "@/types/admin-auxiliary";

interface AttributeRevisionHistorySheetProps {
  definition: AttributeDefinition | null;
  open: boolean;
  canRestore: boolean;
  onClose: () => void;
  onRestore: (revision: AttributeDefinitionRevision) => void;
}

export function AttributeRevisionHistorySheet({
  definition,
  open,
  canRestore,
  onClose,
  onRestore,
}: AttributeRevisionHistorySheetProps) {
  const feed = useCursorFeed<AttributeDefinitionRevision>({
    resetKey: definition?.id,
    enabled: open && Boolean(definition),
    load: async (cursor, signal) => {
      if (!definition) return { items: [] };
      const page = await attributeDefinitionService.listRevisions(
        definition.id,
        cursor,
        signal,
      );
      return { items: page.records, next_cursor: page.next_cursor };
    },
    keyOf: (revision) => revision.id,
  });
  const loading = feed.status === "loading";
  const loadingMore = feed.status === "loading_more";

  return (
    <Sheet open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <SheetContent className="w-full gap-0 sm:max-w-xl" aria-busy={loading || loadingMore}>
        <SheetHeader className="border-b border-border px-5 py-5 pr-14">
          <div className="mb-2 flex h-11 w-11 items-center justify-center rounded-2xl border border-border bg-muted/60">
            <History className="h-5 w-5" aria-hidden="true" />
          </div>
          <SheetTitle className="text-lg">
            {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_TITLE}
          </SheetTitle>
          <SheetDescription>
            {definition
              ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_DESCRIPTION(definition.key)
              : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_DESCRIPTION_FALLBACK}
          </SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
          {loading ? (
            <HistoryState icon={<Loader2 className="h-5 w-5 animate-spin motion-reduce:animate-none" />}>
              {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_LOADING}
            </HistoryState>
          ) : feed.error && feed.items.length === 0 ? (
            <HistoryState icon={<Database className="h-5 w-5" />}>
              <span>{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_ERROR}</span>
              <Button type="button" variant="outline" size="sm" onClick={feed.reload}>
                {TEXT.COMMON.RETRY}
              </Button>
            </HistoryState>
          ) : feed.items.length === 0 ? (
            <HistoryState icon={<History className="h-5 w-5" />}>
              {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_EMPTY}
            </HistoryState>
          ) : (
            <ol className="space-y-3" aria-label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_LIST_LABEL}>
              {feed.items.map((revision) => {
                const active = revision.id === definition?.active_revision_id;
                const draft = revision.id === definition?.draft_revision_id && !active;
                const restorable = Boolean(
                  canRestore && definition?.status === "active" && !active,
                );
                return (
                  <li
                    key={revision.id}
                    className="rounded-2xl border border-border bg-card p-4 shadow-sm"
                  >
                    <div className="flex min-w-0 items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="font-semibold">
                            {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REVISION_LABEL(revision.revision)}
                          </p>
                          {active ? (
                            <Badge>{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_ACTIVE}</Badge>
                          ) : draft ? (
                            <Badge variant="secondary">
                              {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_DRAFT}
                            </Badge>
                          ) : null}
                        </div>
                        <p className="mt-1 truncate text-sm text-muted-foreground">
                          {revision.label}
                        </p>
                      </div>
                      {restorable ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="shrink-0"
                          onClick={() => onRestore(revision)}
                        >
                          <RotateCcw className="mr-2 h-4 w-4" aria-hidden="true" />
                          {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.RESTORE_ACTION}
                        </Button>
                      ) : null}
                    </div>

                    <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                      <RevisionFact
                        label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_DATA_TYPE}
                        value={attributeDataTypeLabel(revision.data_type)}
                      />
                      <RevisionFact
                        label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_VISIBILITY}
                        value={visibilityLabel(revision.visibility)}
                      />
                      <RevisionFact
                        label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_EDITABLE}
                        value={
                          revision.user_editable
                            ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_YES
                            : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_NO
                        }
                      />
                      <RevisionFact
                        label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_CREATED_AT}
                        value={formatDateTime(revision.created_at)}
                      />
                    </dl>
                    {revision.description ? (
                      <p className="mt-4 border-t border-border pt-3 text-sm leading-relaxed text-muted-foreground">
                        {revision.description}
                      </p>
                    ) : null}
                  </li>
                );
              })}
            </ol>
          )}

          {feed.error && feed.items.length > 0 ? (
            <div className="mt-4 flex items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
              <span>{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_MORE_ERROR}</span>
              <Button type="button" variant="outline" size="sm" onClick={() => void feed.loadMore()}>
                {TEXT.COMMON.RETRY}
              </Button>
            </div>
          ) : null}
          {feed.hasMore ? (
            <Button
              type="button"
              variant="outline"
              className="mt-4 w-full"
              disabled={loadingMore}
              onClick={() => void feed.loadMore()}
            >
              {loadingMore ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {loadingMore
                ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_LOADING_MORE
                : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.HISTORY_LOAD_MORE}
            </Button>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function RevisionFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 truncate font-medium">{value}</dd>
    </div>
  );
}

function HistoryState({
  icon,
  children,
}: {
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-52 flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-border px-6 text-center text-sm text-muted-foreground">
      <span className="text-foreground">{icon}</span>
      {children}
    </div>
  );
}

function visibilityLabel(value: AttributeDefinitionRevision["visibility"]): string {
  switch (value) {
    case "public":
      return ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_PUBLIC;
    case "private":
      return ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_PRIVATE;
    case "admin":
      return ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_ADMIN;
  }
}
