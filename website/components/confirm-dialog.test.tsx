import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { text } from "@/i18n/text";

test("confirmation dialog exposes named destructive controls and honors disabled state", () => {
  const onConfirm = vi.fn();
  const onOpenChange = vi.fn();
  render(
    <ConfirmDialog
      open
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      title="Delete this item?"
      description="This cannot be undone."
      confirmLabel="Delete item"
      confirmDisabled
    />,
  );

  assert.ok(screen.getByRole("dialog", { name: "Delete this item?" }));
  assert.equal(screen.getByRole("button", { name: "Delete item" }).hasAttribute("disabled"), true);
  fireEvent.click(screen.getByRole("button", { name: text("COMMON.CANCEL") }));
  assert.deepEqual(onOpenChange.mock.calls, [[false]]);
  assert.equal(onConfirm.mock.calls.length, 0);
});

test("confirmation invokes the supplied action only when available", () => {
  const onConfirm = vi.fn();
  render(
    <ConfirmDialog
      open
      onConfirm={onConfirm}
      onOpenChange={() => undefined}
      title="Confirm action"
      description="Proceed?"
      confirmLabel="Proceed"
    />,
  );

  fireEvent.click(screen.getByRole("button", { name: "Proceed" }));
  assert.equal(onConfirm.mock.calls.length, 1);
});
