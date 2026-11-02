"use client";

import {
  AlertTriangle,
  Check,
  Clipboard,
  Fingerprint,
  ShieldCheck,
} from "lucide-react";
import { useState } from "react";
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
import { formatDateTime } from "@/lib/format";
import type { AuditLogEntry } from "@/types/audit";

interface AuditDetailSheetProps {
  entry: AuditLogEntry;
  onClose: () => void;
}

function formattedData(value: unknown): string | null {
  if (value === undefined || value === null) return null;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function DataSnapshot({ label, value }: { label: string; value: unknown }) {
  const content = formattedData(value);
  return (
    <section className="space-y-2" aria-label={label}>
      <h3 className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
        {label}
      </h3>
      {content ? (
        <pre className="max-h-72 overflow-auto rounded-xl border border-border bg-muted/45 p-4 font-mono text-xs leading-5 text-foreground">
          {content}
        </pre>
      ) : (
        <p className="rounded-xl border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">
          {ADMIN_TEXT.AUDIT.NO_DIFF}
        </p>
      )}
    </section>
  );
}

export function AuditDetailSheet({ entry, onClose }: AuditDetailSheetProps) {
  const [copyStatus, setCopyStatus] = useState<"idle" | "success" | "error">(
    "idle",
  );

  async function copyCorrelationId() {
    if (!entry.correlation_id) return;
    setCopyStatus("idle");
    try {
      await navigator.clipboard.writeText(entry.correlation_id);
      setCopyStatus("success");
      window.setTimeout(() => setCopyStatus("idle"), 1_500);
    } catch {
      setCopyStatus("error");
    }
  }

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent className="w-full overflow-y-auto border-border bg-background sm:max-w-xl">
        <SheetHeader className="border-b border-border px-5 pb-5 pt-5 sm:px-6">
              <div className="mb-2 flex h-11 w-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
                <ShieldCheck className="h-5 w-5" aria-hidden="true" />
              </div>
              <SheetTitle className="text-xl font-semibold tracking-tight">
                {ADMIN_TEXT.AUDIT.DETAIL_TITLE}
              </SheetTitle>
              <SheetDescription className="max-w-md leading-6">
                {ADMIN_TEXT.AUDIT.DETAIL_DESCRIPTION}
              </SheetDescription>
        </SheetHeader>

        <div className="space-y-6 px-5 pb-8 sm:px-6">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary" className="font-mono">
                  {entry.action}
                </Badge>
                <Badge variant="outline" className="font-mono">
                  {entry.resource}
                </Badge>
              </div>

              <dl className="grid gap-4 rounded-2xl border border-border bg-card p-4 text-sm sm:grid-cols-2">
                <div className="min-w-0 space-y-1 sm:col-span-2">
                  <dt className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.AUDIT.EVENT_ID}
                  </dt>
                  <dd className="break-all font-mono text-xs text-foreground">
                    {entry.id}
                  </dd>
                </div>
                <div className="min-w-0 space-y-1">
                  <dt className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.AUDIT.ACTOR_USER_ID}
                  </dt>
                  <dd className="break-all font-mono text-xs text-foreground">
                    {entry.actor_user_id ?? ADMIN_TEXT.AUDIT.SYSTEM_ACTOR}
                  </dd>
                </div>
                <div className="min-w-0 space-y-1">
                  <dt className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.AUDIT.TARGET_LABEL}
                  </dt>
                  <dd className="break-all font-mono text-xs text-foreground">
                    {entry.resource_id ?? TEXT.COMMON.NOT_AVAILABLE}
                  </dd>
                </div>
                <div className="space-y-1 sm:col-span-2">
                  <dt className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.AUDIT.OCCURRED_AT}
                  </dt>
                  <dd className="tabular-nums text-foreground">
                    {formatDateTime(entry.occurred_at)}
                  </dd>
                </div>
                <div className="space-y-1 sm:col-span-2">
                  <dt className="text-xs font-medium text-muted-foreground">
                    {ADMIN_TEXT.AUDIT.REASON}
                  </dt>
                  <dd className="whitespace-pre-wrap leading-6 text-foreground">
                    {entry.reason ?? TEXT.COMMON.NOT_AVAILABLE}
                  </dd>
                </div>
              </dl>

              <section className="space-y-2">
                <h3 className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                  <Fingerprint className="h-4 w-4" aria-hidden="true" />
                  {ADMIN_TEXT.AUDIT.CORRELATION_ID}
                </h3>
                <div className="flex min-w-0 items-center gap-2 rounded-xl border border-border bg-muted/45 p-2 pl-3">
                  <code className="min-w-0 flex-1 break-all text-xs text-foreground">
                    {entry.correlation_id}
                  </code>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="min-h-11 min-w-11 shrink-0"
                    onClick={() => void copyCorrelationId()}
                    aria-label={ADMIN_TEXT.AUDIT.COPY_CID}
                    title={
                      copyStatus === "success"
                        ? ADMIN_TEXT.AUDIT.COPY_CID_SUCCESS
                        : copyStatus === "error"
                          ? ADMIN_TEXT.AUDIT.COPY_CID_ERROR
                          : ADMIN_TEXT.AUDIT.COPY_CID
                    }
                  >
                    {copyStatus === "success" ? (
                      <Check className="h-4 w-4" aria-hidden="true" />
                    ) : copyStatus === "error" ? (
                      <AlertTriangle className="h-4 w-4" aria-hidden="true" />
                    ) : (
                      <Clipboard className="h-4 w-4" aria-hidden="true" />
                    )}
                  </Button>
                </div>
                <span className="sr-only" aria-live="polite">
                  {copyStatus === "success"
                    ? ADMIN_TEXT.AUDIT.COPY_CID_SUCCESS
                    : copyStatus === "error"
                      ? ADMIN_TEXT.AUDIT.COPY_CID_ERROR
                      : null}
                </span>
              </section>

          <DataSnapshot label={ADMIN_TEXT.AUDIT.BEFORE} value={entry.before} />
          <DataSnapshot label={ADMIN_TEXT.AUDIT.AFTER} value={entry.after} />
        </div>
      </SheetContent>
    </Sheet>
  );
}
