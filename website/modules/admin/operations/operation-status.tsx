import { Badge } from "@/components/ui/badge";
import {
  OPERATION_REQUESTED_ACTION,
  OPERATION_STATUS,
} from "@/constants/operation";
import {
  operationRequestedActionLabel,
  operationStatusLabel,
} from "@/lib/operations/presentation";
import type { Operation } from "@/types/operation";

export function OperationStatusBadge({
  operation,
}: {
  operation: Operation;
}) {
  const requestedAction = operationRequestedActionLabel(operation);
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Badge
        variant={
          operation.status === OPERATION_STATUS.FAILED
            ? "destructive"
            : operation.status === OPERATION_STATUS.RUNNING ||
                operation.status === OPERATION_STATUS.COMPLETED
              ? "default"
              : operation.status === OPERATION_STATUS.PAUSED
                ? "secondary"
                : "outline"
        }
      >
        {operationStatusLabel(operation.status)}
      </Badge>
      {operation.requested_action !== OPERATION_REQUESTED_ACTION.NONE &&
      requestedAction ? (
        <Badge variant="outline" className="border-warning/40 text-warning">
          {requestedAction}
        </Badge>
      ) : null}
    </div>
  );
}
