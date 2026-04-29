import { apiClient } from "@/lib/api-client";
import { DIFFICULTY_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { DifficultyResponse } from "@/types/difficulty";

export const difficultyService = {
  async find(query?: QueryOptions): Promise<Paginated<DifficultyResponse>> {
    return apiClient.post<Paginated<DifficultyResponse>>(DIFFICULTY_API.FIND, query);
  },
  async getAll(): Promise<DifficultyResponse[]> {
    return apiClient.get<DifficultyResponse[]>(DIFFICULTY_API.FIND_ALL);
  },
  async create(data: {
    name: string;
    level: number;
    exp_reward?: number;
    description?: string;
  }): Promise<DifficultyResponse> {
    return apiClient.post<DifficultyResponse>(DIFFICULTY_API.CREATE, data);
  },
  async update(
    id: number,
    data: {
      name?: string;
      level?: number;
      exp_reward?: number;
      description?: string;
    }
  ): Promise<DifficultyResponse> {
    return apiClient.put<DifficultyResponse>(DIFFICULTY_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(DIFFICULTY_API.DELETE(id));
  },
};
