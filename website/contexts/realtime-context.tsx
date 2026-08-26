"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  EVENT_AUTHORIZATION_CAPABILITIES_CHANGED_V1,
  REALTIME_CONNECTION_MODE,
  REALTIME_POLICY,
  authorizationCapabilitiesTopic,
  resolveAnonymousRealtimeWebSocketUrl,
  resolveTicketedRealtimeWebSocketUrl,
} from "@/constants/realtime";
import { useSession } from "@/contexts/session-context";
import { ApiError } from "@/lib/api/error";
import {
  AuthorizationRevisionSync,
  authorizationRevisionFromEvent,
  type AuthorizationRefreshResult,
} from "@/lib/auth/authorization-revision-sync";
import {
  WebSocketClient,
  type RealtimeConnectionState,
  type RealtimeMessageHandler,
  type RealtimeTopicState,
} from "@/lib/realtime/websocket-client";
import { realtimeService } from "@/services/realtime.service";

interface AttachedRegistration {
  attach(client: WebSocketClient): () => void;
  release?: () => void;
}

export interface RealtimeContextValue {
  state: RealtimeConnectionState;
  subscribe<TData>(
    type: string,
    handler: RealtimeMessageHandler<TData>,
  ): () => void;
  subscribeToTopic(topic: string): () => void;
  onResyncRequired(handler: () => void): () => void;
  onStateChange(handler: (state: RealtimeConnectionState) => void): () => void;
  onTopicStateChange(
    topic: string,
    handler: (state: RealtimeTopicState) => void,
  ): () => void;
}

const RealtimeContext = createContext<RealtimeContextValue | null>(null);

function validSubscriptionValue(value: string, maximumLength: number): boolean {
  return (
    value.length > 0 &&
    value.length <= maximumLength &&
    !/[\u0000-\u001f\u007f]/.test(value)
  );
}

/**
 * Owns exactly one WebSocket for the current browser tab. Anonymous sessions
 * receive only public topics; authenticated sessions consume a single-use
 * ticket and may additionally subscribe to private topics. Feature modules
 * never own credentials, reconnect loops, heartbeat timers, or another socket.
 */
export function RealtimeProvider({ children }: { children: ReactNode }) {
  const session = useSession();
  const [state, setState] = useState<RealtimeConnectionState>("idle");
  const stateRef = useRef<RealtimeConnectionState>("idle");
  const clientRef = useRef<WebSocketClient | null>(null);
  const registrationsRef = useRef(new Map<symbol, AttachedRegistration>());
  const stateHandlersRef = useRef(
    new Map<symbol, (state: RealtimeConnectionState) => void>(),
  );

  const publishState = useCallback((next: RealtimeConnectionState) => {
    stateRef.current = next;
    setState(next);
    for (const handler of stateHandlersRef.current.values()) handler(next);
  }, []);

  const register = useCallback(
    (registration: AttachedRegistration): (() => void) => {
      const key = Symbol("realtime-registration");
      registrationsRef.current.set(key, registration);
      const client = clientRef.current;
      if (client) registration.release = registration.attach(client);
      return () => {
        const current = registrationsRef.current.get(key);
        if (!current) return;
        current.release?.();
        registrationsRef.current.delete(key);
      };
    },
    [],
  );

  const subscribe = useCallback<RealtimeContextValue["subscribe"]>(
    <TData,>(type: string, handler: RealtimeMessageHandler<TData>) => {
      if (!validSubscriptionValue(type, 192)) {
        throw new TypeError("invalid realtime event type");
      }
      return register({
        attach: (client) => client.subscribe(type, handler),
      });
    },
    [register],
  );

  const subscribeToTopic = useCallback(
    (topic: string): (() => void) => {
      if (!validSubscriptionValue(topic, 512)) {
        throw new TypeError("invalid realtime topic");
      }
      return register({
        attach: (client) => client.subscribeToTopic(topic),
      });
    },
    [register],
  );

  const onResyncRequired = useCallback(
    (handler: () => void): (() => void) =>
      register({ attach: (client) => client.onResyncRequired(handler) }),
    [register],
  );

  const onTopicStateChange = useCallback(
    (
      topic: string,
      handler: (state: RealtimeTopicState) => void,
    ): (() => void) => {
      if (!validSubscriptionValue(topic, 512)) {
        throw new TypeError("invalid realtime topic");
      }
      return register({
        attach: (client) => client.onTopicStateChange(topic, handler),
      });
    },
    [register],
  );

  const onStateChange = useCallback(
    (handler: (next: RealtimeConnectionState) => void): (() => void) => {
      const key = Symbol("realtime-state-handler");
      stateHandlersRef.current.set(key, handler);
      handler(stateRef.current);
      return () => stateHandlersRef.current.delete(key);
    },
    [],
  );

  const connectionKey =
    session.state.status === "authenticated"
      ? `authenticated:${session.state.session.user.id}:${session.state.session.session.id}`
      : session.state.status === "anonymous"
        ? "anonymous"
        : null;

  useEffect(() => {
    if (!connectionKey) return;

    const connection =
      session.state.status === "authenticated"
        ? {
            mode: REALTIME_CONNECTION_MODE.TICKET,
            issueTicket: (signal: AbortSignal) =>
              realtimeService.issueTicket(signal),
            url: resolveTicketedRealtimeWebSocketUrl,
          }
        : {
            mode: REALTIME_CONNECTION_MODE.ANONYMOUS,
            url: resolveAnonymousRealtimeWebSocketUrl,
          };
    const client = new WebSocketClient({
      connection,
      reconnect: {
        initialDelayMs: REALTIME_POLICY.RECONNECT_INITIAL_DELAY_MS,
        maximumDelayMs: REALTIME_POLICY.RECONNECT_MAXIMUM_DELAY_MS,
        multiplier: REALTIME_POLICY.RECONNECT_MULTIPLIER,
        jitterRatio: REALTIME_POLICY.RECONNECT_JITTER_RATIO,
      },
      openingTimeoutMs: REALTIME_POLICY.OPENING_TIMEOUT_MS,
      heartbeatIntervalMs: REALTIME_POLICY.HEARTBEAT_INTERVAL_MS,
      heartbeatTimeoutMs: REALTIME_POLICY.HEARTBEAT_TIMEOUT_MS,
    });
    clientRef.current = client;
    const removeInternalState = client.onStateChange((next) => {
      if (clientRef.current !== client) return;
      if (next === "open") client.resubscribe();
      publishState(next);
    });
    const registrations = registrationsRef.current;
    for (const registration of registrations.values()) {
      registration.release = registration.attach(client);
    }
    client.connect();

    return () => {
      if (clientRef.current === client) clientRef.current = null;
      for (const registration of registrations.values()) {
        registration.release?.();
        registration.release = undefined;
      }
      removeInternalState();
      client.close();
      publishState("idle");
    };
  }, [connectionKey, publishState, session.state.status]);

  const value = useMemo<RealtimeContextValue>(
    () => ({
      state,
      subscribe,
      subscribeToTopic,
      onResyncRequired,
      onStateChange,
      onTopicStateChange,
    }),
    [
      onResyncRequired,
      onStateChange,
      onTopicStateChange,
      state,
      subscribe,
      subscribeToTopic,
    ],
  );

  return (
    <RealtimeContext.Provider value={value}>
      {children}
    </RealtimeContext.Provider>
  );
}

