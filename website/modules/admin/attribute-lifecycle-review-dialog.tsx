"use client";

import { useId, useState } from "react";
import {
  Archive,
  ArrowRight,
  CheckCircle2,
  DatabaseZap,
  Loader2,
  ShieldCheck,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { attributeDataTypeLabel } from "@/lib/admin/auxiliary-form";
import { cn } from "@/lib/utils";
import type {
  AttributeDataType,
  AttributeDefinition,
  AttributeLifecycleAction,
  ReviewedAttributeLifecycleCommand,
} from "@/types/admin-auxiliary";

interface AttributeLifecycleReviewDialogProps {
  definition: AttributeDefinition | null;
  candidateRevision?: AttributeDefinition["draft_revision"] | null;
  action: AttributeLifecycleAction | null;
  reviewed: ReviewedAttributeLifecycleCommand | null;
  isPreviewing: boolean;
  isApplying: boolean;
  reauthenticationRequired: boolean;
  error: string | null;
  onPreview: (reason: string) => void;
  onApply: (password: string) => void;
  onClose: () => void;
}

export function AttributeLifecycleReviewDialog({
  definition,
  candidateRevision,
  action,
  reviewed,
  isPreviewing,
  isApplying,
  reauthenticationRequired,
  error,
  onPreview,
  onApply,
  onClose,
}: AttributeLifecycleReviewDialogProps) {
  const reasonID = useId();
  const passwordID = useId();
  const [reason, setReason] = useState("");
  const [password, setPassword] = useState("");
  const busy = isPreviewing || isApplying;
  const normalizedReason = reason.trim();
  const reasonValid = normalizedReason.length >= 3 && normalizedReason.length <= 500;

  if (!definition || !action) return null;

  const preview = reviewed?.review.preview;
  const candidate = preview?.candidate_revision ?? candidateRevision ?? definition.draft_revision;
  const current = preview?.current_revision ?? definition.active_revision;
  const archive = action === "archive";
  const restoring = Boolean(
    !archive && candidate && candidate.id !== definition.draft_revision_id,
  );
  const Icon = archive ? Archive : CheckCircle2;

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose(); }}>
      <DialogContent
        className="max-h-[92dvh] overflow-y-auto sm:max-w-2xl"
        aria-busy={busy}
      >
        <DialogHeader className="min-w-0 pr-7">
          <div className="mb-2 flex h-11 w-11 items-center justify-center rounded-2xl border border-border bg-muted/60 text-foreground">
            <Icon className="h-5 w-5" aria-hidden="true" />
          </div>
          <DialogTitle className="break-words text-xl leading-snug">
            {archive
              ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVE_TITLE
              : restoring
                ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.RESTORE_TITLE
                : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ACTIVATE_TITLE}
          </DialogTitle>
          <DialogDescription className="break-words">
            {archive
              ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVE_DESCRIPTION
              : restoring
                ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.RESTORE_DESCRIPTION
                : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ACTIVATE_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-1">
          <div className="grid gap-3 rounded-2xl border border-border bg-muted/25 p-4 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] sm:items-center">
            <RevisionSummary
              eyebrow={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.CURRENT_REVISION}
              label={current?.label ?? definition.key}
              revision={current?.revision}
              dataType={current?.data_type ?? definition.active_revision.data_type}
            />
            <ArrowRight className="hidden h-4 w-4 text-muted-foreground sm:block" aria-hidden="true" />
            <RevisionSummary
              eyebrow={
                archive
                  ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVED_STATE
                  : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.CANDIDATE_REVISION
              }
              label={archive ? definition.key : (candidate?.label ?? definition.key)}
              revision={archive ? undefined : candidate?.revision}
              dataType={archive
                ? definition.active_revision.data_type
                : (candidate?.data_type ?? definition.draft_revision.data_type)}
              align="right"
            />
          </div>

          {preview ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <Metric
                label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VERSION_CHANGE}
                value={`${preview.expected_version} → ${preview.resulting_version}`}
              />
              <Metric
                label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.AFFECTED_VALUES}
                value={String(preview.source_value_count)}
              />
              <Metric
                label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.RETAINED_TARGET_VALUES}
                value={String(preview.target_value_count)}
              />
              <Metric
                label={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION}
                value={
                  preview.requires_migration
                    ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION_REQUIRED
                    : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION_NOT_REQUIRED
                }
              />
            </div>
          ) : null}

          {preview?.requires_migration ? (
            <div className="flex gap-3 rounded-2xl border border-amber-500/35 bg-amber-500/10 p-4 text-sm text-foreground">
              <DatabaseZap className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" aria-hidden="true" />
              <div className="space-y-1">
                <p className="font-medium">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION_NOTICE_TITLE}</p>
                <p className="leading-relaxed text-muted-foreground">
                  {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.MIGRATION_NOTICE_DESCRIPTION}
                </p>
              </div>
            </div>
          ) : null}

          <div className="space-y-2">
            <Label htmlFor={reasonID}>{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REASON}</Label>
            <Textarea
              id={reasonID}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.LIFECYCLE_REASON_PLACEHOLDER}
              maxLength={500}
              readOnly={Boolean(reviewed)}
              aria-readonly={Boolean(reviewed)}
              aria-invalid={reason.length > 0 && !reasonValid}
              className="min-h-24 resize-y"
            />
            <p className="text-xs text-muted-foreground">
              {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REASON_HELP}
            </p>
          </div>

          {reauthenticationRequired ? (
            <div className="space-y-3 rounded-2xl border border-border p-4">
              <div className="flex items-start gap-3">
                <ShieldCheck className="mt-0.5 h-5 w-5 shrink-0 text-primary" aria-hidden="true" />
                <div>
                  <p className="font-medium">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REAUTH_TITLE}</p>
                  <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                    {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REAUTH_DESCRIPTION}
                  </p>
                </div>
              </div>
              <Label htmlFor={passwordID}>{TEXT.AUTH.PASSWORD}</Label>
              <Input
                id={passwordID}
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={busy}
              />
            </div>
          ) : null}

          {error ? (
            <p
              className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
              role="alert"
              aria-live="assertive"
            >
              {error}
            </p>
          ) : null}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
            {TEXT.COMMON.CANCEL}
          </Button>
          {reviewed ? (
            <Button
              type="button"
              variant={archive ? "destructive" : "default"}
              onClick={() => onApply(password)}
              disabled={busy || (reauthenticationRequired && password.length === 0)}
            >
              {isApplying ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
              {isApplying
                ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.APPLYING
                : archive
                  ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ARCHIVE_APPLY
                  : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.ACTIVATE_APPLY}
            </Button>
          ) : (
            <Button
              type="button"
              onClick={() => onPreview(normalizedReason)}
              disabled={busy || !reasonValid}
            >
              {isPreviewing ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
              {isPreviewing
                ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REVIEWING
                : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REVIEW_ACTION}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RevisionSummary({
  eyebrow,
  label,
  revision,
  dataType,
  align = "left",
}: {
  eyebrow: string;
  label: string;
  revision?: number;
  dataType: AttributeDataType;
  align?: "left" | "right";
}) {
  return (
    <div className={cn("min-w-0", align === "right" && "sm:text-right")}>
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">{eyebrow}</p>
      <p className="mt-1 truncate font-semibold">{label}</p>
      <p className="mt-1 text-xs text-muted-foreground">
        {revision
          ? `${ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REVISION_LABEL(revision)} · `
          : ""}
        {attributeDataTypeLabel(dataType)}
      </p>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border px-3 py-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 font-medium tabular-nums">{value}</p>
    </div>
  );
}
