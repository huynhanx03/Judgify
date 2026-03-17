/**
 * User service layer.
 * Currently returns mock data. When backend is ready, uncomment apiClient calls.
 */

// import { apiClient } from "@/lib/api-client";
// import { USER_API } from "@/constants/api";
import type { UserProfile, UpdateProfileRequest } from "@/types/user";
import { MOCK_USER_PROFILE } from "@/mock/users";

/**
 * Fetches the current user's profile.
 *
 * Backend endpoint: GET /users/profile
 * Backend response format:
 * { code: 200001, message: "success", data: { username, first_name, last_name, gender, birthday } }
 */
export async function getProfile(): Promise<UserProfile> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.get<UserProfile>(USER_API.PROFILE);

  await new Promise((resolve) => setTimeout(resolve, 200));
  return MOCK_USER_PROFILE;
}

/**
 * Updates the current user's profile.
 *
 * Backend endpoint: PUT /users/profile
 * Backend request body: { first_name, last_name, gender, birthday }
 */
export async function updateProfile(
  request: UpdateProfileRequest
): Promise<UserProfile> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.put<UserProfile>(USER_API.PROFILE, request);

  await new Promise((resolve) => setTimeout(resolve, 300));
  return { ...MOCK_USER_PROFILE, ...request };
}
