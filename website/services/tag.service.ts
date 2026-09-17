import { api } from "@/lib/api/client";
import { TAG_API } from "@/constants/api/problem";
import type { Paginated, QueryOptions } from "@/types/api";
import type { Tag } from "@/types/tag";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import {
  tagListSchema,
  tagPageSchema,
  tagSchema,
} from "@/lib/problems/public-problem-schema";

export const tagService = {
  async find(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<Tag>> {
    return api<Paginated<Tag>, QueryOptions>(TAG_API.FIND, {
      method: "POST",
      body: query ?? { pagination: { page: 1, page_size: 20 } },
      signal,
      schema: tagPageSchema,
    });
  },
  async getAll(signal?: AbortSignal): Promise<Tag[]> {
    return api<Tag[], never>(TAG_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: tagListSchema,
    });
  },
  async create(data: { name: string; element_ids?: string[] }): Promise<Tag> {
    return api<Tag, { name: string; element_ids?: string[] }>(TAG_API.CREATE, {
      method: "POST",
      body: {
        ...data,
        element_ids: data.element_ids?.map((id) => entityIDSchema.parse(id)),
      },
      schema: tagSchema,
    });
  },
  async update(id: string, data: { name?: string; element_ids?: string[] }): Promise<Tag> {
    return api<Tag, { name?: string; element_ids?: string[] }>(
      TAG_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body: {
          ...data,
          element_ids: data.element_ids?.map((elementID) =>
            entityIDSchema.parse(elementID),
          ),
        },
        schema: tagSchema,
      },
    );
  },
  async delete(id: string): Promise<void> {
    await api<void, never>(TAG_API.DELETE(entityIDSchema.parse(id)), {
      method: "DELETE",
      schema: voidSchema,
    });
  },
};
