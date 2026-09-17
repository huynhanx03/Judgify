import {
  OPERATION_KIND_MAXIMUM_LENGTH,
  OPERATION_KIND_PATTERN,
  OPERATION_MAXIMUM_PAGE_SIZE,
  OPERATION_PAGE_SIZE,
  OPERATION_STATUSES,
} from "@/constants/operation";
import {
  cursorSchema,
  entityIDSchema,
} from "@/lib/api/contracts";
import type { OperationListQuery, OperationStatus } from "@/types/operation";

export function toOperationSearchParams(
  query: OperationListQuery,
): URLSearchParams {
  const limit = query.limit ?? OPERATION_PAGE_SIZE;
  if (
    !Number.isSafeInteger(limit) ||
    limit < 1 ||
    limit > OPERATION_MAXIMUM_PAGE_SIZE
  ) {
    throw new RangeError("operation page size is outside the supported boundary");
  }
  const params = new URLSearchParams({ limit: String(limit) });
  if (query.status !== undefined) {
    if (!OPERATION_STATUSES.includes(query.status as OperationStatus)) {
      throw new TypeError("invalid operation status");
    }
    params.set("status", query.status);
  }
  if (query.kind !== undefined) {
    const kind = query.kind.trim();
    if (
      kind.length < 1 ||
      kind.length > OPERATION_KIND_MAXIMUM_LENGTH ||
      !OPERATION_KIND_PATTERN.test(kind)
    ) {
      throw new TypeError("invalid operation kind");
    }
    params.set("kind", kind);
  }
  if (query.actor_id !== undefined) {
    params.set("actor_id", entityIDSchema.parse(query.actor_id));
  }
  if (query.cursor !== undefined) {
    params.set("cursor", cursorSchema.parse(query.cursor));
  }
  return params;
}

