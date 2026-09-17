import {
  REALTIME_CONNECTION_MODE,
  REALTIME_POLICY,
  REALTIME_PROTOCOL,
} from "@/constants/realtime";
import { isoDateTimeSchema } from "@/lib/api/contracts";

export type RealtimeOperation =
  | typeof REALTIME_PROTOCOL.OPERATION_SUBSCRIBE
  | typeof REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE
  | typeof REALTIME_PROTOCOL.OPERATION_EVENT
  | typeof REALTIME_PROTOCOL.OPERATION_PING
  | typeof REALTIME_PROTOCOL.OPERATION_PONG
  | typeof REALTIME_PROTOCOL.OPERATION_ACK
  | typeof REALTIME_PROTOCOL.OPERATION_ERROR
  | typeof REALTIME_PROTOCOL.OPERATION_SUBSCRIBED
  | typeof REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBED
  | typeof REALTIME_PROTOCOL.OPERATION_RESYNC_REQUIRED;

/** Wire shape shared with go-common's strict v1 envelope. */
export interface RealtimeEnvelope<TData = unknown> {
  v: number;
  op: RealtimeOperation;
  id?: string;
  cursor?: string;
  topic?: string;
  type: string;
  aggregate_version?: number;
  occurred_at?: string;
  data?: TData;
}

export type RealtimeMessageHandler<TData = unknown> = (
  data: TData,
  envelope: RealtimeEnvelope<TData>,
) => void;

export interface ReconnectPolicy {
  initialDelayMs: number;
  maximumDelayMs: number;
  multiplier: number;
  jitterRatio?: number;
}

export type WebSocketConnectionOptions =
  | {
      mode: typeof REALTIME_CONNECTION_MODE.ANONYMOUS;
      /** Resolves the public ws/wss endpoint without credentials. */
      url: () => string;
    }
  | {
      mode: typeof REALTIME_CONNECTION_MODE.TICKET;
      /** Must issue a fresh single-use credential for every upgrade attempt. */
      issueTicket: (signal: AbortSignal) => Promise<string>;
      /** Resolves the externally reachable endpoint for that ticket. */
      url: (ticket: string) => string;
    };

export interface WebSocketClientOptions {
  connection: WebSocketConnectionOptions;
  reconnect: ReconnectPolicy;
  openingTimeoutMs?: number;
  heartbeatIntervalMs?: number;
  heartbeatTimeoutMs?: number;
  random?: () => number;
}

export type RealtimeConnectionState =
  | "idle"
  | "connecting"
  | "open"
  | "reconnecting"
  | "closed";

export const REALTIME_TOPIC_STATE = {
  INACTIVE: "inactive",
  PENDING: "pending",
  SUBSCRIBED: "subscribed",
  ERROR: "error",
} as const;

export type RealtimeTopicState =
  (typeof REALTIME_TOPIC_STATE)[keyof typeof REALTIME_TOPIC_STATE];

export interface RealtimeProtocolError {
  code: string;
  retryable: boolean;
}

interface PendingSubscription {
  topic: string;
  operation:
    | typeof REALTIME_PROTOCOL.OPERATION_SUBSCRIBE
    | typeof REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE;
}

interface PendingReplay {
  requestID: string;
  topic: string;
  afterCursor?: string;
}

interface ReplayCompletePayload {
  request_id: string;
  topic: string;
  cursor?: string;
  head_cursor?: string;
  delivered: number;
  has_more: boolean;
}

const MAXIMUM_REALTIME_FRAME_BYTES = 64 * 1024;
const UTF8_ENCODER = new TextEncoder();
const ENVELOPE_KEYS = new Set([
  "v",
  "op",
  "id",
  "cursor",
  "topic",
  "type",
  "aggregate_version",
  "occurred_at",
  "data",
]);

/**
 * Endpoint-agnostic typed WebSocket transport. It owns ticket-per-upgrade,
 * reconnect, protocol parsing and topic handshakes. Features keep REST as the
 * source of truth and explicitly resubscribe before their reconnect snapshot.
 */
