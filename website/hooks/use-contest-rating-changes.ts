"use client";

import { useEffect } from "react";

import {
  contestPublicTopic,
  EVENT_CONTEST_RATING_CALCULATED_V1,
  EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
} from "@/constants/realtime";
import { useRealtime } from "@/contexts/realtime-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { parseContestInvalidationV1Envelope } from "@/lib/realtime/contest-event-schema";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeEnvelope,
} from "@/lib/realtime/websocket-client";
import { contestService } from "@/services/contest.service";
import type { EntityID } from "@/types/api";
import type { RatingChange } from "@/types/contest";

// Rating REST data remains authoritative. The contest public topic carries a
// content-free invalidation after the immutable run becomes active.
export function useContestRatingChanges(
  contestID: EntityID | null,
  enabled: boolean,
) {
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
  } = useRealtime();
  const resource = useRetryableResource<RatingChange[]>({
    resetKey: contestID,
    enabled: enabled && contestID !== null,
    initialData: [],
    keepPreviousData: true,
    load: (signal) =>
      contestID
        ? contestService.getRatingChanges(contestID, signal)
        : Promise.resolve([]),
  });
  const retry = resource.retry;

  useEffect(() => {
    if (!enabled || !contestID) return;
    const topic = contestPublicTopic(contestID);
    const allowedTopics = new Set([topic]);
    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") retry();
    };
    const invalidate = (
      payload: unknown,
      envelope: RealtimeEnvelope<unknown>,
    ) => {
        try {
          parseContestInvalidationV1Envelope(
            payload,
            envelope,
            contestID,
            allowedTopics,
          );
          refreshWhenVisible();
        } catch {
          // Ignore malformed or cross-topic frames.
        }
      };
    const removeEvents = [
      subscribe<unknown>(
        EVENT_CONTEST_RATING_CALCULATED_V1,
        invalidate,
      ),
      subscribe<unknown>(
        EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
        invalidate,
      ),
    ];
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
      removeTopicState();
      releaseTopic();
      removeEvents.forEach((remove) => remove());
      removeResync();
      removeState();
      window.removeEventListener("focus", refreshWhenVisible);
      document.removeEventListener(
        "visibilitychange",
        refreshWhenVisible,
      );
    };
  }, [
    contestID,
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
