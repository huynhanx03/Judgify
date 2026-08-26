"use client";

import { useEffect } from "react";
import {
  contestLifecycleTopic,
  EVENT_CONTEST_LIFECYCLE_CHANGED_V1,
} from "@/constants/realtime";
import { useRealtime } from "@/contexts/realtime-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { parseContestInvalidationV1Envelope } from "@/lib/realtime/contest-event-schema";
import { REALTIME_TOPIC_STATE } from "@/lib/realtime/websocket-client";

interface UseContestLifecycleResourceOptions<T> {
  resourceKey: string;
  enabled?: boolean;
  initialData: T;
  load: (signal: AbortSignal) => Promise<T>;
}

/**
 * Keeps public contest projections coherent through the shared WebSocket.
 * REST is authoritative; lifecycle events, reconnects, resync requests, focus,
 * and visibility changes are bounded invalidations rather than polling loops.
 */
export function useContestLifecycleResource<T>({
  resourceKey,
  enabled = true,
  initialData,
  load,
}: UseContestLifecycleResourceOptions<T>) {
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
  } = useRealtime();
  const resource = useRetryableResource({
    resetKey: resourceKey,
    enabled,
    initialData,
    keepPreviousData: true,
    load,
  });
  const retry = resource.retry;

  useEffect(() => {
    if (!enabled) return;

    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") retry();
    };
    const topic = contestLifecycleTopic();
    const allowedTopics = new Set([topic]);
    const removeEvent = subscribe<unknown>(
      EVENT_CONTEST_LIFECYCLE_CHANGED_V1,
      (payload, envelope) => {
        try {
          parseContestInvalidationV1Envelope(
            payload,
            envelope,
            undefined,
            allowedTopics,
          );
          refreshWhenVisible();
        } catch {
          // Ignore malformed or misrouted frames. REST remains authoritative.
        }
      },
    );
    const removeResync = onResyncRequired(refreshWhenVisible);
    const removeState = onStateChange((state) => {
      if (state === "open") refreshWhenVisible();
    });
    const releaseTopic = subscribeToTopic(topic);
    const removeTopicState = onTopicStateChange(topic, (state) => {
      if (state === REALTIME_TOPIC_STATE.SUBSCRIBED) {
        refreshWhenVisible();
      }
    });

    window.addEventListener("focus", refreshWhenVisible);
    document.addEventListener("visibilitychange", refreshWhenVisible);
    return () => {
      releaseTopic();
      removeTopicState();
      removeEvent();
      removeResync();
      removeState();
      window.removeEventListener("focus", refreshWhenVisible);
      document.removeEventListener(
        "visibilitychange",
        refreshWhenVisible,
      );
    };
  }, [
    enabled,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
    retry,
    subscribe,
    subscribeToTopic,
  ]);

  return resource;
}