export class WebSocketClient {
  private socket: WebSocket | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private openingTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatDeadline: ReturnType<typeof setTimeout> | null = null;
  private pendingHeartbeatID: string | null = null;
  private ticketRequest: AbortController | null = null;
  private reconnectAttempt = 0;
  private connectionAttempt = 0;
  private isOpening = false;
  private closedByConsumer = false;
  private onlineListenerAttached = false;
  private state: RealtimeConnectionState = "idle";
  private messageSequence = 0;
  private readonly handlers = new Map<
    string,
    Set<RealtimeMessageHandler<unknown>>
  >();
  private readonly topicReferences = new Map<string, number>();
  private readonly confirmedTopics = new Set<string>();
  private readonly topicStates = new Map<string, RealtimeTopicState>();
  private readonly pendingSubscriptions = new Map<string, PendingSubscription>();
  private readonly pendingReplays = new Map<string, PendingReplay>();
  private readonly replayRequestByTopic = new Map<string, string>();
  private readonly completedReplayRequests = new Set<string>();
  private readonly topicCursors = new Map<string, string>();
  private readonly deliveredEvents = new Set<string>();
  private readonly topicStateHandlers = new Map<
    string,
    Set<(state: RealtimeTopicState) => void>
  >();
  private readonly stateHandlers = new Set<
    (state: RealtimeConnectionState) => void
  >();
  private readonly resyncHandlers = new Set<() => void>();
  private readonly protocolErrorHandlers = new Set<
    (error: RealtimeProtocolError) => void
  >();

  constructor(private readonly options: WebSocketClientOptions) {}

  connect(): void {
    if (this.socket?.readyState === WebSocket.OPEN || this.isOpening) return;
    if (this.socket?.readyState === WebSocket.CONNECTING) return;

    this.closedByConsumer = false;
    this.attachOnlineListener();
    this.startConnection(this.reconnectAttempt === 0 ? "connecting" : "reconnecting");
  }

  close(): void {
    this.closedByConsumer = true;
    this.connectionAttempt += 1;
    this.isOpening = false;
    this.clearReconnectTimer();
    this.clearOpeningTimer();
    this.stopHeartbeat();
    this.abortTicketRequest();
    this.detachOnlineListener();
    this.resetTopicReadiness();
    this.socket?.close();
    this.socket = null;
    this.setState("closed");
  }

  subscribe<TData>(
    type: string,
    handler: RealtimeMessageHandler<TData>,
  ): () => void {
    const handlers =
      this.handlers.get(type) ?? new Set<RealtimeMessageHandler<unknown>>();
    handlers.add(handler as RealtimeMessageHandler<unknown>);
    this.handlers.set(type, handlers);

    return () => {
      handlers.delete(handler as RealtimeMessageHandler<unknown>);
      if (handlers.size === 0) this.handlers.delete(type);
    };
  }

  /** Queues a topic and sends the v1 subscribe control envelope when possible. */
  subscribeToTopic(topic: string): () => void {
    const references = this.topicReferences.get(topic) ?? 0;
    this.topicReferences.set(topic, references + 1);
    if (references === 0) {
      this.setTopicState(topic, REALTIME_TOPIC_STATE.PENDING);
      this.sendSubscription(topic, REALTIME_PROTOCOL.OPERATION_SUBSCRIBE);
    }

    return () => this.unsubscribeFromTopic(topic);
  }

  /** Removes this consumer's topic reference and sends an unsubscribe handshake. */
  unsubscribeFromTopic(topic: string): void {
    const references = this.topicReferences.get(topic);
    if (!references) return;
    if (references > 1) {
      this.topicReferences.set(topic, references - 1);
      return;
    }
    this.topicReferences.delete(topic);
    this.confirmedTopics.delete(topic);
    this.clearPendingReplay(topic);
    this.sendSubscription(topic, REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE);
    this.setTopicState(topic, REALTIME_TOPIC_STATE.INACTIVE);
  }

  /** Call immediately after open, then obtain an authoritative REST snapshot. */
  resubscribe(): void {
    for (const topic of this.topicReferences.keys()) {
      this.sendSubscription(topic, REALTIME_PROTOCOL.OPERATION_SUBSCRIBE);
    }
  }

  onStateChange(
    handler: (state: RealtimeConnectionState) => void,
  ): () => void {
    this.stateHandlers.add(handler);
    handler(this.state);
    return () => this.stateHandlers.delete(handler);
  }

  onResyncRequired(handler: () => void): () => void {
    this.resyncHandlers.add(handler);
    return () => this.resyncHandlers.delete(handler);
  }

  onTopicStateChange(
    topic: string,
    handler: (state: RealtimeTopicState) => void,
  ): () => void {
    const handlers = this.topicStateHandlers.get(topic) ?? new Set();
    handlers.add(handler);
    this.topicStateHandlers.set(topic, handlers);
    handler(this.topicStates.get(topic) ?? REALTIME_TOPIC_STATE.INACTIVE);
    return () => {
      handlers.delete(handler);
      if (handlers.size === 0) this.topicStateHandlers.delete(topic);
    };
  }

