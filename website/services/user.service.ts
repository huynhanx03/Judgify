import { apiClient } from "@/lib/api-client";
import { USER_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { AdminUser } from "@/types/admin";
import type { UserProfile, UpdateProfileRequest } from "@/types/user";

export const userService = {
  // Profile (public)
  async getProfile(): Promise<UserProfile> {
    return apiClient.get<UserProfile>(USER_API.PROFILE);
  },

  async updateProfile(request: UpdateProfileRequest): Promise<UserProfile> {
    return apiClient.put<UserProfile>(USER_API.PROFILE, request);
  },

  // User management (admin)
  async find(query?: QueryOptions): Promise<Paginated<AdminUser>> {
    return apiClient.post<Paginated<AdminUser>>(
      USER_API.FIND,
      query ?? { pagination: { page: 1, page_size: 200 } }
    );
  },

  async create(data: {
    username: string;
    password: string;
    role_id: number;
    first_name: string;
    last_name: string;
    gender: number;
    birthday: string;
  }): Promise<void> {
    await apiClient.post(USER_API.CREATE, data);
  },

  async updateRole(id: number, data: { role_id: number }): Promise<AdminUser> {
    return apiClient.patch<AdminUser>(USER_API.UPDATE(id), data);
  },

  async delete(id: number): Promise<void> {
    await apiClient.delete(USER_API.DELETE(id));
  },
};
