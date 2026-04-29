/** Contest endpoints (Contest module). */
export const CONTEST_API = {
  FIND: "/contests/find",
  GET: (id: number) => `/contests/${id}`,
  CREATE: "/contests",
  UPDATE: (id: number) => `/contests/${id}`,
  DELETE: (id: number) => `/contests/${id}`,
  REGISTER: (id: number) => `/contests/${id}/register`,
  UNREGISTER: (id: number) => `/contests/${id}/unregister`,
  STANDINGS: (id: number) => `/contests/${id}/standings`,
  RATING_CHANGES: (id: number) => `/contests/${id}/rating-changes`,
} as const;

/** Submission endpoints (Submission module). */
export const SUBMISSION_API = {
  CREATE: "/submissions",
  GET: (id: number) => `/submissions/${id}`,
  /** Get all submissions for a problem. */
  BY_PROBLEM: (problemId: number) => `/problems/${problemId}/submissions`,
  /** Get current user's submissions for a problem. */
  MY_BY_PROBLEM: (problemId: number) => `/problems/${problemId}/my-submissions`,
} as const;
