/**
 * Difficulty service layer — calls backend difficulty module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { DIFFICULTY_API } from "@/constants/api";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Paginated, QueryOptions } from "@/types/api";

/** Fetches all difficulties (no pagination, for dropdowns/filters). */
export async function getAllDifficulties(): Promise<DifficultyResponse[]> {
  return apiClient.get<DifficultyResponse[]>(DIFFICULTY_API.FIND_ALL);
}

/** Fetches a paginated list of difficulties. */
export async function getDifficulties(
  query?: QueryOptions
): Promise<Paginated<DifficultyResponse>> {
  return apiClient.post<Paginated<DifficultyResponse>>(DIFFICULTY_API.FIND, query);
}
