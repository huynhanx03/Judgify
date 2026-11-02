import { RANK_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import {
  rankListSchema,
  rankPageSchema,
  rankSchema,
} from "@/lib/cultivation/catalog-schema";
import type { Paginated, QueryOptions } from "@/types/api";
import type { RankResponse } from "@/types/cultivation";

type CreateRankInput = {
  name: string;
  min_rating: number;
  description?: string;
};

type UpdateRankInput = Partial<CreateRankInput>;

export const rankService = {
  find(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<RankResponse>> {
    return api<Paginated<RankResponse>, QueryOptions>(RANK_API.FIND, {
      method: "POST",
      body: query,
      signal,
      schema: rankPageSchema,
    });
  },

  getAll(signal?: AbortSignal): Promise<RankResponse[]> {
    return api<RankResponse[], never>(RANK_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: rankListSchema,
    });
  },

  create(data: CreateRankInput): Promise<RankResponse> {
    return api<RankResponse, CreateRankInput>(RANK_API.CREATE, {
      method: "POST",
      body: data,
      schema: rankSchema,
    });
  },

  update(id: string, data: UpdateRankInput): Promise<RankResponse> {
    return api<RankResponse, UpdateRankInput>(
      RANK_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body: data,
        schema: rankSchema,
      },
    );
  },

  async delete(id: string): Promise<void> {
    await api<void, never>(
      RANK_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", schema: voidSchema },
    );
  },
};