  onProtocolError(
    handler: (error: RealtimeProtocolError) => void,
  ): () => void {
    this.protocolErrorHandlers.add(handler);
    return () => this.protocolErrorHandlers.delete(handler);
  }

  private startConnection(state: "connecting" | "reconnecting"): void {
    this.clearReconnectTimer();
    this.clearOpeningTimer();
    this.abortTicketRequest();
    this.isOpening = true;
    this.setState(state);
    const attempt = ++this.connectionAttempt;
    const ticketRequest = new AbortController();
    this.ticketRequest = ticketRequest;
    this.startOpeningTimer(attempt);
    void this.openSocket(attempt, ticketRequest);
  }

  private async openSocket(
    attempt: number,
    ticketRequest: AbortController,
  ): Promise<void> {
    let url: string;
    try {
      const connection = this.options.connection;
      if (connection.mode === REALTIME_CONNECTION_MODE.TICKET) {
        const ticket = await connection.issueTicket(ticketRequest.signal);
        url = connection.url(ticket);
      } else {
        url = connection.url();
      }
    } catch {
      if (ticketRequest.signal.aborted) return;
      this.handleConnectionFailure(attempt);
      return;
    }

    if (
      ticketRequest.signal.aborted ||
      this.closedByConsumer ||
      attempt !== this.connectionAttempt
    ) {
      return;
    }
    if (this.ticketRequest === ticketRequest) this.ticketRequest = null;

    let socket: WebSocket;
    try {
      socket = new WebSocket(url);
    } catch {
      this.handleConnectionFailure(attempt);
      return;
    }
    this.isOpening = false;
    this.socket = socket;

    socket.addEventListener("open", () => {
      if (socket !== this.socket || attempt !== this.connectionAttempt) return;
      this.clearOpeningTimer();
      this.abortTicketRequest();
      this.reconnectAttempt = 0;
      this.resetTopicReadiness();
      this.startHeartbeat();
      this.setState("open");
    });

    socket.addEventListener("message", (event) => {
      if (socket !== this.socket) return;
      if (typeof event.data !== "string") {
        this.handleProtocolViolation();
        return;
      }
      const envelope = parseEnvelope(event.data);
      if (!envelope) {
        this.handleProtocolViolation();
        return;
      }
      this.handleEnvelope(envelope);
    });

    socket.addEventListener("close", () => {
      if (socket !== this.socket) return;
      this.clearOpeningTimer();
      this.stopHeartbeat();
      this.abortTicketRequest();
      this.socket = null;
      this.resetTopicReadiness();
      if (this.closedByConsumer) {
        this.setState("closed");
        return;
      }
      this.scheduleReconnect();
    });
  }

  private handleConnectionFailure(attempt: number): void {
    if (attempt !== this.connectionAttempt || this.closedByConsumer) return;
    this.clearOpeningTimer();
    this.abortTicketRequest();
    this.isOpening = false;
    this.scheduleReconnect();
  }

