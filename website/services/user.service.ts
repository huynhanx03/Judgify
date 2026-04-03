/**
 * User service layer.
 * Calls backend identity module endpoints via apiClient.
 */

import { apiClient } from "@/lib/api-client";
import { USER_API } from "@/constants/api";
import type { UserProfile, UpdateProfileRequest } from "@/types/user";

/**
 * Fetches the current user's profile.
 * Backend endpoint: GET /users/profile
 */
export async function getProfile(): Promise<UserProfile> {
  return apiClient.get<UserProfile>(USER_API.PROFILE);
}

/**
 * Updates the current user's profile.
 * Backend endpoint: PUT /users/profile
 */
export async function updateProfile(
  request: UpdateProfileRequest
): Promise<UserProfile> {
  return apiClient.put<UserProfile>(USER_API.PROFILE, request);
}
