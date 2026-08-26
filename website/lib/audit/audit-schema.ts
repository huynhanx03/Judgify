import {
  arraySchema,
  booleanSchema,
  jsonValueSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  correlationIDSchema,
  cursorSchema,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type { AuditLogEntry, AuditLogPage } from "@/types/audit";

const MAXIMUM_DOCUMENT_NODES = 2_000;
const MAXIMUM_DOCUMENT_DEPTH = 16;

/**
 * Audit documents are intentionally object-shaped at the transport boundary.
 * The backend redacts and bounds them before release; this second boundary
 * prevents an accidentally malformed row from reaching an admin renderer.
 */
const auditDocumentSchema: Schema<Record<string, unknown>> = {
  parse(value: unknown, path = "$"): Record<string, unknown> {
    const source = recordAt(value, path);
    let nodes = 0;
    const walk = (candidate: unknown, currentPath: string, depth: number): void => {
      nodes += 1;
      if (nodes > MAXIMUM_DOCUMENT_NODES || depth > MAXIMUM_DOCUMENT_DEPTH) {
        throw new TypeError(`${currentPath}: audit document exceeds boundary`);
      }
      if (candidate === null || typeof candidate === "string" || typeof candidate === "boolean") {
        return;
      }
      if (typeof candidate === "number") {
        if (!Number.isFinite(candidate)) {
          throw new TypeError(`${currentPath}: invalid audit number`);
        }
        return;
      }
      if (Array.isArray(candidate)) {
        candidate.forEach((item, index) => walk(item, `${currentPath}[${index}]`, depth + 1));
        return;
      }
      const nested = recordAt(candidate, currentPath);
      for (const [key, child] of Object.entries(nested)) {
        if (key === "__proto__" || key === "constructor" || key === "prototype") {
          throw new TypeError(`${currentPath}.${key}: unsafe audit key`);
        }
        walk(child, `${currentPath}.${key}`, depth + 1);
      }
    };
    walk(source, path, 0);
    // Keep the explicit jsonValue parse so this boundary stays aligned with
    // the shared JSON safety policy if its limits are tightened later.
    jsonValueSchema.parse(source, path);
    return source;
  },
};

const rawAuditEntrySchema = strictObjectSchema({
  id: entityIDSchema,
  actor_user_id: optionalSchema(entityIDSchema),
  action: stringSchema({ minimumLength: 1, maximumLength: 128 }),
  resource: stringSchema({ minimumLength: 1, maximumLength: 128 }),
  resource_id: stringSchema({ minimumLength: 1, maximumLength: 256 }),
  reason: stringSchema({ maximumLength: 1_024 }),
  before: auditDocumentSchema,
  after: auditDocumentSchema,
  correlation_id: correlationIDSchema,
  occurred_at: isoDateTimeSchema,
});

export const auditLogEntrySchema: Schema<AuditLogEntry> = {
  parse(value: unknown, path = "$"): AuditLogEntry {
    const entry = rawAuditEntrySchema.parse(value, path);
    return {
      ...entry,
      ...(entry.actor_user_id === undefined
        ? {}
        : { actor_user_id: entry.actor_user_id }),
    };
  },
};

const rawAuditLogPageSchema = strictObjectSchema({
  items: arraySchema(auditLogEntrySchema, { maximumLength: 100 }),
  next_cursor: optionalSchema(cursorSchema),
  has_more: booleanSchema,
});

export const auditLogPageSchema: Schema<AuditLogPage> = {
  parse(value: unknown, path = "$"): AuditLogPage {
    const page = rawAuditLogPageSchema.parse(value, path);
    const identities = new Set(page.items.map((item) => item.id));
    if (identities.size !== page.items.length) {
      throw new TypeError(`${path}.items: duplicate audit identity`);
    }
    if (page.has_more !== Boolean(page.next_cursor)) {
      throw new TypeError(`${path}: inconsistent audit cursor`);
    }
    return page;
  },
};