  private handleEnvelope(envelope: RealtimeEnvelope): void {
    if (envelope.op === REALTIME_PROTOCOL.OPERATION_RESYNC_REQUIRED) {
      if (envelope.topic) {
        this.topicCursors.delete(envelope.topic);
        this.clearPendingReplay(envelope.topic);
      } else {
        this.topicCursors.clear();
        this.pendingReplays.clear();
        this.replayRequestByTopic.clear();
      }
      for (const handler of this.resyncHandlers) handler();
      return;
    }
    if (envelope.op === REALTIME_PROTOCOL.OPERATION_PONG) {
      if (
        this.pendingHeartbeatID === null ||
        envelope.id !== this.pendingHeartbeatID
      ) {
        this.handleProtocolViolation();
        return;
      }
      this.clearHeartbeatDeadline();
      this.pendingHeartbeatID = null;
      return;
    }
    if (envelope.op === REALTIME_PROTOCOL.OPERATION_SUBSCRIBED && envelope.topic) {
      const pending = envelope.id
        ? this.pendingSubscriptions.get(envelope.id)
        : undefined;
      if (
        !pending ||
        pending.operation !== REALTIME_PROTOCOL.OPERATION_SUBSCRIBE ||
        pending.topic !== envelope.topic ||
        !this.topicReferences.has(envelope.topic)
      ) {
        this.handleProtocolViolation();
        return;
      }
      this.pendingSubscriptions.delete(envelope.id!);
      this.confirmedTopics.add(envelope.topic);
      this.setTopicState(envelope.topic, REALTIME_TOPIC_STATE.SUBSCRIBED);
      this.sendReplay(envelope.topic);
      return;
    }
    if (envelope.op === REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBED && envelope.topic) {
      const pending = envelope.id
        ? this.pendingSubscriptions.get(envelope.id)
        : undefined;
      if (
        !pending ||
        pending.operation !== REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE ||
        pending.topic !== envelope.topic
      ) {
        this.handleProtocolViolation();
        return;
      }
      this.pendingSubscriptions.delete(envelope.id!);
      this.confirmedTopics.delete(envelope.topic);
      if (this.topicReferences.has(envelope.topic)) {
        this.sendSubscription(
          envelope.topic,
          REALTIME_PROTOCOL.OPERATION_SUBSCRIBE,
        );
      } else {
        this.setTopicState(envelope.topic, REALTIME_TOPIC_STATE.INACTIVE);
      }
      return;
    }
    if (envelope.op === REALTIME_PROTOCOL.OPERATION_ERROR) {
      const pending = envelope.id
        ? this.pendingSubscriptions.get(envelope.id)
        : undefined;
      const pendingReplay = envelope.id
        ? this.pendingReplays.get(envelope.id)
        : undefined;
      if (
        envelope.id &&
        !pending &&
        !pendingReplay &&
        !this.completedReplayRequests.has(envelope.id)
      ) {
        this.handleProtocolViolation();
        return;
      }
      if (envelope.id) this.pendingSubscriptions.delete(envelope.id);
      if (pending && this.topicReferences.has(pending.topic)) {
        this.confirmedTopics.delete(pending.topic);
        this.setTopicState(pending.topic, REALTIME_TOPIC_STATE.ERROR);
      }
      if (pendingReplay) {
        this.clearPendingReplay(pendingReplay.topic);
        this.topicCursors.delete(pendingReplay.topic);
        for (const handler of this.resyncHandlers) handler();
      }
      const error = parseProtocolError(envelope.data);
      if (!error) {
        this.handleProtocolViolation();
        return;
      }
      for (const handler of this.protocolErrorHandlers) handler(error);
      return;
    }
    if (
      envelope.op === REALTIME_PROTOCOL.OPERATION_ACK &&
      envelope.type === REALTIME_PROTOCOL.TYPE_REPLAY_COMPLETE
    ) {
      this.handleReplayComplete(envelope);
      return;
    }
    if (envelope.op !== REALTIME_PROTOCOL.OPERATION_EVENT) {
      this.handleProtocolViolation();
      return;
    }
    if (
      !envelope.topic ||
      !this.topicReferences.has(envelope.topic) ||
      !this.confirmedTopics.has(envelope.topic)
    ) {
      for (const handler of this.resyncHandlers) handler();
      return;
    }

    const eventKey = `${envelope.topic}\u0000${envelope.id}\u0000${envelope.cursor}`;
    this.topicCursors.set(envelope.topic, envelope.cursor!);
    if (this.deliveredEvents.has(eventKey)) return;
    this.rememberBounded(this.deliveredEvents, eventKey);

    let handlerFailed = false;
    for (const handler of this.handlers.get(envelope.type) ?? []) {
      try {
        handler(envelope.data, envelope);
      } catch {
        handlerFailed = true;
      }
    }
    if (handlerFailed) {
      for (const handler of this.resyncHandlers) handler();
    }
  }

  private sendSubscription(
    topic: string,
    op:
      | typeof REALTIME_PROTOCOL.OPERATION_SUBSCRIBE
      | typeof REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE,
  ): boolean {
    const id = this.nextMessageID();
    this.clearPendingTopic(topic);
    const sent = this.send({
      v: REALTIME_PROTOCOL.VERSION,
      op,
      id,
      topic,
      type: REALTIME_PROTOCOL.TYPE_SUBSCRIPTION,
    });
    if (sent) {
      this.pendingSubscriptions.set(id, { topic, operation: op });
      if (op === REALTIME_PROTOCOL.OPERATION_SUBSCRIBE) {
        this.setTopicState(topic, REALTIME_TOPIC_STATE.PENDING);
      }
    }
    return sent;
  }

