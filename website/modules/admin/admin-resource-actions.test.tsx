import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, test, vi } from "vitest";

const auth = vi.hoisted(() => ({ can: vi.fn() }));
vi.mock("@/contexts/auth-context", () => ({ useAuth: () => auth }));

import { AdminResourceActions } from "@/modules/admin/admin-resource-actions";
import { AUTHORIZATION_ACTION } from "@/constants/authorization";

beforeEach(() => auth.can.mockReset());

test("does not expose mutation controls without the exact grants", () => {
  auth.can.mockReturnValue(false);
  const { container } = render(
    <AdminResourceActions resource="problem" onEdit={() => undefined} onDelete={() => undefined} />,
  );
  assert.equal(container.innerHTML, "");
});

test("renders only actions authorized for the current principal", () => {
  const onEdit = vi.fn();
  const onDelete = vi.fn();
  auth.can.mockImplementation((_resource: string, action: string) => action === AUTHORIZATION_ACTION.UPDATE);
  render(
    <AdminResourceActions
      resource="problem"
      onEdit={onEdit}
      onDelete={onDelete}
      editLabel="Edit problem"
      deleteLabel="Delete problem"
    />,
  );

  fireEvent.click(screen.getByRole("button", { name: "Edit problem" }));
  assert.equal(onEdit.mock.calls.length, 1);
  assert.equal(screen.queryByRole("button", { name: "Delete problem" }), null);
  assert.deepEqual(auth.can.mock.calls, [
    ["problem", AUTHORIZATION_ACTION.UPDATE],
    ["problem", AUTHORIZATION_ACTION.DELETE],
  ]);
});

test("does not offer an action when its callback is unavailable", () => {
  auth.can.mockReturnValue(true);
  render(<AdminResourceActions resource="problem" deleteLabel="Delete problem" />);
  assert.equal(screen.queryByRole("button", { name: "Delete problem" }), null);
});
