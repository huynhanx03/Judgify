"use client";

import { useState } from "react";
import { AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { SubmissionDetail } from "@/modules/problem/submission-detail";
import { submissionService } from "@/services/submission.service";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { getErrorMessage, notify } from "@/lib/toast";
import { ApiError } from "@/lib/api/error";
import { ADMIN_JUDGE_OPERATION_REASON_LIMITS } from "@/constants/submission";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import type { AdminSubmissionDetail } from "@/types/submission";

interface AdminSubmissionDetailDialogProps {
  submissionId: string | null;
  onOpenChange: (open: boolean) => void;
  canUpdate: boolean;
  onOperationSuccess: () => void;
}

export function AdminSubmissionDetailDialog({
  submissionId,
  onOpenChange,
  canUpdate,
  onOperationSuccess,
}: AdminSubmissionDetailDialogProps) {
  const detailResource = useRetryableResource<AdminSubmissionDetail | null>({
    resetKey: submissionId,
    enabled: submissionId !== null,
    initialData: null,
    load: (signal) => {
      if (!submissionId) {
        return Promise.reject(new TypeError("submission ID is required"));
      }
      return submissionService.getAdminById(submissionId, signal);
    },
  });
  const submission = detailResource.data;
  const detailError =
    detailResource.status === "error"
      ? getErrorMessage(
          detailResource.error,
          ADMIN_TEXT.SUBMISSIONS.DETAIL_ERROR_DESCRIPTION,
        )
      : null;

  return (
    <Dialog open={submissionId !== null} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[min(90vh,900px)] grid-rows-[auto_minmax(0,1fr)] overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border px-5 pb-4 pt-5 pr-14">
          <DialogTitle>{ADMIN_TEXT.SUBMISSIONS.DETAIL_TITLE}</DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.SUBMISSIONS.DETAIL_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        {detailResource.status === "loading" ||
        detailResource.status === "idle" ? (
          <div
            className="flex min-h-72 items-center justify-center gap-3 text-sm text-muted-foreground"
            role="status"
            aria-live="polite"
          >
            <Loader2
              className="size-5 animate-spin text-primary motion-reduce:animate-none"
              aria-hidden="true"
            />
            <span>{ADMIN_TEXT.SUBMISSIONS.DETAIL_LOADING}</span>
          </div>
        ) : null}

        {detailResource.status === "error" ? (
          <div
            className="flex min-h-72 flex-col items-center justify-center px-6 text-center"
            role="alert"
          >
            <span className="flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
              <AlertTriangle className="size-5" aria-hidden="true" />
            </span>
            <h2 className="mt-4 font-semibold">
              {ADMIN_TEXT.SUBMISSIONS.DETAIL_ERROR_TITLE}
            </h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
              {detailError}
            </p>
            <Button
              type="button"
              variant="outline"
              className="mt-5 min-h-11 cursor-pointer gap-2"
              onClick={detailResource.retry}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.SUBMISSIONS.RETRY_DETAIL}
            </Button>
          </div>
        ) : null}

        {detailResource.status === "ready" && submission ? (
          <div className="min-h-0 overflow-y-auto py-4 overscroll-contain">
            <div className="mb-4 flex flex-wrap items-center gap-2 px-4 text-xs text-muted-foreground">
              <span className="rounded-full border border-border bg-muted/40 px-2.5 py-1 font-mono">
                {submission.id}
              </span>
              <span className="rounded-full border border-border bg-muted/40 px-2.5 py-1">
                {ADMIN_TEXT.SUBMISSIONS.VERSION(
                  submission.verdict_version,
                )}
              </span>
            </div>
            <SubmissionDetail submission={submission} />
            <AdminSubmissionOperationPanel
              key={`${submission.id}:${canUpdate}`}
              submission={submission}
              canUpdate={canUpdate}
              onSuccess={() => {
                detailResource.retry();
                onOperationSuccess();
              }}
            />
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

type OperationKind = "rejudge" | "cancel";

function AdminSubmissionOperationPanel({
  submission,
  canUpdate,
  onSuccess,
}: {
  submission: AdminSubmissionDetail;
  canUpdate: boolean;
  onSuccess: () => void;
}) {
  const [operation, setOperation] = useState<OperationKind | null>(null);
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState<"conflict" | "error" | null>(null);
  const isTerminal =
    submission.latest_generation_phase === "terminal" ||
    submission.latest_generation_phase === "cancelled";
  const normalizedReason = reason.trim();
  const reasonIsValid =
    normalizedReason.length >=
      ADMIN_JUDGE_OPERATION_REASON_LIMITS.MINIMUM_CHARACTERS &&
    normalizedReason.length <=
      ADMIN_JUDGE_OPERATION_REASON_LIMITS.MAXIMUM_CHARACTERS &&
    !/[\p{C}]/u.test(normalizedReason);

  async function confirmOperation() {
    if (!canUpdate || !operation || !reasonIsValid || pending) return;
    setPending(true);
    setNotice(null);
    try {
      if (operation === "rejudge") {
        await submissionService.rejudgeAdmin(submission.id, {
          expected_version: submission.aggregate_version,
          reason: normalizedReason,
        });
        notify.success(ADMIN_TEXT.SUBMISSIONS.REJUDGE_SUCCESS);
      } else {
        await submissionService.cancelAdmin(submission.id, {
          generation: submission.latest_generation,
          expected_version: submission.aggregate_version,
          reason: normalizedReason,
        });
        notify.success(ADMIN_TEXT.SUBMISSIONS.CANCEL_SUCCESS);
      }
      setOperation(null);
      setReason("");
      onSuccess();
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setNotice("conflict");
        notify.error(ADMIN_TEXT.SUBMISSIONS.OPERATION_CONFLICT);
        onSuccess();
      } else {
        setNotice("error");
        notify.error(ADMIN_TEXT.SUBMISSIONS.OPERATION_ERROR);
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <section
      className="mx-4 mt-5 rounded-xl border border-border bg-muted/20 p-4"
      aria-labelledby="submission-operations-title"
    >
      <h2 id="submission-operations-title" className="text-sm font-semibold">
        {ADMIN_TEXT.SUBMISSIONS.OPERATIONS_TITLE}
      </h2>
      <p className="mt-1 text-sm leading-6 text-muted-foreground">
        {ADMIN_TEXT.SUBMISSIONS.OPERATIONS_DESCRIPTION}
      </p>
      {canUpdate ? (
        <div className="mt-4 flex flex-col gap-2 sm:flex-row">
          {isTerminal ? (
            <Button
              type="button"
              variant="outline"
              className="min-h-11 cursor-pointer"
              onClick={() => setOperation("rejudge")}
            >
              {ADMIN_TEXT.SUBMISSIONS.REJUDGE}
            </Button>
          ) : (
            <Button
              type="button"
              variant="destructive"
              className="min-h-11 cursor-pointer"
              onClick={() => setOperation("cancel")}
            >
              {ADMIN_TEXT.SUBMISSIONS.CANCEL_JUDGING}
            </Button>
          )}
        </div>
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">
          {ADMIN_TEXT.SUBMISSIONS.UPDATE_RESTRICTED}
        </p>
      )}
      {notice ? (
        <p className="mt-3 text-sm text-destructive" role="alert">
          {notice === "conflict"
            ? ADMIN_TEXT.SUBMISSIONS.OPERATION_CONFLICT
            : ADMIN_TEXT.SUBMISSIONS.OPERATION_ERROR}
        </p>
      ) : null}

      <ConfirmDialog
        open={operation !== null}
        onOpenChange={(open) => {
          if (!open && !pending) setOperation(null);
        }}
        title={
          operation === "rejudge"
            ? ADMIN_TEXT.SUBMISSIONS.REJUDGE_TITLE
            : ADMIN_TEXT.SUBMISSIONS.CANCEL_TITLE
        }
        description={
          operation === "rejudge"
            ? ADMIN_TEXT.SUBMISSIONS.REJUDGE_DESCRIPTION
            : ADMIN_TEXT.SUBMISSIONS.CANCEL_DESCRIPTION
        }
        confirmLabel={
          operation === "rejudge"
            ? ADMIN_TEXT.SUBMISSIONS.REJUDGE_CONFIRM
            : ADMIN_TEXT.SUBMISSIONS.CANCEL_CONFIRM
        }
        confirmVariant={operation === "cancel" ? "destructive" : "default"}
        loading={pending}
        confirmDisabled={!canUpdate || !reasonIsValid}
        onConfirm={confirmOperation}
      >
        <div className="space-y-2">
          <Label htmlFor="submission-operation-reason">
            {ADMIN_TEXT.SUBMISSIONS.REASON_LABEL}
          </Label>
          <Textarea
            id="submission-operation-reason"
            value={reason}
            maxLength={ADMIN_JUDGE_OPERATION_REASON_LIMITS.MAXIMUM_CHARACTERS}
            onChange={(event) => setReason(event.target.value)}
            placeholder={ADMIN_TEXT.SUBMISSIONS.REASON_PLACEHOLDER}
            aria-invalid={reason.length > 0 && !reasonIsValid}
            aria-describedby="submission-operation-reason-help"
          />
          <p
            id="submission-operation-reason-help"
            className="text-xs text-muted-foreground"
          >
            {ADMIN_TEXT.SUBMISSIONS.REASON_HELP(
              ADMIN_JUDGE_OPERATION_REASON_LIMITS.MINIMUM_CHARACTERS,
              ADMIN_JUDGE_OPERATION_REASON_LIMITS.MAXIMUM_CHARACTERS,
            )}
          </p>
          {reason.length > 0 && !reasonIsValid ? (
            <p className="text-xs text-destructive">
              {ADMIN_TEXT.SUBMISSIONS.REASON_INVALID}
            </p>
          ) : null}
          {pending ? (
            <p
              className="text-xs text-muted-foreground"
              role="status"
              aria-live="polite"
            >
              {ADMIN_TEXT.SUBMISSIONS.OPERATION_PENDING}
            </p>
          ) : null}
        </div>
      </ConfirmDialog>
    </section>
  );
}
