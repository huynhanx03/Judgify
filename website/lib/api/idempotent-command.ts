import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";

/**
 * Runs one semantic mutation with a receipt-aligned idempotency key.
 *
 * Retryable and transport-unknown outcomes deliberately retain the attempt so
 * a user retry cannot create a second command. Definitive client failures
 * release it because the server has rejected that exact semantic command.
 */
export async function runIdempotentCommand<TResult>(
  purpose: string,
  execute: (idempotencyKey: string) => Promise<TResult>,
): Promise<TResult> {
  const attempt = commandAttemptStore.getOrCreate(purpose);
  try {
    const result = await execute(attempt.attempt_id);
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
}
