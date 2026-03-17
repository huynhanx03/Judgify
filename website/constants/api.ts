/**
 * Centralized API configuration and endpoint paths.
 * All backend URLs are managed here so changes only require edits in one place.
 */

/** Base URL for the backend API. Reads from environment variable or falls back to localhost. */
export const BASE_API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/** Authentication endpoints (Identity module). */
export const AUTH_API = {
  LOGIN: "/auth/login",
  REGISTER: "/auth/register",
  REFRESH: "/auth/refresh",
  CHANGE_PASSWORD: "/auth/change-password",
  FORGOT_PASSWORD: "/auth/forgot-password",
  RESET_PASSWORD: "/auth/reset-password",
  OAUTH_CALLBACK: (provider: string) => `/auth/oauth/${provider}/callback`,
  OAUTH_REGISTER: (provider: string) => `/auth/oauth/${provider}/register`,
  OAUTH_LINK: (provider: string) => `/auth/oauth/${provider}/link`,
} as const;

/** User endpoints (Identity module). */
export const USER_API = {
  PROFILE: "/users/profile",
  DELETE: (id: number) => `/users/${id}`,
} as const;

/** Problem endpoints (Problem module). */
export const PROBLEM_API = {
  FIND: "/problems/find",
  GET: (id: number) => `/problems/${id}`,
  CREATE: "/problems",
  UPDATE: (id: number) => `/problems/${id}`,
  DELETE: (id: number) => `/problems/${id}`,
  TEST_CASES: (id: number) => `/problems/${id}/test-cases`,
} as const;

/** Tag endpoints (Problem module). */
export const TAG_API = {
  FIND: "/tags/find",
  GET: (id: number) => `/tags/${id}`,
  CREATE: "/tags",
  UPDATE: (id: number) => `/tags/${id}`,
  DELETE: (id: number) => `/tags/${id}`,
} as const;
