import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import {
  DataViewport,
  ResourceCollectionFeedback,
  ResourceErrorFeedback,
  ResourceLoadingFeedback,
} from "@/modules/shared/resource-feedback";

test("data viewport gives a keyboard user a named, focusable scroll region without visual framing", () => {
  render(
    <DataViewport label="Problem results">
      <p>Problem rows</p>
    </DataViewport>,
  );

  const viewport = screen.getByRole("region", { name: "Problem results" });
  assert.equal(viewport.tabIndex, 0);
  assert.ok(viewport.className.includes("overflow-y-auto"));
  assert.ok(!viewport.className.includes("border"));
});

test("collection failure uses the receiving content area instead of adding an alert card", () => {
  render(
    <ResourceCollectionFeedback
      title="Could not load articles"
      description="Try again shortly."
      retryLabel="Retry"
      onRetry={() => undefined}
    />,
  );

  const feedback = screen.getByRole("alert");
  assert.ok(!feedback.className.includes("border"));
  assert.ok(!feedback.className.includes("rounded"));
  assert.ok(screen.getByRole("button", { name: "Retry" }));
});

test("resource error feedback announces a recoverable error and invokes its retry action", () => {
  const onRetry = vi.fn();
  render(
    <ResourceErrorFeedback
      title="Could not load results"
      description="Your current filters are preserved."
      retryLabel="Retry"
      onRetry={onRetry}
    />,
  );

  assert.ok(screen.getByRole("alert"));
  assert.ok(screen.getByText("Your current filters are preserved."));
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  assert.equal(onRetry.mock.calls.length, 1);
});

test("compact feedback preserves a full-width recovery target and loading exposes status", () => {
  const { rerender } = render(
    <ResourceErrorFeedback
      compact
      title="Could not load results"
      retryLabel="Retry"
      onRetry={() => undefined}
    />,
  );
  assert.ok(screen.getByRole("button", { name: "Retry" }).className.includes("w-full"));

  rerender(<ResourceLoadingFeedback bare label="Loading results" />);
  const loading = screen.getByRole("status");
  assert.ok(!loading.className.includes("border"));
  assert.ok(!loading.className.includes("rounded"));
  assert.ok(screen.getByText("Loading results"));
});
