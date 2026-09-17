import {
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  compareISODateTime,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  assertSubmissionCountInvariant,
  submissionTestCountSchema,
} from "@/lib/submissions/submission-schema";
import type { SubmissionJudgedV1 } from "@/types/realtime";
import type { RealtimeEnvelope } from "@/lib/realtime/websocket-client";

const rawSubmissionJudgedV1Schema = strictObjectSchema({
  submission_id: entityIDSchema,
  user_id: entityIDSchema,
  problem_id: entityIDSchema,
  contest_id: optionalSchema(entityIDSchema),
  generation: integerSchema({ minimum: 1, label: "submission generation" }),
  verdict: stringSchema({
    minimumLength: 1,
    maximumLength: 64,
    pattern: /^[a-z][a-z_]{0,62}$/,
    label: "submission verdict",
  }),
  passed_count: submissionTestCountSchema,
  total_count: submissionTestCountSchema,
  aggregate_version: integerSchema({
    minimum: 1,
    label: "submission aggregate version",
  }),
  submitted_at: isoDateTimeSchema,
  judged_at: isoDateTimeSchema,
});

/**
 * Exact browser boundary for the durable terminal invalidation. Invalid or
 * future-incompatible messages are ignored by the feature hook and reconciled
 * from authoritative REST state.
 */
export const submissionJudgedV1Schema: Schema<SubmissionJudgedV1> = {
  parse(value: unknown, path = "$"): SubmissionJudgedV1 {
    const event = rawSubmissionJudgedV1Schema.parse(value, path);
    assertSubmissionCountInvariant(
      event.passed_count,
      event.total_count,
      path,
    );
    if (compareISODateTime(event.judged_at, event.submitted_at) < 0) {
      throw new TypeError(
        `${path}: submission was judged before it was submitted`,
      );
    }
    return event;
  },
};

/**
 * Validates the event metadata that routes, versions, and orders the payload.
 * The transport owns generic protocol parsing; this boundary owns the exact
 * submission event/topic relationship.
 */
export function parseSubmissionJudgedV1Envelope(
  payload: unknown,
  envelope: RealtimeEnvelope<unknown>,
  expectedTopic: string,
): SubmissionJudgedV1 {
  const event = submissionJudgedV1Schema.parse(payload);
  if (
    envelope.topic !== expectedTopic ||
    envelope.aggregate_version !== event.aggregate_version ||
    envelope.id !== `${event.submission_id}:${event.aggregate_version}` ||
    typeof envelope.cursor !== "string" ||
    envelope.cursor.length < 1 ||
    envelope.cursor.length > 256 ||
    /[\u0000-\u001f\u007f]/.test(envelope.cursor) ||
    typeof envelope.occurred_at !== "string" ||
    compareISODateTime(
      isoDateTimeSchema.parse(envelope.occurred_at),
      event.judged_at,
    ) !== 0
  ) {
    throw new TypeError("$.envelope: invalid submission event metadata");
  }
  return event;
}
