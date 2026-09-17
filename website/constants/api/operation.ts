import type { EntityID } from "@/types/api";

/** Domain-neutral durable background-operation control plane. */
export const OPERATION_API = {
  LIST: (query: string) =>
    query ? `/admin/operations?${query}` : "/admin/operations",
  GET: (id: EntityID) => `/admin/operations/${encodeURIComponent(id)}`,
  BATCHES: (id: EntityID, query: string) => {
    const endpoint = `/admin/operations/${encodeURIComponent(id)}/batches`;
    return query ? `${endpoint}?${query}` : endpoint;
  },
  CONTROL: (id: EntityID) =>
    `/admin/operations/${encodeURIComponent(id)}/control`,
} as const;
