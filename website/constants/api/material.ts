/** Material category endpoints (Material module). */
export const MATERIAL_CATEGORY_API = {
  FIND_ALL: "/material-categories",
  FIND: "/material-categories/find",
  GET: (id: number) => `/material-categories/${id}`,
  CREATE: "/material-categories",
  UPDATE: (id: number) => `/material-categories/${id}`,
  DELETE: (id: number) => `/material-categories/${id}`,
} as const;

/** Material article endpoints (Material module). */
export const MATERIAL_API = {
  FIND: "/materials/find",
  GET: (id: number) => `/materials/${id}`,
  CREATE: "/materials",
  UPDATE: (id: number) => `/materials/${id}`,
  DELETE: (id: number) => `/materials/${id}`,
} as const;
