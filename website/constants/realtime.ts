import { BASE_API_URL } from "@/constants/api/base";
import { REALTIME_API } from "@/constants/api/realtime";

/** go-common WebSocket protocol v1. */
export const REALTIME_PROTOCOL = {
  VERSION: 1,
  OPERATION_SUBSCRIBE: "subscribe",
  OPERATION_UNSUBSCRIBE: "unsubscribe",
  OPERATION_EVENT: "event",
  OPERATION_PING: "ping",
  OPERATION_PONG: "pong",
  OPERATION_ACK: "ack",
  OPERATION_ERROR: "error",
  OPERATION_SUBSCRIBED: "subscribed",
  OPERATION_UNSUBSCRIBED: "unsubscribed",
  OPERATION_RESYNC_REQUIRED: "resync_required",
  TYPE_SUBSCRIPTION: "protocol.subscription.v1",
  TYPE_PING: "protocol.ping.v1",
  TYPE_PONG: "protocol.pong.v1",
  TYPE_ERROR: "protocol.error.v1",
  TYPE_RESYNC_REQUIRED: "protocol.resync-required.v1",
  TYPE_REPLAY_REQUEST: "realtime.replay.request.v1",
  TYPE_REPLAY_COMPLETE: "realtime.replay.complete.v1",
  TYPE_CONTEST_LIFECYCLE_CHANGED: "contest.lifecycle.changed.v1",
  TYPE_CONTEST_STANDINGS_CHANGED: "contest.standings.changed.v1",
  TYPE_CONTEST_RATING_CALCULATED: "contest.rating.calculated.v1",
  TYPE_CONTEST_RATING_PROJECTION_ACTIVATED:
    "contest.rating.projection_activated.v1",
  TYPE_CONTEST_ANNOUNCEMENT_PUBLISHED:
    "contest.announcement.published.v1",
  TYPE_CONTEST_ANNOUNCEMENT_WITHDRAWN:
    "contest.announcement.withdrawn.v1",
  TYPE_CONTEST_CLARIFICATION_SUBMITTED:
    "contest.clarification.submitted.v1",
  TYPE_CONTEST_CLARIFICATION_ANSWERED:
    "contest.clarification.answered.v1",
  TYPE_CONTEST_CLARIFICATION_CLOSED:
    "contest.clarification.closed.v1",
  TYPE_CONTEST_CLARIFICATION_WITHDRAWN:
    "contest.clarification.withdrawn.v1",
  TYPE_SUBMISSION_JUDGED: "submission.judged.v1",
  TYPE_AUTHORIZATION_CAPABILITIES_CHANGED:
    "authorization.capabilities.changed.v1",
  TYPE_NOTIFICATION_CREATED: "notification.created.v1",
  TYPE_NOTIFICATION_READ: "notification.read.v1",
  TYPE_NOTIFICATION_ARCHIVED: "notification.archived.v1",
  TOPIC_CONTEST: "contest",
  TOPIC_STANDINGS: "standings",
  TOPIC_USER: "user",
  TOPIC_SUBMISSIONS: "submissions",
  TOPIC_AUTHORIZATION: "authorization",
  TOPIC_CAPABILITIES: "capabilities",
  TOPIC_NOTIFICATIONS: "notifications",
  REQUEST_ID_PREFIX: "realtime-",
} as const;

export const EVENT_CONTEST_STANDINGS_CHANGED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_STANDINGS_CHANGED;
export const EVENT_CONTEST_LIFECYCLE_CHANGED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_LIFECYCLE_CHANGED;
export const EVENT_CONTEST_RATING_CALCULATED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_RATING_CALCULATED;
export const EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_RATING_PROJECTION_ACTIVATED;
export const EVENT_CONTEST_ANNOUNCEMENT_PUBLISHED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_ANNOUNCEMENT_PUBLISHED;
export const EVENT_CONTEST_ANNOUNCEMENT_WITHDRAWN_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_ANNOUNCEMENT_WITHDRAWN;
export const EVENT_CONTEST_CLARIFICATION_SUBMITTED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_CLARIFICATION_SUBMITTED;
export const EVENT_CONTEST_CLARIFICATION_ANSWERED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_CLARIFICATION_ANSWERED;
export const EVENT_CONTEST_CLARIFICATION_CLOSED_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_CLARIFICATION_CLOSED;
export const EVENT_CONTEST_CLARIFICATION_WITHDRAWN_V1 =
  REALTIME_PROTOCOL.TYPE_CONTEST_CLARIFICATION_WITHDRAWN;
export const EVENT_SUBMISSION_JUDGED_V1 =
  REALTIME_PROTOCOL.TYPE_SUBMISSION_JUDGED;
export const EVENT_AUTHORIZATION_CAPABILITIES_CHANGED_V1 =
  REALTIME_PROTOCOL.TYPE_AUTHORIZATION_CAPABILITIES_CHANGED;
