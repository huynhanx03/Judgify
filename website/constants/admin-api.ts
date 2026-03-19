/**
 * Admin API endpoint constants.
 * Extends the base API endpoints with admin-specific routes.
 */

export const ROLE_API = {
  FIND: "/roles/find",
  GET: (id: number) => `/roles/${id}`,
  CREATE: "/roles",
  UPDATE: (id: number) => `/roles/${id}`,
  DELETE: (id: number) => `/roles/${id}`,
} as const;

export const PERMISSION_API = {
  FIND: "/permissions/find",
  GET: (id: number) => `/permissions/${id}`,
  CREATE: "/permissions",
  UPDATE: (id: number) => `/permissions/${id}`,
  DELETE: (id: number) => `/permissions/${id}`,
} as const;

export const RESOURCE_API = {
  FIND: "/resources/find",
  GET: (id: number) => `/resources/${id}`,
  CREATE: "/resources",
  UPDATE: (id: number) => `/resources/${id}`,
  DELETE: (id: number) => `/resources/${id}`,
} as const;

export const USER_ADMIN_API = {
  FIND: "/users/find",
  DELETE: (id: number) => `/users/${id}`,
} as const;
