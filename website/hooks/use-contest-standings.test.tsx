import assert from "node:assert/strict";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, test, vi } from "vitest";

import { useContestStandings } from "@/hooks/use-contest-standings";
import type { EntityID } from "@/types/api";

const CONTEST_ID = "contest-1" as EntityID;

const dependencies = vi.hoisted(() => ({
  event: undefined as undefined | ((payload: unknown, envelope: unknown) => void),
  getMyStanding: vi.fn(),
  getStandings: vi.fn(),
  onResyncRequired: vi.fn(),
  onStateChange: vi.fn(),
  onTopicStateChange: vi.fn(),
  parse: vi.fn(),
  releaseTopic: vi.fn(),
  removeEvent: vi.fn(),
  removeResync: vi.fn(),
  removeState: vi.fn(),
  removeTopicState: vi.fn(),
  subscribe: vi.fn(),
  subscribeToTopic: vi.fn(),
}));

vi.mock("@/contexts/realtime-context", () => ({
  useRealtime: () => ({
    subscribe: dependencies.subscribe.mockImplementation((_event, callback) => {
      dependencies.event = callback;
      return dependencies.removeEvent;
    }),
    subscribeToTopic: dependencies.subscribeToTopic.mockImplementation(
      () => dependencies.releaseTopic,
    ),
    onResyncRequired: dependencies.onResyncRequired.mockImplementation(
      () => dependencies.removeResync,
    ),
    onStateChange: dependencies.onStateChange.mockImplementation(
      () => dependencies.removeState,
    ),
    onTopicStateChange: dependencies.onTopicStateChange.mockImplementation(
      () => dependencies.removeTopicState,
    ),
  }),
}));

vi.mock("@/services/contest.service", () => ({
  contestService: {
    getMyStanding: dependencies.getMyStanding,
    getStandings: dependencies.getStandings,
  },
}));

vi.mock("@/lib/realtime/contest-standings-event-schema", () => ({
  parseContestStandingsChangedV1Envelope: dependencies.parse,
}));

const snapshot = {
  contest_id: "contest-1",
  frozen_at: "2026-01-01T00:00:00.000Z",
  items: [{ rank: 1, user_id: "user-1", username: "ada", solved_count: 3, penalty: 12, problem_results: {}, pending_attempts: 0, pending_by_problem: {} }],
  last_cursor: "cursor-1",
  projection_version: 7,
  view: "official" as const,
};

function resetDependencies() {
  dependencies.event = undefined;
  for (const value of Object.values(dependencies)) {
    if (typeof value === "function" && "mockReset" in value) value.mockReset();
  }
  dependencies.subscribe.mockImplementation((_event, callback) => {
    dependencies.event = callback;
    return dependencies.removeEvent;
  });
  dependencies.subscribeToTopic.mockImplementation(() => dependencies.releaseTopic);
  dependencies.onResyncRequired.mockImplementation(() => dependencies.removeResync);
  dependencies.onStateChange.mockImplementation(() => dependencies.removeState);
  dependencies.onTopicStateChange.mockImplementation(() => dependencies.removeTopicState);
}

afterEach(() => {
  resetDependencies();
  vi.restoreAllMocks();
});

test("loads the public projection without requesting a private standing for an anonymous visitor", async () => {
  dependencies.getStandings.mockResolvedValue(snapshot);
  const { result } = renderHook(() => useContestStandings(CONTEST_ID, true, false));

  await waitFor(() => assert.equal(result.current.hasLoaded, true));
  assert.deepEqual(result.current.standings, snapshot.items);
  assert.equal(result.current.view, "official");
  assert.equal(result.current.projectionVersion, 7);
  assert.equal(result.current.myStanding, null);
  assert.equal(dependencies.getMyStanding.mock.calls.length, 0);
  assert.equal(result.current.hasError, false);
});

test("keeps the public standings usable when the optional private standing fails", async () => {
  dependencies.getStandings.mockResolvedValue(snapshot);
  dependencies.getMyStanding.mockRejectedValue(new TypeError("forbidden"));
  const { result } = renderHook(() => useContestStandings(CONTEST_ID, true, true));

  await waitFor(() => assert.equal(result.current.hasLoaded, true));
  await waitFor(() => assert.equal(result.current.hasSelfError, true));
  assert.equal(result.current.hasError, false);
  assert.deepEqual(result.current.standings, snapshot.items);
});

test("ignores rejected realtime frames and supports an explicit authoritative retry", async () => {
  dependencies.getStandings.mockResolvedValue(snapshot);
  const { result } = renderHook(() => useContestStandings(CONTEST_ID, true, false));
  await waitFor(() => assert.equal(result.current.hasLoaded, true));
  const initialCalls = dependencies.getStandings.mock.calls.length;

  dependencies.parse.mockImplementationOnce(() => {
    throw new TypeError("cross-topic");
  });
  act(() => dependencies.event?.({}, {}));
  assert.equal(dependencies.getStandings.mock.calls.length, initialCalls);

  act(() => result.current.retry());
  await waitFor(() => assert.equal(dependencies.getStandings.mock.calls.length, initialCalls + 1));
});

test("does not request or subscribe before standings are enabled and scoped to a contest", () => {
  renderHook(() => useContestStandings(null, false, true));

  assert.equal(dependencies.getStandings.mock.calls.length, 0);
  assert.equal(dependencies.subscribe.mock.calls.length, 0);
  assert.equal(dependencies.subscribeToTopic.mock.calls.length, 0);
});
