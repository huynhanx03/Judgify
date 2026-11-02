import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import { ADMIN_TEXT } from "@/constants/admin-text";
import { PermissionMatrix } from "@/modules/admin/permission-matrix";

test("permission matrix reuses the labelled table viewport and exposes each toggle by name", () => {
  const onToggle = vi.fn();
  render(
    <PermissionMatrix
      resources={[{ key: "problem", label: "Bài tập", actions: [{ key: "read", label: "Xem" }] }]}
      policies={[]}
      onToggle={onToggle}
    />,
  );

  assert.ok(screen.getByRole("region", { name: ADMIN_TEXT.PERMISSIONS.TABLE_LABEL }));
  const grant = screen.getByRole("button", { name: ADMIN_TEXT.PERMISSIONS.GRANT("Xem", "Bài tập") });
  fireEvent.click(grant);
  assert.deepEqual(onToggle.mock.calls, [["problem", "read", false]]);
});
