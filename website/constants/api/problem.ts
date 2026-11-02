import type { EntityID } from "@/types/api";

/** Problem endpoints (Problem module). */
export const PROBLEM_API = {
  FIND: "/problems/find",
  GET: (id: EntityID) => `/problems/${encodeURIComponent(id)}`,
  SAMPLES: (id: EntityID) => `/problems/${encodeURIComponent(id)}/samples`,
  ADMIN_FIND: "/admin/problems/find",
  ADMIN_GET: (id: EntityID) => `/admin/problems/${encodeURIComponent(id)}`,
  ADMIN_AUTHORING: (id: EntityID) =>
    `/admin/problems/${encodeURIComponent(id)}/authoring`,
  ADMIN_AUTHORING_CATALOG: "/admin/problems/authoring/catalog",
  CREATE: "/problems",
  DRAFT: (id: EntityID) => `/problems/${encodeURIComponent(id)}/drafts`,
  PUBLISH: (id: EntityID) => `/problems/${encodeURIComponent(id)}/publish`,
  ARCHIVE: (id: EntityID) => `/problems/${encodeURIComponent(id)}/archive`,
} as const;

/** Tag endpoints (Problem module). */
export const TAG_API = {
  FIND_ALL: "/tags",
  FIND: "/tags/find",
  GET: (id: EntityID) => `/tags/${encodeURIComponent(id)}`,
  CREATE: "/tags",
  UPDATE: (id: EntityID) => `/tags/${encodeURIComponent(id)}`,
  DELETE: (id: EntityID) => `/tags/${encodeURIComponent(id)}`,
} as const;

/** Difficulty endpoints (Problem module). */
export const DIFFICULTY_API = {
  FIND_ALL: "/difficulties",
  FIND: "/difficulties/find",
  GET: (id: EntityID) => `/difficulties/${encodeURIComponent(id)}`,
  CREATE: "/difficulties",
  UPDATE: (id: EntityID) => `/difficulties/${encodeURIComponent(id)}`,
  DELETE: (id: EntityID) => `/difficulties/${encodeURIComponent(id)}`,
} as const;
