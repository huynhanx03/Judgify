/**
 * Authentication service layer.
 * Currently returns mock data. When backend is ready, uncomment apiClient calls.
 */

// import { apiClient, setTokens, clearTokens } from "@/lib/api-client";
// import { AUTH_API } from "@/constants/api";
import { setTokens, clearTokens } from "@/lib/api-client";
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "@/types/auth";

/**
 * Authenticates user with username and password.
 *
 * Backend response format:
 * { code: 200001, message: "success", data: { access_token: "...", refresh_token: "..." } }
 */
export async function login(request: LoginRequest): Promise<LoginResponse> {
  // TODO: Replace with real API call when backend is ready
  // const data = await apiClient.post<LoginResponse>(AUTH_API.LOGIN, request, { skipAuth: true });
  // setTokens(data.access_token, data.refresh_token);
  // return data;

  // Mock: simulate successful login
  await new Promise((resolve) => setTimeout(resolve, 500));
  const mockResponse: LoginResponse = {
    access_token: "mock_access_token_" + Date.now(),
    refresh_token: "mock_refresh_token_" + Date.now(),
  };
  setTokens(mockResponse.access_token, mockResponse.refresh_token);
  return mockResponse;
}

/**
 * Registers a new user account.
 *
 * Backend response format:
 * { code: 200001, message: "success", data: { success: true } }
 */
export async function register(
  request: RegisterRequest
): Promise<RegisterResponse> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.post<RegisterResponse>(AUTH_API.REGISTER, request, { skipAuth: true });

  void request;
  await new Promise((resolve) => setTimeout(resolve, 500));
  return { success: true };
}

/**
 * Sends a password reset email to the user.
 *
 * Backend endpoint: POST /auth/forgot-password
 */
export async function forgotPassword(username: string): Promise<{ success: boolean }> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.post(AUTH_API.FORGOT_PASSWORD, { username }, { skipAuth: true });

  void username;
  await new Promise((resolve) => setTimeout(resolve, 800));
  return { success: true };
}

/**
 * Resets user password using a token.
 *
 * Backend endpoint: POST /auth/reset-password
 */
export async function resetPassword(token: string, newPassword: string): Promise<{ success: boolean }> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.post(AUTH_API.RESET_PASSWORD, { token, new_password: newPassword }, { skipAuth: true });

  void token;
  void newPassword;
  await new Promise((resolve) => setTimeout(resolve, 500));
  return { success: true };
}

/** Clears stored tokens and ends the user session. */
export function logout(): void {
  clearTokens();
}
