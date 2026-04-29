/** User endpoints (Identity module). */
export const USER_API = {
  FIND: "/users/find",
  CREATE: "/users",
  UPDATE: (id: number) => `/users/${id}`,
  DELETE: (id: number) => `/users/${id}`,
  PROFILE: "/users/profile",
} as const;

/** Role endpoints (Identity module). */
export const ROLE_API = {
  FIND_ALL: "/roles",
  FIND: "/roles/find",
  GET: (id: number) => `/roles/${id}`,
  CREATE: "/roles",
  UPDATE: (id: number) => `/roles/${id}`,
  DELETE: (id: number) => `/roles/${id}`,
} as const;

/** Permission endpoints (Identity module). */
export const PERMISSION_API = {
  FIND_ALL: "/permissions",
  FIND: "/permissions/find",
  GET: (id: number) => `/permissions/${id}`,
  CREATE: "/permissions",
  UPDATE: (id: number) => `/permissions/${id}`,
  DELETE: (id: number) => `/permissions/${id}`,
} as const;

/** Resource endpoints (Identity module). */
export const RESOURCE_API = {
  FIND_ALL: "/resources",
  FIND: "/resources/find",
  GET: (id: number) => `/resources/${id}`,
  CREATE: "/resources",
  UPDATE: (id: number) => `/resources/${id}`,
  DELETE: (id: number) => `/resources/${id}`,
} as const;
