/**
 * Tag service layer — calls backend tag module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { TAG_API } from "@/constants/api";
import type { Tag } from "@/types/tag";
import type { Paginated, QueryOptions } from "@/types/api";

/** Fetches all tags (no pagination, for dropdowns/filters). */
export async function getAllTags(): Promise<Tag[]> {
  return apiClient.get<Tag[]>(TAG_API.FIND_ALL);
}

/** Fetches a paginated list of tags. */
export async function getTags(
  query?: QueryOptions
): Promise<Paginated<Tag>> {
  return apiClient.post<Paginated<Tag>>(TAG_API.FIND, query);
}