  private sendReplay(topic: string, afterCursor?: string): boolean {
    if (
      !this.topicReferences.has(topic) ||
      !this.confirmedTopics.has(topic) ||
      this.replayRequestByTopic.has(topic)
    ) {
      return false;
    }
    const requestID = crypto.randomUUID();
    const cursor = afterCursor ?? this.topicCursors.get(topic);
    const data = {
      request_id: requestID,
      topic,
      ...(cursor ? { after_cursor: cursor } : {}),
      limit: REALTIME_POLICY.REPLAY_PAGE_SIZE,
    };
    const sent = this.send({
      v: REALTIME_PROTOCOL.VERSION,
      op: REALTIME_PROTOCOL.OPERATION_EVENT,
      id: requestID,
      topic,
      type: REALTIME_PROTOCOL.TYPE_REPLAY_REQUEST,
      data,
    });
    if (!sent) return false;
    this.pendingReplays.set(requestID, {
      requestID,
      topic,
      ...(cursor ? { afterCursor: cursor } : {}),
    });
    this.replayRequestByTopic.set(topic, requestID);
    return true;
  }

  private handleReplayComplete(envelope: RealtimeEnvelope): void {
    const requestID = envelope.id;
    if (!requestID) {
      this.handleProtocolViolation();
      return;
    }
    const pending = this.pendingReplays.get(requestID);
    if (!pending) {
      if (this.completedReplayRequests.has(requestID)) return;
      this.handleProtocolViolation();
      return;
    }
    const result = parseReplayComplete(envelope.data);
    if (
      !result ||
      result.request_id !== requestID ||
      result.topic !== pending.topic ||
      envelope.topic !== pending.topic ||
      (envelope.cursor ?? undefined) !== result.cursor ||
      (result.has_more && !result.cursor) ||
      result.delivered > REALTIME_POLICY.REPLAY_PAGE_SIZE
    ) {
      this.handleProtocolViolation();
      return;
    }
    this.clearPendingReplay(pending.topic);
    this.rememberBounded(this.completedReplayRequests, requestID);

    const cursorBeforeReplay = pending.afterCursor;
    const currentCursor = this.topicCursors.get(pending.topic);
    if (
      result.cursor &&
      result.delivered === 0 &&
      currentCursor === cursorBeforeReplay
    ) {
      this.topicCursors.set(pending.topic, result.cursor);
    }
    if (result.has_more) {
      this.sendReplay(pending.topic, result.cursor);
      return;
    }
    if (
      result.head_cursor &&
      (this.topicCursors.get(pending.topic) === cursorBeforeReplay ||
        this.topicCursors.get(pending.topic) === undefined)
    ) {
      this.topicCursors.set(pending.topic, result.head_cursor);
    }
  }

  private clearPendingReplay(topic: string): void {
    const requestID = this.replayRequestByTopic.get(topic);
    if (requestID) this.pendingReplays.delete(requestID);
    this.replayRequestByTopic.delete(topic);
  }

  private rememberBounded(values: Set<string>, value: string): void {
    values.add(value);
    while (values.size > REALTIME_POLICY.MAXIMUM_DEDUPLICATION_ENTRIES) {
      const oldest = values.values().next().value as string | undefined;
      if (!oldest) break;
      values.delete(oldest);
    }
  }

  private send<TData>(envelope: RealtimeEnvelope<TData>): boolean {
    if (this.socket?.readyState !== WebSocket.OPEN) return false;
    const encoded = JSON.stringify(envelope);
    if (utf8ByteLength(encoded) > MAXIMUM_REALTIME_FRAME_BYTES) {
      return false;
    }
    this.socket.send(encoded);
    return true;
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    const interval = this.options.heartbeatIntervalMs;
    const timeout = this.options.heartbeatTimeoutMs;
    if (
      interval === undefined ||
      timeout === undefined ||
      !Number.isSafeInteger(interval) ||
      !Number.isSafeInteger(timeout) ||
      interval < 1 ||
      timeout < 1 ||
      timeout >= interval
    ) {
      return;
    }
    this.heartbeatTimer = setInterval(() => {
      if (this.pendingHeartbeatID !== null) {
        this.handleProtocolViolation();
        return;
      }
      const id = this.nextMessageID();
      if (!this.send({
        v: REALTIME_PROTOCOL.VERSION,
        op: REALTIME_PROTOCOL.OPERATION_PING,
        id,
        type: REALTIME_PROTOCOL.TYPE_PING,
      })) {
        this.handleProtocolViolation();
        return;
      }
      this.pendingHeartbeatID = id;
      this.heartbeatDeadline = setTimeout(
        () => this.handleProtocolViolation(),
        timeout,
      );
    }, interval);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.clearHeartbeatDeadline();
    this.pendingHeartbeatID = null;
  }

