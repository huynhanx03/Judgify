/**
 * Authentication-related types mirroring backend identity DTOs.
 */

/** POST /auth/login request body. */
export interface LoginRequest {
  username: string;
  password: string;
}

/** POST /auth/login response data. */
export interface LoginResponse {
  access_token: string;
  refresh_token: string;
}

/** POST /auth/register request body. */
export interface RegisterRequest {
  username: string;
  password: string;
  first_name: string;
  last_name: string;
  /** 0 = male, 1 = female, 2 = other */
  gender: number;
  /** Format: "2006-01-02" */
  birthday: string;
  /** Selected root bone ID (1 required) */
  root_bone_id: number;
  /** Selected talent IDs (exactly 3 required) */
  talent_ids: number[];
}

/** POST /auth/register response data. */
export interface RegisterResponse {
  success: boolean;
}

/** POST /auth/refresh response data. */
export interface RefreshTokenResponse {
  access_token: string;
}

/** POST /auth/change-password request body. */
export interface ChangePasswordRequest {
  current_password: string;
  new_password: string;
}

/** POST /auth/change-password response data. */
export interface ChangePasswordResponse {
  success: boolean;
}

/** POST /auth/forgot-password request body. */
export interface ForgotPasswordRequest {
  username: string;
}

/** POST /auth/reset-password request body. */
export interface ResetPasswordRequest {
  token: string;
  new_password: string;
}
