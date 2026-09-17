import type {
  CorrelationID,
  Cursor,
  EntityID,
  ISODateTime,
} from "@/types/api";
import {
  OPERATION_CONTROL_ACTION,
  OPERATION_REQUESTED_ACTION,
  OPERATION_STATUS,
} from "@/constants/operation";

export type OperationStatus =
  (typeof OPERATION_STATUS)[keyof typeof OPERATION_STATUS];

export type OperationControlAction =
  (typeof OPERATION_CONTROL_ACTION)[keyof typeof OPERATION_CONTROL_ACTION];

export type OperationRequestedAction =
  (typeof OPERATION_REQUESTED_ACTION)[keyof typeof OPERATION_REQUESTED_ACTION];

export interface OperationLease {
  operation_id: EntityID;
  until: ISODateTime;
}

/** Safe administrative projection; request payload and lease token never leave the API. */
export interface Operation {
  id: EntityID;
  kind: string;
  status: OperationStatus;
  total: number;
  processed: number;
  succeeded: number;
  failed: number;
  batch_size: number;
  version: number;
  lease?: OperationLease;
  requested_action: OperationRequestedAction;
  attempt_count: number;
  last_error_code?: string;
  actor_user_id: EntityID;
  audit_id: EntityID;
  correlation_id: CorrelationID;
  not_before?: ISODateTime;
  available_at: ISODateTime;
  created_at: ISODateTime;
  updated_at: ISODateTime;
  started_at?: ISODateTime;
  finished_at?: ISODateTime;
}

export interface OperationPage {
  items: Operation[];
  next_cursor?: Cursor;
}

export interface OperationBatchFailure {
  reference: string;
  code: string;
}

export interface OperationBatch {
  operation_id: EntityID;
  sequence: number;
  processed: number;
  succeeded: number;
  failed: number;
  failures: OperationBatchFailure[];
  started_at: ISODateTime;
  completed_at: ISODateTime;
}

export interface OperationBatchPage {
  items: OperationBatch[];
  next_cursor?: string;
}

export interface OperationListQuery {
  status?: OperationStatus;
  kind?: string;
  actor_id?: EntityID;
  cursor?: Cursor;
  limit?: number;
}

export interface OperationControlCommand {
  action: OperationControlAction;
  expected_version: number;
  reason: string;
}

export interface OperationControlResult {
  operation: Operation;
  audit_id: EntityID;
  replay: boolean;
}
