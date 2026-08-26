import { RARITY_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import {
  rarityListSchema,
  rarityPageSchema,
  raritySchema,
} from "@/lib/cultivation/catalog-schema";
import type { Paginated, QueryOptions } from "@/types/api";
import type { RarityResponse } from "@/types/cultivation";

type CreateRarityInput = {
  name: string;
  code: string;
  weight: number;
  description?: string;
};

type UpdateRarityInput = Partial<CreateRarityInput>;

export const rarityService = {
  find(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<RarityResponse>> {
    return api<Paginated<RarityResponse>, QueryOptions>(RARITY_API.FIND, {
      method: "POST",
      body: query,
      signal,
      schema: rarityPageSchema,
    });
  },

  getAll(signal?: AbortSignal): Promise<RarityResponse[]> {
    return api<RarityResponse[], never>(RARITY_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: rarityListSchema,
    });
  },

  create(data: CreateRarityInput): Promise<RarityResponse> {
    return api<RarityResponse, CreateRarityInput>(RARITY_API.CREATE, {
      method: "POST",
      body: data,
      schema: raritySchema,
    });
  },

  update(id: string, data: UpdateRarityInput): Promise<RarityResponse> {
    return api<RarityResponse, UpdateRarityInput>(
      RARITY_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body: data,
        schema: raritySchema,
      },
    );
  },

  async delete(id: string): Promise<void> {
    await api<void, never>(
      RARITY_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", schema: voidSchema },
    );
  },
};
