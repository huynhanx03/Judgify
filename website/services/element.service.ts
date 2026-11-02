import { ELEMENT_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import {
  elementListSchema,
  elementPageSchema,
  elementSchema,
} from "@/lib/cultivation/catalog-schema";
import type { Paginated, QueryOptions } from "@/types/api";
import type { ElementResponse } from "@/types/cultivation";

type CreateElementInput = {
  name: string;
  code: string;
  description?: string;
};

type UpdateElementInput = Partial<CreateElementInput>;

export const elementService = {
  find(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<ElementResponse>> {
    return api<Paginated<ElementResponse>, QueryOptions>(ELEMENT_API.FIND, {
      method: "POST",
      body: query,
      signal,
      schema: elementPageSchema,
    });
  },

  getAll(signal?: AbortSignal): Promise<ElementResponse[]> {
    return api<ElementResponse[], never>(ELEMENT_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: elementListSchema,
    });
  },

  create(data: CreateElementInput): Promise<ElementResponse> {
    return api<ElementResponse, CreateElementInput>(ELEMENT_API.CREATE, {
      method: "POST",
      body: data,
      schema: elementSchema,
    });
  },

  update(id: string, data: UpdateElementInput): Promise<ElementResponse> {
    return api<ElementResponse, UpdateElementInput>(
      ELEMENT_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body: data,
        schema: elementSchema,
      },
    );
  },

  async delete(id: string): Promise<void> {
    await api<void, never>(
      ELEMENT_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", schema: voidSchema },
    );
  },
};
