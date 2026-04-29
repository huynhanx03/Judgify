import { apiClient } from "@/lib/api-client";
import { TRAIT_API, USER_TRAIT_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  TraitResponse,
  GachaRollResponse,
  CreateUserTraitRequest,
  UserTraitResponse,
} from "@/types/cultivation";

export const cultivationService = {
  // Traits — Read (public, skipAuth)
  async getAllTraits(): Promise<TraitResponse[]> {
    return apiClient.get<TraitResponse[]>(TRAIT_API.FIND_ALL, { skipAuth: true });
  },
  async findTraits(query: QueryOptions): Promise<Paginated<TraitResponse>> {
    return apiClient.post<Paginated<TraitResponse>>(TRAIT_API.FIND, query, {
      skipAuth: true,
    });
  },

  // Traits — CRUD
  async createTrait(data: {
    type: string;
    name: string;
    rarity_id: number;
    description?: string;
    metadata?: Record<string, unknown>;
  }): Promise<TraitResponse> {
    return apiClient.post<TraitResponse>(TRAIT_API.CREATE, data);
  },
  async updateTrait(
    id: number,
    data: {
      type?: string;
      name?: string;
      rarity_id?: number;
      description?: string;
      metadata?: Record<string, unknown>;
    }
  ): Promise<TraitResponse> {
    return apiClient.put<TraitResponse>(TRAIT_API.UPDATE(id), data);
  },
  async deleteTrait(id: number): Promise<void> {
    await apiClient.delete(TRAIT_API.DELETE(id));
  },

  // Gacha & User Traits
  async gachaRoll(): Promise<GachaRollResponse> {
    return apiClient.get<GachaRollResponse>(TRAIT_API.ROLL, { skipAuth: true });
  },
  async createUserTrait(
    request: CreateUserTraitRequest
  ): Promise<UserTraitResponse> {
    return apiClient.post<UserTraitResponse>(USER_TRAIT_API.CREATE, request);
  },
};
