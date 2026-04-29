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
