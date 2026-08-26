import { OPERATION_API } from "@/constants/api/operation";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/error";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  normalizeOperationControl,
  operationControlPurpose,
} from "@/lib/operations/control";
import {
  operationControlResultSchema,
  operationBatchPageSchema,
  operationPageSchema,
  operationSchema,
} from "@/lib/operations/operation-schema";
import { toOperationSearchParams } from "@/lib/operations/query";
import type {
  Operation,
  OperationBatchPage,
  OperationControlCommand,
  OperationControlResult,
  OperationListQuery,
  OperationPage,
} from "@/types/operation";

export const operationService = {
  list(
    query: OperationListQuery = {},
    signal?: AbortSignal,
  ): Promise<OperationPage> {
    const params = toOperationSearchParams(query);
    return api<OperationPage, never>(
      OPERATION_API.LIST(params.toString()),
      {
        method: "GET",
        signal,
        schema: operationPageSchema,
      },
    );
  },

  get(id: string, signal?: AbortSignal): Promise<Operation> {
    return api<Operation, never>(
      OPERATION_API.GET(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: operationSchema,
      },
    );
  },

  async listBatches(
    id: string,
    cursor?: string,
    signal?: AbortSignal,
  ): Promise<OperationBatchPage> {
    const params = new URLSearchParams({ limit: "25" });
    if (cursor) params.set("cursor", cursor);
    const operationID = entityIDSchema.parse(id);
    const page = await api<OperationBatchPage, never>(
      OPERATION_API.BATCHES(operationID, params.toString()),
      {
        method: "GET",
        signal,
        schema: operationBatchPageSchema,
      },
    );
    if (page.items.some((batch) => batch.operation_id !== operationID)) {
      throw new TypeError("invalid operation batch ownership");
    }
    return page;
  },

  async control(
    id: string,
    input: OperationControlCommand,
  ): Promise<OperationControlResult> {
    const operationID = entityIDSchema.parse(id);
    const command = normalizeOperationControl(input);
    const purpose = await operationControlPurpose(operationID, command);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const result = await api<
        OperationControlResult,
        OperationControlCommand
      >(OPERATION_API.CONTROL(operationID), {
        method: "POST",
        body: command,
        idempotencyKey: attempt.attempt_id,
        schema: operationControlResultSchema,
      });
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return result;
    } catch (error) {
      if (
        error instanceof ApiError &&
        !error.retryable &&
        error.status < 500
      ) {
        commandAttemptStore.resolve(purpose, attempt.attempt_id);
      }
      throw error;
    }
  },
};
