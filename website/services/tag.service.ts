/**
 * Tag service layer.
 * Currently returns mock data. When backend is ready, uncomment apiClient calls.
 */

// import { apiClient } from "@/lib/api-client";
// import { TAG_API } from "@/constants/api";
import type { Tag } from "@/types/tag";
import { MOCK_TAGS } from "@/mock/tags";

/**
 * Fetches all available tags.
 *
 * Backend endpoint: POST /tags/find
 * Backend response format:
 * { code: 200001, message: "success", data: { records: [{ id, name }], pagination: {...} } }
 */
export async function getTags(): Promise<Tag[]> {
  // TODO: Replace with real API call when backend is ready
  // const result = await apiClient.post<Paginated<Tag>>(TAG_API.FIND, {});
  // return result.records;

  await new Promise((resolve) => setTimeout(resolve, 100));
  return MOCK_TAGS;
}
