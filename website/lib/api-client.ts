/**
 * Centralized API client for all backend communication.
 * Handles request/response parsing, Authorization headers, and automatic token refresh.
 *
 * All components and services MUST use this client instead of calling fetch directly.
 */

import { BASE_API_URL, AUTH_API } from "@/constants/api";
import type { ApiResponse } from "@/types/api";

/** Token storage keys used in localStorage. */
const TOKEN_KEYS = {
  ACCESS: "judgify_access_token",
  REFRESH: "judgify_refresh_token",
} as const;

/** Retrieves the stored access token. */
function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEYS.ACCESS);
}

/** Retrieves the stored refresh token. */
function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEYS.REFRESH);
}

/** Stores both tokens after login or refresh. */
export function setTokens(accessToken: string, refreshToken?: string): void {
  localStorage.setItem(TOKEN_KEYS.ACCESS, accessToken);
  if (refreshToken) {
    localStorage.setItem(TOKEN_KEYS.REFRESH, refreshToken);
  }
}

/** Clears all stored tokens (logout). */
export function clearTokens(): void {
  localStorage.removeItem(TOKEN_KEYS.ACCESS);
  localStorage.removeItem(TOKEN_KEYS.REFRESH);
}

/** Flag to prevent multiple simultaneous refresh attempts. */
let isRefreshing = false;
let refreshPromise: Promise<string | null> | null = null;

/**
 * Attempts to refresh the access token using the stored refresh token.
 * If a refresh is already in progress, returns the existing promise to avoid duplicate calls.
 */
async function refreshAccessToken(): Promise<string | null> {
  if (isRefreshing && refreshPromise) {
    return refreshPromise;
  }

  isRefreshing = true;
  refreshPromise = (async () => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) {
      clearTokens();
      return null;
    }

    try {
      const response = await fetch(`${BASE_API_URL}${AUTH_API.REFRESH}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${refreshToken}`,
        },
      });

      if (!response.ok) {
        clearTokens();
        return null;
      }

      /**
       * Backend response format:
       * { code: number, message: string, data: { access_token: string } }
       */
      const result: ApiResponse<{ access_token: string }> =
        await response.json();
      const newAccessToken = result.data.access_token;
      setTokens(newAccessToken);
      return newAccessToken;
    } catch {
      clearTokens();
      return null;
    } finally {
      isRefreshing = false;
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

/** Request configuration options for the API client. */
interface RequestConfig {
  /** Skip adding Authorization header (e.g., for login/register). */
  skipAuth?: boolean;
  /** Custom headers to merge with defaults. */
  headers?: Record<string, string>;
}

/**
 * Core fetch wrapper that handles:
 * - Prepending BASE_API_URL to the endpoint
 * - Adding Authorization header with access token
 * - Parsing JSON response and extracting data from ApiResponse wrapper
 * - Automatic token refresh on 401 responses
 */
async function request<T>(
  endpoint: string,
  options: RequestInit = {},
  config: RequestConfig = {}
): Promise<T> {
  const url = `${BASE_API_URL}${endpoint}`;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...config.headers,
  };

  // Attach access token unless explicitly skipped
  if (!config.skipAuth) {
    const token = getAccessToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
  }

  let response = await fetch(url, {
    ...options,
    headers,
  });

  // If unauthorized (401), attempt token refresh and retry the request once
  if (response.status === 401 && !config.skipAuth) {
    const newToken = await refreshAccessToken();
    if (newToken) {
      headers["Authorization"] = `Bearer ${newToken}`;
      response = await fetch(url, {
        ...options,
        headers,
      });
    } else {
      // Refresh failed — redirect to login or throw
      throw new ApiError(401, "Phiên đăng nhập hết hạn");
    }
  }

  /**
   * Backend always returns: { code: number, message: string, data: T }
   * We parse the wrapper and return data directly.
   */
  const result: ApiResponse<T> = await response.json();

  if (!response.ok) {
    throw new ApiError(result.code, result.message, result.data);
  }

  return result.data;
}

/** Custom error class for API errors with code and structured data. */
export class ApiError extends Error {
  constructor(
    public code: number,
    message: string,
    public data?: unknown
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * API client with typed HTTP methods.
 * Usage: apiClient.get<Problem>('/problems/1')
 */
export const apiClient = {
  /** Send a GET request and parse response data. */
  get<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return request<T>(endpoint, { method: "GET" }, config);
  },

  /** Send a POST request with a JSON body and parse response data. */
  post<T>(
    endpoint: string,
    body?: unknown,
    config?: RequestConfig
  ): Promise<T> {
    return request<T>(
      endpoint,
      {
        method: "POST",
        body: body ? JSON.stringify(body) : undefined,
      },
      config
    );
  },

  /** Send a PUT request with a JSON body and parse response data. */
  put<T>(
    endpoint: string,
    body?: unknown,
    config?: RequestConfig
  ): Promise<T> {
    return request<T>(
      endpoint,
      {
        method: "PUT",
        body: body ? JSON.stringify(body) : undefined,
      },
      config
    );
  },

  /** Send a DELETE request and parse response data. */
  delete<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return request<T>(endpoint, { method: "DELETE" }, config);
  },
};
