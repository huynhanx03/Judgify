import { apiClient } from "@/lib/api-client";
import { CONTEST_API } from "@/constants/api";
import type { Paginated, QueryOptions } from "@/types/api";
import type { Contest, Standing, RatingChange } from "@/types/contest";

export const contestService = {
  // Read
  async find(query?: QueryOptions): Promise<Paginated<Contest>> {
    return apiClient.post<Paginated<Contest>>(CONTEST_API.FIND, query);
  },
  async getById(id: number): Promise<Contest> {
    return apiClient.get<Contest>(CONTEST_API.GET(id));
  },

  // User actions
  async register(id: number): Promise<void> {
    return apiClient.post(CONTEST_API.REGISTER(id));
  },
  async unregister(id: number): Promise<void> {
    return apiClient.post(CONTEST_API.UNREGISTER(id));
  },
  async getStandings(id: number): Promise<Standing[]> {
    return apiClient.get<Standing[]>(CONTEST_API.STANDINGS(id));
  },
  async getRatingChanges(id: number): Promise<RatingChange[]> {
    return apiClient.get<RatingChange[]>(CONTEST_API.RATING_CHANGES(id));
  },

  // CRUD
  async create(data: {
    title: string;
    description?: string;
    start_time: string;
    end_time: string;
    max_participants?: number;
    problem_ids?: number[];
  }): Promise<Contest> {
    return apiClient.post<Contest>(CONTEST_API.CREATE, data);
  },
  async update(
    id: number,
    data: {
      title?: string;
      description?: string;
      start_time?: string;
      end_time?: string;
      max_participants?: number;
      problem_ids?: number[];
    }
  ): Promise<Contest> {
    return apiClient.put<Contest>(CONTEST_API.UPDATE(id), data);
  },
  async delete(id: number): Promise<void> {
    await apiClient.delete(CONTEST_API.DELETE(id));
  },
};
