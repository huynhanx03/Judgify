/**
 * Compatibility helpers for existing service call sites. The canonical
 * implementation lives in `lib/api/idempotency.ts`.
 *
 * This wrapper performs one network attempt only. Unknown-outcome recovery is
 * owned by the caller/coordinator so an unsafe command is never silently
 * replayed with a newly generated identity.
 */

import { createIdempotencyKey } from "@/lib/api/idempotency";

export function idempotencyHeaders(
  key = createIdempotencyKey(),
): Record<string, string> {
  return { "Idempotency-Key": key };
}

export function withStableIdempotencyKey<T>(
  operation: (headers: Record<string, string>) => Promise<T>,
  key = createIdempotencyKey(),
): Promise<T> {
  return operation(idempotencyHeaders(key));
}

export {
  CommandAttemptStore,
  commandAttemptStore,
  createIdempotencyKey,
} from "@/lib/api/idempotency";
