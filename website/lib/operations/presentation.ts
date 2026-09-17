import { ADMIN_TEXT } from "@/constants/admin-text";
import {
  OPERATION_CONTROL_ACTION,
  OPERATION_KIND,
  OPERATION_REQUESTED_ACTION,
  OPERATION_STATUS,
} from "@/constants/operation";
import { CONTEST_RATING } from "@/constants/contest";
import type {
  Operation,
  OperationControlAction,
  OperationStatus,
} from "@/types/operation";

export function operationStatusLabel(status: OperationStatus): string {
  switch (status) {
    case OPERATION_STATUS.PENDING:
      return ADMIN_TEXT.OPERATIONS.STATUS.PENDING;
    case OPERATION_STATUS.RUNNING:
      return ADMIN_TEXT.OPERATIONS.STATUS.RUNNING;
    case OPERATION_STATUS.PAUSED:
      return ADMIN_TEXT.OPERATIONS.STATUS.PAUSED;
    case OPERATION_STATUS.COMPLETED:
      return ADMIN_TEXT.OPERATIONS.STATUS.COMPLETED;
    case OPERATION_STATUS.FAILED:
      return ADMIN_TEXT.OPERATIONS.STATUS.FAILED;
    case OPERATION_STATUS.CANCELLED:
      return ADMIN_TEXT.OPERATIONS.STATUS.CANCELLED;
  }
}

export function operationKindLabel(kind: string): string {
  switch (kind) {
    case OPERATION_KIND.NOTIFICATION_CAMPAIGN:
      return ADMIN_TEXT.OPERATIONS.KIND_LABELS.NOTIFICATION_CAMPAIGN;
	case OPERATION_KIND.CULTIVATION_RANKING_RECONCILIATION:
		return ADMIN_TEXT.OPERATIONS.KIND_LABELS.CULTIVATION_RANKING_RECONCILIATION;
    case OPERATION_KIND.IDENTITY_ATTRIBUTE_VALUE_MIGRATION:
      return ADMIN_TEXT.OPERATIONS.KIND_LABELS.IDENTITY_ATTRIBUTE_VALUE_MIGRATION;
    case CONTEST_RATING.RERATING_OPERATION_KIND:
      return ADMIN_TEXT.OPERATIONS.KIND_LABELS.CONTEST_RATING_RERATING;
    default:
      return kind;
  }
}

export function operationControlLabel(
  action: OperationControlAction,
): string {
  switch (action) {
    case OPERATION_CONTROL_ACTION.PAUSE:
      return ADMIN_TEXT.OPERATIONS.PAUSE;
    case OPERATION_CONTROL_ACTION.RESUME:
      return ADMIN_TEXT.OPERATIONS.RESUME;
    case OPERATION_CONTROL_ACTION.CANCEL:
      return ADMIN_TEXT.OPERATIONS.CANCEL_OPERATION;
  }
}

export function operationControlDescription(
  action: OperationControlAction,
): string {
  switch (action) {
    case OPERATION_CONTROL_ACTION.PAUSE:
      return ADMIN_TEXT.OPERATIONS.PAUSE_DESCRIPTION;
    case OPERATION_CONTROL_ACTION.RESUME:
      return ADMIN_TEXT.OPERATIONS.RESUME_DESCRIPTION;
    case OPERATION_CONTROL_ACTION.CANCEL:
      return ADMIN_TEXT.OPERATIONS.CANCEL_DESCRIPTION;
  }
}

export function operationRequestedActionLabel(
  operation: Operation,
): string | null {
  switch (operation.requested_action) {
    case OPERATION_REQUESTED_ACTION.NONE:
      return null;
    case OPERATION_REQUESTED_ACTION.PAUSE:
      return ADMIN_TEXT.OPERATIONS.REQUESTED_ACTION.PAUSE;
    case OPERATION_REQUESTED_ACTION.CANCEL:
      return ADMIN_TEXT.OPERATIONS.REQUESTED_ACTION.CANCEL;
  }
}

export function availableOperationControls(
  operation: Operation,
): OperationControlAction[] {
  if (operation.requested_action !== OPERATION_REQUESTED_ACTION.NONE) {
    return [];
  }
  switch (operation.status) {
    case OPERATION_STATUS.PENDING:
    case OPERATION_STATUS.RUNNING:
      return [
        OPERATION_CONTROL_ACTION.PAUSE,
        OPERATION_CONTROL_ACTION.CANCEL,
      ];
    case OPERATION_STATUS.PAUSED:
      return [
        OPERATION_CONTROL_ACTION.RESUME,
        OPERATION_CONTROL_ACTION.CANCEL,
      ];
    case OPERATION_STATUS.COMPLETED:
    case OPERATION_STATUS.FAILED:
    case OPERATION_STATUS.CANCELLED:
      return [];
  }
}
