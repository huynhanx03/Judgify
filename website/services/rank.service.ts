import { apiClient } from "@/lib/api-client";
import { RANK_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { RankResponse } from "@/types/cultivation";

export const rankService = {
  async find(query: QueryOptions): Promise<Paginated<RankResponse>> {
    return apiClient.post<Paginated<RankResponse>>(RANK_API.FIND, query);
  },
  async getAll(): Promise<RankResponse[]> {
    return apiClient.get<RankResponse[]>(RANK_API.FIND_ALL);
  },
  async create(data: { name: string; min_rating: number; description?: string }): Promise<RankResponse> {
    return apiClient.post<RankResponse>(RANK_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; min_rating?: number; description?: string }): Promise<RankResponse> {
    return apiClient.put<RankResponse>(RANK_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(RANK_API.DELETE(id));
  },
};