  private clearHeartbeatDeadline(): void {
    if (this.heartbeatDeadline === null) return;
    clearTimeout(this.heartbeatDeadline);
    this.heartbeatDeadline = null;
  }

  private handleProtocolViolation(): void {
    if (this.closedByConsumer) return;
    this.stopHeartbeat();
    this.resetTopicReadiness();
    for (const handler of this.resyncHandlers) handler();
    this.socket?.close();
  }

  private nextMessageID(): string {
    this.messageSequence += 1;
    return `${REALTIME_PROTOCOL.REQUEST_ID_PREFIX}${this.messageSequence}`;
  }

  private scheduleReconnect(): void {
    if (this.closedByConsumer) return;
    this.clearOpeningTimer();
    const {
      initialDelayMs,
      maximumDelayMs,
      multiplier,
      jitterRatio = 0,
    } = this.options.reconnect;
    const baseDelay = Math.min(
      initialDelayMs * multiplier ** this.reconnectAttempt,
      maximumDelayMs,
    );
    const randomValue = this.options.random?.() ?? Math.random();
    const boundedRandom = Number.isFinite(randomValue)
      ? Math.min(1, Math.max(0, randomValue))
      : 0.5;
    const jitterFactor =
      1 + jitterRatio * (boundedRandom * 2 - 1);
    const delay = Math.max(0, Math.round(baseDelay * jitterFactor));
    this.reconnectAttempt += 1;
    this.setState("reconnecting");
    this.reconnectTimer = setTimeout(() => {
      this.startConnection("reconnecting");
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer === null) return;
    clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }

  private startOpeningTimer(attempt: number): void {
    const timeout = this.options.openingTimeoutMs;
    if (!timeout || timeout < 1) return;
    this.openingTimer = setTimeout(() => {
      if (
        attempt !== this.connectionAttempt ||
        this.closedByConsumer ||
        this.socket?.readyState === WebSocket.OPEN
      ) {
        return;
      }
      this.connectionAttempt += 1;
      this.isOpening = false;
      this.abortTicketRequest();
      const socket = this.socket;
      this.socket = null;
      socket?.close();
      this.scheduleReconnect();
    }, timeout);
  }

  private clearOpeningTimer(): void {
    if (this.openingTimer === null) return;
    clearTimeout(this.openingTimer);
    this.openingTimer = null;
  }

  private abortTicketRequest(): void {
    this.ticketRequest?.abort(
      new DOMException(
        "Realtime connection attempt superseded",
        "AbortError",
      ),
    );
    this.ticketRequest = null;
  }

  private readonly handleOnline = (): void => {
    if (
      this.closedByConsumer ||
      this.isOpening ||
      this.socket?.readyState === WebSocket.CONNECTING ||
      this.socket?.readyState === WebSocket.OPEN
    ) {
      return;
    }
    this.startConnection("reconnecting");
  };

  private attachOnlineListener(): void {
    if (this.onlineListenerAttached || typeof window === "undefined") return;
    window.addEventListener("online", this.handleOnline);
    this.onlineListenerAttached = true;
  }

  private detachOnlineListener(): void {
    if (!this.onlineListenerAttached || typeof window === "undefined") return;
    window.removeEventListener("online", this.handleOnline);
    this.onlineListenerAttached = false;
  }

  private setState(state: RealtimeConnectionState): void {
    if (state === this.state) return;
    this.state = state;
    for (const handler of this.stateHandlers) handler(state);
  }

  private setTopicState(topic: string, state: RealtimeTopicState): void {
    if (this.topicStates.get(topic) === state) return;
    this.topicStates.set(topic, state);
    for (const handler of this.topicStateHandlers.get(topic) ?? []) {
      handler(state);
    }
  }

  private clearPendingTopic(topic: string): void {
    for (const [id, pending] of this.pendingSubscriptions) {
      if (pending.topic === topic) this.pendingSubscriptions.delete(id);
    }
  }

  private resetTopicReadiness(): void {
    this.confirmedTopics.clear();
    this.pendingSubscriptions.clear();
    this.pendingReplays.clear();
    this.replayRequestByTopic.clear();
    for (const topic of this.topicReferences.keys()) {
      this.setTopicState(topic, REALTIME_TOPIC_STATE.PENDING);
    }
  }
}

function parseReplayComplete(value: unknown): ReplayCompletePayload | null {
  if (
    !isPlainRecord(value) ||
    Object.keys(value).some(
      (key) =>
        key !== "request_id" &&
        key !== "topic" &&
        key !== "cursor" &&
        key !== "head_cursor" &&
        key !== "delivered" &&
        key !== "has_more",
    ) ||
    !isCanonicalUUID(value.request_id) ||
    !validRequiredTerm(value.topic, 256) ||
    (value.cursor !== undefined && !isReplayCursor(value.cursor)) ||
    (value.head_cursor !== undefined && !isReplayCursor(value.head_cursor)) ||
    typeof value.delivered !== "number" ||
    !Number.isSafeInteger(value.delivered) ||
    value.delivered < 0 ||
    typeof value.has_more !== "boolean"
  ) {
    return null;
  }
  return {
    request_id: value.request_id,
    topic: value.topic,
    cursor: value.cursor,
    head_cursor: value.head_cursor,
    delivered: value.delivered,
    has_more: value.has_more,
  };
}

function parseProtocolError(value: unknown): RealtimeProtocolError | null {
  if (
    !isPlainRecord(value) ||
    Object.keys(value).some(
      (key) => key !== "code" && key !== "retryable",
    ) ||
    !validRequiredTerm(value.code, 128) ||
    typeof value.retryable !== "boolean"
  ) {
    return null;
  }
  return { code: value.code, retryable: value.retryable };
}

function isCanonicalUUID(value: unknown): value is string {
  return (
    typeof value === "string" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
      value,
    )
  );
}

