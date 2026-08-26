"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

export type AsyncResourceStatus = "idle" | "loading" | "ready" | "error";

interface AsyncResourceState<T> {
  data: T;
  dataKey: string | number | boolean | null | undefined;
  error: unknown | null;
  outcome: "ready" | "error" | null;
  settledRequest: object | null;
}

interface UseRetryableResourceOptions<T> {
  /** A stable identity for the resource. Changing it resets stale data. */
  resetKey: string | number | boolean | null | undefined;
  /** Set false when the caller intentionally cannot or should not fetch. */
  enabled?: boolean;
  /** Safe data rendered before the first successful response. */
  initialData: T;
  /** Preserve a rendered snapshot while a new key or retry is in flight. */
  keepPreviousData?: boolean;
  load: (signal: AbortSignal) => Promise<T>;
  onSuccess?: (data: T) => void;
}

/**
 * Small async resource state machine with retry and stale-response fencing.
 *
 * The loader is held in a ref so callers can close over the latest ID without
 * turning function identity into an accidental refetch trigger.
 */
export function useRetryableResource<T>({
  resetKey,
  enabled = true,
  initialData,
  keepPreviousData = false,
  load,
  onSuccess,
}: UseRetryableResourceOptions<T>) {
  const loadRef = useRef(load);
  const onSuccessRef = useRef(onSuccess);
  const initialDataRef = useRef(initialData);
  const requestSequenceRef = useRef(0);
  const [retryVersion, setRetryVersion] = useState(0);
  const requestIdentity = useMemo(
    () => ({ enabled, keepPreviousData, resetKey, retryVersion }),
    [enabled, keepPreviousData, resetKey, retryVersion],
  );
  const [state, setState] = useState<AsyncResourceState<T>>({
    data: initialData,
    dataKey: resetKey,
    error: null,
    outcome: null,
    settledRequest: null,
  });

  useEffect(() => {
    loadRef.current = load;
    onSuccessRef.current = onSuccess;
    initialDataRef.current = initialData;
  }, [initialData, load, onSuccess]);

  useEffect(() => {
    const requestSequence = ++requestSequenceRef.current;

    if (!enabled) {
      return;
    }

    const controller = new AbortController();

    void loadRef.current(controller.signal)
      .then((data) => {
        if (requestSequence !== requestSequenceRef.current) return;
        onSuccessRef.current?.(data);
        setState({
          data,
          dataKey: resetKey,
          error: null,
          outcome: "ready",
          settledRequest: requestIdentity,
        });
      })
      .catch((error: unknown) => {
        if (requestSequence !== requestSequenceRef.current) return;
        setState((current) => ({
          data: keepPreviousData ? current.data : initialDataRef.current,
          dataKey: keepPreviousData ? current.dataKey : resetKey,
          error,
          outcome: "error",
          settledRequest: requestIdentity,
        }));
      });

    return () => {
      if (requestSequence === requestSequenceRef.current) {
        requestSequenceRef.current += 1;
      }
      controller.abort(
        new DOMException("Resource request superseded", "AbortError"),
      );
    };
  }, [enabled, keepPreviousData, requestIdentity, resetKey]);

  const retry = useCallback(() => {
    if (!enabled) return;
    setRetryVersion((version) => version + 1);
  }, [enabled]);

  const requestIsSettled = state.settledRequest === requestIdentity;
  const status: AsyncResourceStatus = !enabled
    ? "idle"
    : requestIsSettled
      ? (state.outcome ?? "loading")
      : "loading";
  const exposeSettledData =
    enabled && (requestIsSettled || keepPreviousData);
  const data = exposeSettledData ? state.data : initialData;
  const dataKey = exposeSettledData ? state.dataKey : resetKey;

  return {
    data,
    dataKey,
    error: status === "error" ? state.error : null,
    status,
    isPreviousData: dataKey !== resetKey,
    /** Opaque identity for the current enable/key/retry request generation. */
    requestToken: requestIdentity,
    retry,
  };
}