export function useRealtime(): RealtimeContextValue {
  const context = useContext(RealtimeContext);
  if (!context) {
    throw new Error("useRealtime must be used within a RealtimeProvider");
  }
  return context;
}

/** Keeps authorization capabilities coherent through the shared connection. */
export function AuthorizationRealtimeSynchronizer() {
  const session = useSession();
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
  } = useRealtime();
  const sessionRef = useRef(session);
  useEffect(() => {
    sessionRef.current = session;
  }, [session]);
  const sessionKey =
    session.state.status === "authenticated"
      ? `${session.state.session.user.id}:${session.state.session.session.id}`
      : null;

  useEffect(() => {
    if (!sessionKey) return;

    let active = true;
    const topic = authorizationCapabilitiesTopic();
    const isCurrentSession = () => {
      const current = sessionRef.current.state;
      return (
        active &&
        current.status === "authenticated" &&
        `${current.session.user.id}:${current.session.session.id}` === sessionKey
      );
    };
    const sync = new AuthorizationRevisionSync({
      currentRevision: () => {
        const current = sessionRef.current.state;
        return isCurrentSession() && current.status === "authenticated"
          ? current.session.authorization_revision
          : null;
      },
      isCurrentSession,
      refresh: async (): Promise<AuthorizationRefreshResult> => {
        try {
          return (await sessionRef.current.revalidate()) ? "ready" : "invalid";
        } catch (error) {
          return error instanceof ApiError && error.status === 401
            ? "invalid"
            : "error";
        }
      },
      initialRetryDelayMs:
        REALTIME_POLICY.AUTHORIZATION_REFRESH_RETRY_INITIAL_DELAY_MS,
      maximumRetryDelayMs:
        REALTIME_POLICY.AUTHORIZATION_REFRESH_RETRY_MAXIMUM_DELAY_MS,
      retryMultiplier: REALTIME_POLICY.AUTHORIZATION_REFRESH_RETRY_MULTIPLIER,
    });
    const removeEvent = subscribe<unknown>(
      EVENT_AUTHORIZATION_CAPABILITIES_CHANGED_V1,
      (data, envelope) => {
        const revision = authorizationRevisionFromEvent(data, envelope, topic);
        if (revision !== null) sync.requestRevision(revision);
      },
    );
    const removeResync = onResyncRequired(() => sync.requestResync());
    const removeState = onStateChange((connectionState) => {
      if (connectionState === "open" && isCurrentSession()) sync.requestResync();
    });
    const releaseTopic = subscribeToTopic(topic);

    return () => {
      active = false;
      removeEvent();
      removeResync();
      removeState();
      releaseTopic();
      sync.stop();
    };
  }, [
    onResyncRequired,
    onStateChange,
    sessionKey,
    subscribe,
    subscribeToTopic,
  ]);

  return null;
}
