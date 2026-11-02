import assert from "node:assert/strict";
import { act, renderHook } from "@testing-library/react";
import { afterEach, test, vi } from "vitest";

import {
  EVENT_CONTEST_RATING_CALCULATED_V1,
  EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1,
} from "@/constants/realtime";
import { useContestRatingChanges } from "@/hooks/use-contest-rating-changes";
import type { EntityID } from "@/types/api";

const CONTEST_ID = "contest-1" as EntityID;

const dependencies = vi.hoisted(() => ({
  events: new Map<string, (payload: unknown, envelope: unknown) => void>(),
  onResync: undefined as undefined | (() => void),
  onState: undefined as undefined | ((state: string) => void),
  onTopicState: undefined as undefined | ((state: string) => void),
  parse: vi.fn(),
  releaseTopic: vi.fn(),
  remove: vi.fn(),
  retry: vi.fn(),
  subscribe: vi.fn(),
  subscribeToTopic: vi.fn(),
}));

vi.mock("@/contexts/realtime-context", () => ({
  useRealtime: () => ({
    subscribe: dependencies.subscribe.mockImplementation((event, callback) => {
      dependencies.events.set(event, callback);
      return dependencies.remove;
    }),
    subscribeToTopic: dependencies.subscribeToTopic.mockImplementation(
      () => dependencies.releaseTopic,
    ),
    onResyncRequired: (callback: () => void) => {
      dependencies.onResync = callback;
      return dependencies.remove;
    },
    onStateChange: (callback: (state: string) => void) => {
      dependencies.onState = callback;
      return dependencies.remove;
    },
    onTopicStateChange: (_topic: string, callback: (state: string) => void) => {
      dependencies.onTopicState = callback;
      return dependencies.remove;
    },
  }),
}));

vi.mock("@/hooks/use-retryable-resource", () => ({
  useRetryableResource: () => ({ retry: dependencies.retry }),
}));

vi.mock("@/lib/realtime/contest-event-schema", () => ({
  parseContestInvalidationV1Envelope: dependencies.parse,
}));

function resetDependencies() {
  dependencies.events.clear();
  dependencies.onResync = undefined;
  dependencies.onState = undefined;
  dependencies.onTopicState = undefined;
  for (const value of Object.values(dependencies)) {
    if (typeof value === "function" && "mockReset" in value) value.mockReset();
  }
  dependencies.subscribe.mockImplementation((event, callback) => {
    dependencies.events.set(event, callback);
    return dependencies.remove;
  });
  dependencies.subscribeToTopic.mockImplementation(() => dependencies.releaseTopic);
}

afterEach(() => {
  resetDependencies();
  vi.restoreAllMocks();
});

test("refreshes rating data for both projection lifecycle events and rejects malformed frames", () => {
  const { unmount } = renderHook(() => useContestRatingChanges(CONTEST_ID, true));
  const calculated = dependencies.events.get(EVENT_CONTEST_RATING_CALCULATED_V1)!;
  const activated = dependencies.events.get(EVENT_CONTEST_RATING_PROJECTION_ACTIVATED_V1)!;

  act(() => calculated({}, {}));
  act(() => activated({}, {}));
  assert.equal(dependencies.retry.mock.calls.length, 2);
  assert.equal(dependencies.subscribeToTopic.mock.calls[0][0], "contest:contest-1:public");
  assert.equal(dependencies.parse.mock.calls[0][2], CONTEST_ID);

  dependencies.parse.mockImplementationOnce(() => {
    throw new TypeError("wrong contest");
  });
  act(() => calculated({}, {}));
  assert.equal(dependencies.retry.mock.calls.length, 2);

  unmount();
  assert.equal(dependencies.releaseTopic.mock.calls.length, 1);
  assert.equal(dependencies.remove.mock.calls.length, 5);
});

test("does not attach a rating stream without both a contest and an enabled view", () => {
  renderHook(() => useContestRatingChanges(null, true));
  assert.equal(dependencies.subscribe.mock.calls.length, 0);
  assert.equal(dependencies.subscribeToTopic.mock.calls.length, 0);
});
