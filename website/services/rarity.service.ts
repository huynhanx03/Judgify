import { apiClient } from "@/lib/api-client";
import { RARITY_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { RarityResponse } from "@/types/cultivation";

export const rarityService = {
  async find(query: QueryOptions): Promise<Paginated<RarityResponse>> {
    return apiClient.post<Paginated<RarityResponse>>(RARITY_API.FIND, query);
  },
  async getAll(): Promise<RarityResponse[]> {
    return apiClient.get<RarityResponse[]>(RARITY_API.FIND_ALL);
  },
  async create(data: { name: string; code: string; weight: number; description?: string }): Promise<RarityResponse> {
    return apiClient.post<RarityResponse>(RARITY_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; code?: string; weight?: number; description?: string }): Promise<RarityResponse> {
    return apiClient.put<RarityResponse>(RARITY_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(RARITY_API.DELETE(id));
  },
};
