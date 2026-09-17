import type { EntityID } from "@/types/api";

export const NOTIFICATION_CAMPAIGN_API = {
  LIST: (query: string) =>
    query
      ? `/admin/notification-campaigns?${query}`
      : "/admin/notification-campaigns",
  CREATE: "/admin/notification-campaigns",
  GET: (id: EntityID) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}`,
  REVISE: (id: EntityID) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}`,
  DRY_RUN: (id: EntityID) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}/dry-run`,
  SCHEDULE: (id: EntityID) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}/schedule`,
  CANCEL: (id: EntityID) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}/cancel`,
  RECIPIENTS: (id: EntityID, query: string) =>
    `/admin/notification-campaigns/${encodeURIComponent(id)}/recipients${
      query ? `?${query}` : ""
    }`,
} as const;
