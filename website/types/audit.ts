import type {
  CorrelationID,
  Cursor,
  EntityID,
  ISODateTime,
} from "@/types/api";

/** One immutable, already-redacted administrative audit record. */
export interface AuditLogEntry {
  id: EntityID;
  actor_user_id?: EntityID | null;
  action: string;
  resource: string;
  resource_id: string;
  reason: string;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
  correlation_id: CorrelationID;
  occurred_at: ISODateTime;
}

/** Cursor page returned by GET /admin/audit. */
export interface AuditLogPage {
  items: AuditLogEntry[];
  next_cursor?: Cursor | null;
  has_more: boolean;
}

export interface AuditListQuery {
  actor_user_id?: string;
  action?: string;
  resource?: string;
  resource_id?: string;
  correlation_id?: string;
  from?: string;
  to?: string;
  limit?: number;
  cursor?: string;
}
