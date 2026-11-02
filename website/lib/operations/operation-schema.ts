import {
  arraySchema,
  booleanSchema,
  ContractError,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  compareISODateTime,
  correlationIDSchema,
  cursorSchema,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  OPERATION_CONTROL_ACTION,
  OPERATION_KIND_MAXIMUM_LENGTH,
  OPERATION_KIND_PATTERN,
  OPERATION_REQUESTED_ACTION,
  OPERATION_STATUS,
  OPERATION_STATUSES,
} from "@/constants/operation";
import type {
  Operation,
  OperationBatch,
  OperationBatchPage,
  OperationControlResult,
  OperationLease,
  OperationPage,
} from "@/types/operation";

const nonNegativeCount = integerSchema({
  minimum: 0,
  label: "operation count",
});
const positiveVersion = integerSchema({
  minimum: 1,
  label: "operation version",
});
const operationKind = stringSchema({
  minimumLength: 1,
  maximumLength: OPERATION_KIND_MAXIMUM_LENGTH,
  pattern: OPERATION_KIND_PATTERN,
  label: "operation kind",
});
const safeFailureCode = stringSchema({
  minimumLength: 1,
  maximumLength: 64,
  pattern: OPERATION_KIND_PATTERN,
  label: "operation failure code",
});

const operationLeaseSchema: Schema<OperationLease> = strictObjectSchema({
  operation_id: entityIDSchema,
  until: isoDateTimeSchema,
});

const rawOperationSchema = strictObjectSchema({
  id: entityIDSchema,
  kind: operationKind,
  status: enumSchema(OPERATION_STATUSES),
  total: nonNegativeCount,
  processed: nonNegativeCount,
  succeeded: nonNegativeCount,
  failed: nonNegativeCount,
  batch_size: integerSchema({
    minimum: 1,
    maximum: 1000,
    label: "operation batch size",
  }),
  version: positiveVersion,
  lease: optionalSchema(operationLeaseSchema),
  requested_action: enumSchema(
    Object.values(OPERATION_REQUESTED_ACTION),
  ),
  attempt_count: nonNegativeCount,
  last_error_code: optionalSchema(safeFailureCode),
  actor_user_id: entityIDSchema,
  audit_id: entityIDSchema,
  correlation_id: correlationIDSchema,
  not_before: optionalSchema(isoDateTimeSchema),
  available_at: isoDateTimeSchema,
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
  started_at: optionalSchema(isoDateTimeSchema),
  finished_at: optionalSchema(isoDateTimeSchema),
});

export const operationSchema: Schema<Operation> = {
  parse(value: unknown, path = "$"): Operation {
    const operation = rawOperationSchema.parse(value, path);
    if (
      operation.processed !== operation.succeeded + operation.failed ||
      operation.processed > operation.total
    ) {
      throw new ContractError("operation progress is inconsistent", path);
    }
    if (
      operation.status === OPERATION_STATUS.COMPLETED &&
      operation.processed !== operation.total
    ) {
      throw new ContractError("completed operation has incomplete progress", path);
    }
    if (
      operation.requested_action !== OPERATION_REQUESTED_ACTION.NONE &&
      operation.status !== OPERATION_STATUS.RUNNING
    ) {
      throw new ContractError(
        "operation control request is attached to an invalid state",
        path,
      );
    }
    if (
      operation.lease &&
      (operation.status !== OPERATION_STATUS.RUNNING ||
        operation.lease.operation_id !== operation.id)
    ) {
      throw new ContractError("operation lease is inconsistent", path);
    }
    if (
      compareISODateTime(operation.updated_at, operation.created_at) < 0 ||
      compareISODateTime(operation.available_at, operation.created_at) < 0 ||
      (operation.not_before &&
        compareISODateTime(operation.available_at, operation.not_before) < 0) ||
      (operation.started_at &&
        compareISODateTime(operation.started_at, operation.created_at) < 0) ||
      (operation.finished_at &&
        compareISODateTime(
          operation.finished_at,
          operation.started_at ?? operation.created_at,
        ) < 0)
    ) {
      throw new ContractError("operation timestamps are inconsistent", path);
    }
    if (
      operation.status === OPERATION_STATUS.FAILED &&
      !operation.last_error_code
    ) {
      throw new ContractError("failed operation has no safe error code", path);
    }
    return operation;
  },
};

export const operationPageSchema: Schema<OperationPage> = strictObjectSchema({
  items: arraySchema(operationSchema, { maximumLength: 100 }),
  next_cursor: optionalSchema(cursorSchema),
});

const operationBatchFailureSchema = strictObjectSchema({
  reference: stringSchema({ minimumLength: 1, maximumLength: 128 }),
  code: safeFailureCode,
});

const rawOperationBatchSchema = strictObjectSchema({
  operation_id: entityIDSchema,
  sequence: integerSchema({ minimum: 1, label: "operation batch sequence" }),
  processed: nonNegativeCount,
  succeeded: nonNegativeCount,
  failed: nonNegativeCount,
  failures: arraySchema(operationBatchFailureSchema, { maximumLength: 100 }),
  started_at: isoDateTimeSchema,
  completed_at: isoDateTimeSchema,
});

const operationBatchSchema: Schema<OperationBatch> = {
  parse(value: unknown, path = "$") {
    const batch = rawOperationBatchSchema.parse(value, path);
    if (
      batch.processed !== batch.succeeded + batch.failed ||
      compareISODateTime(batch.completed_at, batch.started_at) < 0
    ) {
      throw new ContractError("operation batch receipt is inconsistent", path);
    }
    return batch;
  },
};

const operationBatchCursorSchema = stringSchema({
  minimumLength: 1,
  maximumLength: 19,
  pattern: /^[1-9][0-9]*$/,
  label: "operation batch cursor",
});

export const operationBatchPageSchema: Schema<OperationBatchPage> =
  strictObjectSchema({
    items: arraySchema(operationBatchSchema, { maximumLength: 100 }),
    next_cursor: optionalSchema(operationBatchCursorSchema),
  });

export const operationControlResultSchema: Schema<OperationControlResult> =
  strictObjectSchema({
    operation: operationSchema,
    audit_id: entityIDSchema,
    replay: booleanSchema,
  });

export const operationControlActionSchema = enumSchema(
  Object.values(OPERATION_CONTROL_ACTION),
);
