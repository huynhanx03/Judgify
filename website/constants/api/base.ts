/** Base URL for the backend API. Reads from environment variable or falls back to localhost. */
export const BASE_API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
