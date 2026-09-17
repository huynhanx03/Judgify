import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  EVENT_SUBMISSION_JUDGED_V1,
  userSubmissionsTopic,
} from "@/constants/realtime";
import { useRealtime } from "@/contexts/realtime-context";
import {
  REALTIME_TOPIC_STATE,
  type RealtimeTopicState,
} from "@/lib/realtime/websocket-client";
import { parseSubmissionJudgedV1Envelope } from "@/lib/realtime/submission-event-schema";
import { compareISODateTime } from "@/lib/api/contracts";
import { submissionMatchesContest } from "@/lib/submissions/contest-context";
import { submissionService } from "@/services/submission.service";
import type { Cursor, EntityID } from "@/types/api";
import type { SubmissionSummary } from "@/types/submission";

interface RefreshFlight {
  key: string;
  queued: boolean;
  controller: AbortController;
  promise: Promise<void>;
}

interface SubmissionSnapshot {
  key: string;
  submissions: SubmissionSummary[];
  nextCursor: Cursor | null;
  hasMore: boolean;
  loadedPages: number;
}

interface TopicStatus {
  key: string;
  state: RealtimeTopicState;
}

interface LatestRequest {
  key: string;
  id: number;
}

interface LoadMoreFlight {
  key: string;
  cursor: Cursor;
  controller: AbortController;
}

export type ProblemSubmissionsStatus =
  | "idle"
  | "loading"
  | "ready"
  | "error";

interface LoadStatus {
  key: string;
  status: Exclude<ProblemSubmissionsStatus, "idle">;
  error: unknown | null;
}

export interface UseProblemSubmissionsResult {
  submissions: SubmissionSummary[];
  status: ProblemSubmissionsStatus;
  error: unknown | null;
  isLoading: boolean;
  isRefreshing: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  topicState: RealtimeTopicState;
  recordSubmission: (submission: SubmissionSummary) => void;
  refresh: () => Promise<void>;
  loadMore: () => Promise<void>;
}

function mergeSubmissions(
  first: readonly SubmissionSummary[],
  second: readonly SubmissionSummary[],
): SubmissionSummary[] {
  const seen = new Set<string>();
  return [...first, ...second]
    .filter((submission) => {
      if (seen.has(submission.id)) return false;
      seen.add(submission.id);
      return true;
    })
    .sort((left, right) => {
      const temporalOrder = compareISODateTime(
        right.created_at,
        left.created_at,
      );
      if (temporalOrder !== 0) return temporalOrder;
      if (right.id < left.id) return -1;
      if (right.id > left.id) return 1;
      return 0;
    });
}

function validateSubmissionPageContext(
  submissions: readonly SubmissionSummary[],
  problemId: EntityID,
  userId: EntityID,
  contestId: EntityID | null,
): SubmissionSummary[] {
  if (
    submissions.some(
      (submission) =>
        submission.problem_id !== problemId ||
        submission.user_id !== userId ||
        !submissionMatchesContest(submission, contestId),
    )
  ) {
    throw new TypeError("submission page escaped its owner context");
  }
  return [...submissions];
}

/**
 * Owns the authoritative owner snapshot for one problem and contest context.
 * Critical realtime summaries update the visible row immediately, while REST
 * remains authoritative. Reconnect and explicit resync reconcile any delivery
 * gap through the single shared per-tab WebSocket.
 */
