import { OBSERVABILITY_API } from "@/constants/api/observability";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/error";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  debugWindowActivationPurpose,
  normalizeActivateDebugWindow,
  normalizeDebugWindowReason,
} from "@/lib/observability/control";
import { debugWindowSchema } from "@/lib/observability/debug-window-schema";
import type {
  ActivateDebugWindowCommand,
  DebugWindow,
  DeactivateDebugWindowCommand,
} from "@/types/observability";

export const observabilityService = {
  getDebugWindow(signal?: AbortSignal): Promise<DebugWindow> {
    return api<DebugWindow, never>(
      OBSERVABILITY_API.DEBUG_WINDOW,
      {
        method: "GET",
        signal,
        schema: debugWindowSchema,
      },
    );
  },

  async activateDebugWindow(
    input: ActivateDebugWindowCommand,
  ): Promise<DebugWindow> {
    const command = normalizeActivateDebugWindow(input);
    const purpose = await debugWindowActivationPurpose(command);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const result = await api<
        DebugWindow,
        ActivateDebugWindowCommand
      >(OBSERVABILITY_API.DEBUG_WINDOW, {
        method: "POST",
        body: command,
        idempotencyKey: attempt.attempt_id,
        schema: debugWindowSchema,
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

  deactivateDebugWindow(
    key: string,
    input: DeactivateDebugWindowCommand,
  ): Promise<DebugWindow> {
    const leaseKey = entityIDSchema.parse(key);
    const command = {
      reason: normalizeDebugWindowReason(input.reason),
    };
    return api<DebugWindow, DeactivateDebugWindowCommand>(
      OBSERVABILITY_API.DEBUG_WINDOW_LEASE(leaseKey),
      {
        method: "DELETE",
        body: command,
        schema: debugWindowSchema,
      },
    );
  },
};
