import { apiClient } from "@/lib/api-client";
import { LEVEL_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { LevelResponse } from "@/types/cultivation";

export const levelService = {
  async find(query: QueryOptions): Promise<Paginated<LevelResponse>> {
    return apiClient.post<Paginated<LevelResponse>>(LEVEL_API.FIND, query);
  },
  async getAll(): Promise<LevelResponse[]> {
    return apiClient.get<LevelResponse[]>(LEVEL_API.FIND_ALL);
  },
  async create(data: { name: string; min_exp: number; description?: string }): Promise<LevelResponse> {
    return apiClient.post<LevelResponse>(LEVEL_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; min_exp?: number; description?: string }): Promise<LevelResponse> {
    return apiClient.put<LevelResponse>(LEVEL_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(LEVEL_API.DELETE(id));
  },
};
