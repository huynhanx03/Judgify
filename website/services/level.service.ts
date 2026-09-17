import { LEVEL_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { voidSchema } from "@/lib/api/schema";
import {
  levelListSchema,
  levelPageSchema,
  levelSchema,
} from "@/lib/cultivation/catalog-schema";
import type { Paginated, QueryOptions } from "@/types/api";
import type { LevelResponse } from "@/types/cultivation";

type CreateLevelInput = {
  name: string;
  min_exp: number;
  description?: string;
};

type UpdateLevelInput = Partial<CreateLevelInput>;

export const levelService = {
  find(
    query: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<LevelResponse>> {
    return api<Paginated<LevelResponse>, QueryOptions>(LEVEL_API.FIND, {
      method: "POST",
      body: query,
      signal,
      schema: levelPageSchema,
    });
  },

  getAll(signal?: AbortSignal): Promise<LevelResponse[]> {
    return api<LevelResponse[], never>(LEVEL_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: levelListSchema,
    });
  },

  create(data: CreateLevelInput): Promise<LevelResponse> {
    return api<LevelResponse, CreateLevelInput>(LEVEL_API.CREATE, {
      method: "POST",
      body: data,
      schema: levelSchema,
    });
  },

  update(id: string, data: UpdateLevelInput): Promise<LevelResponse> {
    return api<LevelResponse, UpdateLevelInput>(
      LEVEL_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body: data,
        schema: levelSchema,
      },
    );
  },

  async delete(id: string): Promise<void> {
    await api<void, never>(
      LEVEL_API.DELETE(entityIDSchema.parse(id)),
      { method: "DELETE", schema: voidSchema },
    );
  },
};
