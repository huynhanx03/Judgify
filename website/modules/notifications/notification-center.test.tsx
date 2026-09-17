import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, test, vi } from "vitest";

const dependencies = vi.hoisted(() => ({
  replace: vi.fn(),
  refresh: vi.fn(),
  loadMore: vi.fn(),
  markRead: vi.fn(),
  markAllRead: vi.fn(),
  archive: vi.fn(),
  loadPreferences: vi.fn(),
  updatePreference: vi.fn(),
  activeTab: "",
  notifications: {
    items: [],
    unreadCount: 0,
    nextCursor: null,
    status: "ready",
    isLoadingMore: false,
    isMarkingAllRead: false,
    pendingNotificationIDs: new Set<string>(),
    preferences: [],
    preferenceStatus: "ready",
    pendingPreferenceCategories: new Set<string>(),
  },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: dependencies.replace }),
  useSearchParams: () => new URLSearchParams(dependencies.activeTab),
}));
vi.mock("@/contexts/auth-context", () => ({
  useAuth: () => ({ isAuthenticated: true, isLoading: false }),
}));
vi.mock("@/contexts/realtime-context", () => ({
  useRealtime: () => ({ state: "open" }),
}));
vi.mock("@/contexts/notification-context", () => ({
  useNotifications: () => ({
    ...dependencies.notifications,
    refresh: dependencies.refresh,
    loadMore: dependencies.loadMore,
    markRead: dependencies.markRead,
    markAllRead: dependencies.markAllRead,
    archive: dependencies.archive,
    loadPreferences: dependencies.loadPreferences,
    updatePreference: dependencies.updatePreference,
  }),
}));

import { TEXT } from "@/constants/text";
import { NotificationCenter } from "@/modules/notifications/notification-center";

beforeEach(() => {
  Object.assign(dependencies.notifications, {
    items: [],
    unreadCount: 0,
    nextCursor: null,
    status: "ready",
    preferences: [],
    preferenceStatus: "ready",
  });
  dependencies.activeTab = "";
  for (const callback of [
    dependencies.replace,
    dependencies.refresh,
    dependencies.loadMore,
    dependencies.markRead,
    dependencies.markAllRead,
    dependencies.archive,
    dependencies.loadPreferences,
    dependencies.updatePreference,
  ]) callback.mockReset();
});

test("uses the shared unframed page header and empty collection treatment", () => {
  render(<NotificationCenter />);

  const heading = screen.getByRole("heading", { level: 1, name: TEXT.NOTIFICATION.TITLE });
  assert.ok(heading.className.includes("text-balance"));
  assert.equal(document.querySelectorAll("[class*='ring-8']").length, 0);
  assert.ok(screen.getByRole("heading", { level: 2, name: TEXT.NOTIFICATION.EMPTY_TITLE }));
});

test("uses the shared unframed collection recovery state for inbox and preferences", () => {
  Object.assign(dependencies.notifications, {
    status: "error",
    preferenceStatus: "error",
  });
  dependencies.refresh.mockResolvedValue(undefined);
  dependencies.loadPreferences.mockResolvedValue(undefined);

  const view = render(<NotificationCenter />);

  const inboxError = screen.getByRole("alert", { name: "" });
  assert.ok(screen.getByText(TEXT.NOTIFICATION.ERROR_TITLE));
  assert.ok(!inboxError.className.includes("border"));
  fireEvent.click(screen.getByRole("button", { name: TEXT.NOTIFICATION.RETRY }));
  assert.equal(dependencies.refresh.mock.calls.length, 1);

  dependencies.activeTab = "tab=preferences";
  view.rerender(<NotificationCenter />);
  assert.ok(screen.getByText(TEXT.NOTIFICATION.PREFERENCES.ERROR_TITLE));
  fireEvent.click(
    screen.getByRole("button", { name: TEXT.NOTIFICATION.PREFERENCES.RETRY }),
  );
  assert.deepEqual(dependencies.loadPreferences.mock.calls.at(-1), [{ force: true }]);
});
