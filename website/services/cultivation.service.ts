/**
 * Cultivation service layer.
 * Calls backend cultivation module endpoints via apiClient.
 */

import { apiClient } from "@/lib/api-client";
import { TRAIT_API, USER_TRAIT_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  TraitResponse,
  GachaRollResponse,
  CreateUserTraitRequest,
  UserTraitResponse,
} from "@/types/cultivation";

/**
 * Fetches all traits without pagination.
 * Public endpoint - no auth required (used in codex modal).
 * Backend endpoint: GET /traits
 */
export async function findAllTraits(): Promise<TraitResponse[]> {
  return apiClient.get<TraitResponse[]>(TRAIT_API.FIND_ALL, { skipAuth: true });
}

/**
 * Fetches traits with optional filtering (e.g., by type: root_bone | talent).
 * Public endpoint - no auth required (used during registration).
 * Backend endpoint: POST /traits/find
 */
export async function findTraits(
  query: QueryOptions
): Promise<Paginated<TraitResponse>> {
  return apiClient.post<Paginated<TraitResponse>>(TRAIT_API.FIND, query, {
    skipAuth: true,
  });
}

/**
 * Performs a weighted gacha roll on the server.
 * Public endpoint - no auth required (used during registration).
 * Backend endpoint: GET /traits/roll
 */
export async function gachaRoll(): Promise<GachaRollResponse> {
  return apiClient.get<GachaRollResponse>(TRAIT_API.ROLL, {
    skipAuth: true,
  });
}

/**
 * Assigns a trait to the current user.
 * Backend endpoint: POST /user-traits
 */
export async function createUserTrait(
  request: CreateUserTraitRequest
): Promise<UserTraitResponse> {
  return apiClient.post<UserTraitResponse>(USER_TRAIT_API.CREATE, request);
}
