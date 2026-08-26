import type { EntityID } from "@/types/api";

/** Audited, time-bounded process observability controls. */
export const OBSERVABILITY_API = {
  DEBUG_WINDOW: "/admin/observability/debug-window",
  DEBUG_WINDOW_LEASE: (key: EntityID) =>
    `/admin/observability/debug-window/${encodeURIComponent(key)}`,
} as const;
