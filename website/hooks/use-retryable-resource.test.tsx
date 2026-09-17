import assert from "node:assert/strict";
import { act, renderHook, waitFor } from "@testing-library/react";
import { test, vi } from "vitest";

import { useRetryableResource } from "@/hooks/use-retryable-resource";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((success, failure) => {
    resolve = success;
    reject = failure;
  });
  return { promise, reject, resolve };
}

test("settles a resource, exposes its data, and calls success exactly once", async () => {
  const load = vi.fn().mockResolvedValue("ready");
  const onSuccess = vi.fn();
  const { result } = renderHook(() =>
    useRetryableResource({ resetKey: "one", initialData: "empty", load, onSuccess }),
  );

  assert.equal(result.current.status, "loading");
  await waitFor(() => assert.equal(result.current.status, "ready"));
  assert.equal(result.current.data, "ready");
  assert.equal(onSuccess.mock.calls.length, 1);
  assert.equal(load.mock.calls.length, 1);
});

test("does not fetch or retry while intentionally disabled", async () => {
  const load = vi.fn();
  const { result } = renderHook(() =>
    useRetryableResource({ resetKey: "one", enabled: false, initialData: "empty", load }),
  );

  act(() => result.current.retry());
  assert.equal(result.current.status, "idle");
  assert.equal(load.mock.calls.length, 0);
});

test("aborts and fences an obsolete key while accepting the current response", async () => {
  const first = deferred<string>();
  const second = deferred<string>();
  const load = vi.fn()
    .mockImplementationOnce(() => first.promise)
    .mockImplementationOnce(() => second.promise);
  const { result, rerender } = renderHook(
    ({ resetKey }) => useRetryableResource({ resetKey, initialData: "empty", load }),
    { initialProps: { resetKey: "one" } },
  );

  rerender({ resetKey: "two" });
  assert.equal(load.mock.calls[0][0].aborted, true);
  await act(async () => second.resolve("current"));
  await waitFor(() => assert.equal(result.current.data, "current"));
  await act(async () => first.resolve("stale"));
  assert.equal(result.current.data, "current");
  assert.equal(result.current.dataKey, "two");
});

test("keeps a rendered snapshot on retry failure when requested", async () => {
  const load = vi.fn()
    .mockResolvedValueOnce("settled")
    .mockRejectedValueOnce(new TypeError("offline"));
  const { result } = renderHook(() =>
    useRetryableResource({
      resetKey: "one",
      initialData: "empty",
      keepPreviousData: true,
      load,
    }),
  );

  await waitFor(() => assert.equal(result.current.data, "settled"));
  act(() => result.current.retry());
  await waitFor(() => assert.equal(result.current.status, "error"));
  assert.equal(result.current.data, "settled");
  assert.match(String(result.current.error), /offline/);
});
