import {
  EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1,
  EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1,
  EVENT_CONTEST_CLARIFICATION_ANSWERED_V1,
  EVENT_CONTEST_CLARIFICATION_CLOSED_V1,
  EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1,
  EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1,
  EVENT_CONTEST_LIFECYCLE_CHANGED_V1,
  EVENT_CONTEST_RATING_CALCULATED_V1,
  EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
} from "@/constants/realtime";
import {
  compareISODateTime,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  enumSchema,
  integerSchema,
  strictObjectSchema,
  type Schema,
} from "@/lib/api/schema";
import type { RealtimeEnvelope } from "@/lib/realtime/websocket-client";
import type { ContestInvalidationV1 } from "@/types/realtime";

const contestResourceSchema = enumSchema([
  "lifecycle",
  "announcement",
  "clarification",
  "rating",
] as const);
const contestStatusSchema = enumSchema([
  "draft",
  "upcoming",
  "running",
  "ended",
  "cancelled",
  "published",
  "withdrawn",
  "open",
  "answered",
  "closed",
  "applied",
] as const);

const rawContestInvalidationV1Schema = strictObjectSchema({
  contest_id: entityIDSchema,
  resource: contestResourceSchema,
  resource_id: entityIDSchema,
  status: contestStatusSchema,
  version: integerSchema({
    minimum: 1,
    label: "contest aggregate version",
  }),
  changed_at: isoDateTimeSchema,
});

export const contestInvalidationV1Schema: Schema<ContestInvalidationV1> = {
  parse(value: unknown, path = "$"): ContestInvalidationV1 {
    const event = rawContestInvalidationV1Schema.parse(value, path);
    const valid =
      (event.resource === "lifecycle" &&
        event.resource_id === event.contest_id &&
        ["draft", "upcoming", "running", "ended", "cancelled"].includes(
          event.status,
        )) ||
      (event.resource === "announcement" &&
        ["published", "withdrawn"].includes(event.status)) ||
      (event.resource === "clarification" &&
        ["open", "answered", "closed", "withdrawn"].includes(event.status)) ||
      (event.resource === "rating" && event.status === "applied");
    if (!valid) {
      throw new TypeError(`${path}: invalid contest invalidation state`);
    }
    return event;
  },
};

/**
 * The transport validates the generic envelope. This boundary additionally
 * binds its topic, event type, cursor, version and timestamp to the contest
 * projection before any feature reacts to it.
 */
export function parseContestInvalidationV1Envelope(
  payload: unknown,
  envelope: RealtimeEnvelope<unknown>,
  expectedContestID: string | undefined,
  allowedTopics: ReadonlySet<string>,
): ContestInvalidationV1 {
  const event = contestInvalidationV1Schema.parse(payload);
  entityIDSchema.parse(envelope.id, "$.envelope.id");
  if (
    (expectedContestID !== undefined &&
      event.contest_id !== expectedContestID) ||
    !envelope.topic ||
    !allowedTopics.has(envelope.topic) ||
    envelope.aggregate_version !== event.version ||
    typeof envelope.occurred_at !== "string" ||
    compareISODateTime(
      isoDateTimeSchema.parse(envelope.occurred_at),
      event.changed_at,
    ) !== 0 ||
    !eventTypeMatchesProjection(envelope.type, event)
  ) {
    throw new TypeError("$.envelope: invalid contest event metadata");
  }
  return event;
}

function eventTypeMatchesProjection(
  eventType: string,
  event: ContestInvalidationV1,
): boolean {
  switch (eventType) {
    case EVENT_CONTEST_LIFECYCLE_CHANGED_V1:
      return event.resource === "lifecycle";
    case EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1:
      return event.resource === "announcement" &&
        event.status === "published";
    case EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1:
      return event.resource === "announcement" &&
        event.status === "withdrawn";
    case EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1:
      return event.resource === "clarification" && event.status === "open";
    case EVENT_CONTEST_CLARIFICATION_ANSWERED_V1:
      return event.resource === "clarification" &&
        event.status === "answered";
    case EVENT_CONTEST_CLARIFICATION_CLOSED_V1:
      return event.resource === "clarification" && event.status === "closed";
    case EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1:
      return event.resource === "clarification" &&
        event.status === "withdrawn";
    case EVENT_CONTEST_RATING_CALCULATED_V1:
    case EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1:
      return event.resource === "rating" &&
        event.status === "applied";
    default:
      return false;
  }
}
