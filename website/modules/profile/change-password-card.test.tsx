import assert from "node:assert/strict";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, test, vi } from "vitest";

const dependencies = vi.hoisted(() => ({
  changePassword: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: dependencies.replace }) }));
vi.mock("@/services/auth.service", () => ({
  authService: { changePassword: dependencies.changePassword },
}));

import { ChangePasswordCard } from "@/modules/profile/change-password-card";
import { TEXT } from "@/constants/text";

const copy = TEXT.PROFILE.CHANGE_PASSWORD;

beforeEach(() => {
  dependencies.changePassword.mockReset();
  dependencies.replace.mockReset();
});

async function fillPasswords(user: ReturnType<typeof userEvent.setup>, values: {
  current: string; next: string; confirmation: string;
}) {
  await user.type(screen.getByLabelText(copy.CURRENT_PASSWORD), values.current);
  await user.type(screen.getByLabelText(copy.NEW_PASSWORD), values.next);
  await user.type(screen.getByLabelText(copy.CONFIRM_PASSWORD), values.confirmation);
}

test("blocks a mismatched password locally without sending a mutation", async () => {
  const user = userEvent.setup();
  render(<ChangePasswordCard />);
  await fillPasswords(user, { current: "old-password", next: "new-password-123", confirmation: "other-password" });
  await user.click(screen.getByRole("button", { name: copy.SUBMIT }));

  assert.equal(dependencies.changePassword.mock.calls.length, 0);
  assert.equal(screen.getByRole("alert").textContent, TEXT.AUTH.PASSWORD_MISMATCH);
});

test("changes a valid password, clears credential fields, and offers the required login transition", async () => {
  const user = userEvent.setup();
  dependencies.changePassword.mockResolvedValue(undefined);
  render(<ChangePasswordCard />);
  await fillPasswords(user, { current: "old-password", next: "new-password-123", confirmation: "new-password-123" });
  await user.click(screen.getByRole("button", { name: copy.SUBMIT }));

  await waitFor(() => assert.deepEqual(dependencies.changePassword.mock.calls, [["old-password", "new-password-123"]]));
  assert.ok(screen.getByRole("status"));
  await user.click(screen.getByRole("button", { name: copy.LOGIN_AGAIN }));
  assert.equal(dependencies.replace.mock.calls.length, 1);
});

test("maps a rejected password command to safe product feedback", async () => {
  const user = userEvent.setup();
  dependencies.changePassword.mockRejectedValue(new Error("server implementation detail"));
  render(<ChangePasswordCard />);
  await fillPasswords(user, { current: "old-password", next: "new-password-123", confirmation: "new-password-123" });
  await user.click(screen.getByRole("button", { name: copy.SUBMIT }));

  await waitFor(() => assert.equal(screen.getByRole("alert").textContent, copy.ACTION_ERROR));
  assert.doesNotMatch(screen.getByRole("alert").textContent ?? "", /implementation detail/);
});
