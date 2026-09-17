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
  EVENT_NOTIFICATION_ARCHIVED_V1,
  EVENT_NOTIFICATION_CREATED_V1,
  EVENT_NOTIFICATION_READ_V1,
  notificationsTopic,
} from "@/constants/realtime";
import {
  NOTIFICATION_POLICY,
  type NotificationCategory,
} from "@/constants/notification";
import { useRealtime } from "@/contexts/realtime-context";
import { useSession } from "@/contexts/session-context";
import { ApiError } from "@/lib/api/error";
import { isoDateTimeSchema } from "@/lib/api/contracts";
import { parseNotificationLivePayload } from "@/lib/notifications/notification-schema";
import { notificationService } from "@/services/notification.service";
import type { RealtimeEnvelope } from "@/lib/realtime/websocket-client";
import type {
  Notification,
  NotificationListResponse,
  NotificationMutationResponse,
  NotificationPreference,
} from "@/types/notification";

export type NotificationResourceStatus =
  | "idle"
  | "loading"
  | "ready"
  | "error";

interface NotificationContextValue {
  items: Notification[];
  unreadCount: number;
  inboxVersion: number;
  nextCursor: string | null;
  status: NotificationResourceStatus;
  error: ApiError | null;
  isLoadingMore: boolean;
  pendingNotificationIDs: ReadonlySet<string>;
  isMarkingAllRead: boolean;
  preferences: NotificationPreference[];
  preferenceStatus: NotificationResourceStatus;
  preferenceError: ApiError | null;
  pendingPreferenceCategories: ReadonlySet<NotificationCategory>;
  refresh(): Promise<void>;
  loadMore(): Promise<void>;
  markRead(id: string): Promise<void>;
  markAllRead(): Promise<void>;
  archive(id: string): Promise<void>;
  loadPreferences(options?: { force?: boolean }): Promise<void>;
  updatePreference(
    category: NotificationCategory,
    inApp: boolean,
  ): Promise<void>;
}

const NotificationContext = createContext<NotificationContextValue | null>(null);

function clientError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  return new ApiError({
    code: "notification_client_error",
    status: 503,
    cid: crypto.randomUUID(),
    retryable: true,
    cause: error,
  });
}

function replaceNotification(
  items: readonly Notification[],
  next: Notification,
): Notification[] {
  const index = items.findIndex((item) => item.id === next.id);
  if (index < 0) return [next, ...items];
  if (items[index].version > next.version) return [...items];
  const copy = [...items];
  const current = items[index];
  copy[index] =
    next.content_kind === "campaign" &&
    !next.campaign_title &&
    current.campaign_title
      ? { ...next, campaign_title: current.campaign_title }
      : next;
  return copy;
}

function mergePage(
  current: readonly Notification[],
  page: readonly Notification[],
): Notification[] {
  const seen = new Set(current.map((notification) => notification.id));
  return [
    ...current,
    ...page.filter((notification) => {
      if (seen.has(notification.id)) return false;
      seen.add(notification.id);
      return true;
    }),
  ];
}

