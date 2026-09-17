import assert from "node:assert/strict";
import { act, renderHook, waitFor } from "@testing-library/react";
import { test, vi } from "vitest";

import { useCursorFeed } from "@/hooks/use-cursor-feed";

type Item = { id: string };

test("loads cursor pages, deduplicates append results, and exposes pagination state", async () => {
  const load = vi.fn()
    .mockResolvedValueOnce({ items: [{ id: "a" }], next_cursor: "next" })
    .mockResolvedValueOnce({ items: [{ id: "a" }, { id: "b" }] });
  const { result } = renderHook(() =>
    useCursorFeed<Item>({ resetKey: "feed", load, keyOf: (item) => item.id }),
  );

  await waitFor(() => assert.equal(result.current.status, "ready"));
  assert.deepEqual(result.current.items, [{ id: "a" }]);
  assert.equal(result.current.hasMore, true);

  await act(async () => result.current.loadMore());
  assert.deepEqual(result.current.items, [{ id: "a" }, { id: "b" }]);
  assert.equal(result.current.hasMore, false);
  assert.deepEqual(load.mock.calls.map(([cursor]) => cursor), [undefined, "next"]);
});

test("preserves the latest feed after a failed refresh and makes the error retryable", async () => {
  const load = vi.fn()
    .mockResolvedValueOnce({ items: [{ id: "a" }] })
    .mockRejectedValueOnce(new TypeError("offline"));
  const { result } = renderHook(() =>
    useCursorFeed<Item>({ resetKey: "feed", load, keyOf: (item) => item.id }),
  );

  await waitFor(() => assert.equal(result.current.status, "ready"));
  act(() => result.current.reload());
  await waitFor(() => assert.equal(result.current.status, "error"));
  assert.deepEqual(result.current.items, [{ id: "a" }]);
  assert.match(String(result.current.error), /offline/);
});

test("does not request pages while disabled", () => {
  const load = vi.fn();
  const { result } = renderHook(() =>
    useCursorFeed<Item>({ resetKey: "feed", enabled: false, load, keyOf: (item) => item.id }),
  );

  act(() => { result.current.reload(); void result.current.loadMore(); });
  assert.equal(result.current.status, "idle");
  assert.equal(load.mock.calls.length, 0);
});
