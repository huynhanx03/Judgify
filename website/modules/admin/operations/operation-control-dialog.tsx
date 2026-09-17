"use client";

import { useMemo, useState } from "react";
import { AlertTriangle, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  OPERATION_CONTROL_ACTION,
  OPERATION_CONTROL_REASON_LIMITS,
} from "@/constants/operation";
import {
  operationControlDescription,
  operationControlLabel,
  operationKindLabel,
} from "@/lib/operations/presentation";
import type { Operation, OperationControlAction } from "@/types/operation";

interface OperationControlDialogProps {
  operation: Operation | null;
  action: OperationControlAction | null;
  pending: boolean;
  error: string | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
}

const encoder = new TextEncoder();

function validReason(reason: string): boolean {
  const normalized = reason.trim();
  return (
    normalized.length >=
      OPERATION_CONTROL_REASON_LIMITS.MINIMUM_CHARACTERS &&
    encoder.encode(normalized).byteLength <=
      OPERATION_CONTROL_REASON_LIMITS.MAXIMUM_BYTES &&
    !/[\p{C}]/u.test(normalized)
  );
}

export function OperationControlDialog({
  operation,
  action,
  pending,
  error,
  onOpenChange,
  onConfirm,
}: OperationControlDialogProps) {
  const [reason, setReason] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const open = Boolean(operation && action);
  const isValid = useMemo(() => validReason(reason), [reason]);

  if (!operation || !action) return null;
  const actionLabel = operationControlLabel(action);
  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!pending) onOpenChange(nextOpen);
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-warning/25 bg-warning/10 text-warning">
            <AlertTriangle className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>
            {ADMIN_TEXT.OPERATIONS.CONTROL_TITLE(actionLabel)}
          </DialogTitle>
          <DialogDescription className="leading-6">
            {operationControlDescription(action)}
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-xl border border-border bg-muted/35 p-3">
          <p className="text-sm font-semibold text-foreground">
            {operationKindLabel(operation.kind)}
          </p>
          <p className="mt-1 font-mono text-xs text-muted-foreground">
            {operation.kind}
          </p>
          <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
            {operation.id} · {ADMIN_TEXT.OPERATIONS.VERSION(operation.version)}
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="operation-control-reason">
            {ADMIN_TEXT.OPERATIONS.REASON}
          </Label>
          <Textarea
            id="operation-control-reason"
            value={reason}
            disabled={pending}
            aria-invalid={submitted && !isValid}
            aria-describedby="operation-control-reason-help"
            placeholder={ADMIN_TEXT.OPERATIONS.REASON_PLACEHOLDER}
            onChange={(event) => {
              setReason(event.target.value);
              setSubmitted(false);
            }}
          />
          <p
            id="operation-control-reason-help"
            className="text-xs leading-5 text-muted-foreground"
          >
            {submitted && !isValid
              ? ADMIN_TEXT.OPERATIONS.REASON_INVALID
              : ADMIN_TEXT.OPERATIONS.REASON_HELP(
                  OPERATION_CONTROL_REASON_LIMITS.MINIMUM_CHARACTERS,
                  OPERATION_CONTROL_REASON_LIMITS.MAXIMUM_BYTES,
                )}
          </p>
        </div>

        {error ? (
          <div
            className="rounded-xl border border-destructive/25 bg-destructive/10 p-3 text-sm text-destructive"
            role="alert"
          >
            {error}
          </div>
        ) : null}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {TEXT.COMMON.CANCEL}
          </Button>
          <Button
            type="button"
            variant={
              action === OPERATION_CONTROL_ACTION.CANCEL
                ? "destructive"
                : "default"
            }
            disabled={pending}
            onClick={() => {
              setSubmitted(true);
              if (isValid) onConfirm(reason.trim());
            }}
          >
            {pending ? (
              <LoaderCircle
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : null}
            {pending
              ? ADMIN_TEXT.OPERATIONS.CONTROL_PENDING
              : actionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
