import assert from "node:assert/strict";
import { act, renderHook } from "@testing-library/react";
import { afterEach, test, vi } from "vitest";

import { EVENT_CONTEST_LIFECYCLE_CHANGED_V1 } from "@/constants/realtime";
import { useContestLifecycleResource } from "@/hooks/use-contest-lifecycle-resource";

const dependencies = vi.hoisted(() => ({
  onEvent: undefined as undefined | ((payload: unknown, envelope: unknown) => void),
  onResync: undefined as undefined | (() => void),
  onState: undefined as undefined | ((state: string) => void),
  onTopicState: undefined as undefined | ((state: string) => void),
  parse: vi.fn(),
  releaseTopic: vi.fn(),
  removeEvent: vi.fn(),
  removeResync: vi.fn(),
  removeState: vi.fn(),
  removeTopicState: vi.fn(),
  retry: vi.fn(),
  subscribe: vi.fn(),
  subscribeToTopic: vi.fn(),
}));

vi.mock("@/contexts/realtime-context", () => ({
  useRealtime: () => ({
    subscribe: dependencies.subscribe.mockImplementation((_event, callback) => {
      dependencies.onEvent = callback;
      return dependencies.removeEvent;
    }),
    subscribeToTopic: dependencies.subscribeToTopic.mockImplementation(
      () => dependencies.releaseTopic,
    ),
    onResyncRequired: (callback: () => void) => {
      dependencies.onResync = callback;
      return dependencies.removeResync;
    },
    onStateChange: (callback: (state: string) => void) => {
      dependencies.onState = callback;
      return dependencies.removeState;
    },
    onTopicStateChange: (_topic: string, callback: (state: string) => void) => {
      dependencies.onTopicState = callback;
      return dependencies.removeTopicState;
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
  dependencies.onEvent = undefined;
  dependencies.onResync = undefined;
  dependencies.onState = undefined;
  dependencies.onTopicState = undefined;
  for (const value of Object.values(dependencies)) {
    if (typeof value === "function" && "mockReset" in value) value.mockReset();
  }
  dependencies.subscribe.mockImplementation((_event, callback) => {
    dependencies.onEvent = callback;
    return dependencies.removeEvent;
  });
  dependencies.subscribeToTopic.mockImplementation(() => dependencies.releaseTopic);
}

afterEach(() => {
  resetDependencies();
  vi.restoreAllMocks();
});

test("reconciles only valid lifecycle invalidations while visible and cleans up subscriptions", () => {
  const { unmount } = renderHook(() =>
    useContestLifecycleResource({
      resourceKey: "contest-list",
      initialData: [],
      load: async () => [],
    }),
  );
  const eventCallback = dependencies.onEvent!;

  assert.equal(dependencies.subscribe.mock.calls[0][0], EVENT_CONTEST_LIFECYCLE_CHANGED_V1);
  assert.equal(dependencies.subscribeToTopic.mock.calls[0][0], "contest:lifecycle");
  act(() => eventCallback({}, {}));
  act(() => dependencies.onResync?.());
  act(() => dependencies.onState?.("open"));
  act(() => dependencies.onTopicState?.("subscribed"));
  act(() => window.dispatchEvent(new Event("focus")));
  assert.equal(dependencies.retry.mock.calls.length, 5);

  dependencies.parse.mockImplementationOnce(() => {
    throw new TypeError("misrouted");
  });
  act(() => eventCallback({}, {}));
  assert.equal(dependencies.retry.mock.calls.length, 5);

  unmount();
  assert.equal(dependencies.releaseTopic.mock.calls.length, 1);
  assert.equal(dependencies.removeEvent.mock.calls.length, 1);
  assert.equal(dependencies.removeResync.mock.calls.length, 1);
  assert.equal(dependencies.removeState.mock.calls.length, 1);
  assert.equal(dependencies.removeTopicState.mock.calls.length, 1);
});

test("does not subscribe or retry when the lifecycle resource is disabled", () => {
  renderHook(() =>
    useContestLifecycleResource({
      resourceKey: "contest-list",
      enabled: false,
      initialData: [],
      load: async () => [],
    }),
  );

  assert.equal(dependencies.subscribe.mock.calls.length, 0);
  assert.equal(dependencies.subscribeToTopic.mock.calls.length, 0);
  assert.equal(dependencies.retry.mock.calls.length, 0);
});
