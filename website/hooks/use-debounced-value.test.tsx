import assert from "node:assert/strict";
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, test, vi } from "vitest";

import { useDebouncedValue } from "@/hooks/use-debounced-value";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

test("debounces successive values and only publishes the latest stable value", () => {
  const { result, rerender } = renderHook(
    ({ value }) => useDebouncedValue(value, 250),
    { initialProps: { value: "first" } },
  );

  rerender({ value: "second" });
  act(() => vi.advanceTimersByTime(249));
  assert.equal(result.current, "first");

  rerender({ value: "final" });
  act(() => vi.advanceTimersByTime(250));
  assert.equal(result.current, "final");
});

test("rejects invalid timing contracts before scheduling browser work", () => {
  assert.throws(
    () => renderHook(() => useDebouncedValue("value", -1)),
    /non-negative/,
  );
  assert.throws(
    () => renderHook(() => useDebouncedValue("value", Number.NaN)),
    /non-negative/,
  );
});
