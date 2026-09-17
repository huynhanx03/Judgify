import { ADMIN_TEXT } from "@/constants/admin-text";
import { OPERATION_STATUS } from "@/constants/operation";
import type { Operation } from "@/types/operation";

function operationPercentage(operation: Operation): number {
  if (operation.total === 0) {
    return operation.status === OPERATION_STATUS.COMPLETED ? 100 : 0;
  }
  return Math.max(
    0,
    Math.min(100, (operation.processed / operation.total) * 100),
  );
}

export function OperationProgress({ operation }: { operation: Operation }) {
  const percentage = operationPercentage(operation);
  const accessibleMaximum = Math.max(1, operation.total);
  const accessibleValue =
    operation.total === 0 && percentage === 100 ? 1 : operation.processed;
  return (
    <div className="min-w-44 space-y-2">
      <div className="flex items-center justify-between gap-3 text-xs tabular-nums">
        <span className="font-medium text-foreground">
          {ADMIN_TEXT.OPERATIONS.PROGRESS_COUNT(
            operation.processed,
            operation.total,
          )}
        </span>
        <span className="text-muted-foreground">{Math.round(percentage)}%</span>
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={accessibleMaximum}
        aria-valuenow={accessibleValue}
        aria-valuetext={`${Math.round(percentage)}%`}
        aria-label={ADMIN_TEXT.OPERATIONS.COLUMN_PROGRESS}
      >
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-300 motion-reduce:transition-none"
          style={{ width: `${percentage}%` }}
        />
      </div>
      <p className="text-xs tabular-nums text-muted-foreground">
        {ADMIN_TEXT.OPERATIONS.SUCCESS_FAILURE(
          operation.succeeded,
          operation.failed,
        )}
      </p>
    </div>
  );
}
