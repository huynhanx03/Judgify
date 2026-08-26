/** Append-only audit read endpoints. */
export const AUDIT_API = {
  LIST: (query: string) => `/admin/audit?${query}`,
} as const;