function isReplayCursor(value: unknown): value is string {
  return typeof value === "string" && /^r1\.[A-Za-z0-9_-]{11}$/.test(value);
}

function parseEnvelope(value: string): RealtimeEnvelope | null {
  if (
    value.length < 1 ||
    value.length > MAXIMUM_REALTIME_FRAME_BYTES ||
    !isWellFormedUnicode(value) ||
    utf8ByteLength(value) > MAXIMUM_REALTIME_FRAME_BYTES
  ) {
    return null;
  }
  try {
    const parsed: unknown = JSON.parse(value);
    if (
      !isPlainRecord(parsed) ||
      Object.keys(parsed).some((key) => !ENVELOPE_KEYS.has(key)) ||
      typeof parsed.v !== "number" ||
      !Number.isSafeInteger(parsed.v) ||
      parsed.v !== REALTIME_PROTOCOL.VERSION ||
      !isRealtimeOperation(parsed.op) ||
      !validRequiredTerm(parsed.type, 128) ||
      !validOptionalEnvelopeTerm(parsed, "id", 128) ||
      !validOptionalEnvelopeTerm(parsed, "cursor", 256) ||
      !validOptionalEnvelopeTerm(parsed, "topic", 256) ||
      !validOptionalPositiveInteger(parsed, "aggregate_version") ||
      !validOptionalOccurredAt(parsed)
    ) {
      return null;
    }
    if (!validOperationEnvelope(parsed)) return null;
    return {
      v: parsed.v,
      op: parsed.op,
      type: parsed.type,
      ...(typeof parsed.id === "string" ? { id: parsed.id } : {}),
      ...(typeof parsed.cursor === "string"
        ? { cursor: parsed.cursor }
        : {}),
      ...(typeof parsed.topic === "string" ? { topic: parsed.topic } : {}),
      ...(typeof parsed.aggregate_version === "number"
        ? { aggregate_version: parsed.aggregate_version }
        : {}),
      ...(typeof parsed.occurred_at === "string"
        ? { occurred_at: parsed.occurred_at }
        : {}),
      ...("data" in parsed ? { data: parsed.data } : {}),
    };
  } catch {
    return null;
  }
}

function isPlainRecord(
  value: unknown,
): value is Record<string, unknown> {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    Object.getPrototypeOf(value) === Object.prototype
  );
}

function validRequiredTerm(
  value: unknown,
  maximumBytes: number,
): value is string {
  return (
    typeof value === "string" &&
    value.length > 0 &&
    isWellFormedUnicode(value) &&
    value === value.trim() &&
    !/[\u0000-\u001f\u007f-\u009f]/u.test(value) &&
    utf8ByteLength(value) <= maximumBytes
  );
}

function validOptionalEnvelopeTerm(
  envelope: Record<string, unknown>,
  key: "id" | "cursor" | "topic",
  maximumBytes: number,
): boolean {
  return (
    !(key in envelope) ||
    validRequiredTerm(envelope[key], maximumBytes)
  );
}

