/** Problem endpoints (Problem module). */
export const PROBLEM_API = {
  FIND: "/problems/find",
  GET: (id: number) => `/problems/${id}`,
  CREATE: "/problems",
  UPDATE: (id: number) => `/problems/${id}`,
  DELETE: (id: number) => `/problems/${id}`,
  TEST_CASES: (id: number) => `/problems/${id}/test-cases`,
  TEST_CASE_UPDATE: (id: number) => `/test-cases/${id}`,
  TEST_CASE_DELETE: (id: number) => `/test-cases/${id}`,
} as const;

/** Tag endpoints (Problem module). */
export const TAG_API = {
  FIND_ALL: "/tags",
  FIND: "/tags/find",
  GET: (id: number) => `/tags/${id}`,
  CREATE: "/tags",
  UPDATE: (id: number) => `/tags/${id}`,
  DELETE: (id: number) => `/tags/${id}`,
} as const;

/** Difficulty endpoints (Problem module). */
export const DIFFICULTY_API = {
  FIND_ALL: "/difficulties",
  FIND: "/difficulties/find",
  GET: (id: number) => `/difficulties/${id}`,
  CREATE: "/difficulties",
  UPDATE: (id: number) => `/difficulties/${id}`,
  DELETE: (id: number) => `/difficulties/${id}`,
} as const;
