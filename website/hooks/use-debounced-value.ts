"use client";

import { useEffect, useState } from "react";

/** Returns the latest value only after it remains unchanged for the delay. */
export function useDebouncedValue<T>(value: T, delayMilliseconds: number): T {
  if (!Number.isFinite(delayMilliseconds) || delayMilliseconds < 0) {
    throw new RangeError("debounce delay must be a non-negative number");
  }

  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const timeout = window.setTimeout(
      () => setDebouncedValue(value),
      delayMilliseconds,
    );
    return () => window.clearTimeout(timeout);
  }, [delayMilliseconds, value]);

  return debouncedValue;
}
