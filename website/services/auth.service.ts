/**
 * Authentication service layer.
 * Calls backend identity module endpoints via apiClient.
 */

import { apiClient, setTokens, clearTokens } from "@/lib/api-client";
import { AUTH_API } from "@/constants/api";
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "@/types/auth";

/**
 * Authenticates user with username and password.
 * Stores tokens on success.
 */
export async function login(request: LoginRequest): Promise<LoginResponse> {
  const data = await apiClient.post<LoginResponse>(AUTH_API.LOGIN, request, {
    skipAuth: true,
  });
  setTokens(data.access_token, data.refresh_token);
  return data;
}

/**
 * Registers a new user account.
 */
export async function register(
  request: RegisterRequest
): Promise<RegisterResponse> {
  return apiClient.post<RegisterResponse>(AUTH_API.REGISTER, request, {
    skipAuth: true,
  });
}

/**
 * Sends a password reset email to the user.
 */
export async function forgotPassword(
  username: string
): Promise<{ message: string }> {
  return apiClient.post<{ message: string }>(
    AUTH_API.FORGOT_PASSWORD,
    { username },
    { skipAuth: true }
  );
}

/**
 * Resets user password using a token.
 */
export async function resetPassword(
  token: string,
  newPassword: string
): Promise<{ success: boolean }> {
  return apiClient.post<{ success: boolean }>(
    AUTH_API.RESET_PASSWORD,
    { token, new_password: newPassword },
    { skipAuth: true }
  );
}

/** Clears stored tokens and ends the user session. */
export function logout(): void {
  clearTokens();
}
