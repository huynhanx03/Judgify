import { apiClient, setTokens, clearTokens } from "@/lib/api-client";
import { AUTH_API } from "@/constants/api";
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
} from "@/types/auth";

export const authService = {
  async login(request: LoginRequest): Promise<LoginResponse> {
    const data = await apiClient.post<LoginResponse>(AUTH_API.LOGIN, request, {
      skipAuth: true,
    });
    setTokens(data.access_token, data.refresh_token);
    return data;
  },

  async register(request: RegisterRequest): Promise<RegisterResponse> {
    return apiClient.post<RegisterResponse>(AUTH_API.REGISTER, request, {
      skipAuth: true,
    });
  },

  async forgotPassword(username: string): Promise<{ message: string }> {
    return apiClient.post<{ message: string }>(
      AUTH_API.FORGOT_PASSWORD,
      { username },
      { skipAuth: true }
    );
  },

  async resetPassword(
    token: string,
    newPassword: string
  ): Promise<{ success: boolean }> {
    return apiClient.post<{ success: boolean }>(
      AUTH_API.RESET_PASSWORD,
      { token, new_password: newPassword },
      { skipAuth: true }
    );
  },

  logout(): void {
    clearTokens();
  },
};
