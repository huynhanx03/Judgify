import { apiClient } from "@/lib/api-client";
import { ROLE_API, PERMISSION_API, RESOURCE_API } from "@/constants/api";
import type { Role, Permission, Resource } from "@/types/admin";

export const roleService = {
  // Roles
  async getAll(): Promise<Role[]> {
    return apiClient.get<Role[]>(ROLE_API.FIND_ALL);
  },
  async create(data: { name: string; level: number; parent_id?: number }): Promise<Role> {
    return apiClient.post<Role>(ROLE_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; level?: number; parent_id?: number }): Promise<Role> {
    return apiClient.put<Role>(ROLE_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(ROLE_API.DELETE(id));
  },

  // Permissions
  async getAllPermissions(): Promise<Permission[]> {
    return apiClient.get<Permission[]>(PERMISSION_API.FIND_ALL);
  },
  async createPermission(data: { role_id: number; resource_id: number; scopes: number }): Promise<Permission> {
    return apiClient.post<Permission>(PERMISSION_API.CREATE, data);
  },
  async updatePermission(id: number, data: { scopes?: number }): Promise<Permission> {
    return apiClient.put<Permission>(PERMISSION_API.UPDATE(id), data);
  },
  async deletePermission(id: number): Promise<void> {
    await apiClient.delete(PERMISSION_API.DELETE(id));
  },

  // Resources
  async getAllResources(): Promise<Resource[]> {
    return apiClient.get<Resource[]>(RESOURCE_API.FIND_ALL);
  },
  async createResource(data: { key: string; description?: string }): Promise<Resource> {
    return apiClient.post<Resource>(RESOURCE_API.CREATE, data);
  },
  async updateResource(id: number, data: { key?: string; description?: string }): Promise<Resource> {
    return apiClient.put<Resource>(RESOURCE_API.UPDATE(id), data);
  },
  async deleteResource(id: number): Promise<void> {
    await apiClient.delete(RESOURCE_API.DELETE(id));
  },
};
