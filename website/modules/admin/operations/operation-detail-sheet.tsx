"use client";

import {
  Clock3,
  Fingerprint,
  ShieldCheck,
  TimerReset,
  UserRound,
  Workflow,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  OPERATION_CONTROL_ACTION,
  OPERATION_REQUESTED_ACTION,
} from "@/constants/operation";
import { formatDateTime } from "@/lib/format";
import {
  availableOperationControls,
  operationControlLabel,
  operationKindLabel,
} from "@/lib/operations/presentation";
import type { Operation, OperationControlAction } from "@/types/operation";
import { OperationProgress } from "./operation-progress";
import { OperationBatchHistory } from "./operation-batch-history";
import { OperationStatusBadge } from "./operation-status";

interface OperationDetailSheetProps {
  operation: Operation;
  canExecute: boolean;
  onClose: () => void;
  onControl: (action: OperationControlAction) => void;
}

function Timestamp({
  label,
  value,
  fallback,
}: {
  label: string;
  value?: string;
  fallback: string;
}) {
  return (
    <div className="space-y-1">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="tabular-nums text-foreground">
        {value ? formatDateTime(value) : fallback}
      </dd>
    </div>
  );
}

function Identifier({
  label,
  value,
  icon: Icon,
}: {
  label: string;
  value: string;
  icon: typeof Fingerprint;
}) {
  return (
    <div className="min-w-0 space-y-1">
      <dt className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
        <Icon className="size-3.5" aria-hidden="true" />
        {label}
      </dt>
      <dd className="break-all font-mono text-xs text-foreground">{value}</dd>
    </div>
  );
}

export function OperationDetailSheet({
  operation,
  canExecute,
  onClose,
  onControl,
}: OperationDetailSheetProps) {
  const controls = availableOperationControls(operation);
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent className="w-full overflow-y-auto border-border bg-background sm:max-w-2xl">
        <SheetHeader className="border-b border-border px-5 pb-5 pt-5 sm:px-6">
          <div className="mb-2 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
            <Workflow className="size-5" aria-hidden="true" />
          </div>
          <SheetTitle className="text-xl font-semibold tracking-tight">
            {ADMIN_TEXT.OPERATIONS.DETAIL_TITLE}
          </SheetTitle>
          <SheetDescription className="max-w-xl leading-6">
            {ADMIN_TEXT.OPERATIONS.DETAIL_DESCRIPTION}
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-6 px-5 pb-6 sm:px-6">
          <section className="space-y-3 rounded-2xl border border-border bg-card p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="text-sm font-semibold text-foreground">
                  {operationKindLabel(operation.kind)}
                </p>
                <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
                  {operation.kind}
                </p>
                <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
                  {operation.id}
                </p>
              </div>
              <OperationStatusBadge operation={operation} />
            </div>
            <OperationProgress operation={operation} />
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline">
                {ADMIN_TEXT.OPERATIONS.VERSION(operation.version)}
              </Badge>
              <Badge variant="outline">
                {ADMIN_TEXT.OPERATIONS.BATCH_SIZE(operation.batch_size)}
              </Badge>
              <Badge variant="outline">
                {ADMIN_TEXT.OPERATIONS.ATTEMPT_COUNT(operation.attempt_count)}
              </Badge>
            </div>
          </section>

          <section className="space-y-3">
            <h3 className="flex items-center gap-2 text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
              <Clock3 className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.OPERATIONS.COLUMN_TIMING}
            </h3>
            <dl className="grid gap-4 rounded-2xl border border-border bg-card p-4 sm:grid-cols-2">
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.CREATED_AT}
                value={operation.created_at}
                fallback={TEXT.COMMON.NOT_AVAILABLE}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.UPDATED_AT_LABEL}
                value={operation.updated_at}
                fallback={TEXT.COMMON.NOT_AVAILABLE}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.STARTED_AT}
                value={operation.started_at}
                fallback={ADMIN_TEXT.OPERATIONS.NOT_STARTED}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.FINISHED_AT}
                value={operation.finished_at}
                fallback={ADMIN_TEXT.OPERATIONS.NOT_FINISHED}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.NOT_BEFORE}
                value={operation.not_before}
                fallback={ADMIN_TEXT.OPERATIONS.NO_SCHEDULE_FLOOR}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.AVAILABLE_AT}
                value={operation.available_at}
                fallback={TEXT.COMMON.NOT_AVAILABLE}
              />
              <Timestamp
                label={ADMIN_TEXT.OPERATIONS.LEASE_UNTIL}
                value={operation.lease?.until}
                fallback={ADMIN_TEXT.OPERATIONS.NO_LEASE}
              />
            </dl>
          </section>

          <section className="space-y-3">
            <h3 className="flex items-center gap-2 text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
              <ShieldCheck className="size-4" aria-hidden="true" />
              {ADMIN_TEXT.OPERATIONS.TRUST_LABEL}
            </h3>
            <dl className="grid gap-4 rounded-2xl border border-border bg-card p-4 sm:grid-cols-2">
              <Identifier
                label={ADMIN_TEXT.OPERATIONS.ACTOR}
                value={operation.actor_user_id}
                icon={UserRound}
              />
              <Identifier
                label={ADMIN_TEXT.OPERATIONS.AUDIT_ID}
                value={operation.audit_id}
                icon={ShieldCheck}
              />
              <div className="sm:col-span-2">
                <Identifier
                  label={ADMIN_TEXT.OPERATIONS.CORRELATION_ID}
                  value={operation.correlation_id}
                  icon={Fingerprint}
                />
              </div>
              <div className="space-y-1 sm:col-span-2">
                <dt className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <TimerReset className="size-3.5" aria-hidden="true" />
                  {ADMIN_TEXT.OPERATIONS.LAST_ERROR}
                </dt>
                <dd className="font-mono text-xs text-foreground">
                  {operation.last_error_code ?? ADMIN_TEXT.OPERATIONS.NO_ERROR}
                </dd>
              </div>
            </dl>
          </section>

          <OperationBatchHistory
            operationID={operation.id}
            operationVersion={operation.version}
          />
        </div>

        <SheetFooter className="sticky bottom-0 border-t border-border bg-background/95 px-5 py-4 backdrop-blur-sm supports-backdrop-filter:bg-background/85 sm:px-6">
          {canExecute && controls.length > 0 ? (
            <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
              {controls.map((action) => (
                <Button
                  key={action}
                  type="button"
                  variant={
                    action === OPERATION_CONTROL_ACTION.CANCEL
                      ? "destructive"
                      : "outline"
                  }
                  onClick={() => onControl(action)}
                >
                  {operationControlLabel(action)}
                </Button>
              ))}
            </div>
          ) : (
            <p className="text-sm leading-6 text-muted-foreground">
              {canExecute
                ? operation.requested_action !==
                  OPERATION_REQUESTED_ACTION.NONE
                  ? ADMIN_TEXT.OPERATIONS.REQUEST_PENDING
                  : ADMIN_TEXT.OPERATIONS.NO_CONTROL_AVAILABLE
                : ADMIN_TEXT.OPERATIONS.CONTROL_RESTRICTED}
            </p>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
