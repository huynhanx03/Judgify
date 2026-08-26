import assert from "node:assert/strict";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, test, vi } from "vitest";

const services = vi.hoisted(() => ({
  me: vi.fn(),
  meOrNull: vi.fn(),
  refresh: vi.fn(),
  logout: vi.fn(),
}));

vi.mock("@/services/session.service", () => ({
  sessionService: {
    me: services.me,
    meOrNull: services.meOrNull,
  },
}));

vi.mock("@/services/auth.service", () => ({
  authService: {
    refresh: services.refresh,
    logout: services.logout,
  },
}));

import {
  anonymousSessionState,
  authenticatedSessionState,
  SessionProvider,
  sessionStateAfterFailure,
  useSession,
} from "@/contexts/session-context";
import { ApiError } from "@/lib/api/error";
import { entityIDSchema } from "@/lib/api/contracts";
import type { SessionView } from "@/types/session";
import { sessionViewSchema } from "@/lib/auth/session-schema";

function wrapper({ children }: { children: React.ReactNode }) {
  return <SessionProvider initialSession={SESSION}>{children}</SessionProvider>;
}

const SESSION: SessionView = sessionViewSchema.parse({
  user: {
    id: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
    username: "coder",
  },
  session: {
    id: "019f6abb-8dd5-7581-8449-2b9ad77873e5",
    expires_at: "2026-07-28T12:00:00.000Z",
  },
  capabilities: ["problem:read", "submission:create"],
  authorization_revision: 9,
});

test("canonical state distinguishes anonymous from authenticated and exposes exact capabilities", () => {
  assert.deepEqual(anonymousSessionState(), { status: "anonymous" });

  const state = authenticatedSessionState(SESSION);
  assert.equal(state.status, "authenticated");
  assert.equal(state.session, SESSION);
});

test("terminal unauthorized converges to anonymous while retryable failure stays visible", () => {
  const unauthorized = new ApiError({
    code: "authentication_required",
    status: 401,
    cid: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    retryable: false,
  });
  assert.deepEqual(sessionStateAfterFailure(unauthorized), {
    status: "anonymous",
  });

  const unavailable = new ApiError({
    code: "session_unavailable",
    status: 503,
    cid: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    retryable: true,
  });
  assert.deepEqual(sessionStateAfterFailure(unavailable), {
    status: "error",
    error: unavailable,
  });
});

beforeEach(() => {
  services.me.mockReset();
  services.meOrNull.mockReset();
  services.refresh.mockReset();
  services.logout.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

test("provider refreshes, updates its canonical projection, and exposes capabilities", async () => {
  const next = sessionViewSchema.parse({
    ...SESSION,
    capabilities: ["contest:read"],
    authorization_revision: 10,
  });
  services.meOrNull.mockResolvedValue(next);
  const { result } = renderHook(() => useSession(), { wrapper });

  assert.equal(result.current.has("problem:read"), true);
  assert.equal(result.current.has("contest:read"), false);

  await act(async () => {
    assert.equal(await result.current.revalidate(), next);
  });

  assert.deepEqual(result.current.state, authenticatedSessionState(next));
  assert.equal(result.current.has("problem:read"), false);
  assert.equal(result.current.has("contest:read"), true);

  act(() => {
    result.current.updatePrincipal({ id: SESSION.user.id, username: "renamed" });
  });
  assert.equal(
    result.current.state.status === "authenticated" && result.current.state.session.user.username,
    "renamed",
  );
});

test("provider establishes a browser session only after the canonical session read", async () => {
  services.me.mockResolvedValue(SESSION);
  const command = vi.fn().mockResolvedValue(undefined);
  const { result } = renderHook(() => useSession(), {
    wrapper: ({ children }) => <SessionProvider initialSession={null}>{children}</SessionProvider>,
  });

  await act(async () => {
    assert.equal(await result.current.establish("login", command), SESSION);
  });

  assert.equal(command.mock.calls.length, 1);
  assert.match(command.mock.calls[0][0], /^[0-9a-f-]{36}$/);
  assert.deepEqual(result.current.state, authenticatedSessionState(SESSION));
});

test("provider turns a terminal session failure into anonymous state", async () => {
  const unauthorized = new ApiError({
    code: "authentication_required",
    status: 401,
    cid: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    retryable: false,
  });
  services.meOrNull.mockRejectedValue(unauthorized);
  const { result } = renderHook(() => useSession(), { wrapper });

  await assert.rejects(() => result.current.revalidate(), ApiError);
  await waitFor(() => assert.deepEqual(result.current.state, anonymousSessionState()));
});

test("refresh and logout both converge through the authoritative session projection", async () => {
  services.refresh.mockResolvedValue(undefined);
  services.meOrNull.mockResolvedValueOnce(SESSION).mockResolvedValueOnce(null);
  services.logout.mockResolvedValue(undefined);
  const { result } = renderHook(() => useSession(), { wrapper });

  await act(async () => {
    assert.equal(await result.current.refresh(), SESSION);
  });
  assert.deepEqual(result.current.state, authenticatedSessionState(SESSION));

  await act(async () => {
    await result.current.logout();
  });
  assert.deepEqual(result.current.state, anonymousSessionState());
  assert.equal(services.logout.mock.calls.length, 1);
});

test("refresh preserves a retryable failure instead of silently making the user anonymous", async () => {
  const unavailable = new ApiError({
    code: "session_unavailable", status: 503,
    cid: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a", retryable: true,
  });
  services.refresh.mockRejectedValue(unavailable);
  const { result } = renderHook(() => useSession(), { wrapper });

  await assert.rejects(() => result.current.refresh(), ApiError);
  await waitFor(() => {
    assert.equal(result.current.state.status, "error");
  });
});

test("logout resolves an unknown outcome only when the canonical read is anonymous", async () => {
  services.logout.mockRejectedValue(new TypeError("connection dropped"));
  services.meOrNull.mockResolvedValue(null);
  const { result } = renderHook(() => useSession(), { wrapper });

  await act(async () => { await result.current.logout(); });
  assert.deepEqual(result.current.state, anonymousSessionState());
});

test("a principal update cannot overwrite another authenticated user", () => {
  const { result } = renderHook(() => useSession(), { wrapper });
  act(() => {
    result.current.updatePrincipal({
      id: entityIDSchema.parse("019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a"),
      username: "other",
    });
  });
  assert.equal(
    result.current.state.status === "authenticated" && result.current.state.session.user.username,
    "coder",
  );
});
