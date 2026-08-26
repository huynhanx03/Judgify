import { api } from "@/lib/api/client";
import { DIFFICULTY_API } from "@/constants/api/problem";
import type { Paginated, QueryOptions } from "@/types/api";
import type { DifficultyResponse } from "@/types/difficulty";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  difficultyListSchema,
  difficultyPageSchema,
  difficultySchema,
} from "@/lib/problems/public-problem-schema";
import { voidSchema } from "@/lib/api/schema";

export const difficultyService = {
  async find(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<DifficultyResponse>> {
    return api<Paginated<DifficultyResponse>, QueryOptions>(
      DIFFICULTY_API.FIND,
      {
        method: "POST",
        body: query ?? { pagination: { page: 1, page_size: 20 } },
        signal,
        schema: difficultyPageSchema,
      },
    );
  },
  async getAll(signal?: AbortSignal): Promise<DifficultyResponse[]> {
    return api<DifficultyResponse[], never>(DIFFICULTY_API.FIND_ALL, {
      method: "GET",
      auth: "none",
      signal,
      schema: difficultyListSchema,
    });
  },
  async create(data: {
    name: string;
    level: number;
    exp_reward?: number;
    description?: string;
  }): Promise<DifficultyResponse> {
    return api<DifficultyResponse, typeof data>(DIFFICULTY_API.CREATE, {
      method: "POST",
      body: data,
      schema: difficultySchema,
    });
  },
  async update(
    id: string,
    data: {
      name?: string;
      level?: number;
      exp_reward?: number;
      description?: string;
    }
  ): Promise<DifficultyResponse> {
    return api<DifficultyResponse, typeof data>(
      DIFFICULTY_API.UPDATE(entityIDSchema.parse(id)),
      { method: "PUT", body: data, schema: difficultySchema },
    );
  },
  async delete(id: string): Promise<void> {
    await api<void, never>(DIFFICULTY_API.DELETE(entityIDSchema.parse(id)), {
      method: "DELETE",
      schema: voidSchema,
    });
  },
};
