import type { EntityID } from "@/types/api";

/** Persisted, self-owned notification inbox endpoints. */
export const NOTIFICATION_API = {
  LIST: "/notifications",
  UNREAD: "/notifications/unread",
  MARK_ALL_READ: "/notifications/read-all",
  MARK_READ: (id: EntityID) =>
    `/notifications/${encodeURIComponent(id)}/read`,
  CAMPAIGN_CONTENT: (id: EntityID) =>
    `/notifications/${encodeURIComponent(id)}/content`,
  ARCHIVE: (id: EntityID) => `/notifications/${encodeURIComponent(id)}`,
  PREFERENCES: "/notification-preferences",
  PREFERENCE: (category: string) =>
    `/notification-preferences/${encodeURIComponent(category)}`,
} as const;
