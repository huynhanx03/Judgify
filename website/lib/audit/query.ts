import {
  AUDIT_FILTER_LIMITS,
  AUDIT_PAGE_SIZE,
  AUDIT_PAGE_SIZE_MAX,
} from "@/constants/audit";
import type { AuditListQuery } from "@/types/audit";

export interface AuditFilterDraft {
  actorUserId: string;
  action: string;
  resource: string;
  resourceId: string;
  correlationId: string;
  from: string;
  to: string;
}

export type AuditInvalidField =
  | "actorUserId"
  | "action"
  | "resource"
  | "resourceId"
  | "correlationId"
  | "timeRange";

export type AuditFilterNormalization =
  | { ok: true; query: AuditListQuery }
  | { ok: false; invalidFields: AuditInvalidField[] };

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const IDENTIFIER_PATTERN = /^[A-Za-z0-9._:-]+$/;

export function createEmptyAuditFilterDraft(): AuditFilterDraft {
  return {
    actorUserId: "",
    action: "",
    resource: "",
    resourceId: "",
    correlationId: "",
    from: "",
    to: "",
  };
}

function trimmedWithin(value: string, maximum: number): string | null {
  const normalized = value.trim();
  return normalized.length <= maximum ? normalized : null;
}

function toISOString(value: string): string | null {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

/** Normalizes only the allowlisted server query and never invents domain filters. */
export function normalizeAuditFilters(
  draft: AuditFilterDraft,
): AuditFilterNormalization {
  const normalizedActorUserId = trimmedWithin(
    draft.actorUserId,
    AUDIT_FILTER_LIMITS.ACTOR_USER_ID,
  );
  const actorUserId = normalizedActorUserId?.toLowerCase() ?? normalizedActorUserId;
  const action = trimmedWithin(draft.action, AUDIT_FILTER_LIMITS.ACTION);
  const resource = trimmedWithin(draft.resource, AUDIT_FILTER_LIMITS.RESOURCE);
  const resourceId = trimmedWithin(
    draft.resourceId,
    AUDIT_FILTER_LIMITS.RESOURCE_ID,
  );
  const correlationId = trimmedWithin(
    draft.correlationId,
    AUDIT_FILTER_LIMITS.CORRELATION_ID,
  );
  const from = toISOString(draft.from);
  const to = toISOString(draft.to);
  const invalidFields: AuditInvalidField[] = [];

  if (actorUserId === null || (actorUserId && !UUID_PATTERN.test(actorUserId))) {
    invalidFields.push("actorUserId");
  }
  if (action === null || (action && !IDENTIFIER_PATTERN.test(action))) {
    invalidFields.push("action");
  }
  if (resource === null || (resource && !IDENTIFIER_PATTERN.test(resource))) {
    invalidFields.push("resource");
  }
  if (resourceId === null) invalidFields.push("resourceId");
  if (
    correlationId === null ||
    (correlationId && !IDENTIFIER_PATTERN.test(correlationId))
  ) {
    invalidFields.push("correlationId");
  }
  if (
    from === null ||
    to === null ||
    (from && to && new Date(from).getTime() >= new Date(to).getTime())
  ) {
    invalidFields.push("timeRange");
  }

  if (invalidFields.length) return { ok: false, invalidFields };

  return {
    ok: true,
    query: {
      ...(actorUserId ? { actor_user_id: actorUserId } : {}),
      ...(action ? { action } : {}),
      ...(resource ? { resource } : {}),
      ...(resourceId ? { resource_id: resourceId } : {}),
      ...(correlationId ? { correlation_id: correlationId } : {}),
      ...(from ? { from } : {}),
      ...(to ? { to } : {}),
      limit: AUDIT_PAGE_SIZE,
    },
  };
}

/** Serializes the closed audit query contract; unknown object keys are ignored. */
export function toAuditSearchParams(query: AuditListQuery): URLSearchParams {
  const params = new URLSearchParams();
  const values: ReadonlyArray<[string, string | number | undefined]> = [
    ["actor_user_id", query.actor_user_id],
    ["action", query.action],
    ["resource", query.resource],
    ["resource_id", query.resource_id],
    ["correlation_id", query.correlation_id],
    ["from", query.from],
    ["to", query.to],
    [
      "limit",
      Math.min(
        AUDIT_PAGE_SIZE_MAX,
        Math.max(1, Math.trunc(query.limit ?? AUDIT_PAGE_SIZE)),
      ),
    ],
    ["cursor", query.cursor],
  ];

  for (const [key, value] of values) {
    if (value !== undefined && value !== "") params.set(key, String(value));
  }
  return params;
}
