import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import { RouteFeedback } from "@/modules/shared/route-feedback";

test("error feedback exposes an accessible recovery action and home navigation", () => {
  const onRetry = vi.fn();
  render(
    <RouteFeedback
      kind="error"
      eyebrow="Problem"
      title="Could not load"
      description="Try again shortly."
      homeHref="/arena"
      homeLabel="Back to arena"
      retryLabel="Try again"
      onRetry={onRetry}
    />,
  );

  assert.equal(screen.getByRole("heading", { name: "Could not load" }).tagName, "H1");
  assert.equal(screen.getByRole("link", { name: "Back to arena" }).getAttribute("href"), "/arena");
  assert.equal(document.querySelectorAll("[class*='ring-8']").length, 0);
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  assert.equal(onRetry.mock.calls.length, 1);
});

test("not-found feedback does not render a non-functional retry button", () => {
  render(
    <RouteFeedback
      kind="not-found"
      eyebrow="Missing"
      title="Not found"
      description="This page does not exist."
      homeHref="/arena"
      homeLabel="Back to arena"
    />,
  );

  assert.equal(screen.queryByRole("button"), null);
  assert.ok(screen.getByRole("link", { name: "Back to arena" }));
});
