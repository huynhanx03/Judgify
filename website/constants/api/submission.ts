import type { EntityID } from "@/types/api";

/** Submission endpoints (Submission module). */
export const SUBMISSION_API = {
  CREATE: "/submissions",
  GET: (id: EntityID) => `/submissions/${encodeURIComponent(id)}`,
  BY_PROBLEM: (problemId: EntityID) =>
    `/problems/${encodeURIComponent(problemId)}/submissions`,
  MY_BY_PROBLEM: (problemId: EntityID, query = "") =>
    `/problems/${encodeURIComponent(problemId)}/my-submissions${
      query ? `?${query}` : ""
    }`,
  ADMIN_FIND: "/admin/submissions/find",
  ADMIN_GET: (id: EntityID) =>
    `/admin/submissions/${encodeURIComponent(id)}`,
  ADMIN_REJUDGE: (id: EntityID) =>
    `/admin/submissions/${encodeURIComponent(id)}/rejudge`,
  ADMIN_CANCEL: (id: EntityID) =>
    `/admin/submissions/${encodeURIComponent(id)}/cancel`,
} as const;
