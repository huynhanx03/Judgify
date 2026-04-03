/**
 * Problem service layer — calls backend problem module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { PROBLEM_API } from "@/constants/api";
import type { Problem } from "@/types/problem";
import type { Paginated, QueryOptions } from "@/types/api";

/** Fetches a paginated list of problems. */
export async function getProblems(
  query?: QueryOptions
): Promise<Paginated<Problem>> {
  return apiClient.post<Paginated<Problem>>(PROBLEM_API.FIND, query);
}

/** Fetches a single problem by ID. */
export async function getProblemById(id: number): Promise<Problem> {
  return apiClient.get<Problem>(PROBLEM_API.GET(id));
}
