import { useEffect, useState } from "react";
import {
  contestStandingsTopic,
  EVENT_CONTEST_STANDINGS_CHANGED_V1,
} from "@/constants/realtime";
import { useRealtime } from "@/contexts/realtime-context";
import { parseContestStandingsChangedV1Envelope } from "@/lib/realtime/contest-standings-event-schema";
import { REALTIME_TOPIC_STATE } from "@/lib/realtime/websocket-client";
import { contestService } from "@/services/contest.service";
import type {
  SelfStandingSnapshot,
  Standing,
  StandingsSnapshot as ContestStandingsSnapshot,
} from "@/types/contest";
import type { EntityID } from "@/types/api";

interface UseContestStandings {
  standings: Standing[];
  view: ContestStandingsSnapshot["view"] | null;
  frozenAt: ContestStandingsSnapshot["frozen_at"] | null;
  projectionVersion: number;
  myStanding: Standing | null;
  hasSelfError: boolean;
  hasLoaded: boolean;
  isRefreshing: boolean;
  hasError: boolean;
  retry: () => void;
}

interface StandingsSnapshot {
  key: string;
  value: ContestStandingsSnapshot | null;
}

interface RefreshStatus {
  key: string;
  active: boolean;
}

interface SelfSnapshot {
  key: string;
  value: SelfStandingSnapshot | null;
}

/**
 * REST owns the authoritative standings snapshot. The shared per-tab socket
 * carries only invalidations, so reconnect and resync always reconcile REST.
 */
export function useContestStandings(
  contestId: EntityID | null,
  enabled: boolean,
  authenticated: boolean,
): UseContestStandings {
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
  } = useRealtime();
  const [snapshot, setSnapshot] = useState<StandingsSnapshot>({
    key: "",
    value: null,
  });
  const [refreshStatus, setRefreshStatus] = useState<RefreshStatus>({
    key: "",
    active: false,
  });
  const [selfSnapshot, setSelfSnapshot] = useState<SelfSnapshot>({
    key: "",
    value: null,
  });
  const [hasSelfError, setHasSelfError] = useState(false);
  const [hasError, setHasError] = useState(false);
  const [retryVersion, setRetryVersion] = useState(0);
  const standings =
    contestId !== null && snapshot.key === contestId
      ? snapshot.value?.items ?? []
      : [];
  const currentSnapshot =
    contestId !== null && snapshot.key === contestId
      ? snapshot.value
      : null;
  const myStanding =
    contestId !== null && selfSnapshot.key === contestId
      ? selfSnapshot.value?.item ?? null
      : null;
  const isRefreshing = Boolean(
    enabled &&
      contestId !== null &&
      refreshStatus.key === contestId &&
      refreshStatus.active,
  );

  useEffect(() => {
    if (!contestId || !enabled) return;

    let active = true;
    let refreshPromise: Promise<void> | null = null;
    let refreshQueued = false;
    let refreshController: AbortController | null = null;
    const topic = contestStandingsTopic(contestId);

    const refresh = (): Promise<void> => {
      if (refreshPromise) {
        refreshQueued = true;
        return refreshPromise;
      }

      refreshPromise = (async () => {
        do {
          refreshQueued = false;
          refreshController?.abort(
            new DOMException("Standings refresh superseded", "AbortError"),
          );
          const controller = new AbortController();
          refreshController = controller;
          if (active) setRefreshStatus({ key: contestId, active: true });
          try {
            const [nextSnapshot, nextSelf] = await Promise.all([
              contestService.getStandings(contestId, controller.signal),
              authenticated
                ? contestService
                    .getMyStanding(contestId, controller.signal)
                    .then((value) => ({ value, error: false }))
                    .catch((error: unknown) => {
                      if (
                        error instanceof DOMException &&
                        error.name === "AbortError"
                      ) {
                        throw error;
                      }
                      return { value: null, error: true };
                    })
                : Promise.resolve({ value: null, error: false }),
            ]);
            if (active && refreshController === controller) {
              setSnapshot({ key: contestId, value: nextSnapshot });
              setSelfSnapshot({ key: contestId, value: nextSelf.value });
              setHasSelfError(nextSelf.error);
              setHasError(false);
            }
          } catch (error) {
            if (
              active &&
              refreshController === controller &&
              !(error instanceof DOMException && error.name === "AbortError")
            ) {
              setHasError(true);
            }
          } finally {
            if (refreshController === controller) {
              refreshController = null;
            }
          }
        } while (active && refreshQueued);
        if (active) setRefreshStatus({ key: contestId, active: false });
      })().finally(() => {
        refreshPromise = null;
      });

      return refreshPromise;
    };

    const removeEvent = subscribe<unknown>(
      EVENT_CONTEST_STANDINGS_CHANGED_V1,
      (payload, envelope) => {
        try {
          parseContestStandingsChangedV1Envelope(
            payload,
            envelope,
            contestId,
            topic,
          );
          void refresh();
        } catch {
          // Ignore malformed or cross-topic invalidations.
        }
      },
    );
    const removeResync = onResyncRequired(() => void refresh());
    const removeState = onStateChange((state) => {
      if (active && state === "open") void refresh();
    });
    const releaseTopic = subscribeToTopic(topic);
    const removeTopicState = onTopicStateChange(topic, (state) => {
      if (active && state === REALTIME_TOPIC_STATE.SUBSCRIBED) {
        void refresh();
      }
    });
    void refresh();

    return () => {
      active = false;
      refreshController?.abort(
        new DOMException("Standings view closed", "AbortError"),
      );
      refreshController = null;
      removeTopicState();
      releaseTopic();
      removeEvent();
      removeResync();
      removeState();
    };
  }, [
    contestId,
    enabled,
    authenticated,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
    retryVersion,
    subscribe,
    subscribeToTopic,
  ]);

  return {
    standings,
    view: currentSnapshot?.view ?? null,
    frozenAt: currentSnapshot?.frozen_at ?? null,
    projectionVersion: currentSnapshot?.projection_version ?? 0,
    myStanding,
    hasSelfError: enabled && authenticated && hasSelfError,
    hasLoaded: currentSnapshot !== null,
    isRefreshing,
    hasError: enabled && hasError,
    retry: () => setRetryVersion((version) => version + 1),
  };
}