/** Canonical REST inbox with shared-WebSocket invalidation acceleration. */
export function NotificationProvider({ children }: { children: ReactNode }) {
  const session = useSession();
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
  } = useRealtime();
  const [items, setItems] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [inboxVersion, setInboxVersion] = useState(0);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [status, setStatus] = useState<NotificationResourceStatus>("idle");
  const [error, setError] = useState<ApiError | null>(null);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [pendingNotificationIDs, setPendingNotificationIDs] = useState(
    () => new Set<string>(),
  );
  const [isMarkingAllRead, setIsMarkingAllRead] = useState(false);
  const [preferences, setPreferences] = useState<NotificationPreference[]>([]);
  const [preferenceStatus, setPreferenceStatus] =
    useState<NotificationResourceStatus>("idle");
  const [preferenceError, setPreferenceError] = useState<ApiError | null>(null);
  const [pendingPreferenceCategories, setPendingPreferenceCategories] = useState(
    () => new Set<NotificationCategory>(),
  );
  const inboxVersionRef = useRef(0);
  const nextCursorRef = useRef<string | null>(null);
  const generationRef = useRef(0);
  const snapshotRequestRef = useRef<Promise<void> | null>(null);
  const preferenceRequestRef = useRef<Promise<void> | null>(null);
  const snapshotAbortRef = useRef<AbortController | null>(null);
  const preferenceAbortRef = useRef<AbortController | null>(null);
  const reconciliationTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastReconciledAtRef = useRef(0);
  const userID =
    session.state.status === "authenticated"
      ? session.state.session.user.id
      : null;

  const commitPage = useCallback((page: NotificationListResponse): boolean => {
    if (page.inbox_version < inboxVersionRef.current) return false;
    inboxVersionRef.current = page.inbox_version;
    nextCursorRef.current = page.next_cursor ?? null;
    setItems(page.items);
    setUnreadCount(page.unread_count);
    setInboxVersion(page.inbox_version);
    setNextCursor(page.next_cursor ?? null);
    setError(null);
    setStatus("ready");
    lastReconciledAtRef.current = Date.now();
    return true;
  }, []);

  const refresh = useCallback(async (): Promise<void> => {
    if (!userID) return;
    if (snapshotRequestRef.current) return snapshotRequestRef.current;
    const generation = generationRef.current;
    const controller = new AbortController();
    snapshotAbortRef.current?.abort();
    snapshotAbortRef.current = controller;
    setStatus((current) => (current === "ready" ? current : "loading"));
    setError(null);
    const request = notificationService
      .list({ limit: NOTIFICATION_POLICY.DEFAULT_PAGE_SIZE }, controller.signal)
      .then((page) => {
        if (
          controller.signal.aborted ||
          generation !== generationRef.current
        ) {
          return;
        }
        if (!commitPage(page)) {
          lastReconciledAtRef.current = 0;
        }
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted || generation !== generationRef.current) return;
        setError(clientError(reason));
        setStatus((current) => (current === "ready" ? current : "error"));
      })
      .finally(() => {
        if (snapshotAbortRef.current === controller) snapshotAbortRef.current = null;
        if (snapshotRequestRef.current === request) snapshotRequestRef.current = null;
      });
    snapshotRequestRef.current = request;
    return request;
  }, [commitPage, userID]);

  const scheduleReconciliation = useCallback(() => {
    if (reconciliationTimerRef.current !== null) return;
    reconciliationTimerRef.current = setTimeout(() => {
      reconciliationTimerRef.current = null;
      void refresh();
    }, NOTIFICATION_POLICY.LIVE_RECONCILIATION_DELAY_MS);
  }, [refresh]);

  const applyMutation = useCallback(
    (response: NotificationMutationResponse, kind: "read" | "read-all" | "archive") => {
      if (response.inbox_version < inboxVersionRef.current) return;
      inboxVersionRef.current = response.inbox_version;
      setInboxVersion(response.inbox_version);
      setUnreadCount(response.unread_count);
      if (kind === "read-all") {
        const readAt = isoDateTimeSchema.parse(new Date().toISOString());
        setItems((current) =>
          current.map((notification) =>
            notification.read_at
              ? notification
              : { ...notification, read_at: readAt },
          ),
        );
      } else {
        const notification = response.notification;
        if (notification) {
          setItems((current) =>
            kind === "archive"
              ? current.filter((item) => item.id !== notification.id)
              : replaceNotification(current, notification),
          );
        }
      }
      scheduleReconciliation();
    },
    [scheduleReconciliation],
  );

  const runNotificationMutation = useCallback(
    async (
      id: string,
      command: () => Promise<NotificationMutationResponse>,
      kind: "read" | "archive",
    ): Promise<void> => {
      if (pendingNotificationIDs.has(id)) return;
      setPendingNotificationIDs((current) => new Set(current).add(id));
      try {
        applyMutation(await command(), kind);
      } finally {
        setPendingNotificationIDs((current) => {
          const next = new Set(current);
          next.delete(id);
          return next;
        });
      }
    },
    [applyMutation, pendingNotificationIDs],
  );

  const markRead = useCallback(
    (id: string) =>
      runNotificationMutation(id, () => notificationService.markRead(id), "read"),
    [runNotificationMutation],
  );

  const archive = useCallback(
    (id: string) =>
      runNotificationMutation(id, () => notificationService.archive(id), "archive"),
    [runNotificationMutation],
  );

  const markAllRead = useCallback(async (): Promise<void> => {
    if (isMarkingAllRead || unreadCount === 0) return;
    setIsMarkingAllRead(true);
    try {
      applyMutation(await notificationService.markAllRead(), "read-all");
    } finally {
      setIsMarkingAllRead(false);
    }
  }, [applyMutation, isMarkingAllRead, unreadCount]);

  const loadMore = useCallback(async (): Promise<void> => {
    const cursor = nextCursorRef.current;
    if (!userID || !cursor || isLoadingMore) return;
    const generation = generationRef.current;
    const versionAtStart = inboxVersionRef.current;
    setIsLoadingMore(true);
    try {
      const page = await notificationService.list({
        cursor,
        limit: NOTIFICATION_POLICY.DEFAULT_PAGE_SIZE,
      });
      if (generation !== generationRef.current) return;
      if (page.inbox_version !== versionAtStart) {
        await refresh();
        return;
      }
      nextCursorRef.current = page.next_cursor ?? null;
      setItems((current) => mergePage(current, page.items));
      setNextCursor(page.next_cursor ?? null);
      setUnreadCount(page.unread_count);
    } finally {
      if (generation === generationRef.current) setIsLoadingMore(false);
    }
  }, [isLoadingMore, refresh, userID]);

  const loadPreferences = useCallback(
    async (options: { force?: boolean } = {}): Promise<void> => {
      if (!userID) return;
      if (!options.force && preferenceStatus === "ready") return;
      if (preferenceRequestRef.current) return preferenceRequestRef.current;
      const generation = generationRef.current;
      const controller = new AbortController();
      preferenceAbortRef.current?.abort();
      preferenceAbortRef.current = controller;
      setPreferenceStatus("loading");
      setPreferenceError(null);
      const request = notificationService
        .preferences(controller.signal)
        .then((next) => {
          if (controller.signal.aborted || generation !== generationRef.current) return;
          setPreferences(next);
          setPreferenceStatus("ready");
        })
        .catch((reason: unknown) => {
          if (controller.signal.aborted || generation !== generationRef.current) return;
          setPreferenceError(clientError(reason));
          setPreferenceStatus("error");
        })
        .finally(() => {
          if (preferenceAbortRef.current === controller) preferenceAbortRef.current = null;
          if (preferenceRequestRef.current === request) preferenceRequestRef.current = null;
        });
      preferenceRequestRef.current = request;
      return request;
    },
    [preferenceStatus, userID],
  );

  const updatePreference = useCallback(
    async (category: NotificationCategory, inApp: boolean): Promise<void> => {
      if (pendingPreferenceCategories.has(category)) return;
      const current = preferences.find((item) => item.category === category);
      if (!current || current.required || current.in_app === inApp) return;
      setPendingPreferenceCategories((pending) => new Set(pending).add(category));
      try {
        const updated = await notificationService.updatePreference(category, {
          in_app: inApp,
          expected_version: current.version,
        });
        setPreferences((values) =>
          values.map((value) => (value.category === category ? updated : value)),
        );
        setPreferenceError(null);
      } catch (reason) {
        const normalized = clientError(reason);
        setPreferenceError(normalized);
        if (normalized.status === 409) await loadPreferences({ force: true });
        throw normalized;
      } finally {
        setPendingPreferenceCategories((pending) => {
          const next = new Set(pending);
          next.delete(category);
          return next;
        });
      }
    },
    [loadPreferences, pendingPreferenceCategories, preferences],
  );

  useEffect(() => {
    generationRef.current += 1;
    snapshotAbortRef.current?.abort();
    preferenceAbortRef.current?.abort();
    if (reconciliationTimerRef.current !== null) {
      clearTimeout(reconciliationTimerRef.current);
      reconciliationTimerRef.current = null;
    }
    snapshotRequestRef.current = null;
    preferenceRequestRef.current = null;
    inboxVersionRef.current = 0;
    nextCursorRef.current = null;
    setItems([]);
    setUnreadCount(0);
    setInboxVersion(0);
    setNextCursor(null);
    setError(null);
    setPreferences([]);
    setPreferenceError(null);
    setPreferenceStatus("idle");
    setPendingNotificationIDs(new Set());
    setPendingPreferenceCategories(new Set());
    if (!userID) {
      setStatus("idle");
      return;
    }
    setStatus("loading");
    void refresh();
    return () => {
      snapshotAbortRef.current?.abort();
      preferenceAbortRef.current?.abort();
    };
  }, [refresh, userID]);

  useEffect(() => {
    if (!userID) return;
    const topic = notificationsTopic(userID);
    const onLiveEvent = (
      data: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
      if (envelope.topic !== topic) throw new TypeError("notification topic mismatch");
      const payload = parseNotificationLivePayload(
        data,
        envelope.aggregate_version,
      );
      if (payload.user_id !== userID) {
        throw new TypeError("notification owner mismatch");
      }
      if (payload.inbox_version <= inboxVersionRef.current) return;
      inboxVersionRef.current = payload.inbox_version;
      setInboxVersion(payload.inbox_version);
      setUnreadCount(payload.unread_count);
      const notification = payload.notification;
      if (envelope.type === EVENT_NOTIFICATION_ARCHIVED_V1 && notification) {
        setItems((current) =>
          current.filter((item) => item.id !== notification.id),
        );
      } else if (notification) {
        setItems((current) => replaceNotification(current, notification));
      }
      scheduleReconciliation();
    };
    const removeCreated = subscribe(
      EVENT_NOTIFICATION_CREATED_V1,
      onLiveEvent,
    );
    const removeRead = subscribe(EVENT_NOTIFICATION_READ_V1, onLiveEvent);
    const removeArchived = subscribe(
      EVENT_NOTIFICATION_ARCHIVED_V1,
      onLiveEvent,
    );
    const removeResync = onResyncRequired(scheduleReconciliation);
    const removeState = onStateChange((connectionState) => {
      if (connectionState === "open") scheduleReconciliation();
    });
    const releaseTopic = subscribeToTopic(topic);
    return () => {
      removeCreated();
      removeRead();
      removeArchived();
      removeResync();
      removeState();
      releaseTopic();
    };
  }, [
    onResyncRequired,
    onStateChange,
    scheduleReconciliation,
    subscribe,
    subscribeToTopic,
    userID,
  ]);

  useEffect(() => {
    if (!userID) return;
    const onVisibilityChange = () => {
      if (
        document.visibilityState === "visible" &&
        Date.now() - lastReconciledAtRef.current >=
          NOTIFICATION_POLICY.VISIBILITY_RECONCILIATION_AGE_MS
      ) {
        void refresh();
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => document.removeEventListener("visibilitychange", onVisibilityChange);
  }, [refresh, userID]);

  useEffect(
    () => () => {
      if (reconciliationTimerRef.current !== null) {
        clearTimeout(reconciliationTimerRef.current);
        reconciliationTimerRef.current = null;
      }
    },
    [],
  );

  const value = useMemo<NotificationContextValue>(
    () => ({
      items,
      unreadCount,
      inboxVersion,
      nextCursor,
      status,
      error,
      isLoadingMore,
      pendingNotificationIDs,
      isMarkingAllRead,
      preferences,
      preferenceStatus,
      preferenceError,
      pendingPreferenceCategories,
      refresh,
      loadMore,
      markRead,
      markAllRead,
      archive,
      loadPreferences,
      updatePreference,
    }),
    [
      archive,
      error,
      inboxVersion,
      isLoadingMore,
      isMarkingAllRead,
      items,
      loadMore,
      loadPreferences,
      markAllRead,
      markRead,
      nextCursor,
      pendingNotificationIDs,
      pendingPreferenceCategories,
      preferenceError,
      preferenceStatus,
      preferences,
      refresh,
      status,
      unreadCount,
      updatePreference,
    ],
  );

  return (
    <NotificationContext.Provider value={value}>
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications(): NotificationContextValue {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error("useNotifications must be used within NotificationProvider");
  }
  return context;
}
