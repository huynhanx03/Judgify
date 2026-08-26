"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

export interface CursorFeedPage<T> {
  items: T[];
  next_cursor?: string;
}

export type CursorFeedStatus =
  | "idle"
  | "loading"
  | "refreshing"
  | "loading_more"
  | "ready"
  | "error";

interface CursorFeedState<T> {
  dataKey: string | number | boolean | null | undefined;
  items: T[];
  nextCursor: string | undefined;
  error: unknown | null;
  outcome: "ready" | "error" | null;
  operation: "idle" | "loading_more";
  settledRequest: object | null;
}

interface UseCursorFeedOptions<T> {
  resetKey: string | number | boolean | null | undefined;
  enabled?: boolean;
  load: (
    cursor: string | undefined,
    signal: AbortSignal,
  ) => Promise<CursorFeedPage<T>>;
  keyOf: (item: T) => string;
}

/**
 * Cursor feed state machine for append-only product feeds.
 *
 * It fences stale responses, aborts superseded requests, preserves the latest
 * snapshot during refresh failures, and deduplicates items by a caller-owned
 * stable identity. It never polls; realtime invalidation calls `reload`.
 */
export function useCursorFeed<T>({
  resetKey,
  enabled = true,
  load,
  keyOf,
}: UseCursorFeedOptions<T>) {
  const loadRef = useRef(load);
  const keyOfRef = useRef(keyOf);
  const activeControllerRef = useRef<AbortController | null>(null);
  const sequenceRef = useRef(0);
  const [reloadVersion, setReloadVersion] = useState(0);
  const requestIdentity = useMemo(
    () => ({ enabled, reloadVersion, resetKey }),
    [enabled, reloadVersion, resetKey],
  );
  const [state, setState] = useState<CursorFeedState<T>>({
    dataKey: resetKey,
    items: [],
    nextCursor: undefined,
    error: null,
    outcome: null,
    operation: "idle",
    settledRequest: null,
  });

  useEffect(() => {
    loadRef.current = load;
    keyOfRef.current = keyOf;
  }, [keyOf, load]);

  useEffect(() => {
    activeControllerRef.current?.abort(
      new DOMException("Cursor feed request superseded", "AbortError"),
    );
    const sequence = ++sequenceRef.current;

    if (!enabled) {
      activeControllerRef.current = null;
      return;
    }

    const controller = new AbortController();
    activeControllerRef.current = controller;

    void loadRef.current(undefined, controller.signal)
      .then((page) => {
        if (
          sequence !== sequenceRef.current ||
          controller.signal.aborted
        ) {
          return;
        }
        setState({
          dataKey: resetKey,
          items: page.items,
          nextCursor: page.next_cursor,
          error: null,
          outcome: "ready",
          operation: "idle",
          settledRequest: requestIdentity,
        });
      })
      .catch((error: unknown) => {
        if (
          sequence !== sequenceRef.current ||
          controller.signal.aborted
        ) {
          return;
        }
        setState((current) => ({
          dataKey: resetKey,
          items: current.dataKey === resetKey ? current.items : [],
          nextCursor:
            current.dataKey === resetKey ? current.nextCursor : undefined,
          error,
          outcome: "error",
          operation: "idle",
          settledRequest: requestIdentity,
        }));
      });

    return () => {
      if (sequence === sequenceRef.current) sequenceRef.current += 1;
      controller.abort(
        new DOMException("Cursor feed request superseded", "AbortError"),
      );
    };
  }, [enabled, requestIdentity, resetKey]);

  const reload = useCallback(() => {
    if (!enabled) return;
    setReloadVersion((current) => current + 1);
  }, [enabled]);

  const requestIsSettled = state.settledRequest === requestIdentity;
  const sameFeed = enabled && state.dataKey === resetKey;
  const items = sameFeed ? state.items : [];
  const nextCursor = sameFeed ? state.nextCursor : undefined;
  const status: CursorFeedStatus = !enabled
    ? "idle"
    : !requestIsSettled
      ? items.length > 0
        ? "refreshing"
        : "loading"
      : state.operation === "loading_more"
        ? "loading_more"
        : (state.outcome ?? "loading");

  const loadMore = useCallback(async () => {
    if (
      !enabled ||
      !nextCursor ||
      status === "loading" ||
      status === "refreshing" ||
      status === "loading_more"
    ) {
      return;
    }
    activeControllerRef.current?.abort(
      new DOMException("Cursor feed request superseded", "AbortError"),
    );
    const controller = new AbortController();
    activeControllerRef.current = controller;
    const sequence = ++sequenceRef.current;
    const cursor = nextCursor;
    setState((current) => ({
      ...current,
      error: null,
      operation: "loading_more",
    }));
    try {
      const page = await loadRef.current(cursor, controller.signal);
      if (
        sequence !== sequenceRef.current ||
        controller.signal.aborted
      ) {
        return;
      }
      if (page.next_cursor === cursor) {
        throw new TypeError("cursor feed did not advance");
      }
      setState((current) => {
        const identities = new Set(current.items.map(keyOfRef.current));
        const appended = page.items.filter((item) => {
          const identity = keyOfRef.current(item);
          if (identities.has(identity)) return false;
          identities.add(identity);
          return true;
        });
        return {
          ...current,
          items: [...current.items, ...appended],
          nextCursor: page.next_cursor,
          error: null,
          outcome: "ready",
          operation: "idle",
        };
      });
    } catch (error) {
      if (
        sequence !== sequenceRef.current ||
        controller.signal.aborted
      ) {
        return;
      }
      setState((current) => ({
        ...current,
        error,
        outcome: "error",
        operation: "idle",
      }));
    }
  }, [enabled, nextCursor, status]);

  return {
    dataKey: sameFeed ? state.dataKey : resetKey,
    items,
    nextCursor,
    error: status === "error" ? state.error : null,
    status,
    hasMore: Boolean(nextCursor),
    reload,
    loadMore,
  };
}