export function useProblemSubmissions(
  problemId: EntityID | null,
  userId: EntityID | null,
  contestId: EntityID | null = null,
): UseProblemSubmissionsResult {
  const {
    subscribe,
    subscribeToTopic,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
  } = useRealtime();
  const queryKey =
    userId && problemId
      ? JSON.stringify([userId, problemId, contestId])
      : null;
  const topic = userId && queryKey ? userSubmissionsTopic(userId) : null;
  const [snapshot, setSnapshot] = useState<SubmissionSnapshot>({
    key: "",
    submissions: [],
    nextCursor: null,
    hasMore: false,
    loadedPages: 0,
  });
  const [loadStatus, setLoadStatus] = useState<LoadStatus>({
    key: "",
    status: "loading",
    error: null,
  });
  const [topicStatus, setTopicStatus] = useState<TopicStatus>({
    key: "",
    state: REALTIME_TOPIC_STATE.INACTIVE,
  });
  const [loadingMoreKey, setLoadingMoreKey] = useState<string | null>(
    null,
  );
  const flightRef = useRef<RefreshFlight | null>(null);
  const loadMoreFlightRef = useRef<LoadMoreFlight | null>(null);
  const requestSequenceRef = useRef(0);
  const latestRequestRef = useRef<LatestRequest | null>(null);
  const activeQueryRef = useRef(queryKey);

  const hasCurrentSnapshot =
    queryKey !== null && snapshot.key === queryKey;
  const submissions = useMemo(
    () => hasCurrentSnapshot ? snapshot.submissions : [],
    [hasCurrentSnapshot, snapshot.submissions],
  );
  const status: ProblemSubmissionsStatus =
    queryKey === null
      ? "idle"
      : loadStatus.key === queryKey
        ? loadStatus.status
        : "loading";
  const error =
    status === "error" && loadStatus.key === queryKey
      ? loadStatus.error
      : null;
  const isLoading = status === "loading" && !hasCurrentSnapshot;
  const isRefreshing = status === "loading" && hasCurrentSnapshot;
  const isLoadingMore =
    queryKey !== null && loadingMoreKey === queryKey;
  const hasMore = hasCurrentSnapshot && snapshot.hasMore;
  const topicState =
    topic !== null && topicStatus.key === topic
      ? topicStatus.state
      : REALTIME_TOPIC_STATE.INACTIVE;

  useEffect(() => {
    activeQueryRef.current = queryKey;
    return () => {
      if (loadMoreFlightRef.current?.key === queryKey) {
        loadMoreFlightRef.current.controller.abort(
          new DOMException("Submission page superseded", "AbortError"),
        );
        loadMoreFlightRef.current = null;
      }
      if (activeQueryRef.current === queryKey) {
        activeQueryRef.current = null;
      }
    };
  }, [queryKey]);

  const refresh = useCallback((): Promise<void> => {
    if (!userId || !problemId || !queryKey) return Promise.resolve();

    const currentFlight = flightRef.current;
    if (currentFlight?.key === queryKey) {
      currentFlight.queued = true;
      return currentFlight.promise;
    }
    currentFlight?.controller.abort(
      new DOMException("Submission history superseded", "AbortError"),
    );

    const flight: RefreshFlight = {
      key: queryKey,
      queued: false,
      controller: new AbortController(),
      promise: Promise.resolve(),
    };
    flight.promise = (async () => {
      do {
        flight.queued = false;
        const requestID = ++requestSequenceRef.current;
        latestRequestRef.current = { key: queryKey, id: requestID };
        if (activeQueryRef.current === queryKey) {
          setLoadStatus({
            key: queryKey,
            status: "loading",
            error: null,
          });
        }

        try {
          const page = await submissionService.getMyByProblem(
            problemId,
            contestId ? { contest_id: contestId } : {},
            flight.controller.signal,
          );
          const next = validateSubmissionPageContext(
            page.items,
            problemId,
            userId,
            contestId,
          );
          if (
            activeQueryRef.current === queryKey &&
            latestRequestRef.current?.key === queryKey &&
            latestRequestRef.current.id === requestID
          ) {
            setSnapshot((current) => {
              const retained =
                current.key === queryKey ? current.submissions : [];
              const keepLoadedCursor =
                current.key === queryKey && current.loadedPages > 1;
              return {
                key: queryKey,
                submissions: mergeSubmissions(next, retained),
                nextCursor: keepLoadedCursor
                  ? current.nextCursor
                  : page.next_cursor,
                hasMore: keepLoadedCursor
                  ? current.hasMore
                  : page.has_more,
                loadedPages: keepLoadedCursor
                  ? current.loadedPages
                  : 1,
              };
            });
            setLoadStatus({
              key: queryKey,
              status: "ready",
              error: null,
            });
          }
        } catch (requestError: unknown) {
          if (
            activeQueryRef.current === queryKey &&
            latestRequestRef.current?.key === queryKey &&
            latestRequestRef.current.id === requestID
          ) {
            setLoadStatus({
              key: queryKey,
              status: "error",
              error: requestError,
            });
          }
        }
      } while (flight.queued && activeQueryRef.current === queryKey);
    })().finally(() => {
      if (flightRef.current === flight) flightRef.current = null;
    });
    flightRef.current = flight;
    return flight.promise;
  }, [contestId, problemId, queryKey, userId]);

  const recordSubmission = useCallback(
    (submission: SubmissionSummary) => {
      if (
        !queryKey ||
        !problemId ||
        !userId ||
        submission.problem_id !== problemId ||
        submission.user_id !== userId ||
        !submissionMatchesContest(submission, contestId)
      ) {
        return;
      }

      const requestID = ++requestSequenceRef.current;
      latestRequestRef.current = { key: queryKey, id: requestID };
      setSnapshot((current) => ({
        key: queryKey,
        submissions: mergeSubmissions(
          [submission],
          current.key === queryKey ? current.submissions : [],
        ),
        nextCursor:
          current.key === queryKey ? current.nextCursor : null,
        hasMore: current.key === queryKey && current.hasMore,
        loadedPages:
          current.key === queryKey ? current.loadedPages : 0,
      }));
      setLoadStatus({ key: queryKey, status: "ready", error: null });
    },
    [contestId, problemId, queryKey, userId],
  );

  const loadMore = useCallback(async (): Promise<void> => {
    if (
      !userId ||
      !problemId ||
      !queryKey ||
      snapshot.key !== queryKey ||
      !snapshot.hasMore ||
      !snapshot.nextCursor ||
      loadMoreFlightRef.current?.key === queryKey ||
      flightRef.current?.key === queryKey
    ) {
      return;
    }

    const cursor = snapshot.nextCursor;
    const flight: LoadMoreFlight = {
      key: queryKey,
      cursor,
      controller: new AbortController(),
    };
    loadMoreFlightRef.current?.controller.abort(
      new DOMException("Submission page superseded", "AbortError"),
    );
    loadMoreFlightRef.current = flight;
    setLoadingMoreKey(queryKey);
    try {
      const page = await submissionService.getMyByProblem(problemId, {
        cursor,
        ...(contestId ? { contest_id: contestId } : {}),
      }, flight.controller.signal);
      if (
        flight.controller.signal.aborted ||
        activeQueryRef.current !== queryKey ||
        loadMoreFlightRef.current !== flight
      ) {
        return;
      }
      const next = validateSubmissionPageContext(
        page.items,
        problemId,
        userId,
        contestId,
      );
      setSnapshot((current) => {
        if (
          current.key !== queryKey ||
          current.nextCursor !== cursor
        ) {
          return current;
        }
        return {
          key: queryKey,
          submissions: mergeSubmissions(current.submissions, next),
          nextCursor: page.next_cursor,
          hasMore: page.has_more,
          loadedPages: current.loadedPages + 1,
        };
      });
      setLoadStatus({
        key: queryKey,
        status: "ready",
        error: null,
      });
    } catch (requestError: unknown) {
      if (
        !flight.controller.signal.aborted &&
        activeQueryRef.current === queryKey &&
        loadMoreFlightRef.current === flight
      ) {
        setLoadStatus({
          key: queryKey,
          status: "error",
          error: requestError,
        });
      }
    } finally {
      if (loadMoreFlightRef.current === flight) {
        loadMoreFlightRef.current = null;
        setLoadingMoreKey((current) =>
          current === queryKey ? null : current,
        );
      }
    }
  }, [
    contestId,
    problemId,
    queryKey,
    snapshot,
    userId,
  ]);

  useEffect(() => {
    if (!userId || !problemId || !topic) return;

    let active = true;
    const removeEventHandler = subscribe<unknown>(
      EVENT_SUBMISSION_JUDGED_V1,
      (payload, envelope) => {
        let event;
        try {
          event = parseSubmissionJudgedV1Envelope(
            payload,
            envelope,
            topic,
          );
        } catch {
          return;
        }
        if (
          event.problem_id !== problemId ||
          event.user_id !== userId ||
          !submissionMatchesContest(event, contestId)
        ) {
          return;
        }
        // The durable outbox event intentionally contains only immutable
        // routing/verdict facts. Do not synthesize a partial submission row;
        // REST is the authoritative source for language and resource usage.
        void refresh();
      },
    );
    const removeResyncHandler = onResyncRequired(() => {
      void refresh();
    });
    const removeStateHandler = onStateChange((connectionState) => {
      if (!active) return;
      if (connectionState === "open") {
        void refresh();
      }
    });
    const releaseTopic = subscribeToTopic(topic);
    const removeTopicStateHandler = onTopicStateChange(
      topic,
      (state) => {
        if (active) setTopicStatus({ key: topic, state });
      },
    );
    void refresh();

    return () => {
      active = false;
      setTopicStatus((current) =>
        current.key === topic
          ? { key: "", state: REALTIME_TOPIC_STATE.INACTIVE }
          : current,
      );
      removeTopicStateHandler();
      releaseTopic();
      removeEventHandler();
      removeResyncHandler();
      removeStateHandler();
    };
  }, [
    contestId,
    onResyncRequired,
    onStateChange,
    onTopicStateChange,
    problemId,
    recordSubmission,
    refresh,
    subscribe,
    subscribeToTopic,
    topic,
    userId,
  ]);

  return {
    submissions,
    status,
    error,
    isLoading,
    isRefreshing,
    isLoadingMore,
    hasMore,
    topicState,
    recordSubmission,
    refresh,
    loadMore,
  };
}
