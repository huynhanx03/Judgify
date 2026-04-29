import { apiClient } from "@/lib/api-client";
import { TAG_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { Tag } from "@/types/tag";

export const tagService = {
  async find(query?: QueryOptions): Promise<Paginated<Tag>> {
    return apiClient.post<Paginated<Tag>>(TAG_API.FIND, query);
  },
  async getAll(): Promise<Tag[]> {
    return apiClient.get<Tag[]>(TAG_API.FIND_ALL);
  },
  async create(data: { name: string; element_ids?: number[] }): Promise<Tag> {
    return apiClient.post<Tag>(TAG_API.CREATE, data);
  },
  async update(id: number, data: { name?: string; element_ids?: number[] }): Promise<Tag> {
    return apiClient.put<Tag>(TAG_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(TAG_API.DELETE(id));
  },
};
