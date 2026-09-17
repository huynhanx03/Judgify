export const PUBLIC_PROFILE_API = {
  GET: (username: string) => `/profiles/${encodeURIComponent(username)}`,
  PRIVACY: "/users/profile/privacy",
} as const