export const EVENT_NOTIFICATION_CREATED_V1 =
  REALTIME_PROTOCOL.TYPE_NOTIFICATION_CREATED;
export const EVENT_NOTIFICATION_READ_V1 =
  REALTIME_PROTOCOL.TYPE_NOTIFICATION_READ;
export const EVENT_NOTIFICATION_ARCHIVED_V1 =
  REALTIME_PROTOCOL.TYPE_NOTIFICATION_ARCHIVED;

/** Authentication modes supported by the shared WebSocket transport. */
export const REALTIME_CONNECTION_MODE = {
  ANONYMOUS: "anonymous",
  TICKET: "ticket",
} as const;

export function contestStandingsTopic(contestId: string): string {
  return [
    REALTIME_PROTOCOL.TOPIC_CONTEST,
    contestId,
    REALTIME_PROTOCOL.TOPIC_STANDINGS,
  ].join(":");
}

/** Public invalidations for contest discovery and lifecycle projections. */
export function contestLifecycleTopic(): string {
  return [
    REALTIME_PROTOCOL.TOPIC_CONTEST,
    "lifecycle",
  ].join(":");
}

export function contestPublicTopic(contestId: string): string {
  return [REALTIME_PROTOCOL.TOPIC_CONTEST, contestId, "public"].join(":");
}

export function contestParticipantsTopic(contestId: string): string {
  return [
    REALTIME_PROTOCOL.TOPIC_CONTEST,
    contestId,
    "participants",
  ].join(":");
}

export function contestStaffTopic(contestId: string): string {
  return [REALTIME_PROTOCOL.TOPIC_CONTEST, contestId, "staff"].join(":");
}

export function contestPrivateUserTopic(
  contestId: string,
  userId: string,
): string {
  return [
    REALTIME_PROTOCOL.TOPIC_CONTEST,
    contestId,
    REALTIME_PROTOCOL.TOPIC_USER,
    userId,
  ].join(":");
}

export function userSubmissionsTopic(userId: string): string {
  return [
    REALTIME_PROTOCOL.TOPIC_USER,
    userId,
    REALTIME_PROTOCOL.TOPIC_SUBMISSIONS,
  ].join(":");
}

export function authorizationCapabilitiesTopic(): string {
  return [
    REALTIME_PROTOCOL.TOPIC_AUTHORIZATION,
    REALTIME_PROTOCOL.TOPIC_CAPABILITIES,
  ].join(":");
}

export function notificationsTopic(userId: string): string {
  return [REALTIME_PROTOCOL.TOPIC_NOTIFICATIONS, userId].join(":");
}

/** Shared realtime behavior policy; feature hooks do not own timing constants. */
export const REALTIME_POLICY = {
  RECONNECT_INITIAL_DELAY_MS: 1_000,
  RECONNECT_MAXIMUM_DELAY_MS: 30_000,
  RECONNECT_MULTIPLIER: 2,
  RECONNECT_JITTER_RATIO: 0.2,
  OPENING_TIMEOUT_MS: 12_000,
  HEARTBEAT_INTERVAL_MS: 25_000,
  HEARTBEAT_TIMEOUT_MS: 10_000,
  AUTHORIZATION_REFRESH_RETRY_INITIAL_DELAY_MS: 500,
  AUTHORIZATION_REFRESH_RETRY_MAXIMUM_DELAY_MS: 5_000,
  AUTHORIZATION_REFRESH_RETRY_MULTIPLIER: 2,
  REPLAY_PAGE_SIZE: 128,
  MAXIMUM_DEDUPLICATION_ENTRIES: 4_096,
  WEBSOCKET_PATH: REALTIME_API.WEBSOCKET,
} as const;

const configuredWebSocketUrl = process.env.NEXT_PUBLIC_REALTIME_WS_URL?.trim();

/**
 * Converts the centralized HTTP API origin into a browser WebSocket endpoint.
 * An explicit realtime URL may be supplied when API and WebSocket traffic use
 * different origins. The ticket remains in the query string only for the
 * upgrade that consumes it.
 */
function resolveRealtimeWebSocketBaseUrl(): URL {
  const browserOrigin = window.location.origin;
  const source = configuredWebSocketUrl || new URL(
    REALTIME_POLICY.WEBSOCKET_PATH,
    new URL(BASE_API_URL || browserOrigin, browserOrigin).origin,
  ).toString();
  const url = new URL(source, browserOrigin);

  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url;
}

/** Resolves the public WebSocket endpoint without any private credential. */
export function resolveAnonymousRealtimeWebSocketUrl(): string {
  const url = resolveRealtimeWebSocketBaseUrl();
  url.searchParams.delete("ticket");
  return url.toString();
}

/** Resolves the endpoint for exactly one single-use private upgrade ticket. */
export function resolveTicketedRealtimeWebSocketUrl(ticket: string): string {
  const url = resolveRealtimeWebSocketBaseUrl();
  url.searchParams.set("ticket", ticket);
  return url.toString();
}
