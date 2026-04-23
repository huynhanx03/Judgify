/**
 * Contest service layer — calls backend contest module endpoints.
 */

import { apiClient } from "@/lib/api-client";
import { CONTEST_API } from "@/constants/api";
import type { Contest, Standing, RatingChange } from "@/types/contest";
import type { Paginated, QueryOptions } from "@/types/api";

/** Fetches a paginated list of contests. */
export async function getContests(
  query?: QueryOptions
): Promise<Paginated<Contest>> {
  return apiClient.post<Paginated<Contest>>(CONTEST_API.FIND, query);
}

/** Fetches a single contest by ID. */
export async function getContestById(id: number): Promise<Contest> {
  return apiClient.get<Contest>(CONTEST_API.GET(id));
}

/** Registers current user for a contest. */
export async function registerContest(id: number): Promise<void> {
  return apiClient.post(CONTEST_API.REGISTER(id));
}

/** Unregisters current user from a contest. */
export async function unregisterContest(id: number): Promise<void> {
  return apiClient.post(CONTEST_API.UNREGISTER(id));
}

/** Fetches contest standings (leaderboard). */
export async function getContestStandings(id: number): Promise<Standing[]> {
  return apiClient.get<Standing[]>(CONTEST_API.STANDINGS(id));
}

/** Creates a new contest (admin). */
export async function createContest(
  data: Partial<Contest>
): Promise<Contest> {
  return apiClient.post<Contest>(CONTEST_API.CREATE, data);
}

/** Updates a contest (admin). */
export async function updateContest(
  id: number,
  data: Partial<Contest>
): Promise<Contest> {
  return apiClient.put<Contest>(CONTEST_API.UPDATE(id), data);
}

/** Deletes a contest (admin). */
export async function deleteContest(id: number): Promise<void> {
  return apiClient.delete(CONTEST_API.DELETE(id));
}

/** Fetches rating changes for an ended contest. */
export async function getContestRatingChanges(
  id: number
): Promise<RatingChange[]> {
  return apiClient.get<RatingChange[]>(CONTEST_API.RATING_CHANGES(id));
}