function validOptionalPositiveInteger(
  envelope: Record<string, unknown>,
  key: "aggregate_version",
): boolean {
  return (
    !(key in envelope) ||
    (typeof envelope[key] === "number" &&
      Number.isSafeInteger(envelope[key]) &&
      (envelope[key] as number) > 0)
  );
}

function validOptionalOccurredAt(
  envelope: Record<string, unknown>,
): boolean {
  if (!("occurred_at" in envelope)) return true;
  try {
    isoDateTimeSchema.parse(envelope.occurred_at);
    return true;
  } catch {
    return false;
  }
}

function validOperationEnvelope(
  envelope: Record<string, unknown>,
): boolean {
  switch (envelope.op) {
    case REALTIME_PROTOCOL.OPERATION_EVENT:
      return (
        validRequiredTerm(envelope.id, 128) &&
        isReplayCursor(envelope.cursor) &&
        validRequiredTerm(envelope.topic, 256) &&
        typeof envelope.aggregate_version === "number" &&
        Number.isSafeInteger(envelope.aggregate_version) &&
        (envelope.aggregate_version as number) > 0 &&
        "occurred_at" in envelope &&
        "data" in envelope
      );
    case REALTIME_PROTOCOL.OPERATION_ACK:
      return (
        isCanonicalUUID(envelope.id) &&
        envelope.type === REALTIME_PROTOCOL.TYPE_REPLAY_COMPLETE &&
        validRequiredTerm(envelope.topic, 256) &&
        (!("cursor" in envelope) || isReplayCursor(envelope.cursor)) &&
        "data" in envelope &&
        absentEnvelopeFields(
          envelope,
          "aggregate_version",
          "occurred_at",
        )
      );
    case REALTIME_PROTOCOL.OPERATION_SUBSCRIBED:
    case REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBED:
      return (
        validRequiredTerm(envelope.id, 128) &&
        validRequiredTerm(envelope.topic, 256) &&
        envelope.type === REALTIME_PROTOCOL.TYPE_SUBSCRIPTION &&
        absentEnvelopeFields(
          envelope,
          "cursor",
          "aggregate_version",
          "occurred_at",
          "data",
        )
      );
    case REALTIME_PROTOCOL.OPERATION_ERROR:
      return (
        envelope.type === REALTIME_PROTOCOL.TYPE_ERROR &&
        "data" in envelope &&
        absentEnvelopeFields(
          envelope,
          "cursor",
          "topic",
          "aggregate_version",
          "occurred_at",
        )
      );
    case REALTIME_PROTOCOL.OPERATION_PONG:
      return (
        validRequiredTerm(envelope.id, 128) &&
        envelope.type === REALTIME_PROTOCOL.TYPE_PONG &&
        absentEnvelopeFields(
          envelope,
          "cursor",
          "topic",
          "aggregate_version",
          "occurred_at",
          "data",
        )
      );
    case REALTIME_PROTOCOL.OPERATION_RESYNC_REQUIRED:
      return (
        envelope.type === REALTIME_PROTOCOL.TYPE_RESYNC_REQUIRED &&
        absentEnvelopeFields(
          envelope,
          "id",
          "cursor",
          "aggregate_version",
          "occurred_at",
          "data",
        )
      );
    default:
      return false;
  }
}

function absentEnvelopeFields(
  envelope: Record<string, unknown>,
  ...keys: readonly string[]
): boolean {
  return keys.every((key) => !(key in envelope));
}

function utf8ByteLength(value: string): number {
  return UTF8_ENCODER.encode(value).byteLength;
}

function isWellFormedUnicode(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (next < 0xdc00 || next > 0xdfff) return false;
      index += 1;
      continue;
    }
    if (code >= 0xdc00 && code <= 0xdfff) return false;
  }
  return true;
}

function isRealtimeOperation(value: unknown): value is RealtimeOperation {
  return value === REALTIME_PROTOCOL.OPERATION_SUBSCRIBE ||
    value === REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBE ||
    value === REALTIME_PROTOCOL.OPERATION_EVENT ||
    value === REALTIME_PROTOCOL.OPERATION_PING ||
    value === REALTIME_PROTOCOL.OPERATION_PONG ||
    value === REALTIME_PROTOCOL.OPERATION_ACK ||
    value === REALTIME_PROTOCOL.OPERATION_ERROR ||
    value === REALTIME_PROTOCOL.OPERATION_SUBSCRIBED ||
    value === REALTIME_PROTOCOL.OPERATION_UNSUBSCRIBED ||
    value === REALTIME_PROTOCOL.OPERATION_RESYNC_REQUIRED;
}
