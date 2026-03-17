/**
 * Problem service layer.
 * Currently returns mock data. When backend is ready, uncomment apiClient calls.
 */

// import { apiClient } from "@/lib/api-client";
// import { PROBLEM_API } from "@/constants/api";
import type { Problem } from "@/types/problem";
import type { Paginated, QueryOptions } from "@/types/api";
import { MOCK_PROBLEMS } from "@/mock/problems";

/**
 * Fetches a paginated list of problems.
 *
 * Backend endpoint: POST /problems/find
 * Backend request body: QueryOptions { pagination, filters, sort }
 * Backend response format:
 * {
 *   code: 200001,
 *   message: "success",
 *   data: {
 *     records: [{ id, title, description, difficulty, time_limit_ms, memory_limit_kb, author_id, is_published, tags, created_at, updated_at }],
 *     pagination: { current_page, page_size, total_pages, total_items, has_next, has_prev }
 *   }
 * }
 */
export async function getProblems(
  query?: QueryOptions
): Promise<Paginated<Problem>> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.post<Paginated<Problem>>(PROBLEM_API.FIND, query);

  await new Promise((resolve) => setTimeout(resolve, 300));

  const page = query?.pagination?.page ?? 1;
  const pageSize = query?.pagination?.page_size ?? 10;

  // Apply difficulty filter if present
  let filtered = MOCK_PROBLEMS;
  const difficultyFilter = query?.filters?.find((f) => f.key === "difficulty");
  if (difficultyFilter && difficultyFilter.value) {
    filtered = filtered.filter(
      (p) => p.difficulty === difficultyFilter.value
    );
  }

  const totalItems = filtered.length;
  const totalPages = Math.ceil(totalItems / pageSize);
  const start = (page - 1) * pageSize;
  const records = filtered.slice(start, start + pageSize);

  return {
    records,
    pagination: {
      current_page: page,
      page_size: pageSize,
      total_pages: totalPages,
      total_items: totalItems,
      has_next: page < totalPages,
      has_prev: page > 1,
    },
  };
}

/**
 * Fetches a single problem by ID.
 *
 * Backend endpoint: GET /problems/:id
 * Backend response format:
 * { code: 200001, message: "success", data: { id, title, description, ... } }
 */
export async function getProblemById(id: number): Promise<Problem | null> {
  // TODO: Replace with real API call when backend is ready
  // return apiClient.get<Problem>(PROBLEM_API.GET(id));

  await new Promise((resolve) => setTimeout(resolve, 200));
  return MOCK_PROBLEMS.find((p) => p.id === id) ?? null;
}
