import assert from "node:assert/strict";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, test, vi } from "vitest";

const dependencies = vi.hoisted(() => ({
  login: vi.fn(),
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
  startOAuth: vi.fn(),
}));

vi.mock("@/contexts/auth-context", () => ({
  useAuth: () => ({ login: dependencies.login }),
}));
vi.mock("@/services/auth.service", () => ({
  authService: { startOAuth: dependencies.startOAuth },
}));
vi.mock("@/lib/toast", () => ({
  notify: { error: dependencies.notifyError, success: dependencies.notifySuccess },
}));

import LoginForm from "@/modules/auth/LoginForm";
import { TEXT } from "@/constants/text";
import { OAUTH_PROVIDERS } from "@/constants/identity";

beforeEach(() => {
  dependencies.login.mockReset();
  dependencies.notifyError.mockReset();
  dependencies.notifySuccess.mockReset();
  dependencies.startOAuth.mockReset();
});

test("submits credentials through the canonical auth context and announces success", async () => {
  const user = userEvent.setup();
  dependencies.login.mockResolvedValue(undefined);
  render(<LoginForm />);

  await user.type(screen.getByLabelText(TEXT.AUTH.USERNAME), "judgify-user");
  await user.type(screen.getByLabelText(TEXT.AUTH.PASSWORD), "correct horse battery staple");
  await user.click(screen.getByRole("button", { name: TEXT.AUTH.LOGIN_BUTTON }));

  await waitFor(() => assert.deepEqual(dependencies.login.mock.calls, [[{
    username: "judgify-user",
    password: "correct horse battery staple",
  }]]));
  assert.deepEqual(dependencies.notifySuccess.mock.calls, [[TEXT.AUTH.LOGIN_SUCCESS]]);
});

test("maps a credential failure to safe localized feedback without exposing the cause", async () => {
  const user = userEvent.setup();
  dependencies.login.mockRejectedValue(new Error("backend implementation detail"));
  render(<LoginForm />);

  await user.type(screen.getByLabelText(TEXT.AUTH.USERNAME), "judgify-user");
  await user.type(screen.getByLabelText(TEXT.AUTH.PASSWORD), "wrong-password");
  await user.click(screen.getByRole("button", { name: TEXT.AUTH.LOGIN_BUTTON }));

  await waitFor(() => assert.deepEqual(dependencies.notifyError.mock.calls, [[TEXT.AUTH.LOGIN_ERROR]]));
  assert.doesNotMatch(JSON.stringify(dependencies.notifyError.mock.calls), /implementation detail/);
});

test("OAuth-start failures recover the controls and use provider-specific safe feedback", async () => {
  const user = userEvent.setup();
  const provider = OAUTH_PROVIDERS[0];
  const name = TEXT.AUTH.OAUTH_PROVIDER_NAME[provider];
  dependencies.startOAuth.mockRejectedValue(new TypeError("network unavailable"));
  render(<LoginForm />);

  await user.click(screen.getByRole("button", { name: TEXT.AUTH.OAUTH_LOGIN_WITH(name) }));
  await waitFor(() => assert.deepEqual(dependencies.startOAuth.mock.calls, [[provider]]));
  await waitFor(() => assert.deepEqual(dependencies.notifyError.mock.calls, [[
    TEXT.AUTH.OAUTH_START_ERROR(name),
  ]]));
  assert.equal(screen.getByRole("button", { name: TEXT.AUTH.OAUTH_LOGIN_WITH(name) }).hasAttribute("disabled"), false);
});
