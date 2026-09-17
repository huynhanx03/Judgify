import {
  EVENT_CONTEST_STANDINGS_CHANGED_V1,
} from "@/constants/realtime";
import {
  compareISODateTime,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  integerSchema,
  strictObjectSchema,
  type Schema,
} from "@/lib/api/schema";
import type { RealtimeEnvelope } from "@/lib/realtime/websocket-client";
import type { ContestStandingsChangedV1 } from "@/types/realtime";

const rawContestStandingsChangedV1Schema = strictObjectSchema({
  contest_id: entityIDSchema,
  submission_id: entityIDSchema,
  verdict_version: integerSchema({
    minimum: 1,
    label: "submission verdict version",
  }),
  changed_at: isoDateTimeSchema,
});

export const contestStandingsChangedV1Schema:
  Schema<ContestStandingsChangedV1> = rawContestStandingsChangedV1Schema;

/** Binds a standings invalidation to its exact topic and event watermark. */
export function parseContestStandingsChangedV1Envelope(
  payload: unknown,
  envelope: RealtimeEnvelope<unknown>,
  contestID: string,
  topic: string,
): ContestStandingsChangedV1 {
  const event = contestStandingsChangedV1Schema.parse(payload);
  if (
    event.contest_id !== contestID ||
    envelope.topic !== topic ||
    envelope.type !== EVENT_CONTEST_STANDINGS_CHANGED_V1 ||
    envelope.id !==
      `${event.submission_id}:${event.verdict_version}` ||
    envelope.aggregate_version !== event.verdict_version ||
    typeof envelope.occurred_at !== "string" ||
    compareISODateTime(
      isoDateTimeSchema.parse(envelope.occurred_at),
      event.changed_at,
    ) !== 0
  ) {
    throw new TypeError("$.envelope: invalid contest standings metadata");
  }
  return event;
}
