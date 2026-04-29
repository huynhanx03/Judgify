import { apiClient } from "@/lib/api-client";
import { ELEMENT_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { ElementResponse } from "@/types/cultivation";

export const elementService = {
  async find(query: QueryOptions): Promise<Paginated<ElementResponse>> {
    return apiClient.post<Paginated<ElementResponse>>(ELEMENT_API.FIND, query);
  },
  async getAll(): Promise<ElementResponse[]> {
    return apiClient.get<ElementResponse[]>(ELEMENT_API.FIND_ALL);
  },
  async create(data: { name: string; code: string; description?: string }): Promise<ElementResponse> {
    return apiClient.post<ElementResponse>(ELEMENT_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; code?: string; description?: string }): Promise<ElementResponse> {
    return apiClient.put<ElementResponse>(ELEMENT_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(ELEMENT_API.DELETE(id));
  },
};
